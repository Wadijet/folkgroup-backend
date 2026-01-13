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

// AIStepRunHandler xử lý các request liên quan đến AI Step Run (Module 2)
type AIStepRunHandler struct {
	*BaseHandler[models.AIStepRun, dto.AIStepRunCreateInput, dto.AIStepRunUpdateInput]
	AIStepRunService *services.AIStepRunService
}

// NewAIStepRunHandler tạo mới AIStepRunHandler
// Trả về:
//   - *AIStepRunHandler: Instance mới của AIStepRunHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIStepRunHandler() (*AIStepRunHandler, error) {
	aiStepRunService, err := services.NewAIStepRunService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI step run service: %v", err)
	}

	handler := &AIStepRunHandler{
		AIStepRunService: aiStepRunService,
	}
	handler.BaseHandler = NewBaseHandler[models.AIStepRun, dto.AIStepRunCreateInput, dto.AIStepRunUpdateInput](aiStepRunService.BaseServiceMongoImpl)

	return handler, nil
}

// InsertOne override method InsertOne để set default values
//
// LÝ DO PHẢI OVERRIDE:
// 1. Set default Status = "pending"
// 2. Set CreatedAt tự động (timestamp milliseconds)
//
// LƯU Ý: ObjectID conversion đã được xử lý tự động bởi transform tag trong DTO
func (h *AIStepRunHandler) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành DTO
		var input dto.AIStepRunCreateInput
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

		// Set default values
		now := time.Now().UnixMilli()
		model.Status = models.AIStepRunStatusPending // Mặc định
		model.CreatedAt = now

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
