// Package crmvc - Hàm trung tâm IngestCustomerTouchpoint.
// Hook và job backfill đều đẩy dữ liệu qua các hàm này.
package crmvc

import (
	"context"
	"fmt"
	"strings"
	"time"

	crmmodels "meta_commerce/internal/api/crm/models"
	fbmodels "meta_commerce/internal/api/fb/models"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// IngestOrderTouchpoint xử lý order: resolve unifiedId, refresh metrics, log activity.
// orderDoc: optional — khi từ hook truyền doc để lấy metadata đầy đủ; nil khi backfill.
func (s *CrmCustomerService) IngestOrderTouchpoint(ctx context.Context, customerId string, ownerOrgID primitive.ObjectID, orderId int64, isUpdate bool, channel string, skipIfExists bool, orderDoc *pcmodels.PcPosOrder) error {
	if customerId == "" {
		return nil
	}
	unifiedId, found := s.ResolveUnifiedId(ctx, customerId, ownerOrgID)
	if !found {
		posColl, _ := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosCustomers)
		if posColl != nil {
			var posCustomer pcmodels.PcPosCustomer
			if posColl.FindOne(ctx, bson.M{"customerId": customerId, "ownerOrganizationId": ownerOrgID}).Decode(&posCustomer) == nil {
				_ = s.MergeFromPosCustomer(ctx, &posCustomer)
				unifiedId, found = s.ResolveUnifiedId(ctx, customerId, ownerOrgID)
			}
		}
	}
	if !found {
		// Backfill: tạo crm_customer tối thiểu từ order — sync sẽ cập nhật sau khi có pc_pos_customers
		custData := extractCustomerDataFromOrder(orderDoc)
		activityAt := getOrderTimestamp(orderDoc)
		unifiedId, _ = s.UpsertMinimalFromPosId(ctx, customerId, ownerOrgID, &custData, activityAt)
		found = unifiedId != ""
	}
	if !found || unifiedId == "" {
		return nil
	}
	// Merge thông tin từ order vào profile (fill gaps khi còn thiếu)
	s.MergeProfileFromOrder(ctx, unifiedId, ownerOrgID, orderDoc)
	_ = s.RefreshMetrics(ctx, unifiedId, ownerOrgID)

	activityType := "order_created"
	if isUpdate {
		activityType = "order_completed"
	}
	// Kiểm tra đơn hủy (status 6) — cả insert và update
	if orderDoc != nil {
		st := getOrderStatus(orderDoc)
		if st == 6 {
			activityType = "order_cancelled"
		}
	}
	sourceRef := map[string]interface{}{"orderId": orderId}
	metadata, displayLabel, displaySubtext := buildOrderActivityMetadata(orderId, orderDoc, channel, activityType)
	activityAt := getOrderTimestamp(orderDoc)
	actSvc, errAct := NewCrmActivityService()
	if errAct != nil {
		return errAct
	}
	// Snapshot chỉ khi profile/metrics thay đổi (có snapshotChanges); snapshotAt = thời điểm sự kiện order
	// Loại trừ activity của chính đơn này (order_created) để so sánh với snapshot TRƯỚC đơn — metrics mới có thay đổi
	// Dùng metrics as of activityAt (chỉ đơn có orderDate <= activityAt) để timeline đúng
	if cust, err := s.FindOne(ctx, bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}, nil); err == nil {
		lastProfile, lastMetrics, _ := actSvc.GetLastSnapshotForCustomer(ctx, unifiedId, ownerOrgID, "pos", sourceRef)
		metricsOverride := s.GetMetricsForSnapshotAt(ctx, &cust, activityAt)
		profileOverride := s.GetProfileForSnapshotAt(ctx, &cust, activityAt)
		if snap := BuildSnapshotWithChanges(&cust, lastProfile, lastMetrics, activityAt, metricsOverride, profileOverride); snap != nil {
			MergeSnapshotIntoMetadata(metadata, snap)
		} else {
			// Upsert: giữ snapshot cũ khi không có thay đổi (tránh mất snapshot khi order_created → order_completed)
			if existing := actSvc.GetExistingActivityBySourceRef(ctx, unifiedId, ownerOrgID, "pos", sourceRef); existing != nil && existing.Metadata != nil {
				for _, key := range []string{"profileSnapshot", "metricsSnapshot", "snapshotChanges", "snapshotAt"} {
					if v, ok := existing.Metadata[key]; ok {
						metadata[key] = v
					}
				}
			}
		}
	}
	input := LogActivityInput{
		UnifiedId:      unifiedId,
		OwnerOrgID:     ownerOrgID,
		Domain:         crmmodels.ActivityDomainOrder,
		ActivityType:   activityType,
		Source:         "pos",
		SourceRef:      sourceRef,
		Metadata:       metadata,
		DisplayLabel:   displayLabel,
		DisplayIcon:    "shopping_cart",
		DisplaySubtext: displaySubtext,
		ActivityAt:     activityAt,
	}
	switch activityType {
	case "order_completed":
		input.DisplayIcon = "check_circle"
	case "order_cancelled":
		input.DisplayIcon = "cancel"
	}
	if skipIfExists {
		_, _ = actSvc.LogActivityIfNotExists(ctx, input)
	} else {
		_ = actSvc.LogActivity(ctx, input)
	}
	// Khi insert activity cũ hơn các activity hiện có → tính lại snapshot của các activity mới hơn
	s.recomputeSnapshotsForNewerActivities(ctx, actSvc, unifiedId, ownerOrgID, activityAt)
	return nil
}

