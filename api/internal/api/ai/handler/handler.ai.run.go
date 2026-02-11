package aihdl

import (
	"fmt"
	aidto "meta_commerce/internal/api/ai/dto"
	aimodels "meta_commerce/internal/api/ai/models"
	basehdl "meta_commerce/internal/api/base/handler"
	aisvc "meta_commerce/internal/api/ai/service"
)

// AIRunHandler xử lý các request liên quan (Module 2)
type AIRunHandler struct {
	*basehdl.BaseHandler[aimodels.AIRun, aidto.AIRunCreateInput, aidto.AIRunUpdateInput]
	AIRunService *aisvc.AIRunService
}

// NewAIRunHandler tạo mới AIRunHandler
func NewAIRunHandler() (*AIRunHandler, error) {
	aiRunService, err := aisvc.NewAIRunService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI run service: %v", err)
	}

	hdl := &AIRunHandler{
		AIRunService: aiRunService,
	}
	hdl.BaseHandler = basehdl.NewBaseHandler[aimodels.AIRun, aidto.AIRunCreateInput, aidto.AIRunUpdateInput](aiRunService.BaseServiceMongoImpl)

	return hdl, nil
}
