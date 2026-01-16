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

// OrganizationHandler xử lý các request liên quan đến Organization
type OrganizationHandler struct {
	BaseHandler[models.Organization, dto.OrganizationCreateInput, dto.OrganizationUpdateInput]
	OrganizationService *services.OrganizationService
}

// NewOrganizationHandler tạo mới OrganizationHandler
func NewOrganizationHandler() (*OrganizationHandler, error) {
	organizationService, err := services.NewOrganizationService()
	if err != nil {
		return nil, fmt.Errorf("failed to create organization service: %v", err)
	}

	handler := &OrganizationHandler{
		OrganizationService: organizationService,
	}
	handler.BaseService = handler.OrganizationService

	// Khởi tạo filterOptions với giá trị mặc định
	handler.filterOptions = FilterOptions{
		DeniedFields: []string{
			"password",
			"token",
			"secret",
			"key",
			"hash",
		},
		AllowedOperators: []string{
			"$eq",
			"$gt",
			"$gte",
			"$lt",
			"$lte",
			"$in",
			"$nin",
			"$exists",
		},
		MaxFields: 10,
	}

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
// - Validation format đã được xử lý tự động bởi struct tag validate:"required" trong BaseHandler
// - ObjectID conversion đã được xử lý tự động bởi transform tag trong DTO
// - Business logic validation và tính toán (Path, Level) đã được chuyển xuống OrganizationService.InsertOne
// - Timestamps sẽ được xử lý tự động bởi BaseServiceMongoImpl.InsertOne trong service
//
// ĐẢM BẢO LOGIC CƠ BẢN:
// ✅ Parse và validate input format (DTO validation)
// ✅ Transform DTO → Model (transform tags)
// ✅ Xử lý ownerOrganizationId (từ request hoặc context)
// ✅ Gọi OrganizationService.InsertOne (service sẽ tính toán Path/Level và insert)
func (h *OrganizationHandler) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành DTO
		var input dto.OrganizationCreateInput
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

		// ✅ Xử lý ownerOrganizationId: Cho phép chỉ định từ request hoặc dùng context
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

		// ✅ Gọi service để insert (service sẽ tự tính toán Path và Level)
		// Business logic validation và tính toán đã được chuyển xuống OrganizationService.InsertOne
		data, err := h.OrganizationService.InsertOne(ctx, *model)
		h.HandleResponse(c, data, err)
		return nil
	})
}
