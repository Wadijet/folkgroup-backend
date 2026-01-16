package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AIStepType định nghĩa các loại step
const (
	AIStepTypeGenerate      = "GENERATE"      // Generate content candidates
	AIStepTypeJudge         = "JUDGE"         // Judge/scoring candidates
	AIStepTypeStepGeneration = "STEP_GENERATION" // Dynamic step generation
)

// AIStepInputSchema định nghĩa input schema cho step (JSON schema)
type AIStepInputSchema map[string]interface{}

// AIStepOutputSchema định nghĩa output schema cho step (JSON schema)
type AIStepOutputSchema map[string]interface{}

// AIStep đại diện cho step definition (Module 2)
// Collection: ai_steps
type AIStep struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"` // ID của step

	// ===== BASIC INFO =====
	Name        string `json:"name" bson:"name" index:"text"`                    // Tên step
	Description string `json:"description,omitempty" bson:"description,omitempty"` // Mô tả step
	Type        string `json:"type" bson:"type" index:"single:1"`                // Loại step: GENERATE, JUDGE, STEP_GENERATION

	// ===== PROMPT TEMPLATE =====
	PromptTemplateID *primitive.ObjectID `json:"promptTemplateId,omitempty" bson:"promptTemplateId,omitempty" index:"single:1"` // ID của prompt template

	// ===== SCHEMAS =====
	InputSchema  AIStepInputSchema  `json:"inputSchema" bson:"inputSchema"`   // Input schema (JSON schema format)
	OutputSchema AIStepOutputSchema `json:"outputSchema" bson:"outputSchema"` // Output schema (JSON schema format)

	// ===== CONFIG =====
	TargetLevel string `json:"targetLevel,omitempty" bson:"targetLevel,omitempty" index:"single:1"` // Level mục tiêu: "L1", "L2", ..., "L8" (tùy chọn)
	ParentLevel string `json:"parentLevel,omitempty" bson:"parentLevel,omitempty" index:"single:1"` // Level của parent: "L1", "L2", ..., "L8" (tùy chọn)

	// ===== STATUS =====
	Status string `json:"status" bson:"status" index:"single:1"` // Trạng thái: "active", "archived", "draft"

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu step

	// ===== METADATA =====
	Metadata map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"` // Metadata bổ sung
	CreatedAt int64                  `json:"createdAt" bson:"createdAt" index:"single:1"` // Thời gian tạo
	UpdatedAt int64                  `json:"updatedAt" bson:"updatedAt"`                   // Thời gian cập nhật
}
