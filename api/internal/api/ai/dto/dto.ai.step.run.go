package aidto

// AIStepRunCreateInput dữ liệu đầu vào khi tạo AI step run
type AIStepRunCreateInput struct {
	WorkflowRunID string                 `json:"workflowRunId" validate:"required" transform:"str_objectid"`
	StepID        string                 `json:"stepId" validate:"required" transform:"str_objectid"`
	Order         int                    `json:"order" validate:"required"`
	Status        string                 `json:"status,omitempty" transform:"string,default=pending" validate:"omitempty,oneof=pending running completed failed skipped"`
	Input         map[string]interface{} `json:"input,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// AIStepRunUpdateInput dữ liệu đầu vào khi cập nhật AI step run
type AIStepRunUpdateInput struct {
	Status            string                 `json:"status,omitempty"`
	Input             map[string]interface{} `json:"input,omitempty"`
	Output            map[string]interface{} `json:"output,omitempty"`
	GenerationBatchID string                 `json:"generationBatchId,omitempty" transform:"str_objectid_ptr,optional"`
	Result            map[string]interface{} `json:"result,omitempty"`
	Error             string                 `json:"error,omitempty"`
	ErrorDetails      map[string]interface{} `json:"errorDetails,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}
