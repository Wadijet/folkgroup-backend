package aihdl

import (
	"fmt"
	aidto "meta_commerce/internal/api/ai/dto"
	aimodels "meta_commerce/internal/api/ai/models"
	basehdl "meta_commerce/internal/api/base/handler"
	aisvc "meta_commerce/internal/api/ai/service"
)

// AIGenerationBatchHandler xử lý các request liên quan (Module 2)
type AIGenerationBatchHandler struct {
	*basehdl.BaseHandler[aimodels.AIGenerationBatch, aidto.AIGenerationBatchCreateInput, aidto.AIGenerationBatchUpdateInput]
	AIGenerationBatchService *aisvc.AIGenerationBatchService
}

// NewAIGenerationBatchHandler tạo mới AIGenerationBatchHandler
func NewAIGenerationBatchHandler() (*AIGenerationBatchHandler, error) {
	aiGenerationBatchService, err := aisvc.NewAIGenerationBatchService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI generation batch service: %v", err)
	}

	hdl := &AIGenerationBatchHandler{
		AIGenerationBatchService: aiGenerationBatchService,
	}
	hdl.BaseHandler = basehdl.NewBaseHandler[aimodels.AIGenerationBatch, aidto.AIGenerationBatchCreateInput, aidto.AIGenerationBatchUpdateInput](aiGenerationBatchService.BaseServiceMongoImpl)

	return hdl, nil
}
