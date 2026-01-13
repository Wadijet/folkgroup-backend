package dto

// AIRunCreateInput dữ liệu đầu vào khi tạo AI run
type AIRunCreateInput struct {
	Type              string                 `json:"type" validate:"required"`     // Loại: GENERATE, JUDGE
	PromptTemplateID  string                 `json:"promptTemplateId,omitempty" transform:"str_objectid_ptr,optional"`   // ID của prompt template (dạng string ObjectID) - tự động convert sang *primitive.ObjectID
	ProviderProfileID string                 `json:"providerProfileId,omitempty" transform:"str_objectid_ptr,optional"`  // ID của AI provider profile (dạng string ObjectID) - tự động convert sang *primitive.ObjectID
	Provider          string                 `json:"provider" validate:"required"` // Provider name: "openai", "anthropic", "google", etc.
	Model             string                 `json:"model" validate:"required"`    // Model name: "gpt-4", "claude-3-opus", etc.
	Prompt            string                 `json:"prompt" validate:"required"`   // Prompt đã được substitute variables
	Variables         map[string]interface{} `json:"variables,omitempty"`          // Variables đã được substitute
	InputSchema       map[string]interface{} `json:"inputSchema,omitempty"`        // Input schema
	StepRunID         string                 `json:"stepRunId,omitempty" transform:"str_objectid_ptr,optional"`          // ID của step run (dạng string ObjectID) - tự động convert sang *primitive.ObjectID
	WorkflowRunID     string                 `json:"workflowRunId,omitempty" transform:"str_objectid_ptr,optional"`      // ID của workflow run (dạng string ObjectID) - tự động convert sang *primitive.ObjectID
	ExperimentID      string                 `json:"experimentId,omitempty" transform:"str_objectid_ptr,optional"`       // ID của experiment (dạng string ObjectID) - tự động convert sang *primitive.ObjectID
	Metadata          map[string]interface{} `json:"metadata,omitempty"`           // Metadata bổ sung
}

// AIRunUpdateInput dữ liệu đầu vào khi cập nhật AI run
type AIRunUpdateInput struct {
	Status            string                       `json:"status,omitempty"`            // Trạng thái: pending, running, completed, failed
	Response          string                       `json:"response,omitempty"`          // Raw response từ AI API
	ParsedOutput      map[string]interface{}       `json:"parsedOutput,omitempty"`      // Parsed output (theo output schema)
	OutputSchema      map[string]interface{}       `json:"outputSchema,omitempty"`      // Output schema
	Cost              *float64                     `json:"cost,omitempty"`              // Cost (USD) của AI call
	Latency           *int64                       `json:"latency,omitempty"`           // Latency (milliseconds)
	InputTokens       *int                         `json:"inputTokens,omitempty"`       // Số lượng input tokens
	OutputTokens      *int                         `json:"outputTokens,omitempty"`      // Số lượng output tokens
	QualityScore      *float64                     `json:"qualityScore,omitempty"`      // Quality score (0.0 - 1.0)
	Error             string                       `json:"error,omitempty"`             // Lỗi nếu có
	ErrorDetails      map[string]interface{}       `json:"errorDetails,omitempty"`      // Chi tiết lỗi
	Messages          []AIConversationMessageInput `json:"messages,omitempty"`          // Conversation messages (để update conversation history)
	Reasoning         string                       `json:"reasoning,omitempty"`         // Reasoning/thinking process của AI
	IntermediateSteps []map[string]interface{}     `json:"intermediateSteps,omitempty"` // Các bước trung gian trong quá trình xử lý
	Metadata          map[string]interface{}       `json:"metadata,omitempty"`          // Metadata bổ sung
}

// AIConversationMessageInput input cho conversation message
type AIConversationMessageInput struct {
	Role      string                 `json:"role" validate:"required"`    // Role: "system", "user", "assistant"
	Content   string                 `json:"content" validate:"required"` // Nội dung message (TEXT)
	Timestamp *int64                 `json:"timestamp,omitempty"`         // Thời gian message (milliseconds, nếu không có sẽ dùng now)
	Metadata  map[string]interface{} `json:"metadata,omitempty"`          // Metadata bổ sung (tokens, model, etc.)
}
