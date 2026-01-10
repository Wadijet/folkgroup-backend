package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AIRunType định nghĩa các loại AI run
const (
	AIRunTypeGenerate = "GENERATE" // Generate content
	AIRunTypeJudge    = "JUDGE"    // Judge/scoring content
)

// AIRunStatus định nghĩa các trạng thái AI run
const (
	AIRunStatusPending   = "pending"   // Chờ xử lý
	AIRunStatusRunning   = "running"   // Đang chạy
	AIRunStatusCompleted = "completed" // Hoàn thành
	AIRunStatusFailed    = "failed"    // Thất bại
)

// AIRun đại diện cho AI run (Module 2)
// Collection: ai_runs
// Lưu tất cả AI API calls (GENERATE + JUDGE) với cost, latency, quality
type AIRun struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"` // ID của AI run

	// ===== BASIC INFO =====
	Type   string `json:"type" bson:"type" index:"single:1"`   // Loại: GENERATE, JUDGE
	Status string `json:"status" bson:"status" index:"single:1"` // Trạng thái: pending, running, completed, failed

	// ===== PROMPT TEMPLATE =====
	PromptTemplateID *primitive.ObjectID `json:"promptTemplateId,omitempty" bson:"promptTemplateId,omitempty" index:"single:1"` // ID của prompt template được sử dụng

	// ===== AI PROVIDER =====
	ProviderProfileID *primitive.ObjectID `json:"providerProfileId,omitempty" bson:"providerProfileId,omitempty" index:"single:1"` // ID của AI provider profile
	Provider          string               `json:"provider" bson:"provider" index:"single:1"`                                     // Provider name: "openai", "anthropic", "google", etc.
	Model             string               `json:"model" bson:"model" index:"single:1"`                                         // Model name: "gpt-4", "claude-3-opus", etc.

	// ===== PROMPT DATA =====
	Prompt      string                 `json:"prompt" bson:"prompt"`                           // Prompt đã được substitute variables (final prompt)
	Variables   map[string]interface{} `json:"variables,omitempty" bson:"variables,omitempty"` // Variables đã được substitute
	InputSchema map[string]interface{} `json:"inputSchema,omitempty" bson:"inputSchema,omitempty"` // Input schema (tùy chọn)

	// ===== RESPONSE DATA =====
	Response      string                 `json:"response,omitempty" bson:"response,omitempty"`         // Raw response từ AI API
	ParsedOutput  map[string]interface{} `json:"parsedOutput,omitempty" bson:"parsedOutput,omitempty"` // Parsed output (theo output schema)
	OutputSchema  map[string]interface{} `json:"outputSchema,omitempty" bson:"outputSchema,omitempty"` // Output schema (tùy chọn)

	// ===== COST & PERFORMANCE =====
	Cost          *float64 `json:"cost,omitempty" bson:"cost,omitempty"`                   // Cost (USD) của AI call
	Latency       *int64   `json:"latency,omitempty" bson:"latency,omitempty"`             // Latency (milliseconds)
	InputTokens   *int     `json:"inputTokens,omitempty" bson:"inputTokens,omitempty"`      // Số lượng input tokens
	OutputTokens  *int     `json:"outputTokens,omitempty" bson:"outputTokens,omitempty"`   // Số lượng output tokens
	QualityScore  *float64 `json:"qualityScore,omitempty" bson:"qualityScore,omitempty"` // Quality score (0.0 - 1.0) - từ judge hoặc human rating

	// ===== ERROR =====
	Error       string                 `json:"error,omitempty" bson:"error,omitempty"`         // Lỗi nếu có
	ErrorDetails map[string]interface{} `json:"errorDetails,omitempty" bson:"errorDetails,omitempty"` // Chi tiết lỗi

	// ===== REFERENCES =====
	StepRunID     *primitive.ObjectID `json:"stepRunId,omitempty" bson:"stepRunId,omitempty" index:"single:1"`     // ID của step run (nếu có)
	WorkflowRunID *primitive.ObjectID `json:"workflowRunId,omitempty" bson:"workflowRunId,omitempty" index:"single:1"` // ID của workflow run (nếu có)
	ExperimentID  *primitive.ObjectID `json:"experimentId,omitempty" bson:"experimentId,omitempty" index:"single:1"` // ID của experiment (nếu có, link về Module 3)

	// ===== TIMESTAMPS =====
	StartedAt   int64 `json:"startedAt,omitempty" bson:"startedAt,omitempty" index:"single:1"` // Thời gian bắt đầu
	CompletedAt int64 `json:"completedAt,omitempty" bson:"completedAt,omitempty"`               // Thời gian hoàn thành
	CreatedAt   int64 `json:"createdAt" bson:"createdAt" index:"single:1"`                      // Thời gian tạo

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu AI run

	// ===== METADATA =====
	Metadata map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"` // Metadata bổ sung
}
