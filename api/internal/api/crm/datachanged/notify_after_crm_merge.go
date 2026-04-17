// Package datachanged — Sau khi CrmPendingMergeWorker merge L1→L2 thành công: emit AID để debounce rồi crm_intel_compute.
package datachanged

import (
	"context"
	"strings"

	"meta_commerce/internal/api/aidecision/eventtypes"
	crmqueue "meta_commerce/internal/api/aidecision/crmqueue"
	crmmodels "meta_commerce/internal/api/crm/models"
	crmvc "meta_commerce/internal/api/crm/service"
	fbmodels "meta_commerce/internal/api/fb/models"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson"
)

// NotifyIntelRecomputeAfterCrmMergeIfNeeded — resolve unifiedId sau merge queue, emit decision_events_queue (l2_datachanged + <prefix>.changed khi map được collection; fallback recompute_requested).
func NotifyIntelRecomputeAfterCrmMergeIfNeeded(ctx context.Context, item *crmmodels.CrmPendingMerge) error {
	if item == nil || item.OwnerOrganizationID.IsZero() {
		return nil
	}
	if len(item.SourceSnapshots) == 0 && item.Document == nil {
		return nil
	}
	ownerOrgID := item.OwnerOrganizationID
	sourceCollection := strings.TrimSpace(item.CollectionName)
	if len(item.SourceCollections) > 0 {
		sourceCollection = strings.Join(item.SourceCollections, ",")
	}

	customerID := strings.TrimSpace(item.InboxCustomerId)
	if customerID == "" && item.Document != nil {
		customerID = extractCustomerIDFromPendingMergeDoc(strings.TrimSpace(item.CollectionName), item.Document)
	}
	if customerID == "" {
		return nil
	}

	svc, err := crmvc.NewCrmCustomerService()
	if err != nil {
		return err
	}
	unifiedID, ok := svc.ResolveUnifiedId(ctx, customerID, ownerOrgID)
	if !ok || unifiedID == "" {
		return nil
	}

	jobHex := ""
	if !item.ID.IsZero() {
		jobHex = item.ID.Hex()
	}
	causalMs := item.UpdatedAtNew
	if causalMs <= 0 && item.CreatedAt > 0 {
		// createdAt trong queue thường là Unix giây (EnqueueCrmPendingMerge)
		causalMs = item.CreatedAt * 1000
	}
	eventType, ok := eventtypes.EventTypeChangedForCollection(sourceCollection)
	if !ok {
		eventType = crmqueue.EventTypeCrmIntelligenceRecomputeRequested
	}
	_, err = crmqueue.EmitAfterL2MergeForCrmIntel(ctx, eventType, unifiedID, ownerOrgID, sourceCollection, jobHex, causalMs, item.TraceID, item.CorrelationID)
	if err != nil {
		logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
			"collection": sourceCollection, "unifiedId": unifiedID,
		}).Warn("[CRM] Không emit sau merge L2 (l2_datachanged) lên decision_events_queue")
	}
	return err
}

func extractCustomerIDFromPendingMergeDoc(collectionName string, doc bson.M) string {
	if doc == nil {
		return ""
	}
	switch strings.TrimSpace(collectionName) {
	case global.MongoDB_ColNames.PcPosCustomers:
		var d pcmodels.PcPosCustomer
		if err := bsonMapToStructForNotify(doc, &d); err != nil {
			return ""
		}
		return strings.TrimSpace(d.CustomerId)
	case global.MongoDB_ColNames.FbCustomers:
		var d fbmodels.FbCustomer
		if err := bsonMapToStructForNotify(doc, &d); err != nil {
			return ""
		}
		return strings.TrimSpace(d.CustomerId)
	case global.MongoDB_ColNames.PcPosOrders:
		var d pcmodels.PcPosOrder
		if err := bsonMapToStructForNotify(doc, &d); err != nil {
			return ""
		}
		cid := strings.TrimSpace(d.CustomerId)
		if cid == "" && d.PosData != nil {
			if m, ok := d.PosData["customer"].(map[string]interface{}); ok {
				if id, ok := m["id"].(string); ok {
					cid = strings.TrimSpace(id)
				}
			}
		}
		return cid
	case global.MongoDB_ColNames.FbConvesations:
		var d fbmodels.FbConversation
		if err := bsonMapToStructForNotify(doc, &d); err != nil {
			return ""
		}
		return strings.TrimSpace(crmvc.ExtractConversationCustomerId(&d))
	case global.MongoDB_ColNames.CustomerNotes:
		var d crmmodels.CrmNote
		if err := bsonMapToStructForNotify(doc, &d); err != nil {
			return ""
		}
		return strings.TrimSpace(d.CustomerId)
	default:
		return ""
	}
}

func bsonMapToStructForNotify(m bson.M, out interface{}) error {
	if m == nil {
		return nil
	}
	data, err := bson.Marshal(m)
	if err != nil {
		return err
	}
	return bson.Unmarshal(data, out)
}
