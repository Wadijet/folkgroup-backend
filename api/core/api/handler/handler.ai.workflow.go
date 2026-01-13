package handler

import (
	"fmt"
	"time"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AIWorkflowHandler xử lý các request liên quan đến AI Workflow (Module 2)
type AIWorkflowHandler struct {
	*BaseHandler[models.AIWorkflow, dto.AIWorkflowCreateInput, dto.AIWorkflowUpdateInput]
	AIWorkflowService *services.AIWorkflowService
}

// NewAIWorkflowHandler tạo mới AIWorkflowHandler
// Trả về:
//   - *AIWorkflowHandler: Instance mới của AIWorkflowHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIWorkflowHandler() (*AIWorkflowHandler, error) {
	aiWorkflowService, err := services.NewAIWorkflowService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI workflow service: %v", err)
	}

	handler := &AIWorkflowHandler{
		AIWorkflowService: aiWorkflowService,
	}
	handler.BaseHandler = NewBaseHandler[models.AIWorkflow, dto.AIWorkflowCreateInput, dto.AIWorkflowUpdateInput](aiWorkflowService.BaseServiceMongoImpl)

	return handler, nil
}

// InsertOne override method InsertOne để chuyển đổi từ DTO sang Model
//
// LÝ DO PHẢI OVERRIDE (không thể dùng CRUD chuẩn):
// 1. Convert nested structures phức tạp:
//    - Steps: Convert từ []dto.AIWorkflowStepReferenceInput (DTO) sang []models.AIWorkflowStepReference (Model)
//    - Mỗi Step có Policy nested: Convert từ dto.AIWorkflowStepPolicyInput sang models.AIWorkflowStepPolicy
//    - DefaultPolicy: Convert từ dto.AIWorkflowStepPolicyInput sang *models.AIWorkflowStepPolicy
// 2. Logic nghiệp vụ đặc biệt:
//    - Validate Status với danh sách giá trị hợp lệ: "active", "archived", "draft"
//    - Set default Status = "active" nếu không có
//    - Set CreatedAt và UpdatedAt tự động (timestamp milliseconds)
// 3. Transform tag không hỗ trợ nested structures:
//    - Transform tag hiện tại chỉ hỗ trợ convert primitive types (string → ObjectID, etc.)
//    - Không hỗ trợ convert nested struct arrays và nested pointer structs
//    - Cần logic đặc biệt để map từng field trong nested structures
//
// KẾT LUẬN: Cần giữ override vì logic convert nested structures quá phức tạp, không thể dùng transform tag
func (h *AIWorkflowHandler) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành DTO
		var input dto.AIWorkflowCreateInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON hoặc không khớp với cấu trúc yêu cầu. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// Validate type
		validStatuses := []string{"active", "archived", "draft"}
		statusValid := false
		if input.Status == "" {
			input.Status = "active" // Mặc định
			statusValid = true
		} else {
			for _, validStatus := range validStatuses {
				if input.Status == validStatus {
					statusValid = true
					break
				}
			}
		}
		if !statusValid {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Status '%s' không hợp lệ. Các giá trị hợp lệ: %v", input.Status, validStatuses),
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// Chuyển đổi DTO sang Model
		now := time.Now().UnixMilli()
		aiWorkflow := models.AIWorkflow{
			Name:        input.Name,
			Description: input.Description,
			Version:     input.Version,
			RootRefType: input.RootRefType,
			TargetLevel: input.TargetLevel,
			Status:      input.Status,
			Metadata:    input.Metadata,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		// Chuyển đổi steps
		steps := make([]models.AIWorkflowStepReference, 0, len(input.Steps))
		for _, stepInput := range input.Steps {
			step := models.AIWorkflowStepReference{
				StepID: stepInput.StepID,
				Order:  stepInput.Order,
			}
			if stepInput.Policy != nil {
				step.Policy = &models.AIWorkflowStepPolicy{
					RetryCount: stepInput.Policy.RetryCount,
					Timeout:    stepInput.Policy.Timeout,
					OnFailure:  stepInput.Policy.OnFailure,
					OnSuccess:  stepInput.Policy.OnSuccess,
					Parallel:   stepInput.Policy.Parallel,
					Condition:  stepInput.Policy.Condition,
				}
			}
			steps = append(steps, step)
		}
		aiWorkflow.Steps = steps

		// Chuyển đổi default policy
		if input.DefaultPolicy != nil {
			aiWorkflow.DefaultPolicy = &models.AIWorkflowStepPolicy{
				RetryCount: input.DefaultPolicy.RetryCount,
				Timeout:    input.DefaultPolicy.Timeout,
				OnFailure:  input.DefaultPolicy.OnFailure,
				OnSuccess:  input.DefaultPolicy.OnSuccess,
				Parallel:   input.DefaultPolicy.Parallel,
				Condition:  input.DefaultPolicy.Condition,
			}
		}

		// ✅ Xử lý ownerOrganizationId: Cho phép chỉ định từ request hoặc dùng context
		// Lấy organization ID từ role context (vì OrganizationContextMiddleware đã set active_role_id)
		if activeRoleIDStr, ok := c.Locals("active_role_id").(string); ok && activeRoleIDStr != "" {
			if activeRoleID, err := primitive.ObjectIDFromHex(activeRoleIDStr); err == nil {
				// Lấy role để suy ra organization ID
				roleService, err := services.NewRoleService()
				if err == nil {
					if role, err := roleService.FindOneById(c.Context(), activeRoleID); err == nil {
						if !role.OwnerOrganizationID.IsZero() {
							aiWorkflow.OwnerOrganizationID = role.OwnerOrganizationID
						}
					}
				}
			}
		}
		
		// Fallback: Nếu vẫn chưa có, thử lấy từ active_organization_id trong context
		if aiWorkflow.OwnerOrganizationID.IsZero() {
			activeOrgID := h.getActiveOrganizationID(c)
			if activeOrgID != nil && !activeOrgID.IsZero() {
				aiWorkflow.OwnerOrganizationID = *activeOrgID
			}
		}

		// ✅ Lưu userID vào context để service có thể check admin
		ctx := c.Context()
		if userIDStr, ok := c.Locals("user_id").(string); ok && userIDStr != "" {
			if userID, err := primitive.ObjectIDFromHex(userIDStr); err == nil {
				ctx = services.SetUserIDToContext(ctx, userID)
			}
		}

		// Thực hiện insert
		data, err := h.BaseService.InsertOne(ctx, aiWorkflow)
		h.HandleResponse(c, data, err)
		return nil
	})
}
