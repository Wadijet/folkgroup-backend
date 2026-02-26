// Package crmvc - Event handlers cho CRM (OnDataChanged).
// Hook đẩy dữ liệu qua IngestCustomerTouchpoint (service.crm.ingest.go).
package crmvc

import (
	"context"

	crmmodels "meta_commerce/internal/api/crm/models"
	fbmodels "meta_commerce/internal/api/fb/models"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/api/events"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
)

func init() {
	events.OnDataChanged(handleCrmDataChange)
}

// handleCrmDataChange xử lý thay đổi dữ liệu — đẩy qua IngestCustomerTouchpoint.
func handleCrmDataChange(ctx context.Context, e events.DataChangeEvent) {
	customerSvc, err := NewCrmCustomerService()
	if err != nil {
		logger.GetAppLogger().WithError(err).Warn("[CRM] Không thể tạo CrmCustomerService trong hook")
		return
	}

	ownerOrgID := events.GetOwnerOrganizationIDFromDocument(e.Document)
	if ownerOrgID.IsZero() {
		return
	}

	switch e.CollectionName {
	case global.MongoDB_ColNames.PcPosCustomers:
		if doc, ok := toPcPosCustomer(e.Document); ok {
			if err := customerSvc.MergeFromPosCustomer(ctx, doc); err != nil {
				logger.GetAppLogger().WithError(err).WithField("customerId", doc.CustomerId).Warn("[CRM] MergeFromPosCustomer lỗi")
			}
		}

	case global.MongoDB_ColNames.FbCustomers:
		if doc, ok := toFbCustomer(e.Document); ok {
			if err := customerSvc.MergeFromFbCustomer(ctx, doc, 0); err != nil {
				logger.GetAppLogger().WithError(err).WithField("customerId", doc.CustomerId).Warn("[CRM] MergeFromFbCustomer lỗi")
			}
		}

	case global.MongoDB_ColNames.PcPosOrders:
		if doc, ok := toPcPosOrder(e.Document); ok {
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
				_ = customerSvc.IngestOrderTouchpoint(ctx, customerId, ownerOrgID, doc.OrderId, e.Operation == events.OpUpdate, channel, false, doc)
			}
		}

	case global.MongoDB_ColNames.FbConvesations:
		if doc, ok := toFbConversation(e.Document); ok && doc.CustomerId != "" {
			_, _ = customerSvc.IngestConversationTouchpoint(ctx, doc.CustomerId, ownerOrgID, doc.ConversationId, false, doc)
		}

	case global.MongoDB_ColNames.CrmNotes:
		if doc, ok := toCrmNote(e.Document); ok {
			switch e.Operation {
			case events.OpInsert:
				_ = customerSvc.IngestNoteTouchpoint(ctx, doc.CustomerId, ownerOrgID, doc.ID.Hex(), false, doc)
			case events.OpUpdate:
				if doc.IsDeleted {
					_ = customerSvc.IngestNoteDeletedTouchpoint(ctx, doc.CustomerId, ownerOrgID, doc.ID.Hex(), doc)
				} else {
					_ = customerSvc.IngestNoteUpdatedTouchpoint(ctx, doc.CustomerId, ownerOrgID, doc.ID.Hex(), doc)
				}
			}
		}

	default:
		return
	}
}

func toPcPosCustomer(doc interface{}) (*pcmodels.PcPosCustomer, bool) {
	if doc == nil {
		return nil, false
	}
	if d, ok := doc.(*pcmodels.PcPosCustomer); ok {
		return d, true
	}
	if d, ok := doc.(pcmodels.PcPosCustomer); ok {
		return &d, true
	}
	return nil, false
}

func toFbCustomer(doc interface{}) (*fbmodels.FbCustomer, bool) {
	if doc == nil {
		return nil, false
	}
	if d, ok := doc.(*fbmodels.FbCustomer); ok {
		return d, true
	}
	if d, ok := doc.(fbmodels.FbCustomer); ok {
		return &d, true
	}
	return nil, false
}

func toPcPosOrder(doc interface{}) (*pcmodels.PcPosOrder, bool) {
	if doc == nil {
		return nil, false
	}
	if d, ok := doc.(*pcmodels.PcPosOrder); ok {
		return d, true
	}
	if d, ok := doc.(pcmodels.PcPosOrder); ok {
		return &d, true
	}
	return nil, false
}

func toFbConversation(doc interface{}) (*fbmodels.FbConversation, bool) {
	if doc == nil {
		return nil, false
	}
	if d, ok := doc.(*fbmodels.FbConversation); ok {
		return d, true
	}
	if d, ok := doc.(fbmodels.FbConversation); ok {
		return &d, true
	}
	return nil, false
}

func toCrmNote(doc interface{}) (*crmmodels.CrmNote, bool) {
	if doc == nil {
		return nil, false
	}
	if d, ok := doc.(*crmmodels.CrmNote); ok {
		return d, true
	}
	if d, ok := doc.(crmmodels.CrmNote); ok {
		return &d, true
	}
	return nil, false
}
