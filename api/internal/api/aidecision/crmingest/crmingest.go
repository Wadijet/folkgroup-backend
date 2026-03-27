// Package crmingest — Đưa thay đổi collection nguồn vào crm_pending_ingest sau khi đã qua decision_events_queue (AI Decision consumer).
// Hook EmitDataChanged không gọi trực tiếp CRM; chỉ aidecision/hooks emit event, consumer gọi EnqueueFromDatachangedEvent.
package crmingest

import (
	"context"

	crmvc "meta_commerce/internal/api/crm/service"
	"meta_commerce/internal/api/events"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
)

// EnqueueFromDatachangedEvent ghi job vào crm_pending_ingest — chỉ gọi từ worker AI Decision (sau event datachanged).
func EnqueueFromDatachangedEvent(ctx context.Context, e events.DataChangeEvent) {
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
