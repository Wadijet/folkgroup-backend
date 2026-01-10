package handler

import (
	"fmt"
	"time"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"
	"meta_commerce/core/utility"

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

// InsertOne override method InsertOne để chuyển đổi từ DTO sang Model
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

		// Validate type
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

		// Validate status
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
		aiStep := models.AIStep{
			Name:        input.Name,
			Description: input.Description,
			Type:        input.Type,
			InputSchema: models.AIStepInputSchema(input.InputSchema),
			OutputSchema: models.AIStepOutputSchema(input.OutputSchema),
			TargetLevel: input.TargetLevel,
			ParentLevel: input.ParentLevel,
			Status:      input.Status,
			Metadata:    input.Metadata,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		// Xử lý PromptTemplateID nếu có
		if input.PromptTemplateID != "" {
			if !primitive.IsValidObjectID(input.PromptTemplateID) {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					fmt.Sprintf("PromptTemplateID '%s' không đúng định dạng MongoDB ObjectID", input.PromptTemplateID),
					common.StatusBadRequest,
					nil,
				))
				return nil
			}
			promptTemplateID := utility.String2ObjectID(input.PromptTemplateID)
			aiStep.PromptTemplateID = &promptTemplateID
		}

		// Thực hiện insert
		ctx := c.Context()
		data, err := h.BaseService.InsertOne(ctx, aiStep)
		h.HandleResponse(c, data, err)
		return nil
	})
}
