package basehdl

import (
	"context"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"time"

	"github.com/gofiber/fiber/v3"
	reportsvc "meta_commerce/internal/api/report/service"
	"meta_commerce/internal/worker"
	"meta_commerce/internal/worker/metrics"
)

// SystemHandler xử lý các route liên quan đến system operations
type SystemHandler struct {
	*BaseHandler[interface{}, interface{}, interface{}]
}

// NewSystemHandler tạo một instance mới của SystemHandler
func NewSystemHandler() (*SystemHandler, error) {
	baseHandler := &BaseHandler[interface{}, interface{}, interface{}]{}
	handler := &SystemHandler{
		BaseHandler: baseHandler,
	}
	return handler, nil
}

// HandleHealth kiểm tra tình trạng hệ thống
// @Summary Kiểm tra tình trạng hệ thống
// @Description Kiểm tra trạng thái của API và database connection
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Hệ thống hoạt động bình thường"
// @Failure 503 {object} map[string]interface{} "Hệ thống đang gặp sự cố"
// @Router /system/health [get]
func (h *SystemHandler) HandleHealth(c fiber.Ctx) error {
	// Kiểm tra database connection
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	healthData := fiber.Map{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"services": fiber.Map{
			"api": "ok",
		},
	}

	// Kiểm tra MongoDB connection
	if global.MongoDB_Session != nil {
		err := global.MongoDB_Session.Ping(ctx, nil)
		if err != nil {
			healthData["status"] = "degraded"
			healthData["services"].(fiber.Map)["database"] = "error"
			healthData["database_error"] = err.Error()
			// Trả về format chuẩn với status code 503
			return c.Status(common.StatusServiceUnavailable).JSON(fiber.Map{
				"code":    common.StatusServiceUnavailable,
				"message": "Hệ thống đang gặp sự cố",
				"data":    healthData,
				"status":  "error",
			})
		}
		healthData["services"].(fiber.Map)["database"] = "ok"
	} else {
		healthData["status"] = "degraded"
		healthData["services"].(fiber.Map)["database"] = "not_initialized"
	}

	// Trả về format chuẩn
	return c.Status(common.StatusOK).JSON(fiber.Map{
		"code":    common.StatusOK,
		"message": common.MsgSuccess,
		"data":    healthData,
		"status":  "success",
	})
}

// HandleJobMetrics trả về metrics thời gian thực hiện từng loại job (avgMs, sampleCount, countLastHour).
func (h *SystemHandler) HandleJobMetrics(c fiber.Ctx) error {
	data := metrics.GetAll()
	return c.Status(common.StatusOK).JSON(fiber.Map{
		"code":    common.StatusOK,
		"message": "Thành công",
		"data":    data,
		"status":  "success",
	})
}

// HandleGetWorkerConfig trả về cấu hình worker hiện tại (ngưỡng throttle + priorities + active + report schedules + state).
// GET /api/v1/system/worker-config
func (h *SystemHandler) HandleGetWorkerConfig(c fiber.Ctx) error {
	ctrl := worker.DefaultController()
	state, cpuPct := ctrl.GetState()
	thresholds := ctrl.GetThresholds()
	priorities := worker.GetPriorityOverrides()            // Override qua API (chỉ các worker đã chỉnh)
	workerPriorities := worker.GetAllEffectivePriorities()  // Mức ưu tiên hiệu dụng từng worker (1–5)
	workerActive := worker.GetAllWorkerActive()             // Trạng thái active/inactive từng worker
	workerActiveOverrides := worker.GetWorkerActiveOverrides() // Override active (chỉ các worker đã chỉnh)
	workerMetadata := worker.GetAllWorkerMetadata()         // Mô tả từng worker (module, description)
	reportSchedules := buildReportSchedulesResponse()       // Lịch chạy report (ads, order, customer) — hiệu dụng
	reportScheduleOverrides := reportsvc.GetReportScheduleOverrides() // Override lịch report (chỉ domain đã chỉnh qua API)
	workerSchedules := worker.GetAllWorkerSchedules()
	workerScheduleOverrides := worker.GetWorkerScheduleOverrides()
	workerPoolSizes := worker.GetAllWorkerPoolSizes()
	workerPoolSizeOverrides := worker.GetPoolSizeOverrides()
	workerRetentions := worker.GetAllWorkerRetentions()
	workerRetentionOverrides := worker.GetWorkerRetentionOverrides()
	alertWebhookURL := worker.GetAlertWebhookURL()
	metrics := ctrl.GetResourceMetrics()
	return c.Status(common.StatusOK).JSON(fiber.Map{
		"code":    common.StatusOK,
		"message": "Thành công",
		"data": fiber.Map{
			"thresholds":              thresholds,
			"priorities":              priorities,
			"workerPriorities":        workerPriorities,
			"workerActive":            workerActive,
			"workerActiveOverrides":   workerActiveOverrides,
			"workerMetadata":          workerMetadata,
			"reportSchedules":         reportSchedules,
			"reportScheduleOverrides": reportScheduleOverrides,
			"workerSchedules":         workerSchedules,
			"workerScheduleOverrides": workerScheduleOverrides,
			"workerPoolSizes":         workerPoolSizes,
			"workerPoolSizeOverrides": workerPoolSizeOverrides,
			"workerRetentions":        workerRetentions,
			"workerRetentionOverrides": workerRetentionOverrides,
			"alertWebhookURL":         alertWebhookURL,
			"state":                   string(state),
			"cpuPercent":              cpuPct,
			"ramPercent":               metrics.RAMPercent,
			"diskPercent":              metrics.DiskPercent,
		},
		"status": "success",
	})
}

