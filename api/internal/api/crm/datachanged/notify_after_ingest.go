// Package datachanged — Sau khi CrmIngestWorker merge thành công: bắn event vào AID để debounce rồi xếp tính lại CRM intelligence.
package datachanged

import (
	"context"
	"strings"

	crmqueue "meta_commerce/internal/api/aidecision/crmqueue"
	crmmodels "meta_commerce/internal/api/crm/models"
	crmvc "meta_commerce/internal/api/crm/service"
	fbmodels "meta_commerce/internal/api/fb/models"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson"
)

// NotifyIntelRefreshAfterIngestIfNeeded — resolve unifiedId sau merge, emit crm.intelligence.recompute_requested (consumer AID debounce → crm_intel_compute).
func NotifyIntelRefreshAfterIngestIfNeeded(ctx context.Context, item *crmmodels.CrmPendingIngest) error {
	if item == nil || item.Document == nil || item.OwnerOrganizationID.IsZero() {
		return nil
	}
	cn := strings.TrimSpace(item.CollectionName)
	ownerOrgID := item.OwnerOrganizationID
	doc := item.Document

	customerID := extractCustomerIDFromPendingIngestDoc(cn, doc)
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
	_, err = crmqueue.EmitCrmIntelligenceRecomputeRequested(ctx, unifiedID, ownerOrgID, cn, jobHex)
	if err != nil {
		logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
			"collection": cn, "unifiedId": unifiedID,
		}).Warn("[CRM] Không emit crm.intelligence.recompute_requested sau ingest")
	}
	return err
}

func extractCustomerIDFromPendingIngestDoc(collectionName string, doc bson.M) string {
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
	case global.MongoDB_ColNames.CrmNotes:
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