func buildOrderActivityMetadata(orderId int64, orderDoc *pcmodels.PcPosOrder, channel string, activityType string) (map[string]interface{}, string, string) {
	metadata := map[string]interface{}{"channel": channel}
	amount := 0.0
	status := 0
	statusName := ""
	pageId := ""
	shopId := int64(0)
	itemCount := 0

	if orderDoc != nil {
		if orderDoc.PosData != nil {
			amount = getFloatFromMap(orderDoc.PosData, "total_price_after_sub_discount")
			if v, ok := orderDoc.PosData["status"]; ok {
				switch x := v.(type) {
				case int:
					status = x
				case float64:
					status = int(x)
				case int64:
					status = int(x)
				}
			}
			if sn, ok := orderDoc.PosData["status_name"].(string); ok {
				statusName = sn
			}
			if pid, ok := orderDoc.PosData["page_id"].(string); ok {
				pageId = pid
			}
			if sid, ok := orderDoc.PosData["shop_id"]; ok {
				switch x := sid.(type) {
				case int64:
					shopId = x
				case float64:
					shopId = int64(x)
				case int:
					shopId = int64(x)
				}
			}
			if items, ok := orderDoc.PosData["items"].([]interface{}); ok {
				itemCount = len(items)
			} else if items, ok := orderDoc.PosData["order_items"].([]interface{}); ok {
				itemCount = len(items)
			}
		}
		if orderDoc.PageId != "" {
			pageId = orderDoc.PageId
		}
		metadata["amount"] = amount
		metadata["status"] = status
		metadata["statusName"] = statusName
		metadata["pageId"] = pageId
		metadata["shopId"] = shopId
		metadata["itemCount"] = itemCount
		metadata["orderSource"] = getOrderSourceFromPosData(orderDoc.PosData)
		if adId, ok := getStringFromMap(orderDoc.PosData, "ad_id"); ok && adId != "" {
			metadata["adId"] = adId
		}
		if postId, ok := getStringFromMap(orderDoc.PosData, "post_id"); ok && postId != "" {
			metadata["postId"] = postId
		}
		// itemSkus: map SKU -> qty của đơn này
		itemSkus := buildItemSkusFromOrder(orderDoc)
		if len(itemSkus) > 0 {
			metadata["itemSkus"] = itemSkus
		}
	}

	amountStr := formatAmountVND(amount)
	displayLabel := fmt.Sprintf("Đơn hàng #%d - %s (%s)", orderId, amountStr, channel)
	displaySubtext := fmt.Sprintf("%s • %d sản phẩm", channel, itemCount)
	if itemCount == 0 {
		displaySubtext = channel
	}
	switch activityType {
	case "order_cancelled":
		displayLabel = fmt.Sprintf("Đơn #%d đã hủy", orderId)
		displaySubtext = channel
	}
	return metadata, displayLabel, displaySubtext
}

