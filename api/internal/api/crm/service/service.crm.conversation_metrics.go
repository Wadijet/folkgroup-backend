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

// buildConversationFilterForCustomerIds tạo filter để match conversation với customer.
// Match theo: customerId (root), panCakeData.customer_id, panCakeData.customers.id, panCakeData.page_customer.id, conversationId.
// conversationIds: từ pc_pos_customers.posData.fb_id — link POS customer với conv khi customerId trong conv khác (Pancake format).
func buildConversationFilterForCustomerIds(customerIds []string, ownerOrgID primitive.ObjectID, conversationIds []string) bson.M {
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
	convCustomerOr := []bson.M{
		{"customerId": bson.M{"$in": ids}},
		{"links.customer.uid": bson.M{"$in": ids}}, // Identity 4 lớp — ưu tiên links
		{"panCakeData.customer_id": bson.M{"$in": ids}},
		{"panCakeData.customer.id": bson.M{"$in": ids}},
		{"panCakeData.customers.id": bson.M{"$in": ids}},
		{"panCakeData.page_customer.id": bson.M{"$in": ids}},
		{"panCakeData.page_customer.customer_id": bson.M{"$in": ids}},
	}
	if len(numIds) > 0 {
		convCustomerOr = append(convCustomerOr,
			bson.M{"panCakeData.customer_id": bson.M{"$in": numIds}},
			bson.M{"panCakeData.customer.id": bson.M{"$in": numIds}},
			bson.M{"panCakeData.customers.id": bson.M{"$in": numIds}},
			bson.M{"panCakeData.page_customer.id": bson.M{"$in": numIds}},
			bson.M{"panCakeData.page_customer.customer_id": bson.M{"$in": numIds}},
		)
	}
	for _, cid := range conversationIds {
		if cid != "" {
			convCustomerOr = append(convCustomerOr, bson.M{"conversationId": cid})
		}
	}
	if len(ids) == 0 && len(conversationIds) == 0 {
		return bson.M{"ownerOrganizationId": ownerOrgID, "customerId": "__NO_MATCH__"}
	}
	return bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$or":                 convCustomerOr,
	}
}

