package aihdl

import (
	"fmt"
	aidto "meta_commerce/internal/api/ai/dto"
	aimodels "meta_commerce/internal/api/ai/models"
	basehdl "meta_commerce/internal/api/base/handler"
	aisvc "meta_commerce/internal/api/ai/service"
)

// AIWorkflowRunHandler xử lý các request liên quan (Module 2)
type AIWorkflowRunHandler struct {
	*basehdl.BaseHandler[aimodels.AIWorkflowRun, aidto.AIWorkflowRunCreateInput, aidto.AIWorkflowRunUpdateInput]
	AIWorkflowRunService *aisvc.AIWorkflowRunService
}

// NewAIWorkflowRunHandler tạo mới AIWorkflowRunHandler
func NewAIWorkflowRunHandler() (*AIWorkflowRunHandler, error) {
	aiWorkflowRunService, err := aisvc.NewAIWorkflowRunService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI workflow run service: %v", err)
	}

	hdl := &AIWorkflowRunHandler{
		AIWorkflowRunService: aiWorkflowRunService,
	}
	hdl.BaseHandler = basehdl.NewBaseHandler[aimodels.AIWorkflowRun, aidto.AIWorkflowRunCreateInput, aidto.AIWorkflowRunUpdateInput](aiWorkflowRunService)

	return hdl, nil
}
