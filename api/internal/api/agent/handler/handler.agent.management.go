package agenthdl

import (
	"fmt"
	agentsvc "meta_commerce/internal/api/agent/service"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/common"
	"meta_commerce/internal/logger"

	"github.com/gofiber/fiber/v3"
)

// AgentManagementHandler xử lý các route cho agent management system (bot management)
type AgentManagementHandler struct {
	managementService *agentsvc.AgentManagementService
}

// NewAgentManagementHandler tạo mới AgentManagementHandler
func NewAgentManagementHandler() (*AgentManagementHandler, error) {
	managementService, err := agentsvc.NewAgentManagementService()
	if err != nil {
		return nil, fmt.Errorf("failed to create agent management service: %w", err)
	}

	return &AgentManagementHandler{
		managementService: managementService,
	}, nil
}

// HandleEnhancedCheckIn xử lý enhanced check-in từ bot
func (h *AgentManagementHandler) HandleEnhancedCheckIn(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var checkInData map[string]interface{}
		if err := c.Bind().Body(&checkInData); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": "Dữ liệu gửi lên không đúng định dạng JSON",
				"status":  "error",
			})
			return nil
		}

		agentId, ok := checkInData["agentId"].(string)
		if !ok || agentId == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationInput.Code,
				"message": "agentId là bắt buộc và phải là string",
				"status":  "error",
			})
			return nil
		}

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

		c.Status(common.StatusOK).JSON(fiber.Map{
			"code":    common.StatusOK,
			"message": "Check-in thành công",
			"data":    response,
			"status":  "success",
		})
		return nil
	})
}
