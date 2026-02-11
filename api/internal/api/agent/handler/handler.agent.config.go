package agenthdl

import (
	"fmt"
	agentdto "meta_commerce/internal/api/agent/dto"
	agentmodels "meta_commerce/internal/api/agent/models"
	agentsvc "meta_commerce/internal/api/agent/service"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/common"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AgentConfigHandler xử lý các route CRUD cho agent config
type AgentConfigHandler struct {
	*basehdl.BaseHandler[agentmodels.AgentConfig, agentdto.AgentConfigCreateInput, agentdto.AgentConfigUpdateInput]
	configService *agentsvc.AgentConfigService
}

// NewAgentConfigHandler tạo mới AgentConfigHandler
func NewAgentConfigHandler() (*AgentConfigHandler, error) {
	configService, err := agentsvc.NewAgentConfigService()
	if err != nil {
		return nil, fmt.Errorf("failed to create agent config service: %w", err)
	}
	return &AgentConfigHandler{
		BaseHandler:   basehdl.NewBaseHandler[agentmodels.AgentConfig, agentdto.AgentConfigCreateInput, agentdto.AgentConfigUpdateInput](configService.BaseServiceMongoImpl),
		configService: configService,
	}, nil
}

// HandleUpdateConfigData xử lý update config data (tạo version mới)
func (h *AgentConfigHandler) HandleUpdateConfigData(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		agentID := c.Params("agentId")
		if agentID == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": "AgentID không được để trống",
				"status":  "error",
			})
			return nil
		}

		var input agentdto.AgentConfigCreateInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON. Chi tiết: %v", err),
				"status":  "error",
			})
			return nil
		}

		if input.ConfigData == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": "ConfigData không được để trống",
				"status":  "error",
			})
			return nil
		}

		var changedBy *primitive.ObjectID
		if userIDStr, ok := c.Locals("user_id").(string); ok && userIDStr != "" {
			if userID, err := primitive.ObjectIDFromHex(userIDStr); err == nil {
				changedBy = &userID
			}
		}

		currentConfig, err := h.configService.GetCurrentConfig(c.Context(), agentID)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		var result *agentmodels.AgentConfig

		if currentConfig != nil {
			result, err = h.configService.UpdateConfig(
				c.Context(),
				agentID,
				input.ConfigData,
				input.ChangeLog,
				changedBy,
			)
		} else {
			result, err = h.configService.SubmitConfig(
				c.Context(),
				agentID,
				input.ConfigData,
				"",
				false,
			)
		}

		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		c.Status(common.StatusOK).JSON(fiber.Map{
			"code":    common.StatusOK,
			"message": "Config đã được cập nhật thành công",
			"data":    result,
			"status":  "success",
		})
		return nil
	})
}
