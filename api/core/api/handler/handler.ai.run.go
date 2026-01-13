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

// AIRunHandler xử lý các request liên quan đến AI Run (Module 2)
type AIRunHandler struct {
	*BaseHandler[models.AIRun, dto.AIRunCreateInput, dto.AIRunUpdateInput]
	AIRunService *services.AIRunService
}

// NewAIRunHandler tạo mới AIRunHandler
// Trả về:
//   - *AIRunHandler: Instance mới của AIRunHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIRunHandler() (*AIRunHandler, error) {
	aiRunService, err := services.NewAIRunService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI run service: %v", err)
	}

	handler := &AIRunHandler{
		AIRunService: aiRunService,
	}
	handler.BaseHandler = NewBaseHandler[models.AIRun, dto.AIRunCreateInput, dto.AIRunUpdateInput](aiRunService.BaseServiceMongoImpl)

	return handler, nil
}

// InsertOne override method InsertOne để validate Type và set default values
//
// LÝ DO PHẢI OVERRIDE:
// 1. Validate Type (GENERATE, JUDGE) - business logic validation
// 2. Set default Status = "pending"
// 3. Set CreatedAt tự động (timestamp milliseconds)
//
// LƯU Ý: ObjectID conversion đã được xử lý tự động bởi transform tag trong DTO
func (h *AIRunHandler) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành DTO
		var input dto.AIRunCreateInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON hoặc không khớp với cấu trúc yêu cầu. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// Validate Type (business logic validation)
		validTypes := []string{models.AIRunTypeGenerate, models.AIRunTypeJudge}
		typeValid := false
		for _, validType := range validTypes {
			if input.Type == validType {
				typeValid = true
				break
			}
		}
		if !typeValid {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Type '%s' không hợp lệ. Các giá trị hợp lệ: %v", input.Type, validTypes),
				common.StatusBadRequest,
				nil,
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
		model.Status = models.AIRunStatusPending // Mặc định
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
