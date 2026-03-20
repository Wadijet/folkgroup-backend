package worker

import (
	"context"
	"time"

	agentsvc "meta_commerce/internal/api/agent/service"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/worker/metrics"
)

// AgentActivityCleanupWorker worker xóa định kỳ các agent activity logs cũ.
// Dữ liệu activity log không cần lưu lâu, chỉ dùng cho debug/monitor gần thời gian thực.
type AgentActivityCleanupWorker struct {
	activityService *agentsvc.AgentActivityService
	interval        time.Duration // Khoảng thời gian giữa các lần chạy
	retentionDays   int64         // Số ngày giữ lại (xóa logs cũ hơn)
}

// NewAgentActivityCleanupWorker tạo mới AgentActivityCleanupWorker.
// Tham số:
//   - interval: Khoảng thời gian giữa các lần chạy (mặc định: 1 giờ)
//   - retentionDays: Số ngày giữ lại logs (mặc định: 1 ngày)
//
// Trả về:
//   - *AgentActivityCleanupWorker: Instance mới
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAgentActivityCleanupWorker(interval time.Duration, retentionDays int64) (*AgentActivityCleanupWorker, error) {
	activityService, err := agentsvc.NewAgentActivityService()
	if err != nil {
		return nil, err
	}

	if interval < 5*time.Minute {
		interval = 1 * time.Hour
	}
	if retentionDays < 1 {
		retentionDays = 1
	}

	return &AgentActivityCleanupWorker{
		activityService: activityService,
		interval:        interval,
		retentionDays:   retentionDays,
	}, nil
}

// Start bắt đầu background worker xóa activity logs cũ. Đọc config mỗi vòng (hỗ trợ thay đổi qua API).
func (w *AgentActivityCleanupWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()

	log.WithFields(map[string]interface{}{
		"interval":      w.interval.String(),
		"retentionDays": w.retentionDays,
	}).Info("🗑️ [AGENT_ACTIVITY_CLEANUP] Starting Agent Activity Cleanup Worker...")

	for {
		interval, _ := GetEffectiveWorkerSchedule(WorkerAgentActivityCleanup, w.interval, 0)
		retentionDays := GetEffectiveWorkerRetention(WorkerAgentActivityCleanup, w.retentionDays)

		if !IsWorkerActive(WorkerAgentActivityCleanup) {
			select {
			case <-ctx.Done():
				log.Info("🗑️ [AGENT_ACTIVITY_CLEANUP] Agent Activity Cleanup Worker stopped")
				return
			case <-time.After(1 * time.Minute):
			}
			continue
		}
		p := GetPriority(WorkerAgentActivityCleanup, PriorityLow)
		if ShouldThrottle(p) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
			}
			continue
		}
		if effInterval := GetEffectiveInterval(interval, p); effInterval > interval {
			select {
			case <-ctx.Done():
				return
			case <-time.After(effInterval - interval):
			}
		}

		select {
		case <-ctx.Done():
			log.Info("🗑️ [AGENT_ACTIVITY_CLEANUP] Agent Activity Cleanup Worker stopped")
			return
		case <-time.After(interval):
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithFields(map[string]interface{}{"panic": r}).Error("🗑️ [AGENT_ACTIVITY_CLEANUP] Panic khi xóa activity logs, sẽ tiếp tục ở lần chạy tiếp theo")
				}
			}()

			cutoff := time.Now().AddDate(0, 0, -int(retentionDays)).UnixMilli()
			start := time.Now()
			deletedCount, err := w.activityService.DeleteOlderThan(ctx, cutoff)
			metrics.RecordDuration("agent_activity_cleanup", time.Since(start))
			if err != nil {
				log.WithError(err).Error("🗑️ [AGENT_ACTIVITY_CLEANUP] Failed to delete old activity logs")
				return
			}
			if deletedCount > 0 {
				log.WithFields(map[string]interface{}{
					"deletedCount":   deletedCount,
					"retentionDays": retentionDays,
				}).Info("🗑️ [AGENT_ACTIVITY_CLEANUP] Đã xóa activity logs cũ")
			}
		}()
	}
}
