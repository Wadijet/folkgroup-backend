package aidto

// AICandidateCreateInput dữ liệu đầu vào khi tạo AI candidate
type AICandidateCreateInput struct {
	GenerationBatchID string                 `json:"generationBatchId" validate:"required" transform:"str_objectid"`
	StepRunID         string                 `json:"stepRunId" validate:"required" transform:"str_objectid"`
	Text              string                 `json:"text" validate:"required"`
	CreatedByAIRunID  string                 `json:"createdByAIRunId" validate:"required" transform:"str_objectid"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

// AICandidateUpdateInput dữ liệu đầu vào khi cập nhật AI candidate
type AICandidateUpdateInput struct {
	Text            string                 `json:"text,omitempty"`
	JudgeScore      *float64               `json:"judgeScore,omitempty"`
	JudgeReasoning  string                 `json:"judgeReasoning,omitempty"`
	JudgedByAIRunID string                 `json:"judgedByAIRunId,omitempty" transform:"str_objectid_ptr,optional"`
	JudgeDetails    map[string]interface{} `json:"judgeDetails,omitempty"`
	Selected        *bool                  `json:"selected,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}
