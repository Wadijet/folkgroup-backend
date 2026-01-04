package dto

// CTALibraryCreateInput là input để tạo CTA Library
type CTALibraryCreateInput struct {
	Code      string   `json:"code" validate:"required"`      // Mã CTA (unique trong organization)
	Label     string   `json:"label" validate:"required"`      // Label hiển thị
	Action    string   `json:"action" validate:"required"`    // URL action
	Style     string   `json:"style,omitempty"`               // Style: "primary", "success", "secondary", "danger"
	Variables []string `json:"variables"`                     // Danh sách variables
	IsActive  bool     `json:"isActive"`                      // Trạng thái hoạt động
}

// CTALibraryUpdateInput là input để cập nhật CTA Library
type CTALibraryUpdateInput struct {
	Label     *string   `json:"label,omitempty"`     // Label hiển thị
	Action    *string   `json:"action,omitempty"`  // URL action
	Style     *string   `json:"style,omitempty"`   // Style
	Variables  *[]string `json:"variables,omitempty"` // Danh sách variables
	IsActive  *bool     `json:"isActive,omitempty"` // Trạng thái hoạt động
}
