package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
)

// AIWorkflowCommandHandler xử lý các request liên quan đến AI Workflow Command (Module 2)
type AIWorkflowCommandHandler struct {
	*BaseHandler[models.AIWorkflowCommand, dto.AIWorkflowCommandCreateInput, dto.AIWorkflowCommandUpdateInput]
	AIWorkflowCommandService *services.AIWorkflowCommandService
}

// NewAIWorkflowCommandHandler tạo mới AIWorkflowCommandHandler
// Trả về:
//   - *AIWorkflowCommandHandler: Instance mới của AIWorkflowCommandHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIWorkflowCommandHandler() (*AIWorkflowCommandHandler, error) {
	aiWorkflowCommandService, err := services.NewAIWorkflowCommandService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI workflow command service: %v", err)
	}

	handler := &AIWorkflowCommandHandler{
		AIWorkflowCommandService: aiWorkflowCommandService,
	}
	handler.BaseHandler = NewBaseHandler[models.AIWorkflowCommand, dto.AIWorkflowCommandCreateInput, dto.AIWorkflowCommandUpdateInput](aiWorkflowCommandService.BaseServiceMongoImpl)

	return handler, nil
}
