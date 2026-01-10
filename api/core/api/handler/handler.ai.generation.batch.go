package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
)

// AIGenerationBatchHandler xử lý các request liên quan đến AI Generation Batch (Module 2)
type AIGenerationBatchHandler struct {
	*BaseHandler[models.AIGenerationBatch, dto.AIGenerationBatchCreateInput, dto.AIGenerationBatchUpdateInput]
	AIGenerationBatchService *services.AIGenerationBatchService
}

// NewAIGenerationBatchHandler tạo mới AIGenerationBatchHandler
// Trả về:
//   - *AIGenerationBatchHandler: Instance mới của AIGenerationBatchHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIGenerationBatchHandler() (*AIGenerationBatchHandler, error) {
	aiGenerationBatchService, err := services.NewAIGenerationBatchService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI generation batch service: %v", err)
	}

	handler := &AIGenerationBatchHandler{
		AIGenerationBatchService: aiGenerationBatchService,
	}
	handler.BaseHandler = NewBaseHandler[models.AIGenerationBatch, dto.AIGenerationBatchCreateInput, dto.AIGenerationBatchUpdateInput](aiGenerationBatchService.BaseServiceMongoImpl)

	return handler, nil
}
