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

// Start bắt đầu background worker để release stuck commands
// Worker sẽ chạy định kỳ theo interval và tự động giải phóng commands bị stuck
func (w *CommandCleanupWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.WithFields(map[string]interface{}{
		"interval":       w.interval.String(),
		"timeoutSeconds": w.timeoutSeconds,
	}).Info("🔄 [COMMAND_CLEANUP] Starting Command Cleanup Worker...")

	for {
		select {
		case <-ctx.Done():
			log.Info("🔄 [COMMAND_CLEANUP] Command Cleanup Worker stopped")
			return
		case <-ticker.C:
			if ShouldThrottle(PriorityLow) {
				continue
			}
			if effInterval := GetEffectiveInterval(w.interval, PriorityLow); effInterval > w.interval {
				time.Sleep(effInterval - w.interval)
			}
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.WithFields(map[string]interface{}{
							"panic": r,
						}).Error("🔄 [COMMAND_CLEANUP] Panic khi release stuck commands, sẽ tiếp tục ở lần chạy tiếp theo")
					}
				}()

				// Gọi service để release stuck commands
				start := time.Now()
				releasedCount, err := w.commandService.ReleaseStuckCommands(ctx, w.timeoutSeconds)
				metrics.RecordDuration("command_cleanup", time.Since(start))
				if err != nil {
					log.WithError(err).Error("🔄 [COMMAND_CLEANUP] Failed to release stuck commands")
					return
				}

				if releasedCount > 0 {
					log.WithFields(map[string]interface{}{
						"releasedCount":   releasedCount,
						"timeoutSeconds":  w.timeoutSeconds,
					}).Info("🔄 [COMMAND_CLEANUP] Released stuck commands")
				}
				// Nếu releasedCount = 0, không log (giảm log noise)
			}()
		}
	}
}
