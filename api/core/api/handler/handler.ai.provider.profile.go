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

// AIProviderProfileHandler xử lý các request liên quan đến AI Provider Profile (Module 2)
type AIProviderProfileHandler struct {
	*BaseHandler[models.AIProviderProfile, dto.AIProviderProfileCreateInput, dto.AIProviderProfileUpdateInput]
	AIProviderProfileService *services.AIProviderProfileService
}

// NewAIProviderProfileHandler tạo mới AIProviderProfileHandler
// Trả về:
//   - *AIProviderProfileHandler: Instance mới của AIProviderProfileHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIProviderProfileHandler() (*AIProviderProfileHandler, error) {
	aiProviderProfileService, err := services.NewAIProviderProfileService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI provider profile service: %v", err)
	}

	handler := &AIProviderProfileHandler{
		AIProviderProfileService: aiProviderProfileService,
	}
	handler.BaseHandler = NewBaseHandler[models.AIProviderProfile, dto.AIProviderProfileCreateInput, dto.AIProviderProfileUpdateInput](aiProviderProfileService.BaseServiceMongoImpl)

	return handler, nil
}

// InsertOne override method InsertOne để validate và set OwnerOrganizationID
//
// LÝ DO PHẢI OVERRIDE:
// 1. Validate provider type và status với danh sách giá trị hợp lệ
// 2. Set default Status = "active" nếu không có
// 3. Set CreatedAt và UpdatedAt tự động (timestamp milliseconds)
// 4. Set OwnerOrganizationID từ context
func (h *AIProviderProfileHandler) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành DTO
		var input dto.AIProviderProfileCreateInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON hoặc không khớp với cấu trúc yêu cầu. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// Validate provider type
		validProviders := []string{
			models.AIProviderTypeOpenAI,
			models.AIProviderTypeAnthropic,
			models.AIProviderTypeGoogle,
			models.AIProviderTypeCohere,
			models.AIProviderTypeCustom,
		}
		providerValid := false
		for _, validProvider := range validProviders {
			if input.Provider == validProvider {
				providerValid = true
				break
			}
		}
		if !providerValid {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Provider '%s' không hợp lệ. Các giá trị hợp lệ: %v", input.Provider, validProviders),
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// Validate status
		validStatuses := []string{
			models.AIProviderProfileStatusActive,
			models.AIProviderProfileStatusInactive,
			models.AIProviderProfileStatusArchived,
		}
		statusValid := false
		if input.Status == "" {
			input.Status = models.AIProviderProfileStatusActive // Mặc định
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

		// Transform DTO sang Model (không có ObjectID conversion nên dùng transform tag tự động)
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

		// Set default values và timestamps
		now := time.Now().UnixMilli()
		model.CreatedAt = now
		model.UpdatedAt = now

		// ✅ Xử lý ownerOrganizationId: Lấy từ role context (giống BaseHandler nhưng cần set trước khi gọi)
		if activeRoleIDStr, ok := c.Locals("active_role_id").(string); ok && activeRoleIDStr != "" {
			if activeRoleID, err := primitive.ObjectIDFromHex(activeRoleIDStr); err == nil {
				roleService, err := services.NewRoleService()
				if err == nil {
					if role, err := roleService.FindOneById(c.Context(), activeRoleID); err == nil {
						if !role.OwnerOrganizationID.IsZero() {
							model.OwnerOrganizationID = role.OwnerOrganizationID
						}
					}
				}
			}
		}

		// Fallback: Nếu vẫn chưa có, thử lấy từ active_organization_id trong context
		if model.OwnerOrganizationID.IsZero() {
			activeOrgID := h.getActiveOrganizationID(c)
			if activeOrgID != nil && !activeOrgID.IsZero() {
				model.OwnerOrganizationID = *activeOrgID
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
		data, err := h.BaseService.InsertOne(ctx, *model)
		h.HandleResponse(c, data, err)
		return nil
	})
}

// Tất cả các CRUD operations khác đã được cung cấp bởi BaseHandler với transform tag tự động
// - UpdateById: Cập nhật AI provider profile
// - FindOneById: Lấy AI provider profile theo ID
// - FindWithPagination: Lấy danh sách AI provider profile với phân trang
// - DeleteById: Xóa AI provider profile theo ID
