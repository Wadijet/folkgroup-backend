// Package crmvc - Backfill activity từ dữ liệu cũ (job bên ngoài gọi endpoint).
package crmvc

import (
	"context"
	"fmt"

	crmdto "meta_commerce/internal/api/crm/dto"
	crmmodels "meta_commerce/internal/api/crm/models"
	fbmodels "meta_commerce/internal/api/fb/models"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

// BackfillTypeOrder, BackfillTypeConversation, BackfillTypeNote là giá trị cho tham số types.
const (
	BackfillTypeOrder        = "order"
	BackfillTypeConversation = "conversation"
	BackfillTypeNote         = "note"
)

// BackfillActivity quét dữ liệu cũ (orders, conversations, notes) và đẩy qua Ingest*Touchpoint.
// types: []string{"order","conversation","note"} — rỗng/nil = chạy tất cả.
// BẮT BUỘC: xử lý theo thứ tự thời gian cũ trước, mới sau (orderDate/convUpdatedAt/createdAt asc)
// để tránh lệch số liệu snapshot. Khi phát sinh activity cũ hơn, recomputeSnapshotsForNewerActivities
// sẽ tính lại snapshot của các activity mới hơn.
// skipIfExists=true để tránh ghi trùng khi chạy nhiều lần.
// limit <= 0: xử lý toàn bộ; limit > 0: giới hạn số bản ghi mỗi loại.
func (s *CrmCustomerService) BackfillActivity(ctx context.Context, ownerOrgID primitive.ObjectID, limit int, types []string) (*crmdto.CrmBackfillActivityResult, error) {
	useLimit := limit > 0
	if !useLimit {
		limit = 0 // Không giới hạn
	}
	runOrder, runConv, runNote := parseBackfillTypes(types)
	result := &crmdto.CrmBackfillActivityResult{}

	if runOrder {
		ordersProcessed := s.backfillOrders(ctx, ownerOrgID, limit, useLimit)
		result.OrdersProcessed = ordersProcessed
	}

	if runConv {
		convProcessed, convLogged, convSkipped := s.backfillConversations(ctx, ownerOrgID, limit, useLimit)
		result.ConversationsProcessed = convProcessed
		result.ConversationsLogged = convLogged
		result.ConversationsSkippedNoResolve = convSkipped
	}

	if runNote {
		notesProcessed := s.backfillNotes(ctx, ownerOrgID, limit, useLimit)
		result.NotesProcessed = notesProcessed
	}

	// Chẩn đoán khi tất cả = 0
	if result.OrdersProcessed == 0 && result.ConversationsProcessed == 0 && result.NotesProcessed == 0 {
		result.Diagnostic = s.backfillDiagnostic(ctx, ownerOrgID, runOrder, runConv)
	}

	return result, nil
}

// extractConversationCustomerId lấy customerId từ FbConversation — ưu tiên panCakeData.customers[0].id (match fb_customers) trước customer_id.
func extractConversationCustomerId(doc *fbmodels.FbConversation) string {
	if doc == nil {
		return ""
	}
	if doc.CustomerId != "" {
		return doc.CustomerId
	}
	if doc.PanCakeData == nil {
		return ""
	}
	pd := doc.PanCakeData
	// 1. Ưu tiên customers[0].id — match fb_customers.customerId (crm sourceIds.fb)
	if arr, ok := pd["customers"].([]interface{}); ok && len(arr) > 0 {
		if m, ok := arr[0].(map[string]interface{}); ok {
			if id := extractIdFromMap(m); id != "" {
				return id
			}
		}
	}
	// 2. customer.id
	if cust, ok := pd["customer"].(map[string]interface{}); ok {
		if id := extractIdFromMap(cust); id != "" {
			return id
		}
	}
	// 3. customer_id (Pancake format — có thể khác fb_customers)
	if s, ok := pd["customer_id"].(string); ok && s != "" {
		return s
	}
	return ""
}

// extractIdFromMap lấy id từ map (string, float64, int).
func extractIdFromMap(m map[string]interface{}) string {
	if m == nil {
		return ""
	}
	if s, ok := m["id"].(string); ok && s != "" {
		return s
	}
	if n, ok := m["id"].(float64); ok {
		return fmt.Sprintf("%.0f", n)
	}
	if n, ok := m["id"].(int); ok {
		return fmt.Sprintf("%d", n)
	}
	if n, ok := m["id"].(int64); ok {
		return fmt.Sprintf("%d", n)
	}
	return ""
}
func parseBackfillTypes(types []string) (runOrder, runConv, runNote bool) {
	if len(types) == 0 {
		return true, true, true
	}
	for _, v := range types {
		switch v {
		case BackfillTypeOrder:
			runOrder = true
		case BackfillTypeConversation:
			runConv = true
		case BackfillTypeNote:
			runNote = true
		}
	}
	return runOrder, runConv, runNote
}

const backfillBatchSize = 1000 // Phân trang CRUD, tránh vượt memory limit

// backfillOrders backfill đơn hàng theo thứ tự thời gian: cũ trước, mới sau.
// Dùng Find + Sort + Skip/Limit (phân trang CRUD) thay vì aggregation.
func (s *CrmCustomerService) backfillOrders(ctx context.Context, ownerOrgID primitive.ObjectID, limit int, useLimit bool) int {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !ok {
		logger.GetAppLogger().Warn("[CRM] Backfill orders: không tìm thấy collection pc_pos_orders")
		return 0
	}
	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$and": []bson.M{
			{"status": bson.M{"$nin": []int{6}}},
			{"posData.status": bson.M{"$nin": []int{6}}},
		},
	}
	// Sort theo posData (thời gian gốc), cũ trước mới sau
	sortOpt := mongoopts.Find().SetSort(bson.D{{Key: "posData.inserted_at", Value: 1}, {Key: "posData.updated_at", Value: 1}, {Key: "_id", Value: 1}})

	count := 0
	for skip := int64(0); ; skip += backfillBatchSize {
		batchLimit := backfillBatchSize
		if useLimit && limit > 0 && count+batchLimit > limit {
			batchLimit = limit - count
		}
		if batchLimit <= 0 {
			break
		}
		opts := sortOpt.SetSkip(skip).SetLimit(int64(batchLimit))
		cursor, err := coll.Find(ctx, filter, opts)
		if err != nil {
			logger.GetAppLogger().WithError(err).Warn("[CRM] Backfill orders: Find lỗi")
			break
		}
		docsInBatch := 0
		for cursor.Next(ctx) {
			docsInBatch++
			var doc pcmodels.PcPosOrder
			if err := cursor.Decode(&doc); err != nil {
				continue
			}
			customerId := doc.CustomerId
			if customerId == "" && doc.PosData != nil {
				if m, ok := doc.PosData["customer"].(map[string]interface{}); ok {
					if id, ok := m["id"].(string); ok {
						customerId = id
					}
				}
			}
			if customerId != "" {
				channel := "offline"
				if doc.PageId != "" {
					channel = "online"
				} else if doc.PosData != nil {
					if pid, ok := doc.PosData["page_id"].(string); ok && pid != "" {
						channel = "online"
					}
				}
				_ = s.IngestOrderTouchpoint(ctx, customerId, ownerOrgID, doc.OrderId, true, channel, true, &doc)
				count++
			}
		}
		cursor.Close(ctx)
		if docsInBatch < batchLimit {
			break
		}
	}
	return count
}

