package dto

// AIStepCreateInput dữ liệu đầu vào khi tạo AI step
// LƯU Ý: InputSchema và OutputSchema sẽ được tự động set từ standard schema theo (Type + TargetLevel + ParentLevel)
// Không cần cung cấp schema, hệ thống sẽ tự động fix cứng để đảm bảo consistency giữa các level
type AIStepCreateInput struct {
	Name             string                 `json:"name" validate:"required"`                                                                            // Tên step
	Description      string                 `json:"description,omitempty"`                                                                               // Mô tả step
	Type             string                 `json:"type" validate:"required,oneof=GENERATE JUDGE STEP_GENERATION"`                                       // Loại step: GENERATE, JUDGE, STEP_GENERATION
	PromptTemplateID string                 `json:"promptTemplateId,omitempty" transform:"str_objectid_ptr,optional"`                                    // ID của prompt template (dạng string ObjectID)
	InputSchema      map[string]interface{} `json:"inputSchema,omitempty"`                                                                               // Input schema (TỰ ĐỘNG SET từ standard - không cần cung cấp)
	OutputSchema     map[string]interface{} `json:"outputSchema,omitempty"`                                                                              // Output schema (TỰ ĐỘNG SET từ standard - không cần cung cấp)
	TargetLevel      string                 `json:"targetLevel,omitempty"`                                                                               // Level mục tiêu: "L1", "L2", ..., "L8" (tùy chọn)
	ParentLevel      string                 `json:"parentLevel,omitempty"`                                                                               // Level của parent: "L1", "L2", ..., "L8" (tùy chọn)
	Status           string                 `json:"status,omitempty" transform:"string,default=active" validate:"omitempty,oneof=active archived draft"` // Trạng thái: "active", "archived", "draft" (mặc định: "active")
	Metadata         map[string]interface{} `json:"metadata,omitempty"`                                                                                  // Metadata bổ sung
}

// AIStepUpdateInput dữ liệu đầu vào khi cập nhật AI step
// LƯU Ý: InputSchema và OutputSchema sẽ được tự động set lại từ standard schema khi update Type/TargetLevel/ParentLevel
// Không cho phép custom schema để đảm bảo consistency
type AIStepUpdateInput struct {
	Name             string                 `json:"name,omitempty"`                                                   // Tên step
	Description      string                 `json:"description,omitempty"`                                            // Mô tả step
	Type             string                 `json:"type,omitempty"`                                                   // Loại step (nếu thay đổi, schema sẽ được set lại)
	PromptTemplateID string                 `json:"promptTemplateId,omitempty" transform:"str_objectid_ptr,optional"` // ID của prompt template
	InputSchema      map[string]interface{} `json:"inputSchema,omitempty"`                                            // Input schema (BỊ IGNORE - sẽ tự động set từ standard)
	OutputSchema     map[string]interface{} `json:"outputSchema,omitempty"`                                           // Output schema (BỊ IGNORE - sẽ tự động set từ standard)
	TargetLevel      string                 `json:"targetLevel,omitempty"`                                            // Level mục tiêu (nếu thay đổi, schema sẽ được set lại)
	ParentLevel      string                 `json:"parentLevel,omitempty"`                                            // Level của parent (nếu thay đổi, schema sẽ được set lại)
	Status           string                 `json:"status,omitempty"`                                                 // Trạng thái
	Metadata         map[string]interface{} `json:"metadata,omitempty"`                                               // Metadata bổ sung
}
