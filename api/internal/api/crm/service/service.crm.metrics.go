// Package crmvc - Aggregate metrics từ pc_pos_orders cho crm_customers.
package crmvc

import (
	"context"
	"strconv"
	"time"

	crmmodels "meta_commerce/internal/api/crm/models"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// orderMetrics kết quả aggregate đơn hàng.
type orderMetrics struct {
	TotalSpent          float64
	OrderCount          int
	LastOrderAt         int64
	SecondLastOrderAt   int64
	RevenueLast30d      float64
	RevenueLast90d      float64
	OrderCountOnline    int
	OrderCountOffline   int
	FirstOrderChannel   string
	LastOrderChannel    string
	CancelledOrderCount int
	OrdersLast30d       int
	OrdersLast90d       int
	OrdersFromAds       int
	OrdersFromOrganic   int
	OrdersFromDirect    int
	OwnedSkuQuantities  map[string]int
}

// aggregateOrderMetricsForCustomer aggregate từ pc_pos_orders. asOf > 0: chỉ đơn có orderDate <= asOf (cho snapshot đúng timeline).
func (s *CrmCustomerService) aggregateOrderMetricsForCustomer(ctx context.Context, customerIds []string, ownerOrgID primitive.ObjectID, phoneNumbers []string, asOf int64) orderMetrics {
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
	if asOf > 0 {
		andList := matchFilter["$and"].([]bson.M)
		andList = append(andList, bson.M{
			"$expr": bson.M{"$lte": bson.A{
				bson.M{"$ifNull": bson.A{"$insertedAt", bson.M{"$ifNull": bson.A{"$posCreatedAt", "$posData.inserted_at"}}}},
				asOf,
			}},
		})
		matchFilter["$and"] = andList
	}

	refTime := time.Now().UnixMilli()
	if asOf > 0 {
		refTime = asOf
	}
	cutoff30 := refTime - 30*24*60*60*1000
	cutoff90 := refTime - 90*24*60*60*1000

	addFieldsStage := bson.M{
		"orderDate": bson.M{"$ifNull": bson.A{"$insertedAt", bson.M{"$ifNull": bson.A{"$posCreatedAt", "$posData.inserted_at"}}}},
		"amount":    bson.M{"$ifNull": bson.A{"$posData.total_price_after_sub_discount", bson.M{"$ifNull": bson.A{"$posData.total_price", 0}}}},
	}
	pipeStages := []bson.D{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$addFields", Value: addFieldsStage}},
	}
	if asOf > 0 {
		pipeStages = append(pipeStages, bson.D{{Key: "$match", Value: bson.M{"orderDate": bson.M{"$lte": asOf}}}})
	}
	pipeStages = append(pipeStages,
		bson.D{{Key: "$sort", Value: bson.M{"orderDate": 1}}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id":                 nil,
			"totalSpent":          bson.M{"$sum": "$amount"},
			"orderCount":          bson.M{"$sum": 1},
			"lastOrderAt":         bson.M{"$max": "$orderDate"},
			"firstOrderAt":        bson.M{"$min": "$orderDate"},
			"firstOrderPageId":    bson.M{"$first": "$pageId"},
			"lastOrderPageId":     bson.M{"$last": "$pageId"},
			"firstOrderPosPageId": bson.M{"$first": "$posData.page_id"},
			"lastOrderPosPageId":  bson.M{"$last": "$posData.page_id"},
			"revenueLast30d":      bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$gte": bson.A{"$orderDate", cutoff30}}, "$amount", 0}}},
			"revenueLast90d":      bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$gte": bson.A{"$orderDate", cutoff90}}, "$amount", 0}}},
			"ordersLast30d":       bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$gte": bson.A{"$orderDate", cutoff30}}, 1, 0}}},
			"ordersLast90d":       bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$gte": bson.A{"$orderDate", cutoff90}}, 1, 0}}},
			"orderDates":          bson.M{"$push": "$orderDate"},
		}}},
		bson.D{{Key: "$addFields", Value: bson.M{
			"secondLastOrderAt": bson.M{"$arrayElemAt": bson.A{bson.M{"$reverseArray": "$orderDates"}, 1}},
		}}},
	)

	cursor, err := coll.Aggregate(ctx, pipeStages)
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
		OrdersLast30d       int         `bson:"ordersLast30d"`
		OrdersLast90d       int         `bson:"ordersLast90d"`
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

	// Đếm đơn hủy (status 6) — cùng filter customer nhưng status = 6
	cancelledFilter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$and": []bson.M{
			{"$or": orConditions},
			{"$or": []bson.M{
				{"status": 6},
				{"posData.status": 6},
			}},
		},
	}
	if asOf > 0 {
		andList := cancelledFilter["$and"].([]bson.M)
		andList = append(andList, bson.M{
			"$expr": bson.M{"$lte": bson.A{
				bson.M{"$ifNull": bson.A{"$insertedAt", bson.M{"$ifNull": bson.A{"$posCreatedAt", "$posData.inserted_at"}}}},
				asOf,
			}},
		})
		cancelledFilter["$and"] = andList
	}
	cancelledCount := int64(0)
	if n, err := coll.CountDocuments(ctx, cancelledFilter); err == nil {
		cancelledCount = n
	}

	// Order source breakdown + ownedSkuQuantities — iterate orders
	ordersFromAds, ordersFromOrganic, ordersFromDirect := 0, 0, 0
	ownedSkus := make(map[string]int)
	cur3, err := coll.Find(ctx, matchFilter, nil)
	if err == nil {
		defer cur3.Close(ctx)
		for cur3.Next(ctx) {
			var ord struct {
				PosData map[string]interface{} `bson:"posData"`
			}
			if cur3.Decode(&ord) != nil {
				continue
			}
			// Order source
			src := getOrderSourceFromPosData(ord.PosData)
			switch src {
			case "meta_ads":
				ordersFromAds++
			case "organic":
				ordersFromOrganic++
			default:
				ordersFromDirect++
			}
			// Owned SKU quantities
			items := extractOrderItemsFromPosData(ord.PosData)
			for _, it := range items {
				sku, qty := getSkuAndOwnedQty(it)
				if sku != "" && qty > 0 {
					ownedSkus[sku] += qty
				}
			}
		}
	}

	return orderMetrics{
		TotalSpent:          result.TotalSpent,
		OrderCount:          result.OrderCount,
		LastOrderAt:         result.LastOrderAt,
		SecondLastOrderAt:   result.SecondLastOrderAt,
		RevenueLast30d:      result.RevenueLast30d,
		RevenueLast90d:      result.RevenueLast90d,
		OrderCountOnline:    onlineCount,
		OrderCountOffline:   offlineCount,
		FirstOrderChannel:   firstCh,
		LastOrderChannel:    lastCh,
		CancelledOrderCount: int(cancelledCount),
		OrdersLast30d:       result.OrdersLast30d,
		OrdersLast90d:       result.OrdersLast90d,
		OrdersFromAds:       ordersFromAds,
		OrdersFromOrganic:   ordersFromOrganic,
		OrdersFromDirect:    ordersFromDirect,
		OwnedSkuQuantities:  ownedSkus,
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

// getOrderSourceFromPosData xác định nguồn đơn từ posData.order_sources. Trả về "meta_ads" | "organic" | "direct".
func getOrderSourceFromPosData(posData map[string]interface{}) string {
	if posData == nil {
		return "organic"
	}
	v := posData["order_sources"]
	if v == nil {
		return "organic"
	}
	// Có thể là array ["-1"], string "-1", hoặc number -1
	switch x := v.(type) {
	case []interface{}:
		if len(x) > 0 {
			if s, ok := x[0].(string); ok && (s == "-1" || s == "meta_ads") {
				return "meta_ads"
			}
			if n, ok := toInt64(x[0]); ok && n == -1 {
				return "meta_ads"
			}
		}
		return "direct"
	case string:
		if x == "-1" || x == "meta_ads" {
			return "meta_ads"
		}
		return "direct"
	case float64:
		if int64(x) == -1 {
			return "meta_ads"
		}
		return "direct"
	case int:
		if x == -1 {
			return "meta_ads"
		}
		return "direct"
	case int64:
		if x == -1 {
			return "meta_ads"
		}
		return "direct"
	}
	return "organic"
}

func toInt64(v interface{}) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case int:
		return int64(x), true
	case float64:
		return int64(x), true
	}
	return 0, false
}