// buildItemSkusFromOrder tạo map SKU -> số lượng từ order (posData.items).
func buildItemSkusFromOrder(orderDoc *pcmodels.PcPosOrder) map[string]int {
	if orderDoc == nil || orderDoc.PosData == nil {
		return nil
	}
	items := extractOrderItemsFromPosData(orderDoc.PosData)
	out := make(map[string]int)
	for _, it := range items {
		sku, qty := getSkuAndOwnedQty(it)
		if sku != "" && qty > 0 {
			out[sku] += qty
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// getConversationTimestamp lấy thời điểm bắt đầu hội thoại từ conversation.
// Ưu tiên inserted_at (thời điểm hội thoại bắt đầu), fallback updated_at, PanCakeUpdatedAt, CreatedAt.
// Tránh trả về 0 vì LogActivity sẽ dùng "now" khi activityAt=0 — timeline sai.
func getConversationTimestamp(convDoc *fbmodels.FbConversation) int64 {
	if convDoc == nil {
		return 0
	}
	// Ưu tiên thời gian gốc từ panCakeData (inserted_at = thời điểm hội thoại bắt đầu)
	if convDoc.PanCakeData != nil {
		for _, key := range []string{"inserted_at", "insertedAt", "created_at", "createdAt"} {
			if t := getTimestampFromMap(convDoc.PanCakeData, key); t > 0 {
				return t
			}
		}
		for _, key := range []string{"updated_at", "updatedAt"} {
			if t := getTimestampFromMap(convDoc.PanCakeData, key); t > 0 {
				return t
			}
		}
	}
	// Fallback: PanCakeUpdatedAt (thời gian cập nhật từ API) hoặc CreatedAt (thời gian tạo document)
	if convDoc.PanCakeUpdatedAt > 0 {
		return convDoc.PanCakeUpdatedAt
	}
	if convDoc.CreatedAt > 0 {
		return convDoc.CreatedAt
	}
	return 0
}

// getTimestampFromMap lấy Unix ms từ map (hỗ trợ string ISO, primitive.DateTime, float64, int64).
// Khi BSON decode panCakeData/posData, inserted_at có thể là primitive.DateTime → phải xử lý để activityAt đúng.
// Giá trị số < 1e12 được coi là Unix seconds (Pancake API có thể trả seconds) → nhân 1000.
func getTimestampFromMap(m map[string]interface{}, key string) int64 {
	if m == nil {
		return 0
	}
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case string:
		// Hỗ trợ nhiều format: ISO với microsecond (Pancake), RFC3339, format có space
		layouts := []string{
			"2006-01-02T15:04:05.000000", "2006-01-02T15:04:05.999999",
			"2006-01-02T15:04:05.000", "2006-01-02T15:04:05",
			"2006-01-02 15:04:05.000000", "2006-01-02 15:04:05",
			time.RFC3339, time.RFC3339Nano,
		}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, x); err == nil {
				return t.UnixMilli()
			}
		}
		return 0
	case primitive.DateTime:
		return x.Time().UnixMilli()
	case float64:
		ms := int64(x)
		if ms > 0 && ms < 1e12 {
			ms *= 1000 // Unix seconds → milliseconds
		}
		return ms
	case int64:
		if x > 0 && x < 1e12 {
			x *= 1000 // Unix seconds → milliseconds
		}
		return x
	case int:
		ms := int64(x)
		if ms > 0 && ms < 1e12 {
			ms *= 1000
		}
		return ms
	}
	return 0
}

func getOrderTimestamp(orderDoc *pcmodels.PcPosOrder) int64 {
	if orderDoc == nil || orderDoc.PosData == nil {
		return 0
	}
	// Chỉ lấy thời gian gốc từ posData. CreatedAt/InsertedAt/PosUpdatedAt... ngoài document là thời gian đồng bộ.
	if t := getTimestampFromMap(orderDoc.PosData, "inserted_at"); t > 0 {
		return t
	}
	if t := getTimestampFromMap(orderDoc.PosData, "updated_at"); t > 0 {
		return t
	}
	return 0
}

func getOrderStatus(orderDoc *pcmodels.PcPosOrder) int {
	if orderDoc == nil {
		return 0
	}
	if orderDoc.Status == 6 {
		return 6
	}
	if orderDoc.PosData != nil {
		if v, ok := orderDoc.PosData["status"]; ok {
			switch x := v.(type) {
			case int:
				return x
			case float64:
				return int(x)
			case int64:
				return int(x)
			}
		}
	}
	return 0
}

func formatAmountVND(amount float64) string {
	if amount >= 1000000 {
		return fmt.Sprintf("%.1fMđ", amount/1000000)
	}
	if amount >= 1000 {
		return fmt.Sprintf("%.0fKđ", amount/1000)
	}
	return fmt.Sprintf("%.0fđ", amount)
}

