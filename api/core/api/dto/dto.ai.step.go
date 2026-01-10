package dto

// AIStepCreateInput dữ liệu đầu vào khi tạo AI step
type AIStepCreateInput struct {
	Name            string                 `json:"name" validate:"required"`                    // Tên step
	Description     string                 `json:"description,omitempty"`                        // Mô tả step
	Type            string                 `json:"type" validate:"required"`                    // Loại step: GENERATE, JUDGE, STEP_GENERATION
	PromptTemplateID string                 `json:"promptTemplateId,omitempty"`                   // ID của prompt template (dạng string ObjectID)
	InputSchema     map[string]interface{} `json:"inputSchema" validate:"required"`            // Input schema (JSON schema format)
	OutputSchema    map[string]interface{} `json:"outputSchema" validate:"required"`            // Output schema (JSON schema format)
	TargetLevel     string                 `json:"targetLevel,omitempty"`                       // Level mục tiêu: "L1", "L2", ..., "L8" (tùy chọn)
	ParentLevel     string                 `json:"parentLevel,omitempty"`                        // Level của parent: "L1", "L2", ..., "L8" (tùy chọn)
	Status          string                 `json:"status,omitempty"`                            // Trạng thái: "active", "archived", "draft" (mặc định: "active")
	Metadata        map[string]interface{} `json:"metadata,omitempty"`                          // Metadata bổ sung
}

// AIStepUpdateInput dữ liệu đầu vào khi cập nhật AI step
type AIStepUpdateInput struct {
	Name            string                 `json:"name,omitempty"`                               // Tên step
	Description     string                 `json:"description,omitempty"`                         // Mô tả step
	Type            string                 `json:"type,omitempty"`                               // Loại step
	PromptTemplateID string                 `json:"promptTemplateId,omitempty"`                  // ID của prompt template
	InputSchema     map[string]interface{} `json:"inputSchema,omitempty"`                        // Input schema
	OutputSchema    map[string]interface{} `json:"outputSchema,omitempty"`                       // Output schema
	TargetLevel     string                 `json:"targetLevel,omitempty"`                       // Level mục tiêu
	ParentLevel     string                 `json:"parentLevel,omitempty"`                        // Level của parent
	Status          string                 `json:"status,omitempty"`                             // Trạng thái
	Metadata        map[string]interface{} `json:"metadata,omitempty"`                          // Metadata bổ sung
}
