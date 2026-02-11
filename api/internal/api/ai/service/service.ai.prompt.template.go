package aisvc

import (
	"fmt"
	"strings"

	aimodels "meta_commerce/internal/api/ai/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	basesvc "meta_commerce/internal/api/base/service"
)

// AIPromptTemplateService là service quản lý AI prompt templates (Module 2)
type AIPromptTemplateService struct {
	*basesvc.BaseServiceMongoImpl[aimodels.AIPromptTemplate]
}

// NewAIPromptTemplateService tạo mới AIPromptTemplateService
func NewAIPromptTemplateService() (*AIPromptTemplateService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AIPromptTemplates)
	if !exist {
		return nil, fmt.Errorf("failed to get ai_prompt_templates collection: %v", common.ErrNotFound)
	}
	return &AIPromptTemplateService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[aimodels.AIPromptTemplate](collection),
	}, nil
}

// RenderPrompt render prompt template với variables từ step input
func (s *AIPromptTemplateService) RenderPrompt(template *aimodels.AIPromptTemplate, variables map[string]interface{}) (string, error) {
	if template == nil {
		return "", fmt.Errorf("template is nil")
	}
	renderedPrompt := template.Prompt
	for _, variable := range template.Variables {
		value, exists := variables[variable.Name]
		if !exists {
			if variable.Default != "" {
				value = variable.Default
			} else if variable.Required {
				return "", fmt.Errorf("required variable '%s' is missing", variable.Name)
			} else {
				value = ""
			}
		}
		placeholder := "{{" + variable.Name + "}}"
		renderedPrompt = strings.ReplaceAll(renderedPrompt, placeholder, fmt.Sprintf("%v", value))
	}
	return renderedPrompt, nil
}
