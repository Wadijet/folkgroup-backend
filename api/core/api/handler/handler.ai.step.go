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

// AIStepHandler xử lý các request liên quan đến AI Step (Module 2)
type AIStepHandler struct {
	*BaseHandler[models.AIStep, dto.AIStepCreateInput, dto.AIStepUpdateInput]
	AIStepService *services.AIStepService
}

// NewAIStepHandler tạo mới AIStepHandler
// Trả về:
//   - *AIStepHandler: Instance mới của AIStepHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIStepHandler() (*AIStepHandler, error) {
	aiStepService, err := services.NewAIStepService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI step service: %v", err)
	}

	handler := &AIStepHandler{
		AIStepService: aiStepService,
	}
	handler.BaseHandler = NewBaseHandler[models.AIStep, dto.AIStepCreateInput, dto.AIStepUpdateInput](aiStepService.BaseServiceMongoImpl)

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
// - Validation enum (step type) đã được xử lý tự động bởi struct tag validate:"oneof=..." trong BaseHandler
// - Default values (status = "active") đã được xử lý tự động bởi transform tag transform:"string,default=active"
// - ObjectID conversion đã được xử lý tự động bởi transform tag trong DTO
// - Business logic validation (schema validation) đã được chuyển xuống AIStepService.InsertOne
// - Timestamps sẽ được xử lý tự động bởi BaseServiceMongoImpl.InsertOne trong service
//
// ĐẢM BẢO LOGIC CƠ BẢN:
// ✅ Parse và validate input format (DTO validation)
// ✅ Transform DTO → Model (transform tags)
// ✅ Xử lý ownerOrganizationId (từ request hoặc context)
// ✅ Gọi AIStepService.InsertOne (service sẽ validate schema và insert)
func (h *AIStepHandler) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành DTO
		var input dto.AIStepCreateInput
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
		// Lưu ý: PromptTemplateID đã được convert tự động bởi transform tag transform:"str_objectid_ptr,optional"
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

		// ✅ Gọi service để insert (service sẽ tự validate schema)
		// Business logic validation đã được chuyển xuống AIStepService.InsertOne
		data, err := h.AIStepService.InsertOne(ctx, *model)
		h.HandleResponse(c, data, err)
		return nil
	})
}

// RenderPrompt render prompt template cho step với variables từ step input
// Bot gọi API này để lấy prompt đã render và AI config trước khi gọi AI API
// POST /api/v2/ai/steps/:id/render-prompt
//
// ĐƠN GIẢN HÓA VỚI VALIDATOR:
// - URL params validation: Dùng DTO với validator để tự động validate và convert ObjectID
// - Request body validation: Đã có validator trong ParseRequestBody
// - Giảm ~15 dòng code validation thủ công
func (h *AIStepHandler) RenderPrompt(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse và validate URL params (tự động validate ObjectID format và convert)
		var params dto.AIStepRenderPromptParams
		if err := h.ParseRequestParams(c, &params); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		stepID, _ := primitive.ObjectIDFromHex(params.ID) // Đã được validate rồi, safe to convert

		// Parse và validate request body (tự động validate với struct tag)
		var input dto.AIStepRenderPromptInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Gọi service để render prompt và resolve config
		renderedPrompt, providerProfileID, provider, model, temperature, maxTokens, err := h.AIStepService.RenderPromptForStep(
			c.Context(),
			stepID,
			input.Variables,
		)
		if err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeInternalServer,
				fmt.Sprintf("Lỗi khi render prompt: %v", err),
				common.StatusInternalServerError,
				err,
			))
			return nil
		}

		// Trả về kết quả
		output := dto.AIStepRenderPromptOutput{
			RenderedPrompt:    renderedPrompt,
			ProviderProfileID: providerProfileID,
			Provider:          provider,
			Model:             model,
			Temperature:       temperature,
			MaxTokens:         maxTokens,
			Variables:         input.Variables,
		}

		h.HandleResponse(c, output, nil)
		return nil
	})
}

// Tất cả các CRUD operations khác đã được cung cấp bởi BaseHandler với transform tag tự động
// - UpdateById: Cập nhật AI step
// - FindOneById: Lấy AI step theo ID
// - FindWithPagination: Lấy danh sách AI step với phân trang
// - DeleteById: Xóa AI step theo ID
