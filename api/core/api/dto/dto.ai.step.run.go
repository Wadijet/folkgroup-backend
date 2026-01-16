package dto

// AIStepRunCreateInput dữ liệu đầu vào khi tạo AI step run
type AIStepRunCreateInput struct {
	WorkflowRunID string                 `json:"workflowRunId" validate:"required" transform:"str_objectid"` // ID của workflow run (dạng string ObjectID) - tự động convert sang primitive.ObjectID
	StepID        string                 `json:"stepId" validate:"required" transform:"str_objectid"`        // ID của step definition (dạng string ObjectID) - tự động convert sang primitive.ObjectID
	Order         int                    `json:"order" validate:"required"`         // Thứ tự trong workflow (0-based)
	Status        string                 `json:"status,omitempty" transform:"string,default=pending" validate:"omitempty,oneof=pending running completed failed skipped"` // Trạng thái: pending, running, completed, failed, skipped (mặc định: pending)
	Input         map[string]interface{} `json:"input,omitempty"`                  // Input data cho step
	Metadata      map[string]interface{} `json:"metadata,omitempty"`                // Metadata bổ sung
}

// AIStepRunUpdateInput dữ liệu đầu vào khi cập nhật AI step run
type AIStepRunUpdateInput struct {
	Status            string                 `json:"status,omitempty"`                 // Trạng thái: pending, running, completed, failed, skipped
	Input             map[string]interface{} `json:"input,omitempty"`                // Input data cho step
	Output            map[string]interface{} `json:"output,omitempty"`               // Output data từ step
	GenerationBatchID string                 `json:"generationBatchId,omitempty" transform:"str_objectid_ptr,optional"`     // ID của generation batch (dạng string ObjectID) - tự động convert sang *primitive.ObjectID
	Result            map[string]interface{} `json:"result,omitempty"`               // Kết quả step
	Error             string                 `json:"error,omitempty"`                 // Lỗi nếu có
	ErrorDetails      map[string]interface{} `json:"errorDetails,omitempty"`          // Chi tiết lỗi
	Metadata          map[string]interface{} `json:"metadata,omitempty"`              // Metadata bổ sung
}
