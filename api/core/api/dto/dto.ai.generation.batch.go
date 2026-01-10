package dto

// AIGenerationBatchCreateInput dữ liệu đầu vào khi tạo AI generation batch
type AIGenerationBatchCreateInput struct {
	StepRunID   string                 `json:"stepRunId" validate:"required"` // ID của step run (dạng string ObjectID)
	TargetCount int                    `json:"targetCount" validate:"required"` // Số lượng candidates muốn generate
	Metadata    map[string]interface{} `json:"metadata,omitempty"`             // Metadata bổ sung
}

// AIGenerationBatchUpdateInput dữ liệu đầu vào khi cập nhật AI generation batch
type AIGenerationBatchUpdateInput struct {
	Status      string                 `json:"status,omitempty"`                 // Trạng thái: pending, generating, completed, failed
	ActualCount int                    `json:"actualCount,omitempty"`             // Số lượng candidates đã generate thực tế
	CandidateIDs []string              `json:"candidateIds,omitempty"`           // Danh sách ID của candidates (dạng string ObjectID)
	Error       string                 `json:"error,omitempty"`                  // Lỗi nếu có
	ErrorDetails map[string]interface{} `json:"errorDetails,omitempty"`           // Chi tiết lỗi
	Metadata    map[string]interface{} `json:"metadata,omitempty"`               // Metadata bổ sung
}
