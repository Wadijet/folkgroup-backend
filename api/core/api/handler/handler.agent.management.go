package handler

import (
	"fmt"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"
	"meta_commerce/core/logger"

	"github.com/gofiber/fiber/v3"
)

// AgentManagementHandler xử lý các route cho agent management system (bot management)
// Khác với AgentHandler (đại lý), handler này quản lý các bot agents
type AgentManagementHandler struct {
	managementService *services.AgentManagementService
}

// NewAgentManagementHandler tạo mới AgentManagementHandler
// Returns:
//   - *AgentManagementHandler: Instance mới của AgentManagementHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAgentManagementHandler() (*AgentManagementHandler, error) {
	managementService, err := services.NewAgentManagementService()
	if err != nil {
		return nil, fmt.Errorf("failed to create agent management service: %w", err)
	}

	return &AgentManagementHandler{
		managementService: managementService,
	}, nil
}

// HandleEnhancedCheckIn xử lý enhanced check-in từ bot
// Endpoint: POST /api/v1/agent/check-in
// Bot gửi thông tin chi tiết về trạng thái, metrics, job status, config
// Server trả về commands và config updates (nếu có)
// Parameters:
//   - c: Fiber context chứa request body với check-in data
// Returns:
//   - error: Lỗi nếu có trong quá trình xử lý
func (h *AgentManagementHandler) HandleEnhancedCheckIn(c fiber.Ctx) error {
	return SafeHandlerWrapper(c, func() error {
		// 1. Parse request body
		var checkInData map[string]interface{}
		if err := c.Bind().Body(&checkInData); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": "Dữ liệu gửi lên không đúng định dạng JSON",
				"status":  "error",
			})
			return nil
		}

		// 2. Validate agentId (bắt buộc)
		agentId, ok := checkInData["agentId"].(string)
		if !ok || agentId == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationInput.Code,
				"message": "agentId là bắt buộc và phải là string",
				"status":  "error",
			})
			return nil
		}

		// 3. Call service
		log := logger.GetAppLogger()
		log.WithFields(map[string]interface{}{
			"agentId": agentId,
		}).Info("🤖 [AGENT] Nhận check-in từ bot")

		response, err := h.managementService.HandleEnhancedCheckIn(c.Context(), agentId, checkInData)
		if err != nil {
			log.WithError(err).WithField("agentId", agentId).Error("🤖 [AGENT] Lỗi khi xử lý check-in")
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code":    common.ErrCodeBusinessOperation.Code,
				"message": fmt.Sprintf("Không thể xử lý check-in: %v", err),
				"status":  "error",
			})
			return nil
		}

		// 4. Return response
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code":    common.StatusOK,
			"message": "Check-in thành công",
			"data":    response,
			"status":  "success",
		})
		return nil
	})
}

