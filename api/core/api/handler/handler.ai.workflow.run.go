package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
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
