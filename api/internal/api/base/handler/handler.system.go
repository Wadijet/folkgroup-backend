package basehdl

import (
	"context"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"time"

	"github.com/gofiber/fiber/v3"
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

