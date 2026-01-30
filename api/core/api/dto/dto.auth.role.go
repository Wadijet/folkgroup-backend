package dto

// RoleCreateInput dùng cho tạo vai trò (tầng transport)
// Đây là contract/interface cho Frontend - định nghĩa cấu trúc dữ liệu cần gửi khi tạo role
// Lưu ý: Backend parse trực tiếp vào Model, nhưng DTO này dùng để Frontend biết cấu trúc cần gửi
type RoleCreateInput struct {
	Name                string `json:"name" validate:"required"`      // Tên của vai trò - BẮT BUỘC
	Describe            string `json:"describe,omitempty"`             // Mô tả vai trò (tùy chọn, để trống được)
	// OwnerOrganizationID: optional. Cần transform để DTO→Model copy sang primitive.ObjectID
	OwnerOrganizationID string `json:"ownerOrganizationId,omitempty" transform:"str_objectid,optional"`
	// Lưu ý: Nếu có ownerOrganizationId trong request, backend sẽ validate quyền với organization đó
}

// RoleUpdateInput dùng cho cập nhật vai trò (tầng transport)
// Đây là contract/interface cho Frontend - định nghĩa cấu trúc dữ liệu cần gửi khi cập nhật role
// Lưu ý: Backend parse trực tiếp vào Model, nhưng DTO này dùng để Frontend biết cấu trúc cần gửi
type RoleUpdateInput struct {
	Name                string `json:"name"`                          // Tên của vai trò - Optional
	Describe            string `json:"describe"`                      // Mô tả vai trò - Optional
	// OwnerOrganizationID: optional. Cần transform để copy sang Model primitive.ObjectID
	OwnerOrganizationID string `json:"ownerOrganizationId,omitempty" transform:"str_objectid,optional"`
	// Lưu ý: Nếu update ownerOrganizationId, backend sẽ validate quyền với organization mới và document hiện tại
}
