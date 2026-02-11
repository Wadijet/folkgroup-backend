package aidto

// AIGenerationBatchCreateInput dữ liệu đầu vào khi tạo AI generation batch
type AIGenerationBatchCreateInput struct {
	StepRunID   string                 `json:"stepRunId" validate:"required" transform:"str_objectid"`
	TargetCount int                    `json:"targetCount" validate:"required"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// AIGenerationBatchUpdateInput dữ liệu đầu vào khi cập nhật AI generation batch
type AIGenerationBatchUpdateInput struct {
	Status       string                 `json:"status,omitempty"`
	ActualCount  int                    `json:"actualCount,omitempty"`
	CandidateIDs []string               `json:"candidateIds,omitempty"`
	Error        string                 `json:"error,omitempty"`
	ErrorDetails map[string]interface{} `json:"errorDetails,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}
