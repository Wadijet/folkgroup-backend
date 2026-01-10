package services

import (
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"
)

// AIRunService là service quản lý AI runs (Module 2)
type AIRunService struct {
	*BaseServiceMongoImpl[models.AIRun]
}

// NewAIRunService tạo mới AIRunService
// Trả về:
//   - *AIRunService: Instance mới của AIRunService
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIRunService() (*AIRunService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AIRuns)
	if !exist {
		return nil, fmt.Errorf("failed to get ai_runs collection: %v", common.ErrNotFound)
	}

	return &AIRunService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.AIRun](collection),
	}, nil
}
