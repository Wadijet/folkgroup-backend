package dto

// AIPromptTemplateVariableInput input cho prompt template variable
type AIPromptTemplateVariableInput struct {
	Name        string `json:"name" validate:"required"`        // Tên biến
	Description string `json:"description,omitempty"`            // Mô tả biến
	Required    bool   `json:"required"`                        // Biến bắt buộc hay không
	Default     string `json:"default,omitempty"`               // Giá trị mặc định
}

// AIConfigInput input cho AI config (dùng chung cho cả default và override)
type AIConfigInput struct {
	Model          string                 `json:"model,omitempty"`          // Model name (ví dụ: "gpt-4", "claude-3-opus")
	Temperature    *float64               `json:"temperature,omitempty"`    // Temperature
	MaxTokens      *int                   `json:"maxTokens,omitempty"`      // Max tokens
	ProviderConfig map[string]interface{} `json:"providerConfig,omitempty"`  // Provider-specific config (ví dụ: OpenAI topP, Anthropic maxTokensToSample)
	PricingConfig  map[string]interface{} `json:"pricingConfig,omitempty"`   // Pricing config: {"gpt-4": {"input": 0.03, "output": 0.06}, ...}
}

// AIPromptTemplateProviderInput input cho provider info của prompt template
type AIPromptTemplateProviderInput struct {
	ProfileID string        `json:"profileId,omitempty" validate:"omitempty,exists=ai_provider_profiles" transform:"str_objectid_ptr,optional"` // ID của AI provider profile (tùy chọn) - reference đến provider profile
	Config    *AIConfigInput `json:"config,omitempty" transform:"nested_struct"`                                                                  // AI config override (model, temperature, maxTokens, providerConfig) - override từ provider profile defaultConfig
}

// AIPromptTemplateCreateInput dữ liệu đầu vào khi tạo AI prompt template
type AIPromptTemplateCreateInput struct {
	Name        string                          `json:"name" validate:"required"`                    // Tên prompt template
	Description string                          `json:"description,omitempty"`                       // Mô tả prompt template
	Type        string                          `json:"type" validate:"required,oneof=generate judge step_generation"`                   // Loại: generate, judge, step_generation
	Version     string                          `json:"version" validate:"required"`                // Version của prompt (semver)
	Prompt      string                          `json:"prompt" validate:"required"`                 // Nội dung prompt (có thể chứa variables: {{variableName}})
	Variables   []AIPromptTemplateVariableInput `json:"variables,omitempty"`                         // Danh sách biến trong prompt
	Provider    *AIPromptTemplateProviderInput  `json:"provider,omitempty" transform:"nested_struct"` // Provider info (profileId, config) - override từ provider profile defaultConfig
	Status      string                          `json:"status,omitempty" transform:"string,default=active" validate:"omitempty,oneof=active archived draft"` // Trạng thái: "active", "archived", "draft" (mặc định: "active")
	Metadata    map[string]interface{}         `json:"metadata,omitempty"`                          // Metadata bổ sung
}

// AIPromptTemplateUpdateInput dữ liệu đầu vào khi cập nhật AI prompt template
type AIPromptTemplateUpdateInput struct {
	Name        string                          `json:"name,omitempty"`                              // Tên prompt template
	Description string                          `json:"description,omitempty"`                       // Mô tả prompt template
	Type        string                          `json:"type,omitempty"`                              // Loại
	Version     string                          `json:"version,omitempty"`                           // Version của prompt
	Prompt      string                          `json:"prompt,omitempty"`                             // Nội dung prompt
	Variables   []AIPromptTemplateVariableInput `json:"variables,omitempty"`                        // Danh sách biến
	Provider    *AIPromptTemplateProviderInput  `json:"provider,omitempty" transform:"nested_struct"` // Provider info (profileId, config) - override từ provider profile defaultConfig
	Status      string                          `json:"status,omitempty"`                            // Trạng thái
	Metadata    map[string]interface{}         `json:"metadata,omitempty"`                          // Metadata bổ sung
}
