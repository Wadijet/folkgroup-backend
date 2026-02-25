// Package crmvc - Backfill activity từ dữ liệu cũ (job bên ngoài gọi endpoint).
package crmvc

import (
	"context"

	crmdto "meta_commerce/internal/api/crm/dto"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

const defaultBackfillLimit = 10000

// BackfillActivity quét dữ liệu cũ (orders, conversations, notes) và đẩy qua Ingest*Touchpoint.
// skipIfExists=true để tránh ghi trùng khi chạy nhiều lần.
func (s *CrmCustomerService) BackfillActivity(ctx context.Context, ownerOrgID primitive.ObjectID, limit int) (*crmdto.CrmBackfillActivityResult, error) {
	if limit <= 0 {
		limit = defaultBackfillLimit
	}
	result := &crmdto.CrmBackfillActivityResult{}

	// 1. Backfill orders (pc_pos_orders) — tạm thời tất cả đơn
	ordersProcessed := s.backfillOrders(ctx, ownerOrgID, limit)
	result.OrdersProcessed = ordersProcessed

	// 2. Backfill conversations (fb_conversations)
	convProcessed := s.backfillConversations(ctx, ownerOrgID, limit)
	result.ConversationsProcessed = convProcessed

	// 3. Backfill notes (crm_notes)
	notesProcessed := s.backfillNotes(ctx, ownerOrgID, limit)
	result.NotesProcessed = notesProcessed

	return result, nil
}

func (s *CrmCustomerService) backfillOrders(ctx context.Context, ownerOrgID primitive.ObjectID, limit int) int {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !ok {
		return 0
	}
	// Đơn được tính: tất cả trừ status Đã hủy (6)
	cancelledStatuses := []int{6}
	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$and": []bson.M{
			{"status": bson.M{"$nin": cancelledStatuses}},
			{"posData.status": bson.M{"$nin": cancelledStatuses}},
		},
	}
	opts := mongoopts.Find().SetLimit(int64(limit))
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return 0
	}
	defer cursor.Close(ctx)

	count := 0
	for cursor.Next(ctx) {
		var doc struct {
			CustomerId string                 `bson:"customerId"`
			OrderId    int64                  `bson:"orderId"`
			PageId     string                 `bson:"pageId"`
			PosData    map[string]interface{} `bson:"posData"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		customerId := doc.CustomerId
		if customerId == "" {
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
			_ = s.IngestOrderTouchpoint(ctx, customerId, ownerOrgID, doc.OrderId, true, channel, true)
			count++
		}
	}
	return count
}

func (s *CrmCustomerService) backfillConversations(ctx context.Context, ownerOrgID primitive.ObjectID, limit int) int {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations)
	if !ok {
		return 0
	}
	filter := bson.M{"ownerOrganizationId": ownerOrgID}
	opts := mongoopts.Find().SetLimit(int64(limit))
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return 0
	}
	defer cursor.Close(ctx)

	count := 0
	for cursor.Next(ctx) {
		var doc struct {
			CustomerId     string `bson:"customerId"`
			ConversationId string `bson:"conversationId"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		if doc.CustomerId != "" {
			_ = s.IngestConversationTouchpoint(ctx, doc.CustomerId, ownerOrgID, doc.ConversationId, true)
			count++
		}
	}
	return count
}

func (s *CrmCustomerService) backfillNotes(ctx context.Context, ownerOrgID primitive.ObjectID, limit int) int {
	noteSvc, err := NewCrmNoteService()
	if err != nil {
		return 0
	}
	// Lấy tất cả notes chưa xóa — cần query trực tiếp vì FindByCustomerId cần customerId
	coll := noteSvc.Collection()
	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"isDeleted":           false,
	}
	opts := mongoopts.Find().SetLimit(int64(limit))
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return 0
	}
	defer cursor.Close(ctx)

	count := 0
	for cursor.Next(ctx) {
		var doc struct {
			ID         primitive.ObjectID `bson:"_id"`
			CustomerId string             `bson:"customerId"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		if doc.CustomerId != "" {
			_ = s.IngestNoteTouchpoint(ctx, doc.CustomerId, ownerOrgID, doc.ID.Hex(), true)
			count++
		}
	}
	return count
}
