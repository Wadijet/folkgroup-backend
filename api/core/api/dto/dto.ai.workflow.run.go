package dto

// AIWorkflowRunCreateInput dữ liệu đầu vào khi tạo AI workflow run
type AIWorkflowRunCreateInput struct {
	WorkflowID  string                 `json:"workflowId" validate:"required" transform:"str_objectid"`  // ID của workflow definition (dạng string ObjectID) - tự động convert sang primitive.ObjectID
	RootRefID   string                 `json:"rootRefId,omitempty" transform:"str_objectid_ptr,optional"`               // ID của root content (dạng string ObjectID) - tự động convert sang *primitive.ObjectID
	RootRefType string                 `json:"rootRefType,omitempty"`             // Loại root reference: "layer", "stp", etc.
	Params      map[string]interface{} `json:"params,omitempty"`                 // Tham số bổ sung cho workflow run
	Metadata    map[string]interface{} `json:"metadata,omitempty"`                // Metadata bổ sung
}

// AIWorkflowRunUpdateInput dữ liệu đầu vào khi cập nhật AI workflow run
type AIWorkflowRunUpdateInput struct {
	Status         string                 `json:"status,omitempty"`                 // Trạng thái: pending, running, completed, failed, cancelled
	CurrentStepIndex int                  `json:"currentStepIndex,omitempty"`      // Index của step hiện tại đang chạy
	StepRunIDs     []string               `json:"stepRunIds,omitempty"`            // Danh sách ID của step runs (dạng string ObjectID)
	Result         map[string]interface{} `json:"result,omitempty"`               // Kết quả workflow run
	Error          string                 `json:"error,omitempty"`                 // Lỗi nếu có
	ErrorDetails   map[string]interface{} `json:"errorDetails,omitempty"`           // Chi tiết lỗi
	Metadata       map[string]interface{} `json:"metadata,omitempty"`             // Metadata bổ sung
}
