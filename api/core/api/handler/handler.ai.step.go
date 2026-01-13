package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"
	"time"

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

// InsertOne override method InsertOne để validate schema theo standard schema
//
// LÝ DO PHẢI OVERRIDE:
// 1. Validate input/output schema phải match với standard schema cho từng step type
//    - Đảm bảo mapping chính xác giữa output của step này và input của step tiếp theo
//    - Cho phép mở rộng thêm fields nhưng không được thiếu required fields
// 2. Set CreatedAt và UpdatedAt tự động
// 3. Set OwnerOrganizationID từ context
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

		// Validate step type
		validTypes := []string{models.AIStepTypeGenerate, models.AIStepTypeJudge, models.AIStepTypeStepGeneration}
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

		// ✅ Validate input/output schema với standard schema
		isValid, errors := models.ValidateStepSchema(input.Type, input.InputSchema, input.OutputSchema)
		if !isValid {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Schema không hợp lệ. Chi tiết: %v", errors),
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// Chuyển đổi DTO sang Model
		now := time.Now().UnixMilli()
		aiStep := models.AIStep{
			Name:         input.Name,
			Description:  input.Description,
			Type:         input.Type,
			InputSchema:  input.InputSchema,
			OutputSchema: input.OutputSchema,
			TargetLevel:  input.TargetLevel,
			ParentLevel:  input.ParentLevel,
			Status:       input.Status,
			Metadata:     input.Metadata,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		// Convert PromptTemplateID từ string sang *ObjectID
		if input.PromptTemplateID != "" {
			promptTemplateID, err := primitive.ObjectIDFromHex(input.PromptTemplateID)
			if err != nil {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					fmt.Sprintf("PromptTemplateID không đúng định dạng: %v", err),
					common.StatusBadRequest,
					err,
				))
				return nil
			}
			aiStep.PromptTemplateID = &promptTemplateID
		}

		// ✅ Xử lý ownerOrganizationId: Lấy từ role context
		if activeRoleIDStr, ok := c.Locals("active_role_id").(string); ok && activeRoleIDStr != "" {
			if activeRoleID, err := primitive.ObjectIDFromHex(activeRoleIDStr); err == nil {
				// Lấy role để suy ra organization ID
				roleService, err := services.NewRoleService()
				if err == nil {
					if role, err := roleService.FindOneById(c.Context(), activeRoleID); err == nil {
						if !role.OwnerOrganizationID.IsZero() {
							aiStep.OwnerOrganizationID = role.OwnerOrganizationID
						}
					}
				}
			}
		}

		// Fallback: Nếu vẫn chưa có, thử lấy từ active_organization_id trong context
		if aiStep.OwnerOrganizationID.IsZero() {
			activeOrgID := h.getActiveOrganizationID(c)
			if activeOrgID != nil && !activeOrgID.IsZero() {
				aiStep.OwnerOrganizationID = *activeOrgID
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
		data, err := h.BaseService.InsertOne(ctx, aiStep)
		h.HandleResponse(c, data, err)
		return nil
	})
}

// Tất cả các CRUD operations khác đã được cung cấp bởi BaseHandler với transform tag tự động
// - UpdateById: Cập nhật AI step
// - FindOneById: Lấy AI step theo ID
// - FindWithPagination: Lấy danh sách AI step với phân trang
// - DeleteById: Xóa AI step theo ID
