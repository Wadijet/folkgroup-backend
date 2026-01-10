package services

import (
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"
)

// AIWorkflowService là service quản lý AI workflows (Module 2)
type AIWorkflowService struct {
	*BaseServiceMongoImpl[models.AIWorkflow]
}

// NewAIWorkflowService tạo mới AIWorkflowService
// Trả về:
//   - *AIWorkflowService: Instance mới của AIWorkflowService
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIWorkflowService() (*AIWorkflowService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AIWorkflows)
	if !exist {
		return nil, fmt.Errorf("failed to get ai_workflows collection: %v", common.ErrNotFound)
	}

	return &AIWorkflowService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.AIWorkflow](collection),
	}, nil
}
