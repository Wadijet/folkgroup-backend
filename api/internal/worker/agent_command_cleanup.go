package worker

import (
	"context"
	"time"

	agentsvc "meta_commerce/internal/api/agent/service"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/worker/metrics"
)

// AgentCommandCleanupWorker worker để tự động giải phóng các agent commands bị stuck
// Chạy định kỳ để release các commands quá lâu không có heartbeat
type AgentCommandCleanupWorker struct {
	commandService *agentsvc.AgentCommandService
	interval       time.Duration // Khoảng thời gian giữa các lần chạy
	timeoutSeconds int64         // Timeout để coi command là stuck (giây)
}

// NewAgentCommandCleanupWorker tạo mới AgentCommandCleanupWorker
// Tham số:
//   - interval: Khoảng thời gian giữa các lần chạy (mặc định: 1 phút)
//   - timeoutSeconds: Timeout để coi command là stuck (mặc định: 300 giây = 5 phút)
//
// Trả về:
//   - *AgentCommandCleanupWorker: Instance mới của AgentCommandCleanupWorker
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAgentCommandCleanupWorker(interval time.Duration, timeoutSeconds int64) (*AgentCommandCleanupWorker, error) {
	commandService, err := agentsvc.NewAgentCommandService()
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

	return &AgentCommandCleanupWorker{
		commandService: commandService,
		interval:       interval,
		timeoutSeconds: timeoutSeconds,
	}, nil
}

// Start bắt đầu background worker để release stuck commands
// Worker sẽ chạy định kỳ theo interval và tự động giải phóng commands bị stuck
func (w *AgentCommandCleanupWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.WithFields(map[string]interface{}{
		"interval":       w.interval.String(),
		"timeoutSeconds": w.timeoutSeconds,
	}).Info("🔄 [AGENT_COMMAND_CLEANUP] Starting Agent Command Cleanup Worker...")

	for {
		select {
		case <-ctx.Done():
			log.Info("🔄 [AGENT_COMMAND_CLEANUP] Agent Command Cleanup Worker stopped")
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
						}).Error("🔄 [AGENT_COMMAND_CLEANUP] Panic khi release stuck commands, sẽ tiếp tục ở lần chạy tiếp theo")
					}
				}()

				// Gọi service để release stuck commands
				start := time.Now()
				releasedCount, err := w.commandService.ReleaseStuckCommands(ctx, w.timeoutSeconds)
				metrics.RecordDuration("agent_command_cleanup", time.Since(start))
				if err != nil {
					log.WithError(err).Error("🔄 [AGENT_COMMAND_CLEANUP] Failed to release stuck commands")
					return
				}

				if releasedCount > 0 {
					log.WithFields(map[string]interface{}{
						"releasedCount":   releasedCount,
						"timeoutSeconds":  w.timeoutSeconds,
					}).Info("🔄 [AGENT_COMMAND_CLEANUP] Released stuck commands")
				}
				// Nếu releasedCount = 0, không log (giảm log noise)
			}()
		}
	}
}
