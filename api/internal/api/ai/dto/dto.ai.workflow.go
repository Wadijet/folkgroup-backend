package aidto

// AIWorkflowCreateInput dữ liệu đầu vào khi tạo AI workflow
type AIWorkflowCreateInput struct {
	Name          string                          `json:"name" validate:"required"`
	Description   string                          `json:"description,omitempty"`
	Version       string                          `json:"version" validate:"required"`
	Steps         []AIWorkflowStepReferenceInput  `json:"steps" validate:"required"`
	RootRefType   string                          `json:"rootRefType" validate:"required"`
	TargetLevel   string                          `json:"targetLevel,omitempty"`
	DefaultPolicy *AIWorkflowStepPolicyInput      `json:"defaultPolicy,omitempty"`
	Status        string                          `json:"status,omitempty" transform:"string,default=active" validate:"omitempty,oneof=active archived draft"`
	Metadata      map[string]interface{}          `json:"metadata,omitempty"`
}

// AIWorkflowStepReferenceInput input cho step reference trong workflow
type AIWorkflowStepReferenceInput struct {
	StepID string                     `json:"stepId" validate:"required"`
	Order  int                        `json:"order" validate:"required"`
	Policy *AIWorkflowStepPolicyInput `json:"policy,omitempty"`
}

// AIWorkflowStepPolicyInput input cho step policy
type AIWorkflowStepPolicyInput struct {
	RetryCount int    `json:"retryCount"`
	Timeout    int    `json:"timeout"`
	OnFailure  string `json:"onFailure"`
	OnSuccess  string `json:"onSuccess"`
	Parallel   bool   `json:"parallel"`
	Condition  string `json:"condition,omitempty"`
}

// AIWorkflowUpdateInput dữ liệu đầu vào khi cập nhật AI workflow
type AIWorkflowUpdateInput struct {
	Name          string                          `json:"name,omitempty"`
	Description   string                          `json:"description,omitempty"`
	Version       string                          `json:"version,omitempty"`
	Steps         []AIWorkflowStepReferenceInput  `json:"steps,omitempty"`
	RootRefType   string                          `json:"rootRefType,omitempty"`
	TargetLevel   string                          `json:"targetLevel,omitempty"`
	DefaultPolicy *AIWorkflowStepPolicyInput      `json:"defaultPolicy,omitempty"`
	Status        string                          `json:"status,omitempty"`
	Metadata      map[string]interface{}         `json:"metadata,omitempty"`
}
