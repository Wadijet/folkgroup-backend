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

// AIWorkflowCommandHandler xử lý các request liên quan đến AI Workflow Command (Module 2)
type AIWorkflowCommandHandler struct {
	*BaseHandler[models.AIWorkflowCommand, dto.AIWorkflowCommandCreateInput, dto.AIWorkflowCommandUpdateInput]
	AIWorkflowCommandService *services.AIWorkflowCommandService
}

// NewAIWorkflowCommandHandler tạo mới AIWorkflowCommandHandler
// Trả về:
//   - *AIWorkflowCommandHandler: Instance mới của AIWorkflowCommandHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIWorkflowCommandHandler() (*AIWorkflowCommandHandler, error) {
	aiWorkflowCommandService, err := services.NewAIWorkflowCommandService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI workflow command service: %v", err)
	}

	handler := &AIWorkflowCommandHandler{
		AIWorkflowCommandService: aiWorkflowCommandService,
	}
	handler.BaseHandler = NewBaseHandler[models.AIWorkflowCommand, dto.AIWorkflowCommandCreateInput, dto.AIWorkflowCommandUpdateInput](aiWorkflowCommandService.BaseServiceMongoImpl)

	return handler, nil
}

// InsertOne override method InsertOne để xử lý ownerOrganizationId và gọi service
//
// LÝ DO PHẢI OVERRIDE (không dùng BaseHandler.InsertOne trực tiếp):
// 1. Xử lý ownerOrganizationId:
//    - Cho phép chỉ định từ request hoặc dùng context
//    - Validate quyền nếu có ownerOrganizationId trong request
//    - BaseHandler.InsertOne không tự động xử lý ownerOrganizationId từ request body
//
// LƯU Ý:
// - Validation enum (CommandType) đã được xử lý tự động bởi struct tag validate:"oneof=..." trong BaseHandler
// - ObjectID conversion đã được xử lý tự động bởi transform tag trong DTO
// - Business logic validation (conditional fields, StepID/ParentLevel matching, RootRefID) đã được chuyển xuống AIWorkflowCommandService.InsertOne
// - Timestamps sẽ được xử lý tự động bởi BaseServiceMongoImpl.InsertOne trong service
//
// ĐẢM BẢO LOGIC CƠ BẢN:
// ✅ Parse và validate input format (DTO validation)
// ✅ Transform DTO → Model (transform tags)
// ✅ Xử lý ownerOrganizationId (từ request hoặc context)
// ✅ Gọi AIWorkflowCommandService.InsertOne (service sẽ validate business logic và insert)
func (h *AIWorkflowCommandHandler) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành DTO
		var input dto.AIWorkflowCommandCreateInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON hoặc không khớp với cấu trúc yêu cầu. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// Transform DTO sang Model sử dụng transform tag (tự động convert ObjectID)
		model, err := h.transformCreateInputToModel(&input)
		if err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Lỗi transform dữ liệu: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// ✅ Xử lý ownerOrganizationId: Cho phép chỉ định từ request hoặc dùng context (BaseHandler logic)
		ownerOrgIDFromRequest := h.getOwnerOrganizationIDFromModel(model)
		if ownerOrgIDFromRequest != nil && !ownerOrgIDFromRequest.IsZero() {
			// Có ownerOrganizationId trong request → Validate quyền
			if err := h.validateUserHasAccessToOrg(c, *ownerOrgIDFromRequest); err != nil {
				h.HandleResponse(c, nil, err)
				return nil
			}
		} else {
			// Không có trong request → Dùng context
			activeOrgID := h.getActiveOrganizationID(c)
			if activeOrgID != nil && !activeOrgID.IsZero() {
				h.setOrganizationID(model, *activeOrgID)
			}
		}

		// ✅ Lưu userID vào context để service có thể check admin
		ctx := c.Context()
		if userIDStr, ok := c.Locals("user_id").(string); ok && userIDStr != "" {
			if userID, err := primitive.ObjectIDFromHex(userIDStr); err == nil {
				ctx = services.SetUserIDToContext(ctx, userID)
			}
		}

		// ✅ Gọi service để insert (service sẽ tự validate business logic)
		// Business logic validation đã được chuyển xuống AIWorkflowCommandService.InsertOne
		data, err := h.AIWorkflowCommandService.InsertOne(ctx, *model)
		h.HandleResponse(c, data, err)
		return nil
	})
}

// ClaimPendingCommands claim các commands đang chờ (pending) với atomic operation
// Endpoint: POST /api/v1/ai/workflow-commands/claim-pending
// Body: { "agentId": "agent-123", "limit": 5 }
//
// Đảm bảo các job khác không lấy lại commands đã được claim cho đến khi được giải phóng
func (h *AIWorkflowCommandHandler) ClaimPendingCommands(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành DTO
		var input dto.AIWorkflowCommandClaimInput
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

		// Trả về danh sách commands đã claim (có thể rỗng nếu không có command pending)
		// Đây là trường hợp hợp lệ, không phải lỗi
		h.HandleResponse(c, claimedCommands, nil)
		return nil
	})
}

// UpdateHeartbeat cập nhật heartbeat và progress của command
// Endpoint: POST /api/v1/ai/workflow-commands/update-heartbeat
// Body: { "commandId": "...", "progress": {...} }
//
// Agent phải gọi endpoint này định kỳ để server biết job đang được thực hiện
func (h *AIWorkflowCommandHandler) UpdateHeartbeat(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse và validate URL params (nếu có commandId trong URL)
		var params dto.UpdateHeartbeatParams
		if err := h.ParseRequestParams(c, &params); err != nil {
			// Nếu không có commandId trong URL, không báo lỗi (có thể có trong body)
			params.CommandID = ""
		}

		// Parse request body thành DTO
		var input dto.AIWorkflowCommandHeartbeatInput
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
// Endpoint: POST /api/v1/ai/workflow-commands/release-stuck
// Query: ?timeoutSeconds=300 (tùy chọn, mặc định 300 giây = 5 phút)
//
// Method này nên được gọi định kỳ bởi background job hoặc admin
func (h *AIWorkflowCommandHandler) ReleaseStuckCommands(c fiber.Ctx) error {
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
