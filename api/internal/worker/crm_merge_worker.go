// Package worker — CrmPendingMergeWorker xử lý queue crm_pending_merge (merge L1→L2 CRM, khác CIO ingest).
package worker

import (
	"context"
	"time"

	crmdec "meta_commerce/internal/api/crm/datachanged"
	crmmodels "meta_commerce/internal/api/crm/models"
	crmvc "meta_commerce/internal/api/crm/service"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/worker/metrics"
)

// CrmPendingMergeWorker đọc crm_pending_merge và gọi merge/touchpoint.
type CrmPendingMergeWorker struct {
	interval  time.Duration
	batchSize int
}

// NewCrmPendingMergeWorker tạo worker.
func NewCrmPendingMergeWorker(interval time.Duration, batchSize int) *CrmPendingMergeWorker {
	if interval < 10*time.Second {
		interval = 30 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 30
	}
	return &CrmPendingMergeWorker{interval: interval, batchSize: batchSize}
}

// Start chạy vòng lặp worker.
func (w *CrmPendingMergeWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()

	log.WithFields(map[string]interface{}{
		"interval":  w.interval.String(),
		"batchSize": w.batchSize,
	}).Info("📋 [CRM_MERGE_QUEUE] Starting CRM pending merge worker...")

	for {
		interval, batchSize := GetEffectiveWorkerSchedule(WorkerCrmPendingMerge, w.interval, w.batchSize)

		if !IsWorkerActive(WorkerCrmPendingMerge) {
			select {
			case <-ctx.Done():
				log.Info("📋 [CRM_MERGE_QUEUE] Worker stopped")
				return
			case <-time.After(1 * time.Minute):
			}
			continue
		}
		p := GetPriority(WorkerCrmPendingMerge, PriorityHigh)
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
			log.Info("📋 [CRM_MERGE_QUEUE] Worker stopped")
			return
		case <-time.After(interval):
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithFields(map[string]interface{}{"panic": r}).Error("📋 [CRM_MERGE_QUEUE] Panic khi xử lý, sẽ tiếp tục lần sau")
				}
			}()

			totalProcessed := 0
			baseBatchSize := GetEffectiveBatchSize(batchSize, p)
			actualBatchSize := baseBatchSize
			if count, err := crmvc.CountUnprocessedCrmPendingMerge(ctx); err == nil && count > int64(baseBatchSize*3) {
				adaptive := int(count / 2)
				if adaptive > 100 {
					adaptive = 100
				}
				if adaptive > actualBatchSize {
					actualBatchSize = adaptive
					log.WithFields(map[string]interface{}{
						"backlog":   count,
						"batchSize": actualBatchSize,
					}).Info("📋 [CRM_MERGE_QUEUE] Backlog cao, tăng batch size adaptive")
				}
			}
			for {
				if ShouldThrottle(p) {
					break
				}
				list, err := crmvc.GetUnprocessedCrmPendingMerge(ctx, actualBatchSize)
				if err != nil {
					log.WithError(err).Error("📋 [CRM_MERGE_QUEUE] Lỗi lấy danh sách pending merge")
					return
				}
				if len(list) == 0 {
					break
				}

				customerSvc, err := crmvc.NewCrmCustomerService()
				if err != nil {
					log.WithError(err).Error("📋 [CRM_MERGE_QUEUE] Không thể tạo CrmCustomerService")
					return
				}

				for _, item := range list {
					start := time.Now()
					notifyCrmPendingMergeLiveStart(&item)
					err := w.processItem(ctx, customerSvc, &item)
					jobType := "customer_pending_merge:" + item.CollectionName
					if len(item.SourceSnapshots) > 0 {
						jobType = "customer_pending_merge:coalesced"
					} else if item.CollectionName == "" {
						jobType = "customer_pending_merge:unknown"
					}
					metrics.RecordDuration(jobType, time.Since(start))
					errStr := ""
					if err != nil {
						errStr = err.Error()
						notifyCrmPendingMergeLiveError(&item, err)
						log.WithError(err).WithFields(map[string]interface{}{
							"collection": item.CollectionName,
							"id":         item.ID.Hex(),
						}).Warn("📋 [CRM_MERGE_QUEUE] Xử lý job lỗi")
					} else {
						if nerr := crmdec.NotifyIntelRecomputeAfterCrmMergeIfNeeded(ctx, &item); nerr != nil {
							log.WithError(nerr).WithFields(map[string]interface{}{
								"collection": item.CollectionName,
								"id":         item.ID.Hex(),
							}).Debug("📋 [CRM_MERGE_QUEUE] Thông báo intel sau merge (AID debounce)")
						}
						notifyCrmPendingMergeLiveDone(&item)
					}
					if setErr := crmvc.SetCrmPendingMergeProcessed(ctx, item.ID, errStr); setErr != nil {
						log.WithError(setErr).Warn("📋 [CRM_MERGE_QUEUE] SetCrmPendingMergeProcessed thất bại")
					} else {
						totalProcessed++
					}
				}
			}

			if totalProcessed > 0 {
				remaining, _ := crmvc.CountUnprocessedCrmPendingMerge(ctx)
				log.WithFields(map[string]interface{}{
					"processed": totalProcessed,
					"remaining": remaining,
				}).Info("📋 [CRM_MERGE_QUEUE] Đã xử lý xong batch")
				if remaining > 50 {
					log.WithFields(map[string]interface{}{"remaining": remaining}).Warn("📋 [CRM_MERGE_QUEUE] Backlog còn cao")
				}
			}
		}()
	}
}

func (w *CrmPendingMergeWorker) processItem(ctx context.Context, customerSvc *crmvc.CrmCustomerService, item *crmmodels.CrmPendingMerge) error {
	ownerOrgID := item.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		return nil
	}

	if len(item.SourceSnapshots) == 0 && item.Document == nil {
		return nil
	}
	return crmvc.ApplyCrmPendingMergeJob(ctx, customerSvc, item)
}
