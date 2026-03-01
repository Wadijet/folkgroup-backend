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
		if err := EnqueueCrmIngest(ctx, e.CollectionName, e.Operation, e.Document, ownerOrgID); err != nil {
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
// Trả về true nếu không đổi → skip Enqueue.
func isMergeRelevantDataUnchanged(collectionName string, doc, prevDoc interface{}) bool {
	key := mergeRelevantDataKey(collectionName)
	if key == "" {
		return false // Không so sánh (vd: CrmNotes) → không skip
	}
	a := extractMapForKey(doc, key)
	b := extractMapForKey(prevDoc, key)
	return mapsEqual(a, b)
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

func mapsEqual(a, b map[string]interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	ba, _ := bson.Marshal(a)
	bb, _ := bson.Marshal(b)
	return string(ba) == string(bb)
}
