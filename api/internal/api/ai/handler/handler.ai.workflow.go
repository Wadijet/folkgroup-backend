package aihdl

import (
	"fmt"
	aidto "meta_commerce/internal/api/ai/dto"
	aimodels "meta_commerce/internal/api/ai/models"
	basehdl "meta_commerce/internal/api/base/handler"
	aisvc "meta_commerce/internal/api/ai/service"
	authsvc "meta_commerce/internal/api/auth/service"
	"meta_commerce/internal/common"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AIWorkflowHandler xử lý các request liên quan đến AI Workflow (Module 2)
type AIWorkflowHandler struct {
	*basehdl.BaseHandler[aimodels.AIWorkflow, aidto.AIWorkflowCreateInput, aidto.AIWorkflowUpdateInput]
	AIWorkflowService *aisvc.AIWorkflowService
}

// NewAIWorkflowHandler tạo mới AIWorkflowHandler
// Trả về:
//   - *AIWorkflowHandler: Instance mới của AIWorkflowHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIWorkflowHandler() (*AIWorkflowHandler, error) {
	aiWorkflowService, err := aisvc.NewAIWorkflowService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI workflow service: %v", err)
	}

	h := &AIWorkflowHandler{
		AIWorkflowService: aiWorkflowService,
	}
	h.BaseHandler = basehdl.NewBaseHandler[aimodels.AIWorkflow, aidto.AIWorkflowCreateInput, aidto.AIWorkflowUpdateInput](aiWorkflowService.BaseServiceMongoImpl)

	return h, nil
}

// InsertOne override method InsertOne để chuyển đổi từ DTO sang Model
func (h *AIWorkflowHandler) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		var input aidto.AIWorkflowCreateInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON hoặc không khớp với cấu trúc yêu cầu. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		if err := h.ValidateInput(&input); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		model, err := h.TransformCreateInputToModel(&input)
		if err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Lỗi transform dữ liệu: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		aiWorkflow := *model

		steps := make([]aimodels.AIWorkflowStepReference, 0, len(input.Steps))
		for _, stepInput := range input.Steps {
			step := aimodels.AIWorkflowStepReference{
				StepID: stepInput.StepID,
				Order:  stepInput.Order,
			}
			if stepInput.Policy != nil {
				step.Policy = &aimodels.AIWorkflowStepPolicy{
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

		if input.DefaultPolicy != nil {
			aiWorkflow.DefaultPolicy = &aimodels.AIWorkflowStepPolicy{
				RetryCount: input.DefaultPolicy.RetryCount,
				Timeout:    input.DefaultPolicy.Timeout,
				OnFailure:  input.DefaultPolicy.OnFailure,
				OnSuccess:  input.DefaultPolicy.OnSuccess,
				Parallel:   input.DefaultPolicy.Parallel,
				Condition:  input.DefaultPolicy.Condition,
			}
		}

		ownerOrgIDFromRequest := h.GetOwnerOrganizationIDFromModel(&aiWorkflow)
		if ownerOrgIDFromRequest != nil && !ownerOrgIDFromRequest.IsZero() {
			if err := h.ValidateUserHasAccessToOrg(c, *ownerOrgIDFromRequest); err != nil {
				h.HandleResponse(c, nil, err)
				return nil
			}
		} else {
			activeOrgID := h.GetActiveOrganizationID(c)
			if activeOrgID != nil && !activeOrgID.IsZero() {
				h.SetOrganizationID(&aiWorkflow, *activeOrgID)
			}
		}

		ctx := c.Context()
		if userIDStr, ok := c.Locals("user_id").(string); ok && userIDStr != "" {
			if userID, err := primitive.ObjectIDFromHex(userIDStr); err == nil {
				ctx = authsvc.SetUserIDToContext(ctx, userID)
			}
		}

		data, err := h.BaseService.InsertOne(ctx, aiWorkflow)
		h.HandleResponse(c, data, err)
		return nil
	})
}