// extractOrderItemsFromPosData lấy items từ posData.items hoặc order_items.
func extractOrderItemsFromPosData(posData map[string]interface{}) []map[string]interface{} {
	if posData == nil {
		return nil
	}
	var out []map[string]interface{}
	appendItem := func(v interface{}) {
		if m := toMap(v); m != nil {
			out = append(out, m)
		}
	}
	if arr, ok := posData["items"].([]interface{}); ok {
		for _, v := range arr {
			appendItem(v)
		}
	}
	if len(out) == 0 {
		if arr, ok := posData["order_items"].([]interface{}); ok {
			for _, v := range arr {
				appendItem(v)
			}
		}
	}
	return out
}

func toMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	if d, ok := v.(primitive.D); ok {
		m := make(map[string]interface{}, len(d))
		for _, e := range d {
			m[e.Key] = e.Value
		}
		return m
	}
	return nil
}

// getSkuAndOwnedQty lấy product_display_id (SKU) và số lượng sở hữu từ item. ownedQty = quantity - return_quantity - returned_count - returning_quantity.
func getSkuAndOwnedQty(item map[string]interface{}) (sku string, qty int) {
	if item == nil {
		return "", 0
	}
	// variation_info.product_display_id
	if vi, ok := item["variation_info"].(map[string]interface{}); ok {
		if s, ok := vi["product_display_id"].(string); ok && s != "" {
			sku = s
		}
	}
	if sku == "" {
		if vi, ok := item["variation_info"].(primitive.D); ok {
			for _, e := range vi {
				if e.Key == "product_display_id" {
					if s, ok := e.Value.(string); ok && s != "" {
						sku = s
					}
					break
				}
			}
		}
	}
	if sku == "" {
		return "", 0
	}
	quantity := getIntFromItem(item, "quantity")
	returnQty := getIntFromItem(item, "return_quantity")
	returnedCount := getIntFromItem(item, "returned_count")
	returningQty := getIntFromItem(item, "returning_quantity")
	owned := quantity - returnQty - returnedCount - returningQty
	if owned <= 0 {
		return sku, 0
	}
	return sku, owned
}

