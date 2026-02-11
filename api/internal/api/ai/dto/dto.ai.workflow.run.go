package aidto

// AIWorkflowRunCreateInput dữ liệu đầu vào khi tạo AI workflow run
type AIWorkflowRunCreateInput struct {
	WorkflowID       string                 `json:"workflowId" validate:"required" transform:"str_objectid"`
	RootRefID        string                 `json:"rootRefId,omitempty" transform:"str_objectid_ptr,optional"`
	RootRefType      string                 `json:"rootRefType,omitempty"`
	Status           string                 `json:"status,omitempty" transform:"string,default=pending" validate:"omitempty,oneof=pending running completed failed cancelled"`
	CurrentStepIndex int                    `json:"currentStepIndex,omitempty" transform:"int,default=0"`
	StepRunIDs       []string               `json:"stepRunIds,omitempty" transform:"str_objectid_array,default=[]"`
	Params           map[string]interface{} `json:"params,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// AIWorkflowRunUpdateInput dữ liệu đầu vào khi cập nhật AI workflow run
type AIWorkflowRunUpdateInput struct {
	Status           string                 `json:"status,omitempty"`
	CurrentStepIndex int                    `json:"currentStepIndex,omitempty"`
	StepRunIDs       []string               `json:"stepRunIds,omitempty"`
	Result           map[string]interface{} `json:"result,omitempty"`
	Error            string                 `json:"error,omitempty"`
	ErrorDetails     map[string]interface{} `json:"errorDetails,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}
