package aihdl

import (
	"fmt"
	aidto "meta_commerce/internal/api/ai/dto"
	aimodels "meta_commerce/internal/api/ai/models"
	basehdl "meta_commerce/internal/api/base/handler"
	aisvc "meta_commerce/internal/api/ai/service"
)

// AIProviderProfileHandler xử lý các request liên quan (Module 2)
type AIProviderProfileHandler struct {
	*basehdl.BaseHandler[aimodels.AIProviderProfile, aidto.AIProviderProfileCreateInput, aidto.AIProviderProfileUpdateInput]
	AIProviderProfileService *aisvc.AIProviderProfileService
}

// NewAIProviderProfileHandler tạo mới AIProviderProfileHandler
func NewAIProviderProfileHandler() (*AIProviderProfileHandler, error) {
	aiProviderProfileService, err := aisvc.NewAIProviderProfileService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI provider profile service: %v", err)
	}

	hdl := &AIProviderProfileHandler{
		AIProviderProfileService: aiProviderProfileService,
	}
	hdl.BaseHandler = basehdl.NewBaseHandler[aimodels.AIProviderProfile, aidto.AIProviderProfileCreateInput, aidto.AIProviderProfileUpdateInput](aiProviderProfileService.BaseServiceMongoImpl)

	return hdl, nil
}
