// Package crmvc - Event handlers cho CRM (OnDataChanged).
// Hook ghi vào crm_pending_ingest; worker xử lý Merge/Ingest (service.crm.ingest.go).
package crmvc

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"

	"meta_commerce/internal/api/events"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
)

func init() {
	events.OnDataChanged(handleCrmDataChange)
}

// handleCrmDataChange ghi event vào queue crm_pending_ingest; worker sẽ xử lý Merge/Ingest.
func handleCrmDataChange(ctx context.Context, e events.DataChangeEvent) {
	if e.Document == nil {
		return
	}
	ownerOrgID := events.GetOwnerOrganizationIDFromDocument(e.Document)
	if ownerOrgID.IsZero() {
		return
	}

	switch e.CollectionName {
	case global.MongoDB_ColNames.PcPosCustomers,
		global.MongoDB_ColNames.FbCustomers,
		global.MongoDB_ColNames.PcPosOrders,
		global.MongoDB_ColNames.FbConvesations,
		global.MongoDB_ColNames.CrmNotes:
		// Skip Merge khi dữ liệu không đổi (OpUpdate + PreviousDocument)
		if e.Operation == events.OpUpdate && e.PreviousDocument != nil {
			if isMergeRelevantDataUnchanged(e.CollectionName, e.Document, e.PreviousDocument) {
				return
			}
		}
		if err := EnqueueCrmIngest(ctx, e.CollectionName, e.Operation, e.Document, e.PreviousDocument, ownerOrgID); err != nil {
			logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
				"collection": e.CollectionName,
				"operation":  e.Operation,
			}).Warn("[CRM] Không thể ghi vào queue crm_pending_ingest")
		}
	default:
		return
	}
}

// isMergeRelevantDataUnchanged so sánh dữ liệu Merge/Ingest quan trọng giữa doc và prevDoc.
// Chỉ so sánh updated_at trong posData/panCakeData (bỏ fallback mapsEqual để giảm CPU).
// Trả về true nếu không đổi → skip Enqueue.
func isMergeRelevantDataUnchanged(collectionName string, doc, prevDoc interface{}) bool {
	key := mergeRelevantDataKey(collectionName)
	if key == "" {
		return false // Không so sánh (vd: CrmNotes) → không skip
	}
	a := extractMapForKey(doc, key)
	b := extractMapForKey(prevDoc, key)
	if a == nil || b == nil {
		return false // Thiếu data → enqueue để an toàn
	}
	t1 := getUpdatedAtFromDataMap(a)
	t2 := getUpdatedAtFromDataMap(b)
	return t1 > 0 && t2 > 0 && t1 == t2
}

// getUpdatedAtFromDataMap lấy timestamp cập nhật từ map posData/panCakeData.
// Thử updated_at (snake_case) trước, fallback updatedAt (camelCase) — nhất quán với getConversationTimestamp.
func getUpdatedAtFromDataMap(m map[string]interface{}) int64 {
	if t := getTimestampFromMap(m, "updated_at"); t > 0 {
		return t
	}
	return getTimestampFromMap(m, "updatedAt")
}

func mergeRelevantDataKey(collectionName string) string {
	switch collectionName {
	case global.MongoDB_ColNames.PcPosCustomers:
		return "posData"
	case global.MongoDB_ColNames.FbCustomers:
		return "panCakeData"
	case global.MongoDB_ColNames.PcPosOrders:
		return "posData" // IngestOrderTouchpoint dùng posData
	case global.MongoDB_ColNames.FbConvesations:
		return "panCakeData"
	case global.MongoDB_ColNames.CrmNotes:
		return "" // Note: so sánh nhiều field; để "" = không skip
	default:
		return ""
	}
}

// extractUpdatedAtFromDoc lấy updated_at (ms) từ document theo collection.
// CrmNotes: top-level updatedAt; còn lại: posData/panCakeData.updated_at, fallback inserted_at, fallback top-level updatedAt.
func extractUpdatedAtFromDoc(collectionName string, doc interface{}) int64 {
	if doc == nil {
		return 0
	}
	data, err := bson.Marshal(doc)
	if err != nil {
		return 0
	}
	var m map[string]interface{}
	if err := bson.Unmarshal(data, &m); err != nil {
		return 0
	}
	// CrmNotes: top-level updatedAt
	if collectionName == global.MongoDB_ColNames.CrmNotes {
		if t := getTimestampFromMap(m, "updatedAt"); t > 0 {
			return t
		}
		return getTimestampFromMap(m, "updated_at")
	}
	// posData/panCakeData: updated_at, fallback inserted_at
	key := mergeRelevantDataKey(collectionName)
	if key != "" {
		if sub, ok := m[key].(map[string]interface{}); ok && sub != nil {
			if t := getTimestampFromMap(sub, "updated_at"); t > 0 {
				return t
			}
			if t := getTimestampFromMap(sub, "updatedAt"); t > 0 {
				return t
			}
			if t := getTimestampFromMap(sub, "inserted_at"); t > 0 {
				return t
			}
			return getTimestampFromMap(sub, "insertedAt")
		}
	}
	// Fallback: top-level updatedAt (thời điểm sync)
	if t := getTimestampFromMap(m, "updatedAt"); t > 0 {
		return t
	}
	return getTimestampFromMap(m, "updated_at")
}

func extractMapForKey(doc interface{}, key string) map[string]interface{} {
	if doc == nil || key == "" {
		return nil
	}
	data, err := bson.Marshal(doc)
	if err != nil {
		return nil
	}
	var m map[string]interface{}
	if err := bson.Unmarshal(data, &m); err != nil {
		return nil
	}
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	if sub, ok := v.(map[string]interface{}); ok {
		return sub
	}
	return nil
}
