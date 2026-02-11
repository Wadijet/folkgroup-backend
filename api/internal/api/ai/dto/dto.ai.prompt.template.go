package aidto

// AIPromptTemplateVariableInput input cho prompt template variable
type AIPromptTemplateVariableInput struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
}

// AIConfigInput input cho AI config (dùng chung cho cả default và override)
type AIConfigInput struct {
	Model          string                 `json:"model,omitempty"`
	Temperature    *float64               `json:"temperature,omitempty"`
	MaxTokens      *int                   `json:"maxTokens,omitempty"`
	ProviderConfig map[string]interface{} `json:"providerConfig,omitempty"`
	PricingConfig  map[string]interface{} `json:"pricingConfig,omitempty"`
}

// AIPromptTemplateProviderInput input cho provider info của prompt template
type AIPromptTemplateProviderInput struct {
	ProfileID string         `json:"profileId,omitempty" validate:"omitempty,exists=ai_provider_profiles" transform:"str_objectid_ptr,optional"`
	Config    *AIConfigInput `json:"config,omitempty" transform:"nested_struct"`
}

// AIPromptTemplateCreateInput dữ liệu đầu vào khi tạo AI prompt template
type AIPromptTemplateCreateInput struct {
	Name        string                          `json:"name" validate:"required"`
	Description string                          `json:"description,omitempty"`
	Type        string                          `json:"type" validate:"required,oneof=generate judge step_generation"`
	Version     string                          `json:"version" validate:"required"`
	Prompt      string                          `json:"prompt" validate:"required"`
	Variables   []AIPromptTemplateVariableInput `json:"variables,omitempty"`
	Provider    *AIPromptTemplateProviderInput  `json:"provider,omitempty" transform:"nested_struct"`
	Status      string                          `json:"status,omitempty" transform:"string,default=active" validate:"omitempty,oneof=active archived draft"`
	Metadata    map[string]interface{}          `json:"metadata,omitempty"`
}

// AIPromptTemplateUpdateInput dữ liệu đầu vào khi cập nhật AI prompt template
type AIPromptTemplateUpdateInput struct {
	Name        string                          `json:"name,omitempty"`
	Description string                          `json:"description,omitempty"`
	Type        string                          `json:"type,omitempty"`
	Version     string                          `json:"version,omitempty"`
	Prompt      string                          `json:"prompt,omitempty"`
	Variables   []AIPromptTemplateVariableInput `json:"variables,omitempty"`
	Provider    *AIPromptTemplateProviderInput  `json:"provider,omitempty" transform:"nested_struct"`
	Status      string                          `json:"status,omitempty"`
	Metadata    map[string]interface{}          `json:"metadata,omitempty"`
}