// IngestConversationTouchpoint xử lý conversation: resolve unifiedId, refresh metrics, log activity.
// Trả về (logged bool, err error): logged=true nếu đã ghi activity; logged=false nếu không resolve được (bỏ qua).
func (s *CrmCustomerService) IngestConversationTouchpoint(ctx context.Context, customerId string, ownerOrgID primitive.ObjectID, conversationId string, skipIfExists bool, convDoc *fbmodels.FbConversation) (bool, error) {
	if customerId == "" {
		return false, nil
	}
	unifiedId, found := s.ResolveUnifiedId(ctx, customerId, ownerOrgID)
	if !found {
		// Thử merge từ fb_customers (Pancake customer_id thường trỏ tới FB customer)
		fbColl, _ := global.RegistryCollections.Get(global.MongoDB_ColNames.FbCustomers)
		if fbColl != nil {
			var fbCustomer fbmodels.FbCustomer
			if fbColl.FindOne(ctx, bson.M{"customerId": customerId, "ownerOrganizationId": ownerOrgID}).Decode(&fbCustomer) == nil {
				_ = s.MergeFromFbCustomer(ctx, &fbCustomer, getConversationTimestamp(convDoc))
				unifiedId, found = s.ResolveUnifiedId(ctx, customerId, ownerOrgID)
			}
		}
	}
	if !found {
		// Fallback: thử merge từ pc_pos_customers (khi Pancake link conversation với POS customer qua customerId)
		posColl, _ := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosCustomers)
		if posColl != nil {
			var posCustomer pcmodels.PcPosCustomer
			if posColl.FindOne(ctx, bson.M{"customerId": customerId, "ownerOrganizationId": ownerOrgID}).Decode(&posCustomer) == nil {
				_ = s.MergeFromPosCustomer(ctx, &posCustomer)
				unifiedId, found = s.ResolveUnifiedId(ctx, customerId, ownerOrgID)
			}
		}
	}
	if !found {
		// Backfill: tạo crm_customer tối thiểu từ conversation — sync sẽ cập nhật sau khi có fb_customers
		custData := extractCustomerDataFromConv(convDoc)
		activityAt := getConversationTimestamp(convDoc)
		unifiedId, _ = s.UpsertMinimalFromFbId(ctx, customerId, ownerOrgID, &custData, activityAt)
		found = unifiedId != ""
	}
	if !found || unifiedId == "" {
		return false, nil
	}
	// Merge thông tin từ conversation vào profile (fill gaps khi còn thiếu)
	s.MergeProfileFromConversation(ctx, unifiedId, ownerOrgID, convDoc)
	_ = s.RefreshMetrics(ctx, unifiedId, ownerOrgID)

	sourceRef := map[string]interface{}{"conversationId": conversationId}
	metadata := map[string]interface{}{}
	activityAt := int64(0)
	if convDoc != nil {
		activityAt = getConversationTimestamp(convDoc)
	}
	actSvc, errAct := NewCrmActivityService()
	if errAct != nil {
		return false, errAct
	}
	// Snapshot chỉ khi profile/metrics thay đổi; snapshotAt = thời điểm sự kiện conversation
	if cust, err := s.FindOne(ctx, bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}, nil); err == nil {
		// Loại trừ activity của chính conversation này để so sánh với snapshot TRƯỚC — metrics mới có thay đổi
		// Dùng metrics và profile as of activityAt để timeline đúng
		lastProfile, lastMetrics, _ := actSvc.GetLastSnapshotForCustomer(ctx, unifiedId, ownerOrgID, "fb", sourceRef)
		metricsOverride := s.GetMetricsForSnapshotAt(ctx, &cust, activityAt)
		profileOverride := s.GetProfileForSnapshotAt(ctx, &cust, activityAt)
		if snap := BuildSnapshotWithChanges(&cust, lastProfile, lastMetrics, activityAt, metricsOverride, profileOverride); snap != nil {
			MergeSnapshotIntoMetadata(metadata, snap)
		} else {
			// Upsert: giữ snapshot cũ khi không có thay đổi
			if existing := actSvc.GetExistingActivityBySourceRef(ctx, unifiedId, ownerOrgID, "fb", sourceRef); existing != nil && existing.Metadata != nil {
				for _, key := range []string{"profileSnapshot", "metricsSnapshot", "snapshotChanges", "snapshotAt"} {
					if v, ok := existing.Metadata[key]; ok {
						metadata[key] = v
					}
				}
			}
		}
	}
	displayLabel := "Bắt đầu hội thoại trên Facebook"
	if convDoc != nil {
		if convDoc.PageId != "" {
			metadata["pageId"] = convDoc.PageId
		}
		activityAt = getConversationTimestamp(convDoc)
		if convDoc.PanCakeData != nil {
			pd := convDoc.PanCakeData
			if t, ok := getStringFromMap(pd, "type"); ok && t != "" {
				metadata["conversationType"] = t
			}
			if postId, ok := getStringFromMap(pd, "post_id"); ok && postId != "" {
				metadata["postId"] = postId
			}
			if mc, ok := pd["message_count"]; ok {
				switch x := mc.(type) {
				case int:
					metadata["messageCount"] = x
				case float64:
					metadata["messageCount"] = int(x)
				case int64:
					metadata["messageCount"] = int(x)
				}
			}
			if arr, ok := pd["ad_ids"].([]interface{}); ok && len(arr) > 0 {
				metadata["fromAds"] = true
			} else if arr, ok := pd["ads"].([]interface{}); ok && len(arr) > 0 {
				metadata["fromAds"] = true
			}
			if arr, ok := pd["tags"].([]interface{}); ok && len(arr) > 0 {
				var tagTexts []string
				for _, t := range arr {
					m := toMap(t)
					if m != nil {
						if txt, ok := m["text"].(string); ok && txt != "" {
							tagTexts = append(tagTexts, txt)
						}
					}
				}
				if len(tagTexts) > 0 {
					metadata["tags"] = tagTexts
				}
			}
		}
	}

	input := LogActivityInput{
		UnifiedId:    unifiedId,
		OwnerOrgID:   ownerOrgID,
		Domain:       crmmodels.ActivityDomainConversation,
		ActivityType: "conversation_started",
		Source:       "fb",
		SourceRef:    sourceRef,
		Metadata:     metadata,
		DisplayLabel: displayLabel,
		DisplayIcon:  "chat",
		ActivityAt:   activityAt,
	}
	if skipIfExists {
		_, _ = actSvc.LogActivityIfNotExists(ctx, input)
	} else {
		_ = actSvc.LogActivity(ctx, input)
	}
	// Khi insert activity cũ hơn các activity hiện có → tính lại snapshot của các activity mới hơn
	s.recomputeSnapshotsForNewerActivities(ctx, actSvc, unifiedId, ownerOrgID, activityAt)
	return true, nil
}

