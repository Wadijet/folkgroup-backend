package dto

// AICandidateCreateInput dữ liệu đầu vào khi tạo AI candidate
type AICandidateCreateInput struct {
	GenerationBatchID string                 `json:"generationBatchId" validate:"required" transform:"str_objectid"` // ID của generation batch (dạng string ObjectID)
	StepRunID         string                 `json:"stepRunId" validate:"required" transform:"str_objectid"`         // ID của step run (dạng string ObjectID)
	Text              string                 `json:"text" validate:"required"`                                       // Nội dung text của candidate
	CreatedByAIRunID  string                 `json:"createdByAIRunId" validate:"required" transform:"str_objectid"`  // ID của AI run tạo ra candidate này (dạng string ObjectID)
	Metadata          map[string]interface{} `json:"metadata,omitempty"`                                             // Metadata bổ sung
}

// AICandidateUpdateInput dữ liệu đầu vào khi cập nhật AI candidate
type AICandidateUpdateInput struct {
	Text            string                 `json:"text,omitempty"`            // Nội dung text của candidate
	JudgeScore      *float64               `json:"judgeScore,omitempty"`      // Quality score từ AI judge (0.0 - 1.0)
	JudgeReasoning  string                 `json:"judgeReasoning,omitempty"`  // Lý do judge score
	JudgedByAIRunID string `json:"judgedByAIRunId,omitempty" transform:"str_objectid_ptr,optional"` // AI run thực hiện judge - transform sang Model *primitive.ObjectID
	JudgeDetails    map[string]interface{} `json:"judgeDetails,omitempty"`    // Chi tiết judge
	Selected        *bool                  `json:"selected,omitempty"`        // Candidate này đã được chọn hay chưa
	Metadata        map[string]interface{} `json:"metadata,omitempty"`        // Metadata bổ sung
}
