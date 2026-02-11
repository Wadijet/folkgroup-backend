package aidto

// AIStepRenderPromptParams params từ URL khi render prompt cho step
type AIStepRenderPromptParams struct {
	ID string `uri:"id" validate:"required" transform:"str_objectid"`
}

// AIStepRenderPromptInput dữ liệu đầu vào khi render prompt cho step
type AIStepRenderPromptInput struct {
	Variables map[string]interface{} `json:"variables" validate:"required"`
}

// AIStepRenderPromptOutput dữ liệu đầu ra khi render prompt cho step
type AIStepRenderPromptOutput struct {
	RenderedPrompt    string                 `json:"renderedPrompt"`
	ProviderProfileID string                 `json:"providerProfileId"`
	Provider          string                 `json:"provider"`
	Model             string                 `json:"model"`
	Temperature       *float64               `json:"temperature,omitempty"`
	MaxTokens         *int                   `json:"maxTokens,omitempty"`
	Variables         map[string]interface{} `json:"variables"`
}