func getIntFromItem(m map[string]interface{}, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	}
	return 0
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

// GetMetricsForSnapshotAt trả về metrics map cho snapshot tại activityAt — chỉ đơn/conv có timestamp <= activityAt.
// Dùng cho BuildSnapshotWithChanges để timeline đúng: metrics tăng dần theo từng order/chat.
func (s *CrmCustomerService) GetMetricsForSnapshotAt(ctx context.Context, c *crmmodels.CrmCustomer, activityAt int64) map[string]interface{} {
	if c == nil || activityAt <= 0 {
		return nil
	}
	ids := []string{c.SourceIds.Pos, c.SourceIds.Fb, c.UnifiedId}
	om := s.aggregateOrderMetricsForCustomer(ctx, ids, c.OwnerOrganizationID, GetPhoneNumbersFromCustomer(c), activityAt)
	cm := s.aggregateConversationMetricsForCustomer(ctx, ids, c.OwnerOrganizationID, activityAt)
	avgOrderValue := 0.0
	if om.OrderCount > 0 {
		avgOrderValue = om.TotalSpent / float64(om.OrderCount)
	}
	tmp := crmmodels.CrmCustomer{
		TotalSpent:                 om.TotalSpent,
		OrderCount:                 om.OrderCount,
		AvgOrderValue:              avgOrderValue,
		LastOrderAt:                om.LastOrderAt,
		SecondLastOrderAt:          om.SecondLastOrderAt,
		RevenueLast30d:             om.RevenueLast30d,
		RevenueLast90d:             om.RevenueLast90d,
		CancelledOrderCount:        om.CancelledOrderCount,
		OrdersLast30d:              om.OrdersLast30d,
		OrdersLast90d:              om.OrdersLast90d,
		OrdersFromAds:              om.OrdersFromAds,
		OrdersFromOrganic:          om.OrdersFromOrganic,
		OrdersFromDirect:           om.OrdersFromDirect,
		OrderCountOnline:           om.OrderCountOnline,
		OrderCountOffline:          om.OrderCountOffline,
		FirstOrderChannel:          om.FirstOrderChannel,
		LastOrderChannel:           om.LastOrderChannel,
		IsOmnichannel:              om.OrderCountOnline > 0 && om.OrderCountOffline > 0,
		HasConversation:            cm.ConversationCount > 0,
		HasOrder:                   om.OrderCount > 0,
		ConversationCount:          cm.ConversationCount,
		ConversationCountByInbox:    cm.ConversationCountByInbox,
		ConversationCountByComment:  cm.ConversationCountByComment,
		LastConversationAt:         cm.LastConversationAt,
		FirstConversationAt:        cm.FirstConversationAt,
		TotalMessages:              cm.TotalMessages,
		LastMessageFromCustomer:    cm.LastMessageFromCustomer,
		ConversationFromAds:         cm.ConversationFromAds,
		ConversationTags:            cm.ConversationTags,
		OwnedSkuQuantities:          om.OwnedSkuQuantities,
	}
	return buildMetricsSnapshot(&tmp)
}

