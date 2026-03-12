// Package worker — AdsExecutionWorker xử lý action_pending_approval status=queued (domain ads).
// Sau khi approve, lệnh được đưa vào queue; worker thực thi qua Meta API với cơ chế retry.
// Tách package riêng để tránh import cycle (approval -> notifytrigger -> delivery -> worker).
package worker

import (
	"context"
	"math"
	"sync"
	"time"

	adssvc "meta_commerce/internal/api/ads/service"
	"meta_commerce/internal/approval"
	pkgapproval "meta_commerce/pkg/approval"
	"meta_commerce/internal/logger"
	coreworker "meta_commerce/internal/worker"
)

const (
	domainAds = "ads"
)

// AdsExecutionWorker worker xử lý ads execution queue: poll status=queued, execute với retry.
type AdsExecutionWorker struct {
	interval  time.Duration
	batchSize int
}

// NewAdsExecutionWorker tạo mới AdsExecutionWorker.
func NewAdsExecutionWorker(interval time.Duration, batchSize int) *AdsExecutionWorker {
	if interval < 15*time.Second {
		interval = 30 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 10
	}
	return &AdsExecutionWorker{interval: interval, batchSize: batchSize}
}

// Start chạy worker trong vòng lặp. Tích hợp Worker Controller để throttle khi CPU/RAM quá tải.
func (w *AdsExecutionWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.WithFields(map[string]interface{}{
		"interval":  w.interval.String(),
		"batchSize": w.batchSize,
	}).Info("📢 [ADS_EXECUTION] Starting Ads Execution Worker...")

	for {
		select {
		case <-ctx.Done():
			log.Info("📢 [ADS_EXECUTION] Ads Execution Worker stopped")
			return
		case <-ticker.C:
			if !coreworker.IsWorkerActive(coreworker.WorkerAdsExecution) {
				time.Sleep(1 * time.Minute)
				continue
			}
			p := coreworker.GetPriority(coreworker.WorkerAdsExecution, coreworker.PriorityNormal)
			if coreworker.ShouldThrottle(p) {
				continue
			}
			if effInterval := coreworker.GetEffectiveInterval(w.interval, p); effInterval > w.interval {
				time.Sleep(effInterval - w.interval)
			}
			w.processBatch(ctx)
		}
	}
}

// processBatch xử lý một batch items từ queue. Dùng worker pool khi poolSize > 1.
func (w *AdsExecutionWorker) processBatch(ctx context.Context) {
	log := logger.GetAppLogger()
	defer func() {
		if r := recover(); r != nil {
			log.WithFields(map[string]interface{}{"panic": r}).Error("📢 [ADS_EXECUTION] Panic khi xử lý, sẽ tiếp tục lần sau")
		}
	}()

	prio := coreworker.GetPriority(coreworker.WorkerAdsExecution, coreworker.PriorityNormal)
	batchSize := coreworker.GetEffectiveBatchSize(w.batchSize, prio)
	list, err := approval.FindQueued(ctx, domainAds, batchSize)
	if err != nil {
		log.WithError(err).Error("📢 [ADS_EXECUTION] Lỗi lấy danh sách queued")
		return
	}
	if len(list) == 0 {
		return
	}

	basePool := coreworker.GetPoolSize(coreworker.WorkerAdsExecution, 4)
	poolSize := coreworker.GetEffectivePoolSize(basePool, prio)
	if poolSize <= 1 {
		for i := range list {
			w.processOne(ctx, &list[i])
		}
		return
	}

	// Worker pool: xử lý song song
	jobs := make(chan *pkgapproval.ActionPending, len(list))
	var wg sync.WaitGroup
	for i := 0; i < poolSize; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for doc := range jobs {
				w.processOne(ctx, doc)
			}
		}()
	}
	for i := range list {
		jobs <- &list[i]
	}
	close(jobs)
	wg.Wait()
}

// processOne xử lý một item: execute, cập nhật kết quả hoặc retry.
func (w *AdsExecutionWorker) processOne(ctx context.Context, doc *pkgapproval.ActionPending) {
	log := logger.GetAppLogger()
	now := time.Now().UnixMilli()
	doc.UpdatedAt = now

	resp, execErr := adssvc.ExecuteAdsAction(ctx, doc)
	if execErr == nil {
		// Thành công: cập nhật executed
		doc.Status = pkgapproval.StatusExecuted
		doc.ExecuteResponse = resp
		doc.ExecutedAt = now
		doc.ExecuteError = ""
		doc.NextRetryAt = nil
		if err := approval.Update(ctx, doc); err != nil {
			log.WithError(err).WithFields(map[string]interface{}{
				"actionId": doc.ID.Hex(),
			}).Error("📢 [ADS_EXECUTION] Lỗi cập nhật kết quả executed")
			return
		}
		approval.NotifyExecuted(ctx, doc)
		// B1 Counterfactual: snapshot khi kill campaign (PAUSE/KILL với rule kill)
		_ = adssvc.SaveKillSnapshotIfKill(ctx, doc)
		log.WithFields(map[string]interface{}{
			"actionId":   doc.ID.Hex(),
			"actionType": doc.ActionType,
		}).Info("📢 [ADS_EXECUTION] Đã thực thi thành công")
		return
	}

	// Thất bại: retry hoặc đánh dấu failed
	doc.RetryCount++
	if doc.MaxRetries <= 0 {
		doc.MaxRetries = pkgapproval.MaxRetriesDefault
	}

	if doc.RetryCount < doc.MaxRetries {
		// Chưa hết retry: lên lịch retry với exponential backoff
		backoffSec := int64(math.Pow(2, float64(doc.RetryCount)))
		if backoffSec > 3600 {
			backoffSec = 3600 // Tối đa 1 giờ
		}
		nextRetryAt := time.Now().Unix() + backoffSec
		doc.NextRetryAt = &nextRetryAt
		doc.ExecuteError = execErr.Error()
		doc.ExecuteResponse = map[string]interface{}{"error": execErr.Error()}
		if err := approval.Update(ctx, doc); err != nil {
			log.WithError(err).WithFields(map[string]interface{}{
				"actionId": doc.ID.Hex(),
			}).Error("📢 [ADS_EXECUTION] Lỗi cập nhật retry")
			return
		}
		log.WithError(execErr).WithFields(map[string]interface{}{
			"actionId":    doc.ID.Hex(),
			"retryCount":  doc.RetryCount,
			"nextRetryAt": nextRetryAt,
		}).Warn("📢 [ADS_EXECUTION] Thực thi thất bại, sẽ retry")
	} else {
		// Đã hết retry: đánh dấu failed và gửi thông báo
		doc.Status = pkgapproval.StatusFailed
		doc.ExecuteError = execErr.Error()
		doc.ExecuteResponse = map[string]interface{}{"error": execErr.Error()}
		doc.ExecutedAt = now
		doc.NextRetryAt = nil
		if err := approval.Update(ctx, doc); err != nil {
			log.WithError(err).WithFields(map[string]interface{}{
				"actionId": doc.ID.Hex(),
			}).Error("📢 [ADS_EXECUTION] Lỗi đánh dấu failed")
			return
		}
		approval.NotifyFailed(ctx, doc)
		log.WithError(execErr).WithFields(map[string]interface{}{
			"actionId":   doc.ID.Hex(),
			"retryCount": doc.RetryCount,
		}).Error("📢 [ADS_EXECUTION] Đã hết retry, đánh dấu failed")
	}
}
