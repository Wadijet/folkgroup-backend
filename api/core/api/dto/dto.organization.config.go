package dto

// ConfigKeyMetaInput là metadata cho từng key config (tầng transport): tên, mô tả, loại dữ liệu, ràng buộc, khóa không cho cấp dưới ghi đè.
type ConfigKeyMetaInput struct {
	Name          string `json:"name"`                    // Tên hiển thị của key (ví dụ "Giờ làm việc", "Múi giờ")
	Description   string `json:"description"`             // Mô tả chi tiết mục đích và cách dùng
	DataType      string `json:"dataType"`                // Loại dữ liệu: string, number, boolean, object, array
	Constraints   string `json:"constraints,omitempty"`   // Quy tắc ràng buộc: enum, min, max, pattern...
	AllowOverride bool   `json:"allowOverride"`           // true = cấp dưới được ghi đè; false = khóa không cho cấp dưới ghi đè
}

// OrganizationConfigUpdateInput dùng cho cập nhật config tổ chức (PUT/PATCH /organization/:id/config).
// Config và ConfigMeta tùy chọn; nếu gửi thì merge/ghi đè theo key.
type OrganizationConfigUpdateInput struct {
	Config     map[string]interface{}       `json:"config,omitempty"`     // Giá trị từng key config
	ConfigMeta map[string]ConfigKeyMetaInput `json:"configMeta,omitempty"` // Metadata từng key: name, mô tả, loại, ràng buộc, AllowOverride
}