// getAddressesFromFirstOrderAsOf lấy addresses từ đơn đầu tiên (theo orderDate) có địa chỉ, trong các đơn có orderDate <= asOf.
// Dùng cho profile as of activityAt — địa chỉ phải theo timeline, không dùng tổng hiện tại.
func (s *CrmCustomerService) getAddressesFromFirstOrderAsOf(ctx context.Context, customerIds []string, ownerOrgID primitive.ObjectID, phoneNumbers []string, asOf int64) []interface{} {
	if asOf <= 0 {
		return nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !ok {
		return nil
	}
	var ids []string
	for _, id := range customerIds {
		if id != "" {
			ids = append(ids, id)
		}
	}
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
	if len(ids) == 0 && len(phoneVariants) == 0 {
		return nil
	}
	cancelledStatuses := []int{6}
	var orConditions []bson.M
	if len(ids) > 0 {
		orConditions = append(orConditions,
			bson.M{"customerId": bson.M{"$in": ids}},
			bson.M{"posData.customer.id": bson.M{"$in": ids}},
			bson.M{"posData.customer_id": bson.M{"$in": ids}},
		)
	}
	if len(phoneVariants) > 0 {
		orConditions = append(orConditions, bson.M{"billPhoneNumber": bson.M{"$in": phoneVariants}}, bson.M{"posData.bill_phone_number": bson.M{"$in": phoneVariants}})
	}
	matchFilter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$and": []bson.M{
			{"$or": orConditions},
			{"status": bson.M{"$nin": cancelledStatuses}},
			{"posData.status": bson.M{"$nin": cancelledStatuses}},
			{"$expr": bson.M{"$lte": bson.A{
				bson.M{"$ifNull": bson.A{"$insertedAt", bson.M{"$ifNull": bson.A{"$posCreatedAt", "$posData.inserted_at"}}}},
				asOf,
			}}},
		},
	}
	addFieldsStage := bson.M{
		"orderDate": bson.M{"$ifNull": bson.A{"$insertedAt", bson.M{"$ifNull": bson.A{"$posCreatedAt", "$posData.inserted_at"}}}},
	}
	pipeStages := []bson.D{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$addFields", Value: addFieldsStage}},
		{{Key: "$match", Value: bson.M{"orderDate": bson.M{"$lte": asOf}}}},
		{{Key: "$sort", Value: bson.M{"orderDate": 1}}},
		{{Key: "$limit", Value: 50}},
	}
	cursor, err := coll.Aggregate(ctx, pipeStages)
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var ord pcmodels.PcPosOrder
		if cursor.Decode(&ord) != nil {
			continue
		}
		custData := extractCustomerDataFromOrder(&ord)
		if len(custData.Addresses) > 0 {
			return custData.Addresses
		}
	}
	return nil
}

