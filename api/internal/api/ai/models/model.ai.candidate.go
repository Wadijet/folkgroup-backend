package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AICandidate đại diện cho content candidate (Module 2)
// Collection: ai_candidates
type AICandidate struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"` // ID của candidate

	// ===== REFERENCES =====
	GenerationBatchID primitive.ObjectID `json:"generationBatchId" bson:"generationBatchId" index:"single:1"` // ID của generation batch
	StepRunID         primitive.ObjectID `json:"stepRunId" bson:"stepRunId" index:"single:1"`                 // ID của step run tạo ra candidate này

	// ===== CONTENT =====
	Text     string                 `json:"text" bson:"text" index:"text"`                           // Nội dung text của candidate
	Metadata map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"`           // Metadata bổ sung (tùy chọn)

	// ===== JUDGING =====
	JudgeScore      *float64                `json:"judgeScore,omitempty" bson:"judgeScore,omitempty" index:"single:1"` // Quality score từ AI judge (0.0 - 1.0)
	JudgeReasoning  string                  `json:"judgeReasoning,omitempty" bson:"judgeReasoning,omitempty"`         // Lý do judge score
	JudgedByAIRunID *primitive.ObjectID      `json:"judgedByAIRunId,omitempty" bson:"judgedByAIRunId,omitempty" index:"single:1"` // ID của AI run thực hiện judge
	JudgeDetails    map[string]interface{}  `json:"judgeDetails,omitempty" bson:"judgeDetails,omitempty"`             // Chi tiết judge (tùy chọn)

	// ===== SELECTION =====
	Selected bool `json:"selected" bson:"selected" index:"single:1"` // Candidate này đã được chọn hay chưa

	// ===== AI RUN REFERENCES =====
	CreatedByAIRunID primitive.ObjectID `json:"createdByAIRunId" bson:"createdByAIRunId" index:"single:1"` // ID của AI run tạo ra candidate này (GENERATE)

	// ===== TIMESTAMPS =====
	CreatedAt int64 `json:"createdAt" bson:"createdAt" index:"single:1"` // Thời gian tạo

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu candidate
}
