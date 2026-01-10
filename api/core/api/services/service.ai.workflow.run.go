package services

import (
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"
)

// AIWorkflowRunService là service quản lý AI workflow runs (Module 2)
type AIWorkflowRunService struct {
	*BaseServiceMongoImpl[models.AIWorkflowRun]
}

// NewAIWorkflowRunService tạo mới AIWorkflowRunService
// Trả về:
//   - *AIWorkflowRunService: Instance mới của AIWorkflowRunService
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIWorkflowRunService() (*AIWorkflowRunService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AIWorkflowRuns)
	if !exist {
		return nil, fmt.Errorf("failed to get ai_workflow_runs collection: %v", common.ErrNotFound)
	}

	return &AIWorkflowRunService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.AIWorkflowRun](collection),
	}, nil
}
