package basehdl

import (
	"context"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"time"

	"github.com/gofiber/fiber/v3"
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

// HandleGetWorkerConfig trả về cấu hình worker hiện tại (ngưỡng throttle + priorities + active + state).
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
	metrics := ctrl.GetResourceMetrics()
	return c.Status(common.StatusOK).JSON(fiber.Map{
		"code":    common.StatusOK,
		"message": "Thành công",
		"data": fiber.Map{
			"thresholds":           thresholds,
			"priorities":           priorities,            // Override (rỗng nếu chưa chỉnh)
			"workerPriorities":     workerPriorities,      // Mức ưu tiên hiệu dụng tất cả workers
			"workerActive":         workerActive,          // Trạng thái active hiệu dụng tất cả workers
			"workerActiveOverrides": workerActiveOverrides, // Override active (rỗng nếu chưa chỉnh)
			"workerMetadata":       workerMetadata,        // Mô tả từng worker (module, description)
			"state":                string(state),
			"cpuPercent":           cpuPct,
			"ramPercent":           metrics.RAMPercent,
			"diskPercent":          metrics.DiskPercent,
		},
		"status": "success",
	})
}

// HandleUpdateWorkerConfig cập nhật cấu hình worker (ngưỡng + priority + active overrides).
// PUT /api/v1/system/worker-config
// Body: { "thresholds": {...}, "priorities": {"crm_bulk": 3}, "workerActive": {"crm_bulk": false} }
func (h *SystemHandler) HandleUpdateWorkerConfig(c fiber.Ctx) error {
	var body struct {
		Thresholds   *worker.WorkerThresholds `json:"thresholds"`
		Priorities   map[string]int           `json:"priorities"`
		WorkerActive map[string]bool          `json:"workerActive"`
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
	return c.Status(common.StatusOK).JSON(fiber.Map{
		"code":    common.StatusOK,
		"message": "Đã cập nhật cấu hình worker",
		"data":    nil,
		"status":  "success",
	})
}

