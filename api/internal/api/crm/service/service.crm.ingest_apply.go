// Package crmvc — Áp dụng merge/touchpoint CRM đồng bộ từ bản ghi nguồn (một cửa với worker crm_pending_ingest).
package crmvc

import (
	"context"
	"strings"

	crmmodels "meta_commerce/internal/api/crm/models"
	"meta_commerce/internal/api/events"
	fbmodels "meta_commerce/internal/api/fb/models"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ApplyCrmIngestFromDocument merge/touchpoint — chỉ gọi từ CrmIngestWorker.processItem (crm_pending_ingest).
func ApplyCrmIngestFromDocument(ctx context.Context, customerSvc *CrmCustomerService, collectionName, operation string, ownerOrgID primitive.ObjectID, doc bson.M) error {
	if customerSvc == nil || doc == nil || ownerOrgID.IsZero() {
		return nil
	}
	collectionName = strings.TrimSpace(collectionName)
	switch collectionName {
	case global.MongoDB_ColNames.PcPosCustomers:
		var d pcmodels.PcPosCustomer
		if err := bsonMapToStructCRM(doc, &d); err != nil {
			return err
		}
		return customerSvc.MergeFromPosCustomer(ctx, &d, 0)

	case global.MongoDB_ColNames.FbCustomers:
		var d fbmodels.FbCustomer
		if err := bsonMapToStructCRM(doc, &d); err != nil {
			return err
		}
		return customerSvc.MergeFromFbCustomer(ctx, &d, 0)

	case global.MongoDB_ColNames.PcPosOrders:
		var d pcmodels.PcPosOrder
		if err := bsonMapToStructCRM(doc, &d); err != nil {
			return err
		}
		customerId := d.CustomerId
		if customerId == "" {
			if m, ok := d.PosData["customer"].(map[string]interface{}); ok {
				if id, ok := m["id"].(string); ok {
					customerId = id
				}
			}
		}
		if customerId == "" {
			return nil
		}
		channel := "offline"
		if d.PageId != "" {
			channel = "online"
		} else if d.PosData != nil {
			if pid, ok := d.PosData["page_id"].(string); ok && pid != "" {
				channel = "online"
			}
		}
		return customerSvc.IngestOrderTouchpoint(ctx, customerId, ownerOrgID, d.OrderId, operation == events.OpUpdate, channel, false, &d)

	case global.MongoDB_ColNames.FbConvesations:
		var d fbmodels.FbConversation
		if err := bsonMapToStructCRM(doc, &d); err != nil {
			return err
		}
		customerId := ExtractConversationCustomerId(&d)
		if customerId == "" {
			return nil
		}
		_, err := customerSvc.IngestConversationTouchpoint(ctx, customerId, ownerOrgID, d.ConversationId, false, &d)
		return err

	case global.MongoDB_ColNames.CrmNotes:
		var d crmmodels.CrmNote
		if err := bsonMapToStructCRM(doc, &d); err != nil {
			return err
		}
		switch operation {
		case events.OpInsert:
			return customerSvc.IngestNoteTouchpoint(ctx, d.CustomerId, ownerOrgID, d.ID.Hex(), false, &d)
		case events.OpUpdate:
			if d.IsDeleted {
				return customerSvc.IngestNoteDeletedTouchpoint(ctx, d.CustomerId, ownerOrgID, d.ID.Hex(), &d)
			}
			return customerSvc.IngestNoteUpdatedTouchpoint(ctx, d.CustomerId, ownerOrgID, d.ID.Hex(), &d)
		}
		return nil

	default:
		return nil
	}
}

// DataChangeDocumentToBsonM chuẩn hoá Document từ DataChangeEvent sang bson.M.
func DataChangeDocumentToBsonM(doc interface{}) (bson.M, error) {
	if doc == nil {
		return nil, nil
	}
	if m, ok := doc.(bson.M); ok {
		return m, nil
	}
	data, err := bson.Marshal(doc)
	if err != nil {
		return nil, err
	}
	var out bson.M
	if err := bson.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CollectSourceCustomerIds gom mọi customerId nguồn (inbox) để khớp entityRefs trên decision_cases_runtime.
func CollectSourceCustomerIds(c *crmmodels.CrmCustomer) []string {
	if c == nil {
		return nil
	}
	seen := make(map[string]struct{})
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		seen[s] = struct{}{}
	}
	add(c.SourceIds.Fb)
	add(c.SourceIds.Pos)
	add(c.SourceIds.Zalo)
	for _, id := range c.SourceIds.AllInboxIds {
		add(id)
	}
	for _, v := range c.SourceIds.FbByPage {
		add(v)
	}
	for _, v := range c.SourceIds.ZaloByPage {
		add(v)
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	return out
}

func bsonMapToStructCRM(m bson.M, out interface{}) error {
	if m == nil {
		return nil
	}
	data, err := bson.Marshal(m)
	if err != nil {
		return err
	}
	return bson.Unmarshal(data, out)
}
