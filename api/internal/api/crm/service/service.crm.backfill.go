// Package crmvc - Backfill activity từ orders, conversations, notes.
// Hợp nhất với sync: thông tin đến trước tạo trước (UpsertMinimal từ order/conv), thông tin sau cập nhật thêm (Sync profile).
package crmvc

import (
	"context"
	"fmt"

	crmdto "meta_commerce/internal/api/crm/dto"
	crmmodels "meta_commerce/internal/api/crm/models"
	fbmodels "meta_commerce/internal/api/fb/models"
	ordermodels "meta_commerce/internal/api/order/models"
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

// progressInt64 lấy int64 từ interface{} (BSON có thể trả về int32, int64, float64).
func progressInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	}
	return 0
}

// BackfillActivity quét orders, conversations, notes và đẩy qua Ingest*Touchpoint.
// Tạo crm_customer từ order/conv nếu chưa có (UpsertMinimalFromPosId/UpsertMinimalFromFbId), log activity.
// types: []string{"order","conversation","note"} — rỗng/nil = chạy tất cả.
// Xử lý theo thứ tự thời gian cũ trước, mới sau. limit <= 0: toàn bộ; limit > 0: giới hạn mỗi loại.
// progress: tiến độ để resume (nil = bắt đầu mới). onProgress: callback sau mỗi batch (nil = không checkpoint).
func (s *CrmCustomerService) BackfillActivity(ctx context.Context, ownerOrgID primitive.ObjectID, limit int, types []string, progress bson.M, onProgress func(bson.M)) (*crmdto.CrmBackfillActivityResult, error) {
	useLimit := limit > 0
	if !useLimit {
		limit = 0
	}
	runOrder, runConv, runNote := parseBackfillTypes(types)
	result := &crmdto.CrmBackfillActivityResult{}

	ordersSkip, convSkip, notesSkip := int64(0), int64(0), int64(0)
	if progress != nil {
		// Hỗ trợ progress lồng nhau (từ RebuildCrm: progress.backfill)
		p := progress
		if bf, ok := progress["backfill"].(map[string]interface{}); ok {
			p = bf
		}
		ordersSkip = progressInt64(p["ordersSkip"])
		convSkip = progressInt64(p["conversationsSkip"])
		notesSkip = progressInt64(p["notesSkip"])
	}

	ordersTotal, convTotal, notesTotal := int64(0), int64(0), int64(0)
	if onProgress != nil && (runOrder || runConv || runNote) {
		ordersTotal, convTotal, notesTotal = s.countBackfillSourceTotals(ctx, ownerOrgID, runOrder, runConv, runNote)
	}

	emitProgress := func(currentSrc string, nextSrcs []string) {
		if onProgress != nil {
			p := s.buildBackfillProgress(ordersSkip, convSkip, notesSkip, ordersTotal, convTotal, notesTotal, runOrder, runConv, runNote, currentSrc, nextSrcs)
			onProgress(p)
		}
	}

	if runOrder {
		var onBatch func(int64)
		if onProgress != nil {
			nextSrcs := []string{}
			if runConv {
				nextSrcs = append(nextSrcs, "conversation")
			}
			if runNote {
				nextSrcs = append(nextSrcs, "note")
			}
			onBatch = func(skip int64) {
				ordersSkip = skip
				emitProgress("order", nextSrcs)
			}
		}
		processed, newSkip := s.backfillOrdersWithProgress(ctx, ownerOrgID, limit, useLimit, ordersSkip, onBatch)
		result.OrdersProcessed = processed
		ordersSkip = newSkip
		if onProgress != nil {
			nextSrcs := []string{}
			if runConv {
				nextSrcs = append(nextSrcs, "conversation")
			}
			if runNote {
				nextSrcs = append(nextSrcs, "note")
			}
			emitProgress("order", nextSrcs)
		}
	}

	if runConv {
		var onBatch func(int64)
		if onProgress != nil {
			nextSrcs := []string{}
			if runNote {
				nextSrcs = append(nextSrcs, "note")
			}
			onBatch = func(skip int64) {
				convSkip = skip
				emitProgress("conversation", nextSrcs)
			}
		}
		processed, logged, skipped, newSkip := s.backfillConversationsWithProgress(ctx, ownerOrgID, limit, useLimit, convSkip, onBatch)
		result.ConversationsProcessed = processed
		result.ConversationsLogged = logged
		result.ConversationsSkippedNoResolve = skipped
		convSkip = newSkip
		if onProgress != nil {
			nextSrcs := []string{}
			if runNote {
				nextSrcs = append(nextSrcs, "note")
			}
			emitProgress("conversation", nextSrcs)
		}
	}

	if runNote {
		var onBatch func(int64)
		if onProgress != nil {
			onBatch = func(skip int64) {
				notesSkip = skip
				emitProgress("note", []string{})
			}
		}
		processed, newSkip := s.backfillNotesWithProgress(ctx, ownerOrgID, limit, useLimit, notesSkip, onBatch)
		result.NotesProcessed = processed
		notesSkip = newSkip
		if onProgress != nil {
			emitProgress("note", []string{})
		}
	}

	// Chẩn đoán khi tất cả = 0
	if result.OrdersProcessed == 0 && result.ConversationsProcessed == 0 && result.NotesProcessed == 0 {
		result.Diagnostic = s.backfillDiagnostic(ctx, ownerOrgID, runOrder, runConv)
	}

	return result, nil
}