// buildReportSchedulesResponse trả về map domain → {interval, batchSize} hiệu dụng (để API GET).
func buildReportSchedulesResponse() map[string]map[string]interface{} {
	configs := reportsvc.GetReportScheduleConfigs()
	out := make(map[string]map[string]interface{}, len(configs))
	for _, c := range configs {
		out[c.Name] = map[string]interface{}{
			"interval":  c.Interval.String(),
			"batchSize": c.BatchSize,
		}
	}
	return out
}

// HandleUpdateWorkerConfig cập nhật cấu hình worker (ngưỡng + priority + active + report schedules).
// PUT /api/v1/system/worker-config
// Body: { "thresholds": {...}, "priorities": {...}, "workerActive": {...}, "reportSchedules": {"ads": {"interval": "15m", "batchSize": 20}} }
func (h *SystemHandler) HandleUpdateWorkerConfig(c fiber.Ctx) error {
	var body struct {
		Thresholds      *worker.WorkerThresholds       `json:"thresholds"`
		Priorities      map[string]int                 `json:"priorities"`
		WorkerActive    map[string]bool                `json:"workerActive"`
		ReportSchedules      map[string]ReportScheduleInput `json:"reportSchedules"`
		ReportSchedulesClear  []string                       `json:"reportSchedulesClear"`
		WorkerSchedules        map[string]ReportScheduleInput `json:"workerSchedules"`
		WorkerSchedulesClear   []string                       `json:"workerSchedulesClear"`
		WorkerPoolSizes        map[string]int                 `json:"workerPoolSizes"`
		WorkerPoolSizesClear  []string                       `json:"workerPoolSizesClear"`
		WorkerRetentions       map[string]int64               `json:"workerRetentions"`
		WorkerRetentionsClear  []string                       `json:"workerRetentionsClear"`
		AlertWebhookURL        *string                        `json:"alertWebhookURL"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code":    common.ErrCodeValidationFormat.Code,
			"message": "Dữ liệu gửi lên không đúng định dạng JSON",
			"status":  "error",
		})
	}
	if body.Thresholds != nil {
		worker.DefaultController().SetThresholds(body.Thresholds)
	}
	if len(body.Priorities) > 0 {
		for name, p := range body.Priorities {
			worker.SetPriorityOverride(name, worker.Priority(p))
		}
	}
	if len(body.WorkerActive) > 0 {
		for name, active := range body.WorkerActive {
			worker.SetWorkerActiveOverride(name, active)
		}
	}
	if len(body.ReportSchedules) > 0 {
		for domain, s := range body.ReportSchedules {
			if s.Interval != "" || s.BatchSize > 0 {
				reportsvc.SetReportScheduleOverride(domain, s.Interval, s.BatchSize)
			}
		}
	}
	for _, domain := range body.ReportSchedulesClear {
		reportsvc.ClearReportScheduleOverride(domain)
	}
	reportWorkerToDomain := map[string]string{
		"report_dirty_ads":     "ads",
		"report_dirty_order":   "order",
		"report_dirty_customer": "customer",
	}
	if len(body.WorkerSchedules) > 0 {
		for workerName, s := range body.WorkerSchedules {
			if s.Interval != "" || s.BatchSize > 0 {
				if domain, ok := reportWorkerToDomain[workerName]; ok {
					reportsvc.SetReportScheduleOverride(domain, s.Interval, s.BatchSize)
				} else {
					worker.SetWorkerScheduleOverride(workerName, s.Interval, s.BatchSize)
				}
			}
		}
	}
	for _, workerName := range body.WorkerSchedulesClear {
		if domain, ok := reportWorkerToDomain[workerName]; ok {
			reportsvc.ClearReportScheduleOverride(domain)
		} else {
			worker.ClearWorkerScheduleOverride(workerName)
		}
	}
	if len(body.WorkerPoolSizes) > 0 {
		for name, size := range body.WorkerPoolSizes {
			if size >= 1 {
				worker.SetPoolSizeOverride(name, size)
			}
		}
	}
	for _, name := range body.WorkerPoolSizesClear {
		worker.ClearPoolSizeOverride(name)
	}
	if len(body.WorkerRetentions) > 0 {
		for name, days := range body.WorkerRetentions {
			if days >= 1 {
				worker.SetWorkerRetentionOverride(name, days)
			}
		}
	}
	for _, name := range body.WorkerRetentionsClear {
		worker.ClearWorkerRetentionOverride(name)
	}
	if body.AlertWebhookURL != nil {
		worker.SetAlertWebhookURL(*body.AlertWebhookURL)
	}
	return c.Status(common.StatusOK).JSON(fiber.Map{
		"code":    common.StatusOK,
		"message": "Đã cập nhật cấu hình worker",
		"data":    nil,
		"status":  "success",
	})
}

// ReportScheduleInput body cho cập nhật lịch report qua API.
type ReportScheduleInput struct {
	Interval  string `json:"interval"`  // duration: "2m", "15m", "24h"
	BatchSize int    `json:"batchSize"` // 0 = không đổi
}

