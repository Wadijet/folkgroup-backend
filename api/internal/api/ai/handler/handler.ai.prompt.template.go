package aihdl

import (
	"fmt"
	aidto "meta_commerce/internal/api/ai/dto"
	aimodels "meta_commerce/internal/api/ai/models"
	basehdl "meta_commerce/internal/api/base/handler"
	aisvc "meta_commerce/internal/api/ai/service"
)

// AIPromptTemplateHandler xử lý các request liên quan (Module 2)
type AIPromptTemplateHandler struct {
	*basehdl.BaseHandler[aimodels.AIPromptTemplate, aidto.AIPromptTemplateCreateInput, aidto.AIPromptTemplateUpdateInput]
	AIPromptTemplateService *aisvc.AIPromptTemplateService
}

// NewAIPromptTemplateHandler tạo mới AIPromptTemplateHandler
func NewAIPromptTemplateHandler() (*AIPromptTemplateHandler, error) {
	aiPromptTemplateService, err := aisvc.NewAIPromptTemplateService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI prompt template service: %v", err)
	}

	hdl := &AIPromptTemplateHandler{
		AIPromptTemplateService: aiPromptTemplateService,
	}
	hdl.BaseHandler = basehdl.NewBaseHandler[aimodels.AIPromptTemplate, aidto.AIPromptTemplateCreateInput, aidto.AIPromptTemplateUpdateInput](aiPromptTemplateService.BaseServiceMongoImpl)

	return hdl, nil
}
