// Package worker - CrmBulkWorker xử lý crm_bulk_jobs: sync, backfill, rebuild, recalculate.
package worker

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	crmmodels "meta_commerce/internal/api/crm/models"
	crmvc "meta_commerce/internal/api/crm/service"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/worker/metrics"
)

// CrmBulkWorker worker xử lý crm_bulk_jobs: đọc job chưa xử lý, gọi Sync/Backfill/Rebuild/Recalculate.
type CrmBulkWorker struct {
	interval      time.Duration
	batchSize     int
	bulkJobSvc    *crmvc.CrmBulkJobService
}

// NewCrmBulkWorker tạo mới CrmBulkWorker.
func NewCrmBulkWorker(interval time.Duration, batchSize int) (*CrmBulkWorker, error) {
	if interval < 30*time.Second {
		interval = 1 * time.Minute
	}
	if batchSize <= 0 {
		batchSize = 3
	}
	bulkJobSvc, err := crmvc.NewCrmBulkJobService()
	if err != nil {
		return nil, err
	}
	return &CrmBulkWorker{
		interval:   interval,
		batchSize:  batchSize,
		bulkJobSvc: bulkJobSvc,
	}, nil
}

// Start chạy worker trong vòng lặp.
func (w *CrmBulkWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.WithFields(map[string]interface{}{
		"interval":   w.interval.String(),
		"batchSize": w.batchSize,
	}).Info("📋 [CRM_BULK] Starting CRM Bulk Worker...")

	for {
		select {
		case <-ctx.Done():
			log.Info("📋 [CRM_BULK] CRM Bulk Worker stopped")
			return
		case <-ticker.C:
			if ShouldThrottle(PriorityNormal) {
				continue
			}
			if effInterval := GetEffectiveInterval(w.interval, PriorityNormal); effInterval > w.interval {
				time.Sleep(effInterval - w.interval)
			}
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.WithFields(map[string]interface{}{"panic": r}).Error("📋 [CRM_BULK] Panic khi xử lý, sẽ tiếp tục lần sau")
					}
				}()

				batchSize := GetEffectiveBatchSize(w.batchSize, PriorityNormal)
				list, err := w.bulkJobSvc.GetUnprocessed(ctx, batchSize)
				if err != nil {
					log.WithError(err).Error("📋 [CRM_BULK] Lỗi lấy danh sách bulk jobs")
					return
				}
				if len(list) == 0 {
					return
				}

				customerSvc, err := crmvc.NewCrmCustomerService()
				if err != nil {
					log.WithError(err).Error("📋 [CRM_BULK] Không thể tạo CrmCustomerService")
					return
				}

				for _, item := range list {
					start := time.Now()
					result, err := w.processJob(ctx, customerSvc, &item)
					metrics.RecordDuration("crm_bulk:"+item.JobType, time.Since(start))
					errStr := ""
					if err != nil {
						errStr = err.Error()
						log.WithError(err).WithFields(map[string]interface{}{
							"jobType": item.JobType,
							"jobId":   item.ID.Hex(),
						}).Warn("📋 [CRM_BULK] Xử lý job lỗi")
					}
					if setErr := w.bulkJobSvc.SetProcessed(ctx, item.ID, errStr, result); setErr != nil {
						log.WithError(setErr).Warn("📋 [CRM_BULK] SetCrmBulkJobProcessed thất bại")
					}
				}
			}()
		}
	}
}

// processJob xử lý một bulk job theo jobType. Trả về (result, error).
func (w *CrmBulkWorker) processJob(ctx context.Context, svc *crmvc.CrmCustomerService, job *crmmodels.CrmBulkJob) (bson.M, error) {
	params := job.Params
	if params == nil {
		params = bson.M{}
	}

	switch job.JobType {
	case crmmodels.CrmBulkJobSync:
		sources := parseStringSlice(params, "sources")
		posCount, fbCount, err := svc.SyncAllCustomers(ctx, job.OwnerOrganizationID, sources)
		if err != nil {
			return nil, err
		}
		return bson.M{"posProcessed": posCount, "fbProcessed": fbCount}, nil

	case crmmodels.CrmBulkJobBackfill:
		limit := parseInt(params, "limit", 0)
		types := parseStringSlice(params, "types")
		result, err := svc.BackfillActivity(ctx, job.OwnerOrganizationID, limit, types)
		if err != nil {
			return nil, err
		}
		return bson.M{
			"ordersProcessed": result.OrdersProcessed,
			"conversationsProcessed": result.ConversationsProcessed,
			"notesProcessed": result.NotesProcessed,
		}, nil

	case crmmodels.CrmBulkJobRebuild:
		limit := parseInt(params, "limit", 0)
		sources := parseStringSlice(params, "sources")
		types := parseStringSlice(params, "types")
		result, err := svc.RebuildCrm(ctx, job.OwnerOrganizationID, limit, sources, types)
		if err != nil {
			return nil, err
		}
		return bson.M{
			"sync":     bson.M{"posProcessed": result.Sync.PosProcessed, "fbProcessed": result.Sync.FbProcessed},
			"backfill": bson.M{"ordersProcessed": result.Backfill.OrdersProcessed, "conversationsProcessed": result.Backfill.ConversationsProcessed, "notesProcessed": result.Backfill.NotesProcessed},
		}, nil

	case crmmodels.CrmBulkJobRecalculateOne:
		unifiedId, _ := getString(params, "unifiedId")
		if unifiedId == "" {
			return nil, nil
		}
		result, err := svc.RecalculateCustomerFromAllSources(ctx, unifiedId, job.OwnerOrganizationID)
		if err != nil {
			return nil, err
		}
		return bson.M{
			"unifiedId": result.UnifiedId,
			"updatedAt": result.UpdatedAt,
		}, nil

	case crmmodels.CrmBulkJobRecalculateAll:
		limit := parseInt(params, "limit", 0)
		result, err := svc.RecalculateAllCustomers(ctx, job.OwnerOrganizationID, limit)
		if err != nil {
			return nil, err
		}
		return bson.M{"totalProcessed": result.TotalProcessed, "totalFailed": result.TotalFailed, "failedIds": result.FailedIds}, nil

	default:
		return nil, nil
	}
}

func parseStringSlice(m bson.M, key string) []string {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	if arr, ok := v.(bson.A); ok {
		var out []string
		for _, a := range arr {
			if s, ok := a.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	if arr, ok := v.([]interface{}); ok {
		var out []string
		for _, a := range arr {
			if s, ok := a.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func parseInt(m bson.M, key string, defaultVal int) int {
	v, ok := m[key]
	if !ok || v == nil {
		return defaultVal
	}
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return defaultVal
}

func getString(m bson.M, key string) (string, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}
