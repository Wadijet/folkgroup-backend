package aihdl

import (
	"fmt"
	agentdto "meta_commerce/internal/api/agent/dto"
	aimodels "meta_commerce/internal/api/ai/models"
	aidto "meta_commerce/internal/api/ai/dto"
	basehdl "meta_commerce/internal/api/base/handler"
	aisvc "meta_commerce/internal/api/ai/service"
	"meta_commerce/internal/common"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AIWorkflowCommandHandler xử lý các request liên quan (Module 2)
type AIWorkflowCommandHandler struct {
	*basehdl.BaseHandler[aimodels.AIWorkflowCommand, aidto.AIWorkflowCommandCreateInput, aidto.AIWorkflowCommandUpdateInput]
	AIWorkflowCommandService *aisvc.AIWorkflowCommandService
}

// NewAIWorkflowCommandHandler tạo mới AIWorkflowCommandHandler
func NewAIWorkflowCommandHandler() (*AIWorkflowCommandHandler, error) {
	aiWorkflowCommandService, err := aisvc.NewAIWorkflowCommandService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI workflow command service: %v", err)
	}

	hdl := &AIWorkflowCommandHandler{
		AIWorkflowCommandService: aiWorkflowCommandService,
	}
	hdl.BaseHandler = basehdl.NewBaseHandler[aimodels.AIWorkflowCommand, aidto.AIWorkflowCommandCreateInput, aidto.AIWorkflowCommandUpdateInput](aiWorkflowCommandService)

	return hdl, nil
}

// ClaimPendingCommands claim các commands đang chờ (pending) với atomic operation
func (h *AIWorkflowCommandHandler) ClaimPendingCommands(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		var input aidto.AIWorkflowCommandClaimInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON hoặc không khớp với cấu trúc yêu cầu. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		claimedCommands, err := h.AIWorkflowCommandService.ClaimPendingCommands(c.Context(), input.AgentID, input.Limit)
		if err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeInternalServer,
				fmt.Sprintf("Lỗi khi claim commands: %v", err),
				common.StatusInternalServerError,
				err,
			))
			return nil
		}

		h.HandleResponse(c, claimedCommands, nil)
		return nil
	})
}

// UpdateHeartbeat cập nhật heartbeat và progress của command
func (h *AIWorkflowCommandHandler) UpdateHeartbeat(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		var params agentdto.UpdateHeartbeatParams
		if err := h.ParseRequestParams(c, &params); err != nil {
			params.CommandID = ""
		}

		var input aidto.AIWorkflowCommandHeartbeatInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		var commandID primitive.ObjectID
		if params.CommandID != "" {
			commandID, _ = primitive.ObjectIDFromHex(params.CommandID)
		} else if input.CommandID != "" {
			commandID, _ = primitive.ObjectIDFromHex(input.CommandID)
		}

		if commandID.IsZero() {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				"commandId là bắt buộc (có thể truyền qua URL params :commandId hoặc body JSON {\"commandId\": \"...\"})",
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		agentId := c.Query("agentId", "")
		if agentId == "" {
			agentId = c.Get("X-Agent-ID", "")
		}
		if agentId == "" {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				"agentId là bắt buộc (có thể truyền qua query parameter ?agentId=... hoặc header X-Agent-ID)",
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		updatedCommand, err := h.AIWorkflowCommandService.UpdateHeartbeat(c.Context(), commandID, agentId, input.Progress)
		if err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeBusinessOperation,
				fmt.Sprintf("Lỗi khi update heartbeat: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		h.HandleResponse(c, updatedCommand, nil)
		return nil
	})
}

// ReleaseStuckCommands giải phóng các commands bị stuck (quá lâu không có heartbeat)
func (h *AIWorkflowCommandHandler) ReleaseStuckCommands(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		var query agentdto.ReleaseStuckCommandsQuery
		if err := h.ParseQueryParams(c, &query); err != nil {
			query.TimeoutSeconds = 300
		}

		timeoutSeconds := query.TimeoutSeconds
		if timeoutSeconds < 60 {
			timeoutSeconds = 300
		}

		releasedCount, err := h.AIWorkflowCommandService.ReleaseStuckCommands(c.Context(), timeoutSeconds)
		if err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeInternalServer,
				fmt.Sprintf("Lỗi khi release stuck commands: %v", err),
				common.StatusInternalServerError,
				err,
			))
			return nil
		}

		h.HandleResponse(c, map[string]interface{}{
			"releasedCount":  releasedCount,
			"timeoutSeconds": timeoutSeconds,
			"message":        fmt.Sprintf("Đã giải phóng %d commands bị stuck", releasedCount),
		}, nil)
		return nil
	})
}
