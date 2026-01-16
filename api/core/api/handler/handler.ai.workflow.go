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
// LÝ DO PHẢI OVERRIDE:
// 1. Convert nested structures phức tạp:
//   - Steps: Convert từ []dto.AIWorkflowStepReferenceInput (DTO) sang []models.AIWorkflowStepReference (Model)
//   - Mỗi Step có Policy nested: Convert từ dto.AIWorkflowStepPolicyInput sang models.AIWorkflowStepPolicy
//   - DefaultPolicy: Convert từ dto.AIWorkflowStepPolicyInput sang *models.AIWorkflowStepPolicy
//   - Transform tag không hỗ trợ nested struct arrays và nested pointer structs
//
// LƯU Ý:
// - Validation enum (status) đã được xử lý tự động bởi struct tag validate:"oneof=..." trong BaseHandler
// - Default values (status = "active") đã được xử lý tự động bởi transform tag transform:"string,default=active"
// - Timestamps và ownerOrganizationId đã được xử lý tự động bởi BaseHandler
//
// ĐẢM BẢO LOGIC CƠ BẢN:
// ✅ Chỉ convert nested structures
// ✅ Gọi BaseHandler.InsertOne để đảm bảo:
//    - Validation với struct tag (validate, oneof)
//    - Default values với transform tag
//    - Set timestamps (CreatedAt, UpdatedAt)
//    - Xử lý ownerOrganizationId từ role context hoặc active_organization_id
//    - Lưu userID vào context để service có thể check admin
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

		// ✅ Validate input với struct tag (validate, oneof) - BaseHandler logic
		if err := h.validateInput(&input); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Transform DTO sang Model sử dụng transform tag (tự động convert ObjectID, default values)
		// Lưu ý: Status default value đã được xử lý tự động bởi transform tag transform:"string,default=active"
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

		// Chuyển đổi nested structures (transform tag không hỗ trợ)
		aiWorkflow := *model

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

		// ✅ Xử lý ownerOrganizationId: Cho phép chỉ định từ request hoặc dùng context (BaseHandler logic)
		ownerOrgIDFromRequest := h.getOwnerOrganizationIDFromModel(&aiWorkflow)
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
				h.setOrganizationID(&aiWorkflow, *activeOrgID)
			}
		}

		// ✅ Lưu userID vào context để service có thể check admin
		ctx := c.Context()
		if userIDStr, ok := c.Locals("user_id").(string); ok && userIDStr != "" {
			if userID, err := primitive.ObjectIDFromHex(userIDStr); err == nil {
				ctx = services.SetUserIDToContext(ctx, userID)
			}
		}

		// Thực hiện insert (BaseService.InsertOne sẽ tự động set timestamps)
		data, err := h.BaseService.InsertOne(ctx, aiWorkflow)
		h.HandleResponse(c, data, err)
		return nil
	})
}
