package aisvc

import (
	"fmt"
	aimodels "meta_commerce/internal/api/ai/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	basesvc "meta_commerce/internal/api/base/service"
)

// AIGenerationBatchService là service quản lý AI generation batches (Module 2)
type AIGenerationBatchService struct {
	*basesvc.BaseServiceMongoImpl[aimodels.AIGenerationBatch]
}

// NewAIGenerationBatchService tạo mới AIGenerationBatchService
func NewAIGenerationBatchService() (*AIGenerationBatchService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AIGenerationBatches)
	if !exist {
		return nil, fmt.Errorf("failed to get ai_generation_batches collection: %v", common.ErrNotFound)
	}
	return &AIGenerationBatchService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[aimodels.AIGenerationBatch](collection),
	}, nil
}