// backfillConversations backfill hội thoại theo thứ tự thời gian: cũ trước, mới sau.
// Dùng Find + Sort + Skip/Limit (phân trang CRUD) thay vì aggregation.
func (s *CrmCustomerService) backfillConversations(ctx context.Context, ownerOrgID primitive.ObjectID, limit int, useLimit bool) (processed, logged, skipped int) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations)
	if !ok {
		logger.GetAppLogger().Warn("[CRM] Backfill conversations: không tìm thấy collection fb_conversations")
		return 0, 0, 0
	}
	filter := bson.M{"ownerOrganizationId": ownerOrgID}
	sortOpt := mongoopts.Find().SetSort(bson.D{
		{Key: "panCakeData.inserted_at", Value: 1}, {Key: "panCakeData.updated_at", Value: 1},
		{Key: "panCakeUpdatedAt", Value: 1}, {Key: "_id", Value: 1},
	})

	decodeErrCount := 0
	for skip := int64(0); ; skip += backfillBatchSize {
		batchLimit := backfillBatchSize
		if useLimit && limit > 0 && processed+batchLimit > limit {
			batchLimit = limit - processed
		}
		if batchLimit <= 0 {
			break
		}
		opts := sortOpt.SetSkip(skip).SetLimit(int64(batchLimit))
		cursor, err := coll.Find(ctx, filter, opts)
		if err != nil {
			logger.GetAppLogger().WithError(err).Warn("[CRM] Backfill conversations: Find lỗi")
			break
		}
		docsInBatch := 0
		for cursor.Next(ctx) {
			docsInBatch++
			var doc fbmodels.FbConversation
			if err := cursor.Decode(&doc); err != nil {
				decodeErrCount++
				if decodeErrCount <= 3 {
					logger.GetAppLogger().WithError(err).Warn("[CRM] Backfill conversations: Decode lỗi")
				}
				continue
			}
			// Ưu tiên customers[0].id (match fb_customers.customerId) trước customer_id (Pancake format có thể khác)
			customerId := extractConversationCustomerId(&doc)
			if customerId != "" {
				processed++
				loggedOk, _ := s.IngestConversationTouchpoint(ctx, customerId, ownerOrgID, doc.ConversationId, true, &doc)
				if loggedOk {
					logged++
				} else {
					skipped++
				}
			}
		}
		cursor.Close(ctx)
		if docsInBatch < batchLimit {
			break
		}
	}
	return processed, logged, skipped
}

