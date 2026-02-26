// Package worker - ClassificationRefreshWorker tính lại phân loại khách hàng (lifecycle, journey, momentum...)
// theo chu kỳ. Chạy khi không có tác động (order/conversation) để tránh phân loại bị sai theo thời gian.
package worker

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	crmvc "meta_commerce/internal/api/crm/service"
	"meta_commerce/internal/logger"
)

// ClassificationRefreshMode chế độ chạy worker.
const (
	ClassificationRefreshModeFull  = "full"  // Tất cả khách có đơn
	ClassificationRefreshModeSmart = "smart" // Chỉ khách gần ngưỡng lifecycle
)

// ClassificationRefreshWorker worker tính lại phân loại khách hàng định kỳ.
//
// Hai chế độ:
//   - full: Duyệt tất cả khách có orderCount >= 1, gọi RefreshMetrics từng batch.
//     Dùng cho chạy hàng ngày (vd: 2h sáng) — đảm bảo toàn bộ phân loại cập nhật.
//   - smart: Chỉ xử lý khách có lastOrderAt trong vùng 28–33, 88–96, 178–186 ngày.
//     Giảm tải vì chỉ refresh khách gần ngưỡng active↔cooling, cooling↔inactive, inactive↔dead.
type ClassificationRefreshWorker struct {
	crmService *crmvc.CrmCustomerService
	interval   time.Duration // Khoảng thời gian giữa các lần chạy (vd: 24h)
	batchSize  int           // Số khách tối đa mỗi lần (vd: 200)
	mode       string        // "full" hoặc "smart"
}

// NewClassificationRefreshWorker tạo worker mới.
//
// Tham số:
//   - interval: Khoảng cách giữa các lần chạy (mặc định: 24h)
//   - batchSize: Số khách tối đa mỗi lần (mặc định: 200)
//   - mode: "full" hoặc "smart"
func NewClassificationRefreshWorker(interval time.Duration, batchSize int, mode string) (*ClassificationRefreshWorker, error) {
	crmService, err := crmvc.NewCrmCustomerService()
	if err != nil {
		return nil, err
	}
	if interval < time.Hour {
		interval = 24 * time.Hour
	}
	if batchSize <= 0 {
		batchSize = 200
	}
	if mode != ClassificationRefreshModeFull && mode != ClassificationRefreshModeSmart {
		mode = ClassificationRefreshModeSmart
	}
	return &ClassificationRefreshWorker{
		crmService: crmService,
		interval:   interval,
		batchSize:  batchSize,
		mode:       mode,
	}, nil
}

// Start chạy worker trong vòng lặp.
func (w *ClassificationRefreshWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.WithFields(map[string]interface{}{
		"interval":  w.interval.String(),
		"batchSize": w.batchSize,
		"mode":      w.mode,
	}).Info("📊 [CLASSIFICATION_REFRESH] Starting Classification Refresh Worker...")

	// Chạy ngay lần đầu sau 1 phút (tránh chạy lúc startup)
	time.Sleep(time.Minute)

	for {
		select {
		case <-ctx.Done():
			log.Info("📊 [CLASSIFICATION_REFRESH] Classification Refresh Worker stopped")
			return
		case <-ticker.C:
			w.runBatch(ctx, log)
		}
	}
}

// runBatch chạy một đợt refresh: lấy batch khách → RefreshMetrics từng người.
func (w *ClassificationRefreshWorker) runBatch(ctx context.Context, log *logrus.Logger) {
	defer func() {
		if r := recover(); r != nil {
			log.WithFields(map[string]interface{}{
				"panic": r,
			}).Error("📊 [CLASSIFICATION_REFRESH] Panic khi xử lý, sẽ tiếp tục lần chạy tiếp theo")
		}
	}()

	skip := 0
	totalProcessed := 0

	for {
		list, err := w.crmService.ListCustomerIdsForClassificationRefresh(ctx, w.mode, w.batchSize, skip)
		if err != nil {
			log.WithError(err).Error("📊 [CLASSIFICATION_REFRESH] Lỗi lấy danh sách khách cần refresh")
			return
		}
		if len(list) == 0 {
			break
		}

		processed := 0
		for _, c := range list {
			if err := w.crmService.RefreshMetrics(ctx, c.UnifiedId, c.OwnerOrgID); err != nil {
				log.WithError(err).WithFields(map[string]interface{}{
					"unifiedId":  c.UnifiedId,
					"ownerOrgId": c.OwnerOrgID.Hex(),
				}).Warn("📊 [CLASSIFICATION_REFRESH] RefreshMetrics thất bại, bỏ qua")
				continue
			}
			processed++
		}
		totalProcessed += processed

		if processed > 0 {
			log.WithFields(map[string]interface{}{
				"batchProcessed": processed,
				"batchSize":      len(list),
				"totalProcessed": totalProcessed,
			}).Info("📊 [CLASSIFICATION_REFRESH] Đã cập nhật phân loại khách hàng")
		}

		// Chế độ smart: chỉ 1 batch (ít khách gần ngưỡng). Full: tiếp tục đến hết.
		if w.mode == ClassificationRefreshModeSmart || len(list) < w.batchSize {
			break
		}
		skip += w.batchSize
	}
}