// aggregateConversationMetricsForCustomer aggregate từ fb_conversations. asOf > 0: chỉ conv đã tồn tại tại thời điểm asOf.
// conversationIds: từ pc_pos_customers.posData.fb_id — link POS customer với conv khi customerId trong conv khác (Pancake format).
func (s *CrmCustomerService) aggregateConversationMetricsForCustomer(ctx context.Context, customerIds []string, conversationIds []string, ownerOrgID primitive.ObjectID, asOf int64) conversationMetrics {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations)
	if !ok {
		return conversationMetrics{}
	}
	var ids []string
	for _, id := range customerIds {
		if id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 && len(conversationIds) == 0 {
		return conversationMetrics{}
	}

	matchFilter := buildConversationFilterForCustomerIds(customerIds, ownerOrgID, conversationIds)

	// Nguyên tắc: panCakeData/posData = ngày tháng nguồn (khi sự kiện xảy ra); root (createdAt, updatedAt, panCakeUpdatedAt) = thời gian đồng bộ.
	// convUpdatedAt: ưu tiên panCakeData.updated_at (nguồn), fallback panCakeUpdatedAt/updatedAt (sync). Dùng cho lastConversationAt, firstConversationAt.
	// Dùng $dateFromString với onError/onNull + $convert để tránh pipeline fail khi format không hợp lệ.
	// Document bị exclude khi $toDate/$split lỗi → conversationCount=0 sai.
	parseStringToLong := func(fieldPath string) bson.M {
		return bson.M{
			"$convert": bson.M{
				"input": bson.M{
					"$dateFromString": bson.M{
						"dateString":     bson.M{"$arrayElemAt": bson.A{bson.M{"$split": bson.A{fieldPath, "."}}, 0}},
						"onError":        nil,
						"onNull":         nil,
					},
				},
				"to":      "long",
				"onError": nil,
				"onNull":  nil,
			},
		}
	}
	convUpdatedAtParsed := bson.M{
		"$switch": bson.M{
			"branches": bson.A{
				bson.M{"case": bson.M{"$eq": bson.A{bson.M{"$type": "$panCakeData.updated_at"}, "string"}},
					"then": parseStringToLong("$panCakeData.updated_at")},
				bson.M{"case": bson.M{"$in": bson.A{bson.M{"$type": "$panCakeData.updated_at"}, bson.A{"long", "int"}}},
					"then": "$panCakeData.updated_at"},
				bson.M{"case": bson.M{"$eq": bson.A{bson.M{"$type": "$panCakeData.updated_at"}, "double"}},
					"then": bson.M{"$toLong": "$panCakeData.updated_at"}},
				bson.M{"case": bson.M{"$eq": bson.A{bson.M{"$type": "$panCakeData.updated_at"}, "date"}},
					"then": bson.M{"$toLong": "$panCakeData.updated_at"}},
				bson.M{"case": bson.M{"$eq": bson.A{bson.M{"$type": "$panCakeData.updated_at"}, "timestamp"}},
					"then": bson.M{"$toLong": "$panCakeData.updated_at"}},
			},
			"default": nil,
		},
	}
	// $ifNull chỉ nhận 2 tham số — dùng lồng nhau cho fallback chain
	convUpdatedAt := bson.M{"$ifNull": bson.A{convUpdatedAtParsed, bson.M{"$ifNull": bson.A{"$panCakeUpdatedAt", "$updatedAt"}}}}
	// convInsertedAt: thời điểm hội thoại bắt đầu — panCakeData.inserted_at (nguồn). Format: "2026-03-03T04:03:22.263935".
	convInsertedAtMs := bson.M{
		"$switch": bson.M{
			"branches": bson.A{
				bson.M{"case": bson.M{"$eq": bson.A{bson.M{"$type": "$panCakeData.inserted_at"}, "string"}},
					"then": parseStringToLong("$panCakeData.inserted_at")},
				bson.M{"case": bson.M{"$in": bson.A{bson.M{"$type": "$panCakeData.inserted_at"}, bson.A{"long", "int"}}},
					"then": "$panCakeData.inserted_at"},
				bson.M{"case": bson.M{"$eq": bson.A{bson.M{"$type": "$panCakeData.inserted_at"}, "double"}},
					"then": bson.M{"$toLong": "$panCakeData.inserted_at"}},
				bson.M{"case": bson.M{"$eq": bson.A{bson.M{"$type": "$panCakeData.inserted_at"}, "date"}},
					"then": bson.M{"$toLong": "$panCakeData.inserted_at"}},
				bson.M{"case": bson.M{"$eq": bson.A{bson.M{"$type": "$panCakeData.inserted_at"}, "timestamp"}},
					"then": bson.M{"$toLong": "$panCakeData.inserted_at"}},
			},
			"default": nil,
		},
	}
	// convExistedAt: chỉ dùng panCakeData.inserted_at (theo sample-data: "2026-03-03T04:03:22.263935").
	// Không fallback — khi thiếu inserted_at thì convExistedAt = null; $or để bao gồm conv không xác định được (tránh loại nhầm).
	addFieldsStage := bson.M{
		"convUpdatedAt":    convUpdatedAt,
		"convInsertedAtMs":  convInsertedAtMs,
		"convExistedAt":    "$convInsertedAtMs", // null khi không parse được inserted_at
		"msgCount":         bson.M{"$ifNull": bson.A{"$panCakeData.message_count", 0}},
		"convType":         bson.M{"$ifNull": bson.A{"$panCakeData.type", "INBOX"}},
		"hasAdIds":         bson.M{"$gt": bson.A{bson.M{"$size": bson.M{"$ifNull": bson.A{"$panCakeData.ad_ids", bson.A{}}}}, 0}},
	}
	pipeStages := []bson.D{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$addFields", Value: addFieldsStage}},
	}
	if asOf > 0 {
		// Filter: conv đã tồn tại tại asOf (convExistedAt <= asOf) HOẶC không xác định được (convExistedAt null).
		// Không dùng fallback sai — theo sample-data chỉ inserted_at là đúng cho timeline.
		pipeStages = append(pipeStages, bson.D{{Key: "$match", Value: bson.M{
			"$or": []bson.M{
				{"convExistedAt": bson.M{"$lte": asOf}},
				{"convExistedAt": nil},
			},
		}}})
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
		ConversationCount        int   `bson:"conversationCount"`
		ConversationCountInbox   int   `bson:"conversationCountInbox"`
		ConversationCountComment int   `bson:"conversationCountComment"`
		LastConversationAt       int64 `bson:"lastConversationAt"`
		FirstConversationAt      int64 `bson:"firstConversationAt"`
		TotalMessages            int   `bson:"totalMessages"`
		AnyFromAds               int   `bson:"anyFromAds"`
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
