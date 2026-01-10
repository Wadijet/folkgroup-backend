package services

import (
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"
)

// AIStepService là service quản lý AI steps (Module 2)
type AIStepService struct {
	*BaseServiceMongoImpl[models.AIStep]
}

// NewAIStepService tạo mới AIStepService
// Trả về:
//   - *AIStepService: Instance mới của AIStepService
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIStepService() (*AIStepService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AISteps)
	if !exist {
		return nil, fmt.Errorf("failed to get ai_steps collection: %v", common.ErrNotFound)
	}

	return &AIStepService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.AIStep](collection),
	}, nil
}
