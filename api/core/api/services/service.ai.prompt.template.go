package services

import (
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"
	"strings"
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

// RenderPrompt render prompt template với variables từ step input
// Tham số:
//   - template: Prompt template cần render
//   - variables: Map các biến và giá trị để thay thế vào prompt (từ step input)
// Trả về:
//   - string: Prompt đã được render (đã thay thế tất cả variables)
//   - error: Lỗi nếu có
func (s *AIPromptTemplateService) RenderPrompt(template *models.AIPromptTemplate, variables map[string]interface{}) (string, error) {
	if template == nil {
		return "", fmt.Errorf("template is nil")
	}

	// Bắt đầu với prompt text gốc
	renderedPrompt := template.Prompt

	// Thay thế từng variable trong prompt
	for _, variable := range template.Variables {
		// Lấy giá trị từ variables map
		value, exists := variables[variable.Name]
		if !exists {
			// Nếu không có trong variables, dùng default value nếu có
			if variable.Default != "" {
				value = variable.Default
			} else if variable.Required {
				// Nếu là required variable nhưng không có giá trị → lỗi
				return "", fmt.Errorf("required variable '%s' is missing", variable.Name)
			} else {
				// Optional variable không có giá trị → để trống
				value = ""
			}
		}

		// Thay thế placeholder {{variableName}} bằng giá trị
		placeholder := "{{" + variable.Name + "}}"
		renderedPrompt = strings.ReplaceAll(renderedPrompt, placeholder, fmt.Sprintf("%v", value))
	}

	return renderedPrompt, nil
}
