// Package worker - CrmIngestWorker xử lý crm_pending_ingest: Merge/Ingest thay vì chạy trong hook.
package worker

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	crmmodels "meta_commerce/internal/api/crm/models"
	crmvc "meta_commerce/internal/api/crm/service"
	fbmodels "meta_commerce/internal/api/fb/models"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/api/events"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
)

// CrmIngestWorker worker xử lý crm_pending_ingest: đọc job chưa xử lý, gọi Merge/Ingest.
type CrmIngestWorker struct {
	interval  time.Duration
	batchSize int
}

// NewCrmIngestWorker tạo mới CrmIngestWorker.
func NewCrmIngestWorker(interval time.Duration, batchSize int) *CrmIngestWorker {
	if interval < 10*time.Second {
		interval = 30 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 30
	}
	return &CrmIngestWorker{interval: interval, batchSize: batchSize}
}

// Start chạy worker trong vòng lặp.
func (w *CrmIngestWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.WithFields(map[string]interface{}{
		"interval":   w.interval.String(),
		"batchSize":  w.batchSize,
	}).Info("📋 [CRM_INGEST] Starting CRM Ingest Worker...")

	for {
		select {
		case <-ctx.Done():
			log.Info("📋 [CRM_INGEST] CRM Ingest Worker stopped")
			return
		case <-ticker.C:
			if ShouldThrottle(PriorityCritical) {
				continue
			}
			if effInterval := GetEffectiveInterval(w.interval, PriorityCritical); effInterval > w.interval {
				time.Sleep(effInterval - w.interval)
			}
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.WithFields(map[string]interface{}{"panic": r}).Error("📋 [CRM_INGEST] Panic khi xử lý, sẽ tiếp tục lần sau")
					}
				}()

				totalProcessed := 0
				batchSize := GetEffectiveBatchSize(w.batchSize, PriorityCritical)
				for {
					list, err := crmvc.GetUnprocessedCrmIngest(ctx, batchSize)
					if err != nil {
						log.WithError(err).Error("📋 [CRM_INGEST] Lỗi lấy danh sách pending ingest")
						return
					}
					if len(list) == 0 {
						break
					}

					customerSvc, err := crmvc.NewCrmCustomerService()
					if err != nil {
						log.WithError(err).Error("📋 [CRM_INGEST] Không thể tạo CrmCustomerService")
						return
					}

					for _, item := range list {
						err := w.processItem(ctx, customerSvc, &item)
						errStr := ""
						if err != nil {
							errStr = err.Error()
							log.WithError(err).WithFields(map[string]interface{}{
								"collection": item.CollectionName,
								"id":         item.ID.Hex(),
							}).Warn("📋 [CRM_INGEST] Xử lý job lỗi")
						}
						if setErr := crmvc.SetCrmIngestProcessed(ctx, item.ID, errStr); setErr != nil {
							log.WithError(setErr).Warn("📋 [CRM_INGEST] SetCrmIngestProcessed thất bại")
						} else {
							totalProcessed++
						}
					}
				}

				if totalProcessed > 0 {
					log.WithFields(map[string]interface{}{"processed": totalProcessed}).Info("📋 [CRM_INGEST] Đã xử lý xong batch")
				}
			}()
		}
	}
}

// processItem xử lý một job: decode document và gọi logic CRM tương ứng.
func (w *CrmIngestWorker) processItem(ctx context.Context, customerSvc *crmvc.CrmCustomerService, item *crmmodels.CrmPendingIngest) error {
	ownerOrgID := item.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		return nil
	}

	switch item.CollectionName {
	case global.MongoDB_ColNames.PcPosCustomers:
		var doc pcmodels.PcPosCustomer
		if err := bsonMapToStruct(item.Document, &doc); err != nil {
			return err
		}
		return customerSvc.MergeFromPosCustomer(ctx, &doc, 0)

	case global.MongoDB_ColNames.FbCustomers:
		var doc fbmodels.FbCustomer
		if err := bsonMapToStruct(item.Document, &doc); err != nil {
			return err
		}
		return customerSvc.MergeFromFbCustomer(ctx, &doc, 0)

	case global.MongoDB_ColNames.PcPosOrders:
		var doc pcmodels.PcPosOrder
		if err := bsonMapToStruct(item.Document, &doc); err != nil {
			return err
		}
		customerId := doc.CustomerId
		if customerId == "" {
			if m, ok := doc.PosData["customer"].(map[string]interface{}); ok {
				if id, ok := m["id"].(string); ok {
					customerId = id
				}
			}
		}
		if customerId == "" {
			return nil
		}
		channel := "offline"
		if doc.PageId != "" {
			channel = "online"
		} else if doc.PosData != nil {
			if pid, ok := doc.PosData["page_id"].(string); ok && pid != "" {
				channel = "online"
			}
		}
		return customerSvc.IngestOrderTouchpoint(ctx, customerId, ownerOrgID, doc.OrderId, item.Operation == events.OpUpdate, channel, false, &doc)

	case global.MongoDB_ColNames.FbConvesations:
		var doc fbmodels.FbConversation
		if err := bsonMapToStruct(item.Document, &doc); err != nil {
			return err
		}
		customerId := crmvc.ExtractConversationCustomerId(&doc)
		if customerId == "" {
			return nil
		}
		_, err := customerSvc.IngestConversationTouchpoint(ctx, customerId, ownerOrgID, doc.ConversationId, false, &doc)
		return err

	case global.MongoDB_ColNames.CrmNotes:
		var doc crmmodels.CrmNote
		if err := bsonMapToStruct(item.Document, &doc); err != nil {
			return err
		}
		switch item.Operation {
		case events.OpInsert:
			return customerSvc.IngestNoteTouchpoint(ctx, doc.CustomerId, ownerOrgID, doc.ID.Hex(), false, &doc)
		case events.OpUpdate:
			if doc.IsDeleted {
				return customerSvc.IngestNoteDeletedTouchpoint(ctx, doc.CustomerId, ownerOrgID, doc.ID.Hex(), &doc)
			}
			return customerSvc.IngestNoteUpdatedTouchpoint(ctx, doc.CustomerId, ownerOrgID, doc.ID.Hex(), &doc)
		}
		return nil

	default:
		return nil
	}
}

func bsonMapToStruct(m bson.M, out interface{}) error {
	if m == nil {
		return nil
	}
	data, err := bson.Marshal(m)
	if err != nil {
		return err
	}
	return bson.Unmarshal(data, out)
}
