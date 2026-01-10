package services

import (
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"
)

// AIPromptTemplateService là service quản lý AI prompt templates (Module 2)
type AIPromptTemplateService struct {
	*BaseServiceMongoImpl[models.AIPromptTemplate]
}

// NewAIPromptTemplateService tạo mới AIPromptTemplateService
// Trả về:
//   - *AIPromptTemplateService: Instance mới của AIPromptTemplateService
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIPromptTemplateService() (*AIPromptTemplateService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AIPromptTemplates)
	if !exist {
		return nil, fmt.Errorf("failed to get ai_prompt_templates collection: %v", common.ErrNotFound)
	}

	return &AIPromptTemplateService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.AIPromptTemplate](collection),
	}, nil
}
