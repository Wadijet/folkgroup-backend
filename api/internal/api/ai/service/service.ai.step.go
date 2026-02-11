package aisvc

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"

	aimodels "meta_commerce/internal/api/ai/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	basesvc "meta_commerce/internal/api/base/service"
)

// AIStepService là service quản lý AI steps (Module 2)
type AIStepService struct {
	*basesvc.BaseServiceMongoImpl[aimodels.AIStep]
}

// NewAIStepService tạo mới AIStepService
func NewAIStepService() (*AIStepService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AISteps)
	if !exist {
		return nil, fmt.Errorf("failed to get ai_steps collection: %v", common.ErrNotFound)
	}
	return &AIStepService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[aimodels.AIStep](collection),
	}, nil
}

// RenderPromptForStep render prompt template cho step với variables và resolve AI config
func (s *AIStepService) RenderPromptForStep(ctx context.Context, stepID primitive.ObjectID, variables map[string]interface{}) (
	renderedPrompt string, providerProfileID string, provider string, model string,
	temperature *float64, maxTokens *int, err error,
) {
	step, err := s.FindOneById(ctx, stepID)
	if err != nil {
		return "", "", "", "", nil, nil, fmt.Errorf("failed to load step: %w", err)
	}
	if step.PromptTemplateID == nil {
		return "", "", "", "", nil, nil, fmt.Errorf("step does not have prompt template")
	}
	promptTemplateService, err := NewAIPromptTemplateService()
	if err != nil {
		return "", "", "", "", nil, nil, fmt.Errorf("failed to create prompt template service: %w", err)
	}
	template, err := promptTemplateService.FindOneById(ctx, *step.PromptTemplateID)
	if err != nil {
		return "", "", "", "", nil, nil, fmt.Errorf("failed to load prompt template: %w", err)
	}
	renderedPrompt, err = promptTemplateService.RenderPrompt(&template, variables)
	if err != nil {
		return "", "", "", "", nil, nil, fmt.Errorf("failed to render prompt: %w", err)
	}
	var finalProviderProfileID *primitive.ObjectID
	var finalModel string
	var finalTemperature *float64
	var finalMaxTokens *int
	var finalProvider string
	if template.Provider != nil && template.Provider.ProfileID != nil {
		finalProviderProfileID = template.Provider.ProfileID
	}
	if finalProviderProfileID != nil {
		providerProfileService, err := NewAIProviderProfileService()
		if err == nil {
			providerProfile, err := providerProfileService.FindOneById(ctx, *finalProviderProfileID)
			if err == nil {
				finalProvider = providerProfile.Provider
				if template.Provider != nil && template.Provider.Config != nil && template.Provider.Config.Model != "" {
					finalModel = template.Provider.Config.Model
				} else if providerProfile.Config != nil && providerProfile.Config.Model != "" {
					finalModel = providerProfile.Config.Model
				}
				if template.Provider != nil && template.Provider.Config != nil && template.Provider.Config.Temperature != nil {
					finalTemperature = template.Provider.Config.Temperature
				} else if providerProfile.Config != nil {
					finalTemperature = providerProfile.Config.Temperature
				}
				if template.Provider != nil && template.Provider.Config != nil && template.Provider.Config.MaxTokens != nil {
					finalMaxTokens = template.Provider.Config.MaxTokens
				} else if providerProfile.Config != nil {
					finalMaxTokens = providerProfile.Config.MaxTokens
				}
			}
		}
	} else {
		if template.Provider != nil && template.Provider.Config != nil {
			finalModel = template.Provider.Config.Model
			finalTemperature = template.Provider.Config.Temperature
			finalMaxTokens = template.Provider.Config.MaxTokens
		}
	}
	providerProfileIDStr := ""
	if finalProviderProfileID != nil {
		providerProfileIDStr = finalProviderProfileID.Hex()
	}
	return renderedPrompt, providerProfileIDStr, finalProvider, finalModel, finalTemperature, finalMaxTokens, nil
}

// ValidateSchema validate input/output schema với standard schema
func (s *AIStepService) ValidateSchema(stepType string, inputSchema map[string]interface{}, outputSchema map[string]interface{}) error {
	isValid, errors := aimodels.ValidateStepSchema(stepType, inputSchema, outputSchema)
	if !isValid {
		return common.NewError(common.ErrCodeValidationFormat, fmt.Sprintf("Schema không hợp lệ. Chi tiết: %v", errors), common.StatusBadRequest, nil)
	}
	return nil
}

// InsertOne override để set schema từ standard và validate
func (s *AIStepService) InsertOne(ctx context.Context, data aimodels.AIStep) (aimodels.AIStep, error) {
	stdInputSchema, stdOutputSchema, err := aimodels.GetStandardSchema(data.Type, data.TargetLevel, data.ParentLevel)
	if err != nil {
		return data, common.NewError(common.ErrCodeValidationFormat, fmt.Sprintf("Không thể lấy standard schema: %v", err), common.StatusBadRequest, nil)
	}
	data.InputSchema = stdInputSchema
	data.OutputSchema = stdOutputSchema
	if err := s.ValidateSchema(data.Type, stdInputSchema, stdOutputSchema); err != nil {
		return data, err
	}
	return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}

// UpdateById override để enforce schema fix cứng khi update
func (s *AIStepService) UpdateById(ctx context.Context, id primitive.ObjectID, data interface{}) (aimodels.AIStep, error) {
	updateData, err := basesvc.ToUpdateData(data)
	if err != nil {
		var zero aimodels.AIStep
		return zero, err
	}
	currentStep, err := s.FindOneById(ctx, id)
	if err != nil {
		var zero aimodels.AIStep
		return zero, err
	}
	stepType := currentStep.Type
	targetLevel := currentStep.TargetLevel
	parentLevel := currentStep.ParentLevel
	if updateData.Set != nil {
		if newType, ok := updateData.Set["type"].(string); ok && newType != "" {
			stepType = newType
		}
		if newTargetLevel, ok := updateData.Set["targetLevel"].(string); ok {
			targetLevel = newTargetLevel
		}
		if newParentLevel, ok := updateData.Set["parentLevel"].(string); ok {
			parentLevel = newParentLevel
		}
	}
	stdInputSchema, stdOutputSchema, err := aimodels.GetStandardSchema(stepType, targetLevel, parentLevel)
	if err != nil {
		var zero aimodels.AIStep
		return zero, common.NewError(common.ErrCodeValidationFormat, fmt.Sprintf("Không thể lấy standard schema: %v", err), common.StatusBadRequest, nil)
	}
	if updateData.Set == nil {
		updateData.Set = make(map[string]interface{})
	}
	updateData.Set["inputSchema"] = stdInputSchema
	updateData.Set["outputSchema"] = stdOutputSchema
	if err := s.ValidateSchema(stepType, stdInputSchema, stdOutputSchema); err != nil {
		var zero aimodels.AIStep
		return zero, err
	}
	return s.BaseServiceMongoImpl.UpdateById(ctx, id, *updateData)
}
