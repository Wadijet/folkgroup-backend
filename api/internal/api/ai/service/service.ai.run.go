package aisvc

import (
	"fmt"
	aimodels "meta_commerce/internal/api/ai/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	basesvc "meta_commerce/internal/api/base/service"
)

// AIRunService là service quản lý AI runs (Module 2)
type AIRunService struct {
	*basesvc.BaseServiceMongoImpl[aimodels.AIRun]
}

// NewAIRunService tạo mới AIRunService
func NewAIRunService() (*AIRunService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AIRuns)
	if !exist {
		return nil, fmt.Errorf("failed to get ai_runs collection: %v", common.ErrNotFound)
	}
	return &AIRunService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[aimodels.AIRun](collection),
	}, nil
}
