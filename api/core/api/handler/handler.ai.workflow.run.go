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

// AIWorkflowRunHandler xử lý các request liên quan đến AI Workflow Run (Module 2)
type AIWorkflowRunHandler struct {
	*BaseHandler[models.AIWorkflowRun, dto.AIWorkflowRunCreateInput, dto.AIWorkflowRunUpdateInput]
	AIWorkflowRunService *services.AIWorkflowRunService
}

// NewAIWorkflowRunHandler tạo mới AIWorkflowRunHandler
// Trả về:
//   - *AIWorkflowRunHandler: Instance mới của AIWorkflowRunHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIWorkflowRunHandler() (*AIWorkflowRunHandler, error) {
	aiWorkflowRunService, err := services.NewAIWorkflowRunService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI workflow run service: %v", err)
	}

	handler := &AIWorkflowRunHandler{
		AIWorkflowRunService: aiWorkflowRunService,
	}
	handler.BaseHandler = NewBaseHandler[models.AIWorkflowRun, dto.AIWorkflowRunCreateInput, dto.AIWorkflowRunUpdateInput](aiWorkflowRunService.BaseServiceMongoImpl)

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
// - Validation enum (status) đã được xử lý tự động bởi struct tag validate:"oneof=..." trong BaseHandler
// - Default values (status = "pending", CurrentStepIndex = 0, StepRunIDs = []) đã được xử lý tự động bởi transform tag
// - ObjectID conversion đã được xử lý tự động bởi transform tag trong DTO
// - Business logic validation (RootRefID, RootRefType) đã được chuyển xuống AIWorkflowRunService.InsertOne
// - Timestamps sẽ được xử lý tự động bởi BaseServiceMongoImpl.InsertOne trong service
//
// ĐẢM BẢO LOGIC CƠ BẢN:
// ✅ Parse và validate input format (DTO validation)
// ✅ Transform DTO → Model (transform tags)
// ✅ Xử lý ownerOrganizationId (từ request hoặc context)
// ✅ Gọi AIWorkflowRunService.InsertOne (service sẽ validate business logic và insert)
func (h *AIWorkflowRunHandler) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành DTO
		var input dto.AIWorkflowRunCreateInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON hoặc không khớp với cấu trúc yêu cầu. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// Transform DTO sang Model sử dụng transform tag (tự động convert ObjectID, default values)
		// Lưu ý:
		// - Status default value đã được xử lý tự động bởi transform tag transform:"string,default=pending"
		// - CurrentStepIndex default value đã được xử lý tự động bởi transform tag transform:"int,default=0"
		// - StepRunIDs default value đã được xử lý tự động bởi transform tag transform:"str_objectid_array,default=[]"
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

		// ✅ Gọi service để insert (service sẽ tự validate RootRefID và RootRefType)
		// Business logic validation đã được chuyển xuống AIWorkflowRunService.InsertOne
		data, err := h.AIWorkflowRunService.InsertOne(ctx, *model)
		h.HandleResponse(c, data, err)
		return nil
	})
}