// countBackfillSourceTotals đếm tổng số bản ghi mỗi nguồn backfill (để tính % tiến độ).
func (s *CrmCustomerService) countBackfillSourceTotals(ctx context.Context, ownerOrgID primitive.ObjectID, runOrder, runConv, runNote bool) (ordersTotal, convTotal, notesTotal int64) {
	if runOrder {
		if coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.OrderCanonical); ok {
			filter := bson.M{
				"ownerOrganizationId": ownerOrgID,
				"$and": []bson.M{
					{"status": bson.M{"$nin": []int{6}}},
					{"posData.status": bson.M{"$nin": []int{6}}},
				},
			}
			ordersTotal, _ = coll.CountDocuments(ctx, filter)
		}
	}
	if runConv {
		if coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations); ok {
			convTotal, _ = coll.CountDocuments(ctx, bson.M{"ownerOrganizationId": ownerOrgID})
		}
	}
	if runNote {
		if coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CustomerNotes); ok {
			notesTotal, _ = coll.CountDocuments(ctx, bson.M{"ownerOrganizationId": ownerOrgID, "isDeleted": false})
		}
	}
	return ordersTotal, convTotal, notesTotal
}

// buildBackfillProgress tạo progress với currentSource, nextSources, percentBySource.
func (s *CrmCustomerService) buildBackfillProgress(ordersSkip, convSkip, notesSkip, ordersTotal, convTotal, notesTotal int64, runOrder, runConv, runNote bool, currentSource string, nextSources []string) bson.M {
	p := bson.M{
		"ordersSkip":       ordersSkip,
		"conversationsSkip": convSkip,
		"notesSkip":        notesSkip,
		"currentSource":    currentSource,
		"nextSources":      nextSources,
	}
	percentBySource := bson.M{}
	if runOrder {
		orderPct := 0
		if ordersTotal > 0 {
			orderPct = int(float64(ordersSkip) / float64(ordersTotal) * 100)
			if orderPct > 100 {
				orderPct = 100
			}
		}
		percentBySource["order"] = orderPct
	}
	if runConv {
		convPct := 0
		if convTotal > 0 {
			convPct = int(float64(convSkip) / float64(convTotal) * 100)
			if convPct > 100 {
				convPct = 100
			}
		}
		percentBySource["conversation"] = convPct
	}
	if runNote {
		notePct := 0
		if notesTotal > 0 {
			notePct = int(float64(notesSkip) / float64(notesTotal) * 100)
			if notePct > 100 {
				notePct = 100
			}
		}
		percentBySource["note"] = notePct
	}
	p["percentBySource"] = percentBySource
	p["totals"] = bson.M{"order": ordersTotal, "conversation": convTotal, "note": notesTotal}
	return p
}