// recomputeSnapshotsForNewerActivities tính lại snapshot của các activity có activityAt > insertedActivityAt.
// Khi backfill/sync tạo activity cũ hơn các activity hiện có, snapshot của activity mới hơn có thể sai (thiếu đơn/conv cũ).
// Duyệt theo activityAt tăng dần, mỗi activity lấy last snapshot trước nó rồi tính lại.
func (s *CrmCustomerService) recomputeSnapshotsForNewerActivities(ctx context.Context, actSvc *CrmActivityService, unifiedId string, ownerOrgID primitive.ObjectID, insertedActivityAt int64) {
	if insertedActivityAt <= 0 {
		return
	}
	newer, err := actSvc.FindActivitiesNewerThan(ctx, unifiedId, ownerOrgID, insertedActivityAt)
	if err != nil || len(newer) == 0 {
		return
	}
	cust, err := s.FindOne(ctx, bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}, nil)
	if err != nil {
		return
	}
	for _, a := range newer {
		lastProfile, lastMetrics, _ := actSvc.GetLastSnapshotBeforeActivityAt(ctx, unifiedId, ownerOrgID, a.ActivityAt)
		profileOverride := s.GetProfileForSnapshotAt(ctx, &cust, a.ActivityAt)
		metricsOverride := s.GetMetricsForSnapshotAt(ctx, &cust, a.ActivityAt)
		snap := BuildSnapshotWithChanges(&cust, lastProfile, lastMetrics, a.ActivityAt, metricsOverride, profileOverride)
		if snap != nil {
			updates := map[string]interface{}{
				"profileSnapshot": snap["profileSnapshot"],
				"metricsSnapshot": snap["metricsSnapshot"],
				"snapshotChanges": snap["snapshotChanges"],
				"snapshotAt":      snap["snapshotAt"],
			}
			_ = actSvc.UpdateActivityMetadata(ctx, a.ID, updates)
		}
	}
}

