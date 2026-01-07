package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	"meta_commerce/core/common"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AgentConfigHandler xử lý các route CRUD cho agent config
// Kế thừa từ BaseHandler để có sẵn các method CRUD
type AgentConfigHandler struct {
	*BaseHandler[models.AgentConfig, dto.AgentConfigCreateInput, dto.AgentConfigUpdateInput]
	configService *services.AgentConfigService
}

// NewAgentConfigHandler tạo mới AgentConfigHandler
// Returns:
//   - *AgentConfigHandler: Instance mới của AgentConfigHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAgentConfigHandler() (*AgentConfigHandler, error) {
	configService, err := services.NewAgentConfigService()
	if err != nil {
		return nil, fmt.Errorf("failed to create agent config service: %w", err)
	}

	return &AgentConfigHandler{
		BaseHandler:   NewBaseHandler[models.AgentConfig, dto.AgentConfigCreateInput, dto.AgentConfigUpdateInput](configService.BaseServiceMongoImpl),
		configService: configService,
	}, nil
}

// HandleUpdateConfigData xử lý update config data (tạo version mới)
// Endpoint: PUT /api/v1/agent-management/config/:agentId/update-data
// Nếu có config active → tạo version mới (deactivate cũ, tạo mới)
// Nếu chưa có config → tạo mới
// Version được server tự động quyết định bằng Unix timestamp
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

		// Parse body thành CreateInput để lấy configData, changeLog, etc.
		var input dto.AgentConfigCreateInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON. Chi tiết: %v", err),
				"status":  "error",
			})
			return nil
		}

		// Validate configData
		if input.ConfigData == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": "ConfigData không được để trống",
				"status":  "error",
			})
			return nil
		}

		// Lấy user ID từ context (nếu có)
		var changedBy *primitive.ObjectID
		if userIDStr, ok := c.Locals("user_id").(string); ok && userIDStr != "" {
			if userID, err := primitive.ObjectIDFromHex(userIDStr); err == nil {
				changedBy = &userID
			}
		}

		// Kiểm tra xem có config active không
		currentConfig, err := h.configService.GetCurrentConfig(c.Context(), agentID)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		var result *models.AgentConfig

		if currentConfig != nil {
			// Có config active → Tạo version mới (UpdateConfig)
			result, err = h.configService.UpdateConfig(
				c.Context(),
				agentID,
				input.ConfigData,
				input.ChangeLog,
				changedBy,
			)
		} else {
			// Chưa có config → Tạo mới (SubmitConfig)
			result, err = h.configService.SubmitConfig(
				c.Context(),
				agentID,
				input.ConfigData,
				"", // configHash sẽ được tính tự động
				false, // submittedByBot = false (admin tạo)
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