// extractConversationCustomerId lấy customerId từ FbConversation.
// Ưu tiên customers[0].id (match fb_customers từ upsertCustomerFromConversation) để hợp nhất sync/backfill.
// Thông tin nào đến trước tạo trước, thông tin sau cập nhật thêm.
func extractConversationCustomerId(doc *fbmodels.FbConversation) string {
	if doc == nil {
		return ""
	}
	if doc.PanCakeData != nil {
		pd := doc.PanCakeData
		// 1. customers[0].id — match fb_customers.customerId (canonical cho fb flow)
		if arr, ok := pd["customers"].([]interface{}); ok && len(arr) > 0 {
			if m, ok := arr[0].(map[string]interface{}); ok {
				if id := extractIdFromMap(m); id != "" {
					return id
				}
			}
		}
		// 2. page_customer.id
		if pc, ok := pd["page_customer"].(map[string]interface{}); ok {
			if id := extractIdFromMap(pc); id != "" {
				return id
			}
		}
		// 3. customer.id
		if cust, ok := pd["customer"].(map[string]interface{}); ok {
			if id := extractIdFromMap(cust); id != "" {
				return id
			}
		}
		// 4. customer_id (Pancake format)
		if s, ok := pd["customer_id"].(string); ok && s != "" {
			return s
		}
		if n, ok := pd["customer_id"].(float64); ok {
			return fmt.Sprintf("%.0f", n)
		}
	}
	if doc.CustomerId != "" {
		return doc.CustomerId
	}
	return ""
}

// ExtractConversationCustomerId lấy customerId từ FbConversation (exported cho worker).
func ExtractConversationCustomerId(doc *fbmodels.FbConversation) string {
	return extractConversationCustomerId(doc)
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

// backfillOrdersWithProgress backfill đơn hàng theo thứ tự thời gian, hỗ trợ checkpoint.
// Trả về (số đã xử lý, skip mới). onBatchDone gọi sau mỗi batch (nil = không dùng checkpoint).
func (s *CrmCustomerService) backfillOrdersWithProgress(ctx context.Context, ownerOrgID primitive.ObjectID, limit int, useLimit bool, startSkip int64, onBatchDone func(int64)) (int, int64) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.OrderCanonical)
	if !ok {
		logger.GetAppLogger().Warn("[CRM] Backfill orders: không tìm thấy collection order_canonical")
		return 0, startSkip
	}
	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$and": []bson.M{
			{"status": bson.M{"$nin": []int{6}}},
			{"posData.status": bson.M{"$nin": []int{6}}},
		},
	}
	sortOpt := mongoopts.Find().SetSort(bson.D{{Key: "insertedAt", Value: 1}, {Key: "_id", Value: 1}})

	count := 0
	for skip := startSkip; ; skip += backfillBatchSize {
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
			var co ordermodels.CommerceOrder
			if err := cursor.Decode(&co); err != nil {
				continue
			}
			doc := commerceOrderAsPosViewForIngest(&co)
			if doc == nil {
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
				_ = s.IngestOrderTouchpoint(ctx, customerId, ownerOrgID, doc.OrderId, true, channel, true, doc)
				count++
			}
		}
		cursor.Close(ctx)
		if onBatchDone != nil {
			onBatchDone(skip + int64(docsInBatch))
		}
		if docsInBatch < batchLimit {
			return count, skip + int64(docsInBatch)
		}
	}
	return count, startSkip + int64(count)
}

