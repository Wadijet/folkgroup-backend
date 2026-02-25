// Package crmvc - Aggregate metrics từ pc_pos_orders cho crm_customers.
package crmvc

import (
	"context"
	"strconv"
	"time"

	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// orderMetrics kết quả aggregate đơn hàng.
type orderMetrics struct {
	TotalSpent         float64
	OrderCount         int
	LastOrderAt        int64
	SecondLastOrderAt  int64
	RevenueLast30d     float64
	RevenueLast90d     float64
	OrderCountOnline   int
	OrderCountOffline  int
	FirstOrderChannel  string
	LastOrderChannel   string
}

// aggregateOrderMetricsForCustomer aggregate từ pc_pos_orders theo danh sách customerIds (pos, fb, unified) và phoneNumbers (cho guest orders).
func (s *CrmCustomerService) aggregateOrderMetricsForCustomer(ctx context.Context, customerIds []string, ownerOrgID primitive.ObjectID, phoneNumbers []string) orderMetrics {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !ok {
		return orderMetrics{}
	}

	var ids []string
	for _, id := range customerIds {
		if id != "" {
			ids = append(ids, id)
		}
	}
	// Thêm biến thể SĐT (84xxx, 0xxx) cho match billPhoneNumber
	var phoneVariants []string
	for _, p := range phoneNumbers {
		if p != "" {
			phoneVariants = append(phoneVariants, p)
			if len(p) >= 3 && p[:2] == "84" {
				phoneVariants = append(phoneVariants, "0"+p[2:])
			} else if len(p) >= 10 && p[0] == '0' {
				phoneVariants = append(phoneVariants, "84"+p[1:])
			}
		}
	}

	// Phải có ít nhất customerIds hoặc phoneNumbers
	if len(ids) == 0 && len(phoneVariants) == 0 {
		return orderMetrics{}
	}

	// Đơn được tính: tất cả trừ status Đã hủy (6)
	cancelledStatuses := []int{6}
	var orConditions []bson.M
	if len(ids) > 0 {
		orConditions = append(orConditions,
			bson.M{"customerId": bson.M{"$in": ids}},
			bson.M{"posData.customer.id": bson.M{"$in": ids}},
			bson.M{"posData.customer_id": bson.M{"$in": ids}}, // fallback khi API trả customer_id dạng flat
		)
	}
	if len(phoneVariants) > 0 {
		orConditions = append(orConditions, bson.M{"billPhoneNumber": bson.M{"$in": phoneVariants}}, bson.M{"posData.bill_phone_number": bson.M{"$in": phoneVariants}})
	}
	matchFilter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$and": []bson.M{
			{"$or": orConditions},
			// Loại trừ đơn Đã hủy (status 6)
			{"status": bson.M{"$nin": cancelledStatuses}},
			{"posData.status": bson.M{"$nin": cancelledStatuses}},
		},
	}

	now := time.Now().UnixMilli()
	cutoff30 := now - 30*24*60*60*1000
	cutoff90 := now - 90*24*60*60*1000

	pipe := mongo.Pipeline{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$addFields", Value: bson.M{
			"orderDate": bson.M{"$ifNull": bson.A{"$insertedAt", bson.M{"$ifNull": bson.A{"$posCreatedAt", "$posData.inserted_at"}}}},
			"amount":    bson.M{"$ifNull": bson.A{"$posData.total_price_after_sub_discount", bson.M{"$ifNull": bson.A{"$posData.total_price", 0}}}},
		}}},
		{{Key: "$sort", Value: bson.M{"orderDate": 1}}},
		{{Key: "$group", Value: bson.M{
			"_id":                nil,
			"totalSpent":         bson.M{"$sum": "$amount"},
			"orderCount":         bson.M{"$sum": 1},
			"lastOrderAt":        bson.M{"$max": "$orderDate"},
			"firstOrderAt":       bson.M{"$min": "$orderDate"},
			"firstOrderPageId":   bson.M{"$first": "$pageId"},
			"lastOrderPageId":    bson.M{"$last": "$pageId"},
			"firstOrderPosPageId": bson.M{"$first": "$posData.page_id"},
			"lastOrderPosPageId":  bson.M{"$last": "$posData.page_id"},
			"revenueLast30d":     bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$gte": bson.A{"$orderDate", cutoff30}}, "$amount", 0}}},
			"revenueLast90d":     bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$gte": bson.A{"$orderDate", cutoff90}}, "$amount", 0}}},
			"orderDates":         bson.M{"$push": "$orderDate"},
		}}},
		{{Key: "$addFields", Value: bson.M{
			"secondLastOrderAt": bson.M{"$arrayElemAt": bson.A{bson.M{"$reverseArray": "$orderDates"}, 1}},
		}}},
	}

	cursor, err := coll.Aggregate(ctx, pipe)
	if err != nil {
		return orderMetrics{}
	}
	defer cursor.Close(ctx)

	var result struct {
		TotalSpent          float64     `bson:"totalSpent"`
		OrderCount          int         `bson:"orderCount"`
		LastOrderAt         int64       `bson:"lastOrderAt"`
		SecondLastOrderAt   int64       `bson:"secondLastOrderAt"`
		RevenueLast30d      float64     `bson:"revenueLast30d"`
		RevenueLast90d      float64     `bson:"revenueLast90d"`
		FirstOrderPageId    interface{} `bson:"firstOrderPageId"`
		LastOrderPageId     interface{} `bson:"lastOrderPageId"`
		FirstOrderPosPageId interface{} `bson:"firstOrderPosPageId"`
		LastOrderPosPageId  interface{} `bson:"lastOrderPosPageId"`
	}
	if cursor.Next(ctx) {
		_ = cursor.Decode(&result)
	}

	// Đếm online/offline: iterate từng document (đơn giản hơn aggregation phức tạp)
	onlineCount := 0
	offlineCount := 0
	cur2, err := coll.Find(ctx, matchFilter, nil)
	if err == nil {
		defer cur2.Close(ctx)
		for cur2.Next(ctx) {
			var ord struct {
				PageId   string                 `bson:"pageId"`
				PosData  map[string]interface{} `bson:"posData"`
			}
			if cur2.Decode(&ord) == nil {
				if isOrderOnline(ord.PageId, ord.PosData) {
					onlineCount++
				} else {
					offlineCount++
				}
			}
		}
	}

	firstOnline := isOnlineChannel(result.FirstOrderPageId, result.FirstOrderPosPageId)
	lastOnline := isOnlineChannel(result.LastOrderPageId, result.LastOrderPosPageId)
	firstCh := "offline"
	if firstOnline {
		firstCh = "online"
	}
	lastCh := "offline"
	if lastOnline {
		lastCh = "online"
	}

	return orderMetrics{
		TotalSpent:        result.TotalSpent,
		OrderCount:        result.OrderCount,
		LastOrderAt:       result.LastOrderAt,
		SecondLastOrderAt: result.SecondLastOrderAt,
		RevenueLast30d:    result.RevenueLast30d,
		RevenueLast90d:    result.RevenueLast90d,
		OrderCountOnline:  onlineCount,
		OrderCountOffline: offlineCount,
		FirstOrderChannel: firstCh,
		LastOrderChannel:  lastCh,
	}
}