// IngestNoteTouchpoint xử lý note: log activity (customerId đã là unifiedId).
// noteDoc: optional — khi từ hook truyền doc để lấy noteText, createdBy.
func (s *CrmCustomerService) IngestNoteTouchpoint(ctx context.Context, customerId string, ownerOrgID primitive.ObjectID, noteId string, skipIfExists bool, noteDoc *crmmodels.CrmNote) error {
	if customerId == "" || noteId == "" {
		return nil
	}

	sourceRef := map[string]interface{}{"noteId": noteId}
	metadata := map[string]interface{}{}
	// Không lưu snapshot cho note — nội dung note đã có trong metadata
	displayLabel := "Ghi chú mới"
	var actorId *primitive.ObjectID
	actorName := ""

	activityAt := int64(0)
	if noteDoc != nil {
		preview := truncateString(noteDoc.NoteText, 100)
		metadata["noteTextPreview"] = preview
		metadata["nextAction"] = noteDoc.NextAction
		metadata["nextActionDate"] = noteDoc.NextActionDate
		if !noteDoc.CreatedBy.IsZero() {
			actorId = &noteDoc.CreatedBy
			metadata["createdById"] = noteDoc.CreatedBy.Hex()
		}
		// TODO: resolve createdByName từ user service nếu có
		displayLabel = fmt.Sprintf("Ghi chú: %s", truncateString(noteDoc.NoteText, 50))
		if actorName != "" {
			displayLabel += " - " + actorName
		}
		activityAt = noteDoc.CreatedAt
	}

	input := LogActivityInput{
		UnifiedId:    customerId,
		OwnerOrgID:   ownerOrgID,
		Domain:       crmmodels.ActivityDomainNote,
		ActivityType: "note_added",
		Source:       "system",
		SourceRef:    sourceRef,
		Metadata:     metadata,
		DisplayLabel: displayLabel,
		DisplayIcon:  "note_add",
		ActorId:      actorId,
		ActorName:    actorName,
		ActivityAt:   activityAt,
	}

	actSvc, err := NewCrmActivityService()
	if err != nil {
		return err
	}
	if skipIfExists {
		_, _ = actSvc.LogActivityIfNotExists(ctx, input)
	} else {
		_ = actSvc.LogActivity(ctx, input)
	}
	return nil
}

// IngestNoteUpdatedTouchpoint ghi lịch sử cập nhật ghi chú.
func (s *CrmCustomerService) IngestNoteUpdatedTouchpoint(ctx context.Context, customerId string, ownerOrgID primitive.ObjectID, noteId string, noteDoc *crmmodels.CrmNote) error {
	if customerId == "" || noteId == "" {
		return nil
	}
	sourceRef := map[string]interface{}{"noteId": noteId}
	metadata := map[string]interface{}{}
	// Không lưu snapshot cho note — nội dung note đã có trong metadata
	displayLabel := "Cập nhật ghi chú"
	var actorId *primitive.ObjectID
	activityAt := int64(0)
	if noteDoc != nil {
		metadata["noteTextPreview"] = truncateString(noteDoc.NoteText, 100)
		if !noteDoc.CreatedBy.IsZero() {
			actorId = &noteDoc.CreatedBy
		}
		displayLabel = "Cập nhật ghi chú: " + truncateString(noteDoc.NoteText, 50)
		activityAt = noteDoc.UpdatedAt
	}
	input := LogActivityInput{
		UnifiedId:    customerId,
		OwnerOrgID:   ownerOrgID,
		Domain:       crmmodels.ActivityDomainNote,
		ActivityType: "note_updated",
		Source:       "system",
		SourceRef:    sourceRef,
		Metadata:     metadata,
		DisplayLabel: displayLabel,
		DisplayIcon:  "note_edit",
		ActorId:      actorId,
		ActivityAt:   activityAt,
	}
	actSvc, err := NewCrmActivityService()
	if err != nil {
		return err
	}
	_ = actSvc.LogActivity(ctx, input)
	return nil
}

