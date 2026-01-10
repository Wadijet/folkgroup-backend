package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AIPromptTemplateType định nghĩa các loại prompt template
const (
	AIPromptTemplateTypeGenerate      = "generate"      // Prompt cho GENERATE step
	AIPromptTemplateTypeJudge         = "judge"        // Prompt cho JUDGE step
	AIPromptTemplateTypeStepGeneration = "step_generation" // Prompt cho STEP_GENERATION step
)

// AIPromptTemplateVariable định nghĩa biến trong prompt template
type AIPromptTemplateVariable struct {
	Name        string `json:"name" bson:"name"`               // Tên biến (ví dụ: "layer", "parentContent")
	Description string `json:"description,omitempty" bson:"description,omitempty"` // Mô tả biến
	Required    bool   `json:"required" bson:"required"`       // Biến bắt buộc hay không
	Default     string `json:"default,omitempty" bson:"default,omitempty"` // Giá trị mặc định
}

// AIPromptTemplate đại diện cho prompt template (Module 2)
// Collection: ai_prompt_templates
type AIPromptTemplate struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"` // ID của prompt template

	// ===== BASIC INFO =====
	Name        string `json:"name" bson:"name" index:"text"`                    // Tên prompt template
	Description string `json:"description,omitempty" bson:"description,omitempty"` // Mô tả prompt template
	Type        string `json:"type" bson:"type" index:"single:1"`                // Loại: generate, judge, step_generation
	Version     string `json:"version" bson:"version" index:"single:1"`           // Version của prompt (semver)

	// ===== PROMPT CONTENT =====
	Prompt      string `json:"prompt" bson:"prompt"`                            // Nội dung prompt (có thể chứa variables: {{variableName}})
	Variables   []AIPromptTemplateVariable `json:"variables,omitempty" bson:"variables,omitempty"` // Danh sách biến trong prompt

	// ===== AI CONFIG =====
	ProviderProfileID *primitive.ObjectID `json:"providerProfileId,omitempty" bson:"providerProfileId,omitempty" index:"single:1"` // ID của AI provider profile (tùy chọn, có thể override trong workflow run)
	Model            string               `json:"model,omitempty" bson:"model,omitempty"` // Model name (ví dụ: "gpt-4", "claude-3-opus") (tùy chọn, có thể override)
	Temperature      *float64             `json:"temperature,omitempty" bson:"temperature,omitempty"` // Temperature (tùy chọn)
	MaxTokens        *int                 `json:"maxTokens,omitempty" bson:"maxTokens,omitempty"` // Max tokens (tùy chọn)

	// ===== STATUS =====
	Status string `json:"status" bson:"status" index:"single:1"` // Trạng thái: "active", "archived", "draft"

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu prompt template

	// ===== METADATA =====
	Metadata map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"` // Metadata bổ sung
	CreatedAt int64                  `json:"createdAt" bson:"createdAt" index:"single:1"` // Thời gian tạo
	UpdatedAt int64                  `json:"updatedAt" bson:"updatedAt"`                   // Thời gian cập nhật
}
