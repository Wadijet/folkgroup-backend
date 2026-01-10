package dto

// AIRunCreateInput dữ liệu đầu vào khi tạo AI run
type AIRunCreateInput struct {
	Type            string                 `json:"type" validate:"required"`                    // Loại: GENERATE, JUDGE
	PromptTemplateID string                `json:"promptTemplateId,omitempty"`                  // ID của prompt template (dạng string ObjectID)
	ProviderProfileID string               `json:"providerProfileId,omitempty"`                 // ID của AI provider profile (dạng string ObjectID)
	Provider        string                 `json:"provider" validate:"required"`                 // Provider name: "openai", "anthropic", "google", etc.
	Model           string                 `json:"model" validate:"required"`                  // Model name: "gpt-4", "claude-3-opus", etc.
	Prompt          string                 `json:"prompt" validate:"required"`                  // Prompt đã được substitute variables
	Variables       map[string]interface{} `json:"variables,omitempty"`                         // Variables đã được substitute
	InputSchema     map[string]interface{} `json:"inputSchema,omitempty"`                       // Input schema
	StepRunID       string                 `json:"stepRunId,omitempty"`                          // ID của step run (dạng string ObjectID)
	WorkflowRunID   string                 `json:"workflowRunId,omitempty"`                     // ID của workflow run (dạng string ObjectID)
	ExperimentID    string                 `json:"experimentId,omitempty"`                       // ID của experiment (dạng string ObjectID)
	Metadata        map[string]interface{} `json:"metadata,omitempty"`                          // Metadata bổ sung
}

// AIRunUpdateInput dữ liệu đầu vào khi cập nhật AI run
type AIRunUpdateInput struct {
	Status        string                 `json:"status,omitempty"`                 // Trạng thái: pending, running, completed, failed
	Response      string                 `json:"response,omitempty"`               // Raw response từ AI API
	ParsedOutput  map[string]interface{} `json:"parsedOutput,omitempty"`           // Parsed output (theo output schema)
	OutputSchema  map[string]interface{} `json:"outputSchema,omitempty"`          // Output schema
	Cost          *float64               `json:"cost,omitempty"`                  // Cost (USD) của AI call
	Latency       *int64                 `json:"latency,omitempty"`                // Latency (milliseconds)
	InputTokens   *int                   `json:"inputTokens,omitempty"`           // Số lượng input tokens
	OutputTokens  *int                   `json:"outputTokens,omitempty"`          // Số lượng output tokens
	QualityScore  *float64               `json:"qualityScore,omitempty"`           // Quality score (0.0 - 1.0)
	Error         string                 `json:"error,omitempty"`                 // Lỗi nếu có
	ErrorDetails  map[string]interface{} `json:"errorDetails,omitempty"`           // Chi tiết lỗi
	Metadata      map[string]interface{} `json:"metadata,omitempty"`               // Metadata bổ sung
}