// IngestNoteDeletedTouchpoint ghi lịch sử xóa ghi chú (soft delete).
func (s *CrmCustomerService) IngestNoteDeletedTouchpoint(ctx context.Context, customerId string, ownerOrgID primitive.ObjectID, noteId string, noteDoc *crmmodels.CrmNote) error {
	if customerId == "" || noteId == "" {
		return nil
	}
	sourceRef := map[string]interface{}{"noteId": noteId}
	metadata := map[string]interface{}{}
	// Không lưu snapshot cho note — nội dung note đã có trong metadata
	displayLabel := "Ghi chú đã xóa"
	var actorId *primitive.ObjectID
	activityAt := int64(0)
	if noteDoc != nil {
		if !noteDoc.CreatedBy.IsZero() {
			actorId = &noteDoc.CreatedBy
		}
		activityAt = noteDoc.UpdatedAt
	}
	input := LogActivityInput{
		UnifiedId:    customerId,
		OwnerOrgID:   ownerOrgID,
		Domain:       crmmodels.ActivityDomainNote,
		ActivityType: "note_deleted",
		Source:       "system",
		SourceRef:    sourceRef,
		Metadata:     metadata,
		DisplayLabel: displayLabel,
		DisplayIcon:  "delete",
		ActorId:      actorId,
		ActivityAt:   activityAt,
	}
	actSvc, err := NewCrmActivityService()
	if err != nil {
		return err
	}
	_ = actSvc.LogActivity(ctx, input)
	return nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// convCustomerData thông tin khách trích từ panCakeData của conversation hoặc order.
type convCustomerData struct {
	Name                string
	Phones              []string
	Emails              []string
	Birthday            string
	Gender              string
	LivesIn             string
	Addresses           []interface{}
	ReferralCode        string
}

// extractCustomerDataFromConv lấy tất cả thông tin khách từ panCakeData: customer, customers[0], page_customer, root.
// Pancake API có thể dùng nhiều cấu trúc khác nhau. Thu thập name, phone, email, birthday, gender, lives_in, addresses.
func extractCustomerDataFromConv(convDoc *fbmodels.FbConversation) convCustomerData {
	var out convCustomerData
	if convDoc == nil || convDoc.PanCakeData == nil {
		return out
	}
	pd := convDoc.PanCakeData
	sources := []map[string]interface{}{}
	if cust, ok := pd["customer"].(map[string]interface{}); ok && cust != nil {
		sources = append(sources, cust)
	}
	if arr, ok := pd["customers"].([]interface{}); ok && len(arr) > 0 {
		if m, ok := arr[0].(map[string]interface{}); ok {
			sources = append(sources, m)
		}
	}
	if pc, ok := pd["page_customer"].(map[string]interface{}); ok && pc != nil {
		sources = append(sources, pc)
	}
	sources = append(sources, pd)

	// Duyệt tất cả nguồn, gộp thông tin (ưu tiên giá trị đầu tiên tìm được)
	for _, m := range sources {
		mergeCustomerDataInto(&out, m)
	}
	return out
}

// mergeCustomerDataInto gộp thông tin từ map vào out (chỉ ghi khi out còn trống).
func mergeCustomerDataInto(out *convCustomerData, m map[string]interface{}) {
	if m == nil {
		return
	}
	// name
	if out.Name == "" {
		for _, k := range []string{"name", "full_name", "customer_name"} {
			if s, ok := getStringFromMap(m, k); ok && s != "" {
				out.Name = s
				break
			}
		}
	}
	if out.Name == "" {
		first, _ := getStringFromMap(m, "first_name")
		last, _ := getStringFromMap(m, "last_name")
		if first != "" || last != "" {
			out.Name = strings.TrimSpace(first + " " + last)
		}
	}
	// phones
	if len(out.Phones) == 0 {
		if arr, ok := m["phone_numbers"].([]interface{}); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok && s != "" {
					out.Phones = append(out.Phones, s)
				} else if n, ok := v.(float64); ok {
					out.Phones = append(out.Phones, fmt.Sprintf("%.0f", n))
				}
			}
		}
	}
	if len(out.Phones) == 0 {
		for _, k := range []string{"phone", "phone_number"} {
			if s, ok := getStringFromMap(m, k); ok && s != "" {
				out.Phones = []string{s}
				break
			}
		}
	}
	// recent_phone_numbers (status 2 hoặc 3 = đã xác thực)
	if len(out.Phones) == 0 {
		if arr, ok := m["recent_phone_numbers"].([]interface{}); ok {
			for _, v := range arr {
				item, ok := v.(map[string]interface{})
				if !ok {
					continue
				}
				status := 0
				if s, ok := item["status"]; ok {
					switch x := s.(type) {
					case int:
						status = x
					case float64:
						status = int(x)
					case int64:
						status = int(x)
					}
				}
				if status != 2 && status != 3 {
					continue
				}
				if s, ok := getStringFromMap(item, "phone_number"); ok && s != "" {
					out.Phones = append(out.Phones, s)
				}
			}
		}
	}
	// emails
	if len(out.Emails) == 0 {
		if arr := getStringArrayFromMap(m, "emails"); len(arr) > 0 {
			out.Emails = uniqueStrings(arr)
		}
	}
	if len(out.Emails) == 0 {
		if s, ok := getStringFromMap(m, "email"); ok && s != "" {
			out.Emails = []string{s}
		}
	}
	// birthday
	if out.Birthday == "" {
		for _, k := range []string{"birthday", "date_of_birth"} {
			if s, ok := getStringFromMap(m, k); ok && s != "" {
				out.Birthday = s
				break
			}
		}
	}
	// gender
	if out.Gender == "" {
		if s, ok := getStringFromMap(m, "gender"); ok && s != "" {
			out.Gender = s
		}
	}
	// lives_in (FB)
	if out.LivesIn == "" {
		if s, ok := getStringFromMap(m, "lives_in"); ok && s != "" {
			out.LivesIn = s
		}
	}
	// addresses
	if len(out.Addresses) == 0 {
		if arr, ok := m["addresses"].([]interface{}); ok && len(arr) > 0 {
			out.Addresses = arr
		}
		if len(out.Addresses) == 0 {
			if arr, ok := m["shop_customer_addresses"].([]interface{}); ok && len(arr) > 0 {
				out.Addresses = arr
			}
		}
		if len(out.Addresses) == 0 {
			if arr, ok := m["shop_customer_address"].([]interface{}); ok && len(arr) > 0 {
				out.Addresses = arr
			}
		}
		if len(out.Addresses) == 0 {
			if s, ok := getStringFromMap(m, "full_address"); ok && s != "" {
				out.Addresses = []interface{}{map[string]interface{}{"full_address": s}}
			}
		}
	if len(out.Addresses) == 0 {
		if s, ok := getStringFromMap(m, "address"); ok && s != "" {
			out.Addresses = []interface{}{map[string]interface{}{"address": s}}
		}
	}
	// referral_code
	if out.ReferralCode == "" {
		if s, ok := getStringFromMap(m, "referral_code"); ok && s != "" {
			out.ReferralCode = s
		}
	}
	if out.ReferralCode == "" {
		if s, ok := getStringFromMap(m, "customer_referral_code"); ok && s != "" {
			out.ReferralCode = s
		}
	}
}
}

