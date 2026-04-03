// Package datachanged — Miền CRM: mọi ingest sau datachanged chỉ ghi crm_pending_ingest; CrmIngestWorker merge rồi emit crm.intelligence.recompute_requested.
// Consumer AI Decision không gọi merge đồng bộ tại đây.
package datachanged

import (
	"context"

	crmvc "meta_commerce/internal/api/crm/service"
	"meta_commerce/internal/api/events"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
)

// IngestFromDataChange xếp job vào crm_pending_ingest (khách POS/FB, đơn, hội thoại, ghi chú).
func IngestFromDataChange(ctx context.Context, e events.DataChangeEvent) {
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
		if err := crmvc.EnqueueCrmIngest(ctx, e.CollectionName, e.Operation, e.Document, e.PreviousDocument, ownerOrgID); err != nil {
			logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
				"collection": e.CollectionName,
				"operation":  e.Operation,
			}).Warn("[CRM] Không thể ghi vào queue crm_pending_ingest")
		}
	default:
		return
	}
}
