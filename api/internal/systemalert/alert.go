// Package systemalert — gửi cảnh báo khi CPU, RAM, disk VPS quá tải cho team system.
// Sử dụng hệ thống thông báo (routing rules, templates, channels) đã có.
package systemalert

import (
	"context"
	"fmt"
	"os"
	"time"

	"meta_commerce/internal/cta"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/notifytrigger"
	"meta_commerce/internal/worker"
)

const (
	eventTypeSystemResourceOverload = "system_resource_overload"
)

// Register đăng ký callback gửi cảnh báo khi tài nguyên quá tải.
// Gọi trước khi worker.DefaultController().Start().
// Cảnh báo gửi qua hệ thống thông báo (routing rules → channels → delivery queue).
func Register() {
	worker.RegisterOverloadAlertCallback(sendOverloadAlert)
}

// sendOverloadAlert gửi thông báo qua hệ thống thông báo khi CPU/RAM/disk quá tải.
// Sử dụng eventType system_resource_overload, routing rules và templates đã init.
func sendOverloadAlert(metrics worker.ResourceMetrics, state string) {
	ctx := context.Background()
	systemOrgID, err := cta.GetSystemOrganizationID(ctx)
	if err != nil {
		logger.GetAppLogger().WithError(err).Error("⚙️ [SYSTEM_ALERT] Không lấy được System Organization ID")
		return
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "https://localhost"
	}

	payload := map[string]interface{}{
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"state":       state,
		"cpuPercent":  fmt.Sprintf("%.1f", metrics.CPUPercent),
		"ramPercent":  fmt.Sprintf("%.1f", metrics.RAMPercent),
		"diskPercent": fmt.Sprintf("%.1f", metrics.DiskPercent),
	}

	queued, err := notifytrigger.TriggerProgrammatic(ctx, eventTypeSystemResourceOverload, payload, systemOrgID, baseURL)
	if err != nil {
		logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
			"cpuPercent":  metrics.CPUPercent,
			"ramPercent":  metrics.RAMPercent,
			"diskPercent": metrics.DiskPercent,
			"state":       state,
		}).Error("⚙️ [SYSTEM_ALERT] Không gửi được cảnh báo qua hệ thống thông báo")
		return
	}

	if queued == 0 {
		logger.GetAppLogger().WithFields(map[string]interface{}{
			"eventType":   eventTypeSystemResourceOverload,
			"cpuPercent": metrics.CPUPercent,
			"ramPercent":  metrics.RAMPercent,
			"diskPercent": metrics.DiskPercent,
		}).Warn("⚙️ [SYSTEM_ALERT] Không có routing rule/channel nào cho system_resource_overload. Cần bật routing rule và cấu hình channels cho System Organization.")
		return
	}

	logger.GetAppLogger().WithFields(map[string]interface{}{
		"cpuPercent":  metrics.CPUPercent,
		"ramPercent":  metrics.RAMPercent,
		"diskPercent": metrics.DiskPercent,
		"state":       state,
		"queued":      queued,
	}).Info("⚙️ [SYSTEM_ALERT] Đã gửi cảnh báo tài nguyên quá tải qua hệ thống thông báo")
}