func isOrderOnline(pageId string, posData map[string]interface{}) bool {
	if pageId != "" {
		return true
	}
	if posData != nil {
		if pid, ok := posData["page_id"].(string); ok && pid != "" {
			return true
		}
	}
	return false
}

func isOnlineChannel(pageId interface{}, posPageId interface{}) bool {
	if s, ok := pageId.(string); ok && s != "" {
		return true
	}
	if s, ok := posPageId.(string); ok && s != "" {
		return true
	}
	return false
}

// checkHasConversation kiểm tra có conversation hoặc message với customerIds.
// Match theo: customerId (root), panCakeData (customer_id, customer.id, customers[].id), fb_messages.customerId.
// Hỗ trợ cả id dạng string và number (Pancake API có thể trả customer_id là số).
func (s *CrmCustomerService) checkHasConversation(ctx context.Context, customerIds []string, ownerOrgID primitive.ObjectID) bool {
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
		return false
	}
	baseFilter := bson.M{"ownerOrganizationId": ownerOrgID}

	// Điều kiện match customer trong fb_conversations — nhiều cấu trúc Pancake có thể dùng
	convCustomerOr := []bson.M{
		{"customerId": bson.M{"$in": ids}},
		bson.M{"panCakeData.customer_id": bson.M{"$in": ids}},
		bson.M{"panCakeData.customer.id": bson.M{"$in": ids}},
		bson.M{"panCakeData.customers.id": bson.M{"$in": ids}},
	}
	if len(numIds) > 0 {
		convCustomerOr = append(convCustomerOr,
			bson.M{"panCakeData.customer_id": bson.M{"$in": numIds}},
			bson.M{"panCakeData.customer.id": bson.M{"$in": numIds}},
			bson.M{"panCakeData.customers.id": bson.M{"$in": numIds}},
		)
	}

	// 1. fb_conversations
	if coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations); ok {
		convFilter := bson.M{
			"$and": []bson.M{
				baseFilter,
				{"$or": convCustomerOr},
			},
		}
		n, err := coll.CountDocuments(ctx, convFilter)
		if err == nil && n > 0 {
			return true
		}
	}

	// 2. fb_messages: customerId (khi sync truyền customerId vào)
	if coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbMessages); ok {
		msgOr := []bson.M{{"customerId": bson.M{"$in": ids}}}
		if len(numIds) > 0 {
			msgOr = append(msgOr, bson.M{"customerId": bson.M{"$in": numIds}})
		}
		msgFilter := bson.M{
			"ownerOrganizationId": ownerOrgID,
			"$or":                msgOr,
		}
		n, err := coll.CountDocuments(ctx, msgFilter)
		if err == nil && n > 0 {
			return true
		}
	}
	return false
}

