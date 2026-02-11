package aihdl

import (
	"fmt"
	aidto "meta_commerce/internal/api/ai/dto"
	aimodels "meta_commerce/internal/api/ai/models"
	basehdl "meta_commerce/internal/api/base/handler"
	aisvc "meta_commerce/internal/api/ai/service"
)

// AIStepRunHandler xử lý các request liên quan (Module 2)
type AIStepRunHandler struct {
	*basehdl.BaseHandler[aimodels.AIStepRun, aidto.AIStepRunCreateInput, aidto.AIStepRunUpdateInput]
	AIStepRunService *aisvc.AIStepRunService
}

// NewAIStepRunHandler tạo mới AIStepRunHandler
func NewAIStepRunHandler() (*AIStepRunHandler, error) {
	aiStepRunService, err := aisvc.NewAIStepRunService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI step run service: %v", err)
	}

	hdl := &AIStepRunHandler{
		AIStepRunService: aiStepRunService,
	}
	hdl.BaseHandler = basehdl.NewBaseHandler[aimodels.AIStepRun, aidto.AIStepRunCreateInput, aidto.AIStepRunUpdateInput](aiStepRunService.BaseServiceMongoImpl)

	return hdl, nil
}
