package services

import (
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"
)

// AIStepRunService là service quản lý AI step runs (Module 2)
type AIStepRunService struct {
	*BaseServiceMongoImpl[models.AIStepRun]
}

// NewAIStepRunService tạo mới AIStepRunService
// Trả về:
//   - *AIStepRunService: Instance mới của AIStepRunService
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIStepRunService() (*AIStepRunService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AIStepRuns)
	if !exist {
		return nil, fmt.Errorf("failed to get ai_step_runs collection: %v", common.ErrNotFound)
	}

	return &AIStepRunService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.AIStepRun](collection),
	}, nil
}
