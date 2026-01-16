package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AgentCommandHandler xử lý các route CRUD cho agent command
// Kế thừa từ BaseHandler để có sẵn các method CRUD
type AgentCommandHandler struct {
	*BaseHandler[models.AgentCommand, dto.AgentCommandCreateInput, dto.AgentCommandUpdateInput]
	AgentCommandService *services.AgentCommandService
}

// NewAgentCommandHandler tạo mới AgentCommandHandler
// Returns:
//   - *AgentCommandHandler: Instance mới của AgentCommandHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAgentCommandHandler() (*AgentCommandHandler, error) {
	commandService, err := services.NewAgentCommandService()
	if err != nil {
		return nil, fmt.Errorf("failed to create agent command service: %w", err)
	}

	handler := &AgentCommandHandler{
		AgentCommandService: commandService,
	}
	handler.BaseHandler = NewBaseHandler[models.AgentCommand, dto.AgentCommandCreateInput, dto.AgentCommandUpdateInput](commandService.BaseServiceMongoImpl)

	return handler, nil
}

// ClaimPendingCommands claim các commands đang chờ (pending) với atomic operation
// Endpoint: POST /api/v1/agent-management/command/claim-pending
// Body: { "agentId": "agent-123", "limit": 5 }
//
// Đảm bảo các job khác không lấy lại commands đã được claim cho đến khi được giải phóng
func (h *AgentCommandHandler) ClaimPendingCommands(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành DTO
		var input dto.AgentCommandClaimInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON hoặc không khớp với cấu trúc yêu cầu. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// Lưu ý: Limit và AgentID đã được validate tự động bởi struct tag:
		// - Limit: validate:"omitempty,min=1,max=100" transform:"int,default=1"
		// - AgentID: validate:"required"

		// Gọi service để claim commands
		claimedCommands, err := h.AgentCommandService.ClaimPendingCommands(c.Context(), input.AgentID, input.Limit)
		if err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeInternalServer,
				fmt.Sprintf("Lỗi khi claim commands: %v", err),
				common.StatusInternalServerError,
				err,
			))
			return nil
		}

		// Trả về danh sách commands đã claim (có thể rỗng nếu không có command pending)
		// Đây là trường hợp hợp lệ, không phải lỗi
		h.HandleResponse(c, claimedCommands, nil)
		return nil
	})
}

// UpdateHeartbeat cập nhật heartbeat và progress của command
// Endpoint: POST /api/v1/agent-management/command/update-heartbeat
// Body: { "commandId": "...", "progress": {...} }
//
// Agent phải gọi endpoint này định kỳ để server biết job đang được thực hiện
//
// ĐƠN GIẢN HÓA VỚI VALIDATOR:
// - URL params validation: Dùng DTO với validator để tự động validate và convert ObjectID
// - Request body validation: Đã có validator trong ParseRequestBody
// - Giảm ~30 dòng code validation thủ công
func (h *AgentCommandHandler) UpdateHeartbeat(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse và validate URL params (nếu có commandId trong URL)
		var params dto.UpdateHeartbeatParams
		if err := h.ParseRequestParams(c, &params); err != nil {
			// Nếu không có commandId trong URL, không báo lỗi (có thể có trong body)
			params.CommandID = ""
		}

		// Parse request body thành DTO
		var input dto.AgentCommandHeartbeatInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Lấy commandId từ URL params hoặc body (ưu tiên URL params)
		var commandID primitive.ObjectID
		if params.CommandID != "" {
			// Lấy từ URL params (đã được validate và convert)
			commandID, _ = primitive.ObjectIDFromHex(params.CommandID)
		} else if input.CommandID != "" {
			// Lấy từ body (đã được transform thành string ObjectID)
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

		// Lấy agentId từ request (có thể từ body hoặc từ context nếu có middleware set)
		// Tạm thời lấy từ query parameter hoặc header, sau này có thể dùng middleware
		agentId := c.Query("agentId", "")
		if agentId == "" {
			// Thử lấy từ header
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

		// Gọi service để update heartbeat
		updatedCommand, err := h.AgentCommandService.UpdateHeartbeat(c.Context(), commandID, agentId, input.Progress)
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
// Endpoint: POST /api/v1/agent-management/command/release-stuck
// Query: ?timeoutSeconds=300 (tùy chọn, mặc định 300 giây = 5 phút)
//
// Method này nên được gọi định kỳ bởi background job hoặc admin
//
// ĐƠN GIẢN HÓA VỚI VALIDATOR:
// - Query params validation: Dùng DTO với validator để tự động validate
// - Giảm ~5 dòng code validation thủ công
func (h *AgentCommandHandler) ReleaseStuckCommands(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse và validate query params (tự động validate với struct tag)
		var query dto.ReleaseStuckCommandsQuery
		if err := h.ParseQueryParams(c, &query); err != nil {
			// Nếu parse lỗi, dùng giá trị mặc định
			query.TimeoutSeconds = 300
		}
		
		// Đảm bảo timeoutSeconds hợp lệ (tối thiểu 60, mặc định 300)
		timeoutSeconds := query.TimeoutSeconds
		if timeoutSeconds < 60 {
			timeoutSeconds = 300 // Mặc định 5 phút, tối thiểu 60 giây
		}

		// Gọi service để release stuck commands
		releasedCount, err := h.AgentCommandService.ReleaseStuckCommands(c.Context(), timeoutSeconds)
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
			"releasedCount":   releasedCount,
			"timeoutSeconds":  timeoutSeconds,
			"message":         fmt.Sprintf("Đã giải phóng %d commands bị stuck", releasedCount),
		}, nil)
		return nil
	})
}
