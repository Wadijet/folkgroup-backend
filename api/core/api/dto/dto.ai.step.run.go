package dto

// AIStepRunCreateInput dữ liệu đầu vào khi tạo AI step run
type AIStepRunCreateInput struct {
	WorkflowRunID string                 `json:"workflowRunId" validate:"required"` // ID của workflow run (dạng string ObjectID)
	StepID        string                 `json:"stepId" validate:"required"`        // ID của step definition (dạng string ObjectID)
	Order         int                    `json:"order" validate:"required"`         // Thứ tự trong workflow (0-based)
	Input         map[string]interface{} `json:"input,omitempty"`                  // Input data cho step
	Metadata      map[string]interface{} `json:"metadata,omitempty"`                // Metadata bổ sung
}

// AIStepRunUpdateInput dữ liệu đầu vào khi cập nhật AI step run
type AIStepRunUpdateInput struct {
	Status            string                 `json:"status,omitempty"`                 // Trạng thái: pending, running, completed, failed, skipped
	Input             map[string]interface{} `json:"input,omitempty"`                // Input data cho step
	Output            map[string]interface{} `json:"output,omitempty"`               // Output data từ step
	GenerationBatchID string                 `json:"generationBatchId,omitempty"`     // ID của generation batch (dạng string ObjectID)
	Result            map[string]interface{} `json:"result,omitempty"`               // Kết quả step
	Error             string                 `json:"error,omitempty"`                 // Lỗi nếu có
	ErrorDetails      map[string]interface{} `json:"errorDetails,omitempty"`          // Chi tiết lỗi
	Metadata          map[string]interface{} `json:"metadata,omitempty"`              // Metadata bổ sung
}
