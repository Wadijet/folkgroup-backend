package aisvc

import (
	"fmt"
	aimodels "meta_commerce/internal/api/ai/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	basesvc "meta_commerce/internal/api/base/service"
)

// AIWorkflowService là service quản lý AI workflows (Module 2)
type AIWorkflowService struct {
	*basesvc.BaseServiceMongoImpl[aimodels.AIWorkflow]
}

// NewAIWorkflowService tạo mới AIWorkflowService
func NewAIWorkflowService() (*AIWorkflowService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AIWorkflows)
	if !exist {
		return nil, fmt.Errorf("failed to get ai_workflows collection: %v", common.ErrNotFound)
	}
	return &AIWorkflowService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[aimodels.AIWorkflow](collection),
	}, nil
}
