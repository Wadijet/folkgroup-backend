package aisvc

import (
	"fmt"
	aimodels "meta_commerce/internal/api/ai/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	basesvc "meta_commerce/internal/api/base/service"
)

// AIStepRunService là service quản lý AI step runs (Module 2)
type AIStepRunService struct {
	*basesvc.BaseServiceMongoImpl[aimodels.AIStepRun]
}

// NewAIStepRunService tạo mới AIStepRunService
func NewAIStepRunService() (*AIStepRunService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AIStepRuns)
	if !exist {
		return nil, fmt.Errorf("failed to get ai_step_runs collection: %v", common.ErrNotFound)
	}
	return &AIStepRunService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[aimodels.AIStepRun](collection),
	}, nil
}