// backfillNotes backfill ghi chú theo thứ tự thời gian: cũ trước, mới sau (createdAt asc).
func (s *CrmCustomerService) backfillNotes(ctx context.Context, ownerOrgID primitive.ObjectID, limit int, useLimit bool) int {
	noteSvc, err := NewCrmNoteService()
	if err != nil {
		return 0
	}
	coll := noteSvc.Collection()
	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"isDeleted":           false,
	}
	opts := mongoopts.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}) // Cũ trước, mới sau
	if useLimit && limit > 0 {
		opts.SetLimit(int64(limit))
	}
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return 0
	}
	defer cursor.Close(ctx)

	count := 0
	for cursor.Next(ctx) {
		var doc crmmodels.CrmNote
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		if doc.CustomerId != "" {
			// Truyền doc để lấy activityAt từ nguồn (CreatedAt)
			_ = s.IngestNoteTouchpoint(ctx, doc.CustomerId, ownerOrgID, doc.ID.Hex(), true, &doc)
			count++
		}
	}
	return count
}

// backfillDiagnostic trả về thông tin chẩn đoán khi backfill trả về toàn 0.
func (s *CrmCustomerService) backfillDiagnostic(ctx context.Context, ownerOrgID primitive.ObjectID, runOrder, runConv bool) *struct {
	TotalConversations        int64    `json:"totalConversations"`
	ConversationsWithOrg      int64    `json:"conversationsWithOrg"`
	TotalOrders               int64    `json:"totalOrders"`
	OrdersWithOrg             int64    `json:"ordersWithOrg"`
	SampleOrgIdsConversations []string `json:"sampleOrgIdsConversations"`
} {
	d := &struct {
		TotalConversations        int64    `json:"totalConversations"`
		ConversationsWithOrg      int64    `json:"conversationsWithOrg"`
		TotalOrders               int64    `json:"totalOrders"`
		OrdersWithOrg             int64    `json:"ordersWithOrg"`
		SampleOrgIdsConversations []string `json:"sampleOrgIdsConversations"`
	}{}

	if runConv {
		if coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations); ok {
			d.TotalConversations, _ = coll.CountDocuments(ctx, bson.M{})
			d.ConversationsWithOrg, _ = coll.CountDocuments(ctx, bson.M{"ownerOrganizationId": ownerOrgID})
			cursor, err := coll.Aggregate(ctx, []bson.D{
				{{Key: "$match", Value: bson.M{"ownerOrganizationId": bson.M{"$exists": true, "$ne": nil}}}},
				{{Key: "$group", Value: bson.M{"_id": "$ownerOrganizationId"}}},
				{{Key: "$limit", Value: 5}},
			})
			if err == nil {
				for cursor.Next(ctx) {
					var grp struct {
						ID primitive.ObjectID `bson:"_id"`
					}
					if cursor.Decode(&grp) == nil && !grp.ID.IsZero() {
						d.SampleOrgIdsConversations = append(d.SampleOrgIdsConversations, grp.ID.Hex())
					}
				}
				cursor.Close(ctx)
			}
		}
	}

	if runOrder {
		if coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders); ok {
			d.TotalOrders, _ = coll.CountDocuments(ctx, bson.M{})
			d.OrdersWithOrg, _ = coll.CountDocuments(ctx, bson.M{"ownerOrganizationId": ownerOrgID})
		}
	}

	return d
}
