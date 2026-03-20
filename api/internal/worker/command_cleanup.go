package worker

import (
	"context"
	"time"

	aisvc "meta_commerce/internal/api/ai/service"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/worker/metrics"
)

// CommandCleanupWorker worker để tự động giải phóng các commands bị stuck
// Chạy định kỳ để release các commands quá lâu không có heartbeat
type CommandCleanupWorker struct {
	commandService *aisvc.AIWorkflowCommandService
	interval       time.Duration // Khoảng thời gian giữa các lần chạy
	timeoutSeconds int64         // Timeout để coi command là stuck (giây)
}

// NewCommandCleanupWorker tạo mới CommandCleanupWorker
// Tham số:
//   - interval: Khoảng thời gian giữa các lần chạy (mặc định: 1 phút)
//   - timeoutSeconds: Timeout để coi command là stuck (mặc định: 300 giây = 5 phút)
//
// Trả về:
//   - *CommandCleanupWorker: Instance mới của CommandCleanupWorker
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewCommandCleanupWorker(interval time.Duration, timeoutSeconds int64) (*CommandCleanupWorker, error) {
	commandService, err := aisvc.NewAIWorkflowCommandService()
	if err != nil {
		return nil, err
	}

	// Set defaults
	if interval < 30*time.Second {
		interval = 1 * time.Minute // Mặc định 1 phút
	}
	if timeoutSeconds < 60 {
		timeoutSeconds = 300 // Mặc định 5 phút
	}

	return &CommandCleanupWorker{
		commandService: commandService,
		interval:       interval,
		timeoutSeconds: timeoutSeconds,
	}, nil
}

// Start bắt đầu background worker để release stuck commands. Đọc config mỗi vòng (hỗ trợ thay đổi qua API).
func (w *CommandCleanupWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()

	log.WithFields(map[string]interface{}{
		"interval":       w.interval.String(),
		"timeoutSeconds": w.timeoutSeconds,
	}).Info("🔄 [COMMAND_CLEANUP] Starting Command Cleanup Worker...")

	for {
		interval, timeoutBatch := GetEffectiveWorkerSchedule(WorkerCommandCleanup, w.interval, int(w.timeoutSeconds))
		timeoutSeconds := int64(timeoutBatch)
		if timeoutSeconds < 60 {
			timeoutSeconds = 300
		}

		if !IsWorkerActive(WorkerCommandCleanup) {
			select {
			case <-ctx.Done():
				log.Info("🔄 [COMMAND_CLEANUP] Command Cleanup Worker stopped")
				return
			case <-time.After(1 * time.Minute):
			}
			continue
		}
		p := GetPriority(WorkerCommandCleanup, PriorityLow)
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
			log.Info("🔄 [COMMAND_CLEANUP] Command Cleanup Worker stopped")
			return
		case <-time.After(interval):
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithFields(map[string]interface{}{"panic": r}).Error("🔄 [COMMAND_CLEANUP] Panic khi release stuck commands, sẽ tiếp tục ở lần chạy tiếp theo")
				}
			}()

			start := time.Now()
			releasedCount, err := w.commandService.ReleaseStuckCommands(ctx, timeoutSeconds)
			metrics.RecordDuration("command_cleanup", time.Since(start))
			if err != nil {
				log.WithError(err).Error("🔄 [COMMAND_CLEANUP] Failed to release stuck commands")
				return
			}
			if releasedCount > 0 {
				log.WithFields(map[string]interface{}{
					"releasedCount":  releasedCount,
					"timeoutSeconds": timeoutSeconds,
				}).Info("🔄 [COMMAND_CLEANUP] Released stuck commands")
			}
		}()
	}
}