// RefreshMetrics cập nhật metrics cho customer theo unifiedId.
func (s *CrmCustomerService) RefreshMetrics(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID) error {
	customer, err := s.FindOne(ctx, bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}, nil)
	if err != nil {
		return err
	}
	ids := []string{customer.SourceIds.Pos, customer.SourceIds.Fb, customer.UnifiedId}
	metrics := s.aggregateOrderMetricsForCustomer(ctx, ids, ownerOrgID, customer.PhoneNumbers)
	hasConv := s.checkHasConversation(ctx, ids, ownerOrgID)

	now := time.Now().UnixMilli()
	update := bson.M{
		"$set": bson.M{
			"hasConversation":    hasConv,
			"hasOrder":           metrics.OrderCount > 0,
			"orderCountOnline":   metrics.OrderCountOnline,
			"orderCountOffline":  metrics.OrderCountOffline,
			"firstOrderChannel":  metrics.FirstOrderChannel,
			"lastOrderChannel":   metrics.LastOrderChannel,
			"isOmnichannel":     metrics.OrderCountOnline > 0 && metrics.OrderCountOffline > 0,
			"totalSpent":        metrics.TotalSpent,
			"orderCount":        metrics.OrderCount,
			"lastOrderAt":       metrics.LastOrderAt,
			"secondLastOrderAt": metrics.SecondLastOrderAt,
			"revenueLast30d":    metrics.RevenueLast30d,
			"revenueLast90d":    metrics.RevenueLast90d,
			"mergeMethod":       customer.MergeMethod,
			"mergedAt":          now,
			"updatedAt":         now,
		},
	}
	_, err = s.Collection().UpdateOne(ctx, bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}, update)
	return err
}