// GetProfileForSnapshotAt trả về profile map cho snapshot tại activityAt — trạng thái profile ở thời điểm đó, không phải hiện tại.
// Addresses lấy từ đơn đầu tiên có địa chỉ (orderDate <= activityAt); base fields từ customer (merge đã xảy ra trước activityAt).
func (s *CrmCustomerService) GetProfileForSnapshotAt(ctx context.Context, c *crmmodels.CrmCustomer, activityAt int64) map[string]interface{} {
	if c == nil || activityAt <= 0 {
		return nil
	}
	p := map[string]interface{}{
		"name":          GetNameFromCustomer(c),
		"phoneNumbers":  GetPhoneNumbersFromCustomer(c),
		"emails":        GetEmailsFromCustomer(c),
		"birthday":      GetBirthdayFromCustomer(c),
		"gender":        GetGenderFromCustomer(c),
		"livesIn":       GetLivesInFromCustomer(c),
		"referralCode":  GetReferralCodeFromCustomer(c),
		"primarySource": c.PrimarySource,
	}
	ids := []string{c.SourceIds.Pos, c.SourceIds.Fb, c.UnifiedId}
	addrs := s.getAddressesFromFirstOrderAsOf(ctx, ids, c.OwnerOrganizationID, GetPhoneNumbersFromCustomer(c), activityAt)
	if len(addrs) > 0 {
		p["addresses"] = addrs
	} else if len(GetAddressesFromCustomer(c)) > 0 && c.MergedAt > 0 && c.MergedAt <= activityAt {
		// Merge đã xảy ra trước activityAt — dùng addresses từ merge (POS shop_customer_addresses)
		p["addresses"] = GetAddressesFromCustomer(c)
	}
	return p
}

// RefreshMetrics cập nhật metrics cho customer theo unifiedId.
func (s *CrmCustomerService) RefreshMetrics(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID) error {
	customer, err := s.FindOne(ctx, bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}, nil)
	if err != nil {
		return err
	}
	ids := []string{customer.SourceIds.Pos, customer.SourceIds.Fb, customer.UnifiedId}
	metrics := s.aggregateOrderMetricsForCustomer(ctx, ids, ownerOrgID, GetPhoneNumbersFromCustomer(&customer), 0)
	hasConv := s.checkHasConversation(ctx, ids, ownerOrgID)
	convMetrics := s.aggregateConversationMetricsForCustomer(ctx, ids, ownerOrgID, 0)

	now := time.Now().UnixMilli()
	cm := BuildCurrentMetricsFromOrderAndConv(metrics, convMetrics, hasConv)
	class := ComputeClassificationFromMetrics(metrics.TotalSpent, metrics.OrderCount, metrics.LastOrderAt, metrics.RevenueLast30d, metrics.RevenueLast90d, metrics.OrderCountOnline, metrics.OrderCountOffline, hasConv)

	setFields := bson.M{
		"totalSpent":         metrics.TotalSpent,
		"orderCount":         metrics.OrderCount,
		"lastOrderAt":        metrics.LastOrderAt,
		"ownedSkuQuantities": metrics.OwnedSkuQuantities,
		"conversationTags":   convMetrics.ConversationTags,
		"mergeMethod":       customer.MergeMethod,
		"mergedAt":          now,
		"updatedAt":         now,
		"currentMetrics":    cm,
	}
	for k, v := range class {
		setFields[k] = v
	}
	update := bson.M{
		"$set":   setFields,
		"$unset": unsetRawFields,
	}
	_, err = s.Collection().UpdateOne(ctx, bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}, update)
	if err != nil {
		return err
	}

	// Không ghi classification_changed — thay đổi phân loại đã được phản ánh trong snapshotChanges của order/conversation activity
	return nil
}

