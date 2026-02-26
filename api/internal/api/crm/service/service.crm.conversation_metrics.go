// Package crmvc - Aggregate conversation metrics từ fb_conversations.
package crmvc

import (
	"context"
	"strconv"
	"strings"

	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// conversationMetrics kết quả aggregate hội thoại.
type conversationMetrics struct {
	ConversationCount          int
	ConversationCountByInbox   int
	ConversationCountByComment int
	LastConversationAt        int64
	FirstConversationAt       int64
	TotalMessages             int
	LastMessageFromCustomer   bool
	ConversationFromAds       bool
	ConversationTags          []string
}

// aggregateConversationMetricsForCustomer aggregate từ fb_conversations. asOf > 0: chỉ conv có convUpdatedAt <= asOf.
func (s *CrmCustomerService) aggregateConversationMetricsForCustomer(ctx context.Context, customerIds []string, ownerOrgID primitive.ObjectID, asOf int64) conversationMetrics {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations)
	if !ok {
		return conversationMetrics{}
	}
	var ids []string
	var numIds []interface{}
	for _, id := range customerIds {
		if id != "" {
			ids = append(ids, id)
			if n, err := strconv.ParseInt(id, 10, 64); err == nil {
				numIds = append(numIds, n)
			}
		}
	}
	if len(ids) == 0 {
		return conversationMetrics{}
	}

	convCustomerOr := []bson.M{
		{"customerId": bson.M{"$in": ids}},
		{"panCakeData.customer_id": bson.M{"$in": ids}},
		{"panCakeData.customer.id": bson.M{"$in": ids}},
		{"panCakeData.customers.id": bson.M{"$in": ids}},
	}
	if len(numIds) > 0 {
		convCustomerOr = append(convCustomerOr,
			bson.M{"panCakeData.customer_id": bson.M{"$in": numIds}},
			bson.M{"panCakeData.customer.id": bson.M{"$in": numIds}},
			bson.M{"panCakeData.customers.id": bson.M{"$in": numIds}},
		)
	}
	matchFilter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$or":                 convCustomerOr,
	}
	if asOf > 0 {
		matchFilter["$expr"] = bson.M{"$lte": bson.A{
			bson.M{"$ifNull": bson.A{"$panCakeUpdatedAt", "$updatedAt"}},
			asOf,
		}}
	}

	addFieldsStage := bson.M{
		"convUpdatedAt": bson.M{"$ifNull": bson.A{"$panCakeUpdatedAt", "$updatedAt"}},
		"msgCount":      bson.M{"$ifNull": bson.A{"$panCakeData.message_count", 0}},
		"convType":      bson.M{"$ifNull": bson.A{"$panCakeData.type", "INBOX"}},
		"hasAdIds":      bson.M{"$gt": bson.A{bson.M{"$size": bson.M{"$ifNull": bson.A{"$panCakeData.ad_ids", bson.A{}}}}, 0}},
	}
	pipeStages := []bson.D{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$addFields", Value: addFieldsStage}},
	}
	if asOf > 0 {
		pipeStages = append(pipeStages, bson.D{{Key: "$match", Value: bson.M{"convUpdatedAt": bson.M{"$lte": asOf}}}})
	}
	pipeStages = append(pipeStages, bson.D{{Key: "$group", Value: bson.M{
		"_id":                     nil,
		"conversationCount":       bson.M{"$sum": 1},
		"conversationCountInbox":  bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$convType", "INBOX"}}, 1, 0}}},
		"conversationCountComment": bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$convType", "COMMENT"}}, 1, 0}}},
		"lastConversationAt":      bson.M{"$max": "$convUpdatedAt"},
		"firstConversationAt":     bson.M{"$min": "$convUpdatedAt"},
		"totalMessages":           bson.M{"$sum": "$msgCount"},
		"anyFromAds":              bson.M{"$max": bson.M{"$cond": bson.A{"$hasAdIds", 1, 0}}},
	}}})

	cursor, err := coll.Aggregate(ctx, pipeStages)
	if err != nil {
		return conversationMetrics{}
	}
	defer cursor.Close(ctx)

	var aggResult struct {
		ConversationCount       int   `bson:"conversationCount"`
		ConversationCountInbox  int   `bson:"conversationCountInbox"`
		ConversationCountComment int  `bson:"conversationCountComment"`
		LastConversationAt      int64 `bson:"lastConversationAt"`
		FirstConversationAt     int64 `bson:"firstConversationAt"`
		TotalMessages           int   `bson:"totalMessages"`
		AnyFromAds              int   `bson:"anyFromAds"`
	}
	if cursor.Next(ctx) {
		_ = cursor.Decode(&aggResult)
	}

	// lastMessageFromCustomer: lấy conversation gần nhất, check last_sent_by.email chứa @facebook.com
	lastMsgFromCustomer := false
	tagSet := make(map[string]bool)
	cur2, err := coll.Find(ctx, matchFilter, nil)
	if err == nil {
		defer cur2.Close(ctx)
		var lastDoc struct {
			PanCakeUpdatedAt int64                  `bson:"panCakeUpdatedAt"`
			PanCakeData      map[string]interface{} `bson:"panCakeData"`
		}
		var maxAt int64
		for cur2.Next(ctx) {
			var doc struct {
				PanCakeUpdatedAt int64                  `bson:"panCakeUpdatedAt"`
				PanCakeData      map[string]interface{} `bson:"panCakeData"`
			}
			if cur2.Decode(&doc) != nil {
				continue
			}
			if doc.PanCakeUpdatedAt > maxAt {
				maxAt = doc.PanCakeUpdatedAt
				lastDoc = doc
			}
			// Thu thập tags
			if pd := doc.PanCakeData; pd != nil {
				if arr, ok := pd["tags"].([]interface{}); ok {
					for _, t := range arr {
						if m := toMap(t); m != nil {
							if txt, ok := m["text"].(string); ok && txt != "" {
								tagSet[txt] = true
							}
						}
					}
				}
			}
		}
		if lastDoc.PanCakeData != nil {
			if lsb, ok := lastDoc.PanCakeData["last_sent_by"].(map[string]interface{}); ok {
				if email, ok := lsb["email"].(string); ok && strings.Contains(email, "@facebook.com") {
					lastMsgFromCustomer = true
				}
			}
		}
	}

	tags := make([]string, 0, len(tagSet))
	for t := range tagSet {
		tags = append(tags, t)
	}

	return conversationMetrics{
		ConversationCount:          aggResult.ConversationCount,
		ConversationCountByInbox:   aggResult.ConversationCountInbox,
		ConversationCountByComment: aggResult.ConversationCountComment,
		LastConversationAt:        aggResult.LastConversationAt,
		FirstConversationAt:       aggResult.FirstConversationAt,
		TotalMessages:             aggResult.TotalMessages,
		LastMessageFromCustomer:   lastMsgFromCustomer,
		ConversationFromAds:       aggResult.AnyFromAds > 0,
		ConversationTags:          tags,
	}
}
