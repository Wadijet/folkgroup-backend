package dto

// AIStepRenderPromptParams params từ URL khi render prompt cho step
type AIStepRenderPromptParams struct {
	ID string `uri:"id" validate:"required" transform:"str_objectid"` // Step ID từ URL params - tự động validate và convert sang ObjectID
}

// AIStepRenderPromptInput dữ liệu đầu vào khi render prompt cho step
type AIStepRenderPromptInput struct {
	Variables map[string]interface{} `json:"variables" validate:"required"` // Variables để thay thế vào prompt template (từ step input)
}

// AIStepRenderPromptOutput dữ liệu đầu ra khi render prompt cho step
type AIStepRenderPromptOutput struct {
	RenderedPrompt   string                 `json:"renderedPrompt"`   // Prompt đã được render (TEXT)
	ProviderProfileID string                 `json:"providerProfileId"` // ID của provider profile (đã resolve)
	Provider          string                 `json:"provider"`         // Tên provider (openai, anthropic, etc.)
	Model             string                 `json:"model"`             // Model name (đã resolve từ prompt template hoặc provider default)
	Temperature       *float64               `json:"temperature,omitempty"` // Temperature (đã resolve)
	MaxTokens         *int                   `json:"maxTokens,omitempty"`   // Max tokens (đã resolve)
	Variables         map[string]interface{} `json:"variables"`              // Variables đã được sử dụng (để trace/debug)
}