// backfillOrders backfill đơn hàng (không checkpoint). Gọi backfillOrdersWithProgress với startSkip=0, onBatchDone=nil.
func (s *CrmCustomerService) backfillOrders(ctx context.Context, ownerOrgID primitive.ObjectID, limit int, useLimit bool) int {
	n, _ := s.backfillOrdersWithProgress(ctx, ownerOrgID, limit, useLimit, 0, nil)
	return n
}

// backfillConversationsWithProgress backfill hội thoại, hỗ trợ checkpoint.
func (s *CrmCustomerService) backfillConversationsWithProgress(ctx context.Context, ownerOrgID primitive.ObjectID, limit int, useLimit bool, startSkip int64, onBatchDone func(int64)) (processed, logged, skipped int, newSkip int64) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations)
	if !ok {
		logger.GetAppLogger().Warn("[CRM] Backfill conversations: không tìm thấy collection fb_conversations")
		return 0, 0, 0, startSkip
	}
	filter := bson.M{"ownerOrganizationId": ownerOrgID}
	sortOpt := mongoopts.Find().SetSort(bson.D{
		{Key: "panCakeData.inserted_at", Value: 1}, {Key: "panCakeData.updated_at", Value: 1},
		{Key: "panCakeUpdatedAt", Value: 1}, {Key: "_id", Value: 1},
	})

	decodeErrCount := 0
	skip := startSkip
	for {
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
		if onBatchDone != nil {
			onBatchDone(skip + int64(docsInBatch))
		}
		if docsInBatch < batchLimit {
			return processed, logged, skipped, skip + int64(docsInBatch)
		}
		skip += backfillBatchSize
	}
	return processed, logged, skipped, skip
}

// backfillConversations backfill hội thoại (không checkpoint).
func (s *CrmCustomerService) backfillConversations(ctx context.Context, ownerOrgID primitive.ObjectID, limit int, useLimit bool) (processed, logged, skipped int) {
	p, l, sk, _ := s.backfillConversationsWithProgress(ctx, ownerOrgID, limit, useLimit, 0, nil)
	return p, l, sk
}

// backfillNotesWithProgress backfill ghi chú, hỗ trợ checkpoint.
func (s *CrmCustomerService) backfillNotesWithProgress(ctx context.Context, ownerOrgID primitive.ObjectID, limit int, useLimit bool, startSkip int64, onBatchDone func(int64)) (int, int64) {
	noteSvc, err := NewCrmNoteService()
	if err != nil {
		return 0, startSkip
	}
	coll := noteSvc.Collection()
	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"isDeleted":           false,
	}
	sortOpt := mongoopts.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}, {Key: "_id", Value: 1}})

	count := 0
	for skip := startSkip; ; skip += backfillBatchSize {
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
			return count, skip
		}
		docsInBatch := 0
		for cursor.Next(ctx) {
			docsInBatch++
			var doc crmmodels.CrmNote
			if err := cursor.Decode(&doc); err != nil {
				continue
			}
			if doc.CustomerId != "" {
				_ = s.IngestNoteTouchpoint(ctx, doc.CustomerId, ownerOrgID, doc.ID.Hex(), true, &doc)
				count++
			}
		}
		cursor.Close(ctx)
		if onBatchDone != nil {
			onBatchDone(skip + int64(docsInBatch))
		}
		if docsInBatch < batchLimit {
			return count, skip + int64(docsInBatch)
		}
	}
	return count, startSkip + int64(count)
}

// backfillNotes backfill ghi chú (không checkpoint).
func (s *CrmCustomerService) backfillNotes(ctx context.Context, ownerOrgID primitive.ObjectID, limit int, useLimit bool) int {
	n, _ := s.backfillNotesWithProgress(ctx, ownerOrgID, limit, useLimit, 0, nil)
	return n
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
		if coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.OrderCanonical); ok {
			d.TotalOrders, _ = coll.CountDocuments(ctx, bson.M{})
			d.OrdersWithOrg, _ = coll.CountDocuments(ctx, bson.M{"ownerOrganizationId": ownerOrgID})
		}
	}

	return d
}
