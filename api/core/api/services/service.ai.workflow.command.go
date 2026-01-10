package services

import (
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"
)

// AIWorkflowCommandService là service quản lý AI workflow commands (Module 2)
type AIWorkflowCommandService struct {
	*BaseServiceMongoImpl[models.AIWorkflowCommand]
}

// NewAIWorkflowCommandService tạo mới AIWorkflowCommandService
// Trả về:
//   - *AIWorkflowCommandService: Instance mới của AIWorkflowCommandService
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIWorkflowCommandService() (*AIWorkflowCommandService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AIWorkflowCommands)
	if !exist {
		return nil, fmt.Errorf("failed to get ai_workflow_commands collection: %v", common.ErrNotFound)
	}

	return &AIWorkflowCommandService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.AIWorkflowCommand](collection),
	}, nil
}
