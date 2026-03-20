// Package worker - CrmIngestWorker xử lý crm_pending_ingest: Merge/Ingest thay vì chạy trong hook.
package worker

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	crmmodels "meta_commerce/internal/api/crm/models"
	crmvc "meta_commerce/internal/api/crm/service"
	"meta_commerce/internal/api/events"
	fbmodels "meta_commerce/internal/api/fb/models"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/worker/metrics"
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

// Start chạy worker trong vòng lặp. Đọc config mỗi vòng (hỗ trợ thay đổi qua API).
func (w *CrmIngestWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()

	log.WithFields(map[string]interface{}{
		"interval":  w.interval.String(),
		"batchSize": w.batchSize,
	}).Info("📋 [CRM_INGEST] Starting CRM Ingest Worker...")

	for {
		interval, batchSize := GetEffectiveWorkerSchedule(WorkerCrmIngest, w.interval, w.batchSize)

		if !IsWorkerActive(WorkerCrmIngest) {
			select {
			case <-ctx.Done():
				log.Info("📋 [CRM_INGEST] CRM Ingest Worker stopped")
				return
			case <-time.After(1 * time.Minute):
			}
			continue
		}
		p := GetPriority(WorkerCrmIngest, PriorityHigh)
		if ShouldThrottle(p) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
			}
			continue
		}
		effInterval := GetEffectiveInterval(interval, p)
		if effInterval > interval {
			select {
			case <-ctx.Done():
				return
			case <-time.After(effInterval - interval):
			}
		}

		select {
		case <-ctx.Done():
			log.Info("📋 [CRM_INGEST] CRM Ingest Worker stopped")
			return
		case <-time.After(interval):
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithFields(map[string]interface{}{"panic": r}).Error("📋 [CRM_INGEST] Panic khi xử lý, sẽ tiếp tục lần sau")
				}
			}()

			totalProcessed := 0
			baseBatchSize := GetEffectiveBatchSize(batchSize, p)
			// Adaptive batch: khi backlog lớn thì tăng batch size để xử lý nhanh hơn (giảm DB round-trips).
			actualBatchSize := baseBatchSize
			if count, err := crmvc.CountUnprocessedCrmIngest(ctx); err == nil && count > int64(baseBatchSize*3) {
				adaptive := int(count / 2)
				if adaptive > 100 {
					adaptive = 100
				}
				if adaptive > actualBatchSize {
					actualBatchSize = adaptive
					log.WithFields(map[string]interface{}{
						"backlog":   count,
						"batchSize": actualBatchSize,
					}).Info("📋 [CRM_INGEST] Backlog cao, tăng batch size adaptive")
				}
			}
			for {
				if ShouldThrottle(p) {
					break
				}
				list, err := crmvc.GetUnprocessedCrmIngest(ctx, actualBatchSize)
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
					start := time.Now()
					err := w.processItem(ctx, customerSvc, &item)
					jobType := "crm_ingest:" + item.CollectionName
					if item.CollectionName == "" {
						jobType = "crm_ingest:unknown"
					}
					metrics.RecordDuration(jobType, time.Since(start))
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
				remaining, _ := crmvc.CountUnprocessedCrmIngest(ctx)
				log.WithFields(map[string]interface{}{
					"processed": totalProcessed,
					"remaining": remaining,
				}).Info("📋 [CRM_INGEST] Đã xử lý xong batch")
				if remaining > 50 {
					log.WithFields(map[string]interface{}{"remaining": remaining}).Warn("📋 [CRM_INGEST] Backlog còn cao, agent có thể đang sync nhanh hơn worker xử lý")
				}
			}
		}()
	}
}

// processItem xử lý một job: decode document và gọi logic CRM tương ứng.
func (w *CrmIngestWorker) processItem(ctx context.Context, customerSvc *crmvc.CrmCustomerService, item *crmmodels.CrmPendingIngest) error {
	ownerOrgID := item.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		return nil
	}

	// collectionName có thể rỗng với job cũ (EnqueueCrmIngest trước đây không set) — lấy từ businessKey
	collectionName := item.CollectionName
	if collectionName == "" && item.BusinessKey != "" {
		if idx := strings.Index(item.BusinessKey, "|"); idx > 0 {
			collectionName = item.BusinessKey[:idx]
		}
	}

	switch collectionName {
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
