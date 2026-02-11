package aidto

// AIRunCreateInput dữ liệu đầu vào khi tạo AI run
type AIRunCreateInput struct {
	Type              string                 `json:"type" validate:"required,oneof=GENERATE JUDGE"`
	Status            string                 `json:"status,omitempty" transform:"string,default=pending" validate:"omitempty,oneof=pending running completed failed"`
	PromptTemplateID  string                 `json:"promptTemplateId,omitempty" transform:"str_objectid_ptr,optional"`
	ProviderProfileID string                 `json:"providerProfileId,omitempty" transform:"str_objectid_ptr,optional"`
	Provider          string                 `json:"provider" validate:"required"`
	Model             string                 `json:"model" validate:"required"`
	Prompt            string                 `json:"prompt" validate:"required"`
	Variables         map[string]interface{} `json:"variables,omitempty"`
	InputSchema       map[string]interface{} `json:"inputSchema,omitempty"`
	StepRunID         string                 `json:"stepRunId,omitempty" transform:"str_objectid_ptr,optional"`
	WorkflowRunID     string                 `json:"workflowRunId,omitempty" transform:"str_objectid_ptr,optional"`
	ExperimentID      string                 `json:"experimentId,omitempty" transform:"str_objectid_ptr,optional"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

// AIRunUpdateInput dữ liệu đầu vào khi cập nhật AI run
type AIRunUpdateInput struct {
	Status            string                       `json:"status,omitempty"`
	Response          string                       `json:"response,omitempty"`
	ParsedOutput      map[string]interface{}       `json:"parsedOutput,omitempty"`
	OutputSchema      map[string]interface{}       `json:"outputSchema,omitempty"`
	Cost              *float64                     `json:"cost,omitempty"`
	Latency           *int64                       `json:"latency,omitempty"`
	InputTokens       *int                         `json:"inputTokens,omitempty"`
	OutputTokens      *int                         `json:"outputTokens,omitempty"`
	QualityScore      *float64                     `json:"qualityScore,omitempty"`
	Error             string                       `json:"error,omitempty"`
	ErrorDetails      map[string]interface{}       `json:"errorDetails,omitempty"`
	Messages          []AIConversationMessageInput `json:"messages,omitempty"`
	Reasoning         string                       `json:"reasoning,omitempty"`
	IntermediateSteps []map[string]interface{}    `json:"intermediateSteps,omitempty"`
	Metadata          map[string]interface{}       `json:"metadata,omitempty"`
}

// AIConversationMessageInput input cho conversation message
type AIConversationMessageInput struct {
	Role      string                 `json:"role" validate:"required"`
	Content   string                 `json:"content" validate:"required"`
	Timestamp *int64                 `json:"timestamp,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}