// extractCustomerDataFromOrder lấy thông tin khách từ posData của order.
// Ưu tiên posData.customer, fallback bill_*, shipping_address, bill_address.
func extractCustomerDataFromOrder(orderDoc *pcmodels.PcPosOrder) convCustomerData {
	var out convCustomerData
	if orderDoc == nil || orderDoc.PosData == nil {
		return out
	}
	pd := orderDoc.PosData
	// Nguồn 1: customer object (có thể có name, phone_numbers, emails, date_of_birth, gender, ...)
	if cust, ok := pd["customer"].(map[string]interface{}); ok && cust != nil {
		mergeCustomerDataInto(&out, cust)
	}
	// Nguồn 2: bill_* (order có thể là guest)
	if out.Name == "" {
		if s, ok := getStringFromMap(pd, "bill_full_name"); ok && s != "" {
			out.Name = s
		}
	}
	if len(out.Phones) == 0 {
		if s, ok := getStringFromMap(pd, "bill_phone_number"); ok && s != "" {
			out.Phones = []string{s}
		}
	}
	if len(out.Emails) == 0 {
		if s, ok := getStringFromMap(pd, "bill_email"); ok && s != "" {
			out.Emails = []string{s}
		}
	}
	// Địa chỉ: shipping_address (object hoặc array), bill_address
	if len(out.Addresses) == 0 {
		if addr, ok := pd["shipping_address"].(map[string]interface{}); ok && addr != nil {
			out.Addresses = []interface{}{addr}
		}
	}
	if len(out.Addresses) == 0 {
		if arr, ok := pd["shipping_address"].([]interface{}); ok && len(arr) > 0 {
			out.Addresses = arr
		}
	}
	if len(out.Addresses) == 0 {
		if addr, ok := pd["bill_address"].(map[string]interface{}); ok && addr != nil {
			out.Addresses = []interface{}{addr}
		}
	}
	if len(out.Addresses) == 0 {
		if s, ok := getStringFromMap(pd, "bill_address"); ok && s != "" {
			out.Addresses = []interface{}{map[string]interface{}{"address": s}}
		}
	}
	// referral_code từ posData
	if s, ok := getStringFromMap(pd, "referral_code"); ok && s != "" {
		out.ReferralCode = s
	}
	if out.ReferralCode == "" {
		if s, ok := getStringFromMap(pd, "customer_referral_code"); ok && s != "" {
			out.ReferralCode = s
		}
	}
	return out
}
