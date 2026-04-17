// Package datachanged — Sau datachanged: xếp job vào crm_pending_merge (merge L1→L2; khác CIO ingest).
package datachanged

import (
	"context"
	"strings"

	crmqueue "meta_commerce/internal/api/aidecision/crmqueue"
	crmvc "meta_commerce/internal/api/crm/service"
	"meta_commerce/internal/api/events"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
)

// EnqueueCrmMergeFromDataChange ghi job vào crm_pending_merge (khách POS/FB, đơn, hội thoại, ghi chú).
// traceID / correlationID từ envelope datachanged (consumer AID); defer flush có thể truyền rỗng.
// bus — bản sao envelope bus AID (có thể nil).
func EnqueueCrmMergeFromDataChange(ctx context.Context, e events.DataChangeEvent, traceID, correlationID string, bus *crmqueue.DomainQueueBusFields) {
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
		global.MongoDB_ColNames.CustomerNotes:
		if err := crmvc.EnqueueCrmPendingMerge(ctx, e.CollectionName, e.Operation, e.Document, e.PreviousDocument, ownerOrgID, strings.TrimSpace(traceID), strings.TrimSpace(correlationID), bus); err != nil {
			logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
				"collection": e.CollectionName,
				"operation":  e.Operation,
			}).Warn("[CRM] Không thể ghi vào queue crm_pending_merge")
		}
	default:
		return
	}
}
