package aidto

// AIStepCreateInput dữ liệu đầu vào khi tạo AI step
type AIStepCreateInput struct {
	Name             string                 `json:"name" validate:"required"`
	Description      string                 `json:"description,omitempty"`
	Type             string                 `json:"type" validate:"required,oneof=GENERATE JUDGE STEP_GENERATION"`
	PromptTemplateID string                 `json:"promptTemplateId,omitempty" transform:"str_objectid_ptr,optional"`
	InputSchema      map[string]interface{} `json:"inputSchema,omitempty"`
	OutputSchema     map[string]interface{} `json:"outputSchema,omitempty"`
	TargetLevel      string                 `json:"targetLevel,omitempty"`
	ParentLevel      string                 `json:"parentLevel,omitempty"`
	Status           string                 `json:"status,omitempty" transform:"string,default=active" validate:"omitempty,oneof=active archived draft"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// AIStepUpdateInput dữ liệu đầu vào khi cập nhật AI step
type AIStepUpdateInput struct {
	Name             string                 `json:"name,omitempty"`
	Description      string                 `json:"description,omitempty"`
	Type             string                 `json:"type,omitempty"`
	PromptTemplateID string                 `json:"promptTemplateId,omitempty" transform:"str_objectid_ptr,optional"`
	InputSchema      map[string]interface{} `json:"inputSchema,omitempty"`
	OutputSchema     map[string]interface{} `json:"outputSchema,omitempty"`
	TargetLevel      string                 `json:"targetLevel,omitempty"`
	ParentLevel      string                 `json:"parentLevel,omitempty"`
	Status           string                 `json:"status,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}
