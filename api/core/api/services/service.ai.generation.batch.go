package services

import (
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"
)

// AIGenerationBatchService là service quản lý AI generation batches (Module 2)
type AIGenerationBatchService struct {
	*BaseServiceMongoImpl[models.AIGenerationBatch]
}

// NewAIGenerationBatchService tạo mới AIGenerationBatchService
// Trả về:
//   - *AIGenerationBatchService: Instance mới của AIGenerationBatchService
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIGenerationBatchService() (*AIGenerationBatchService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AIGenerationBatches)
	if !exist {
		return nil, fmt.Errorf("failed to get ai_generation_batches collection: %v", common.ErrNotFound)
	}

	return &AIGenerationBatchService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.AIGenerationBatch](collection),
	}, nil
}
