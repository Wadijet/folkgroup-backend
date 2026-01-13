package dto

// AIPromptTemplateVariableInput input cho prompt template variable
type AIPromptTemplateVariableInput struct {
	Name        string `json:"name" validate:"required"`        // Tên biến
	Description string `json:"description,omitempty"`            // Mô tả biến
	Required    bool   `json:"required"`                        // Biến bắt buộc hay không
	Default     string `json:"default,omitempty"`               // Giá trị mặc định
}

// AIPromptTemplateCreateInput dữ liệu đầu vào khi tạo AI prompt template
type AIPromptTemplateCreateInput struct {
	Name            string                           `json:"name" validate:"required"`                    // Tên prompt template
	Description     string                           `json:"description,omitempty"`                       // Mô tả prompt template
	Type            string                           `json:"type" validate:"required"`                   // Loại: generate, judge, step_generation
	Version         string                           `json:"version" validate:"required"`                // Version của prompt (semver)
	Prompt          string                           `json:"prompt" validate:"required"`                 // Nội dung prompt (có thể chứa variables: {{variableName}})
	Variables       []AIPromptTemplateVariableInput  `json:"variables,omitempty"`                         // Danh sách biến trong prompt
	Status           string                           `json:"status,omitempty"`                            // Trạng thái: "active", "archived", "draft" (mặc định: "active")
	Metadata         map[string]interface{}          `json:"metadata,omitempty"`                          // Metadata bổ sung
}

// AIPromptTemplateUpdateInput dữ liệu đầu vào khi cập nhật AI prompt template
type AIPromptTemplateUpdateInput struct {
	Name            string                           `json:"name,omitempty"`                              // Tên prompt template
	Description     string                           `json:"description,omitempty"`                       // Mô tả prompt template
	Type            string                           `json:"type,omitempty"`                              // Loại
	Version         string                           `json:"version,omitempty"`                           // Version của prompt
	Prompt          string                           `json:"prompt,omitempty"`                             // Nội dung prompt
	Variables       []AIPromptTemplateVariableInput  `json:"variables,omitempty"`                        // Danh sách biến
	Status           string                           `json:"status,omitempty"`                            // Trạng thái
	Metadata         map[string]interface{}          `json:"metadata,omitempty"`                          // Metadata bổ sung
}
