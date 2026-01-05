package dto

// OrganizationShareCreateInput dùng cho tạo organization share (tầng transport)
// Đây là contract/interface cho Frontend - định nghĩa cấu trúc dữ liệu cần gửi khi tạo share
// Lưu ý: Backend parse trực tiếp vào Model, nhưng DTO này dùng để Frontend biết cấu trúc cần gửi
type OrganizationShareCreateInput struct {
	OwnerOrganizationID string   `json:"ownerOrganizationId" validate:"required"` // Tổ chức sở hữu dữ liệu (phân quyền) - Organization share data - BẮT BUỘC
	ToOrgIDs            []string `json:"toOrgIds,omitempty"`                      // Organizations nhận data - [] hoặc null = share với tất cả organizations - Optional
	PermissionNames     []string `json:"permissionNames,omitempty"`               // [] hoặc null = tất cả permissions, ["Order.Read", "Order.Create"] = chỉ share với permissions cụ thể - Optional
	Description         string   `json:"description,omitempty"`                   // Mô tả về lệnh share để người dùng hiểu được mục đích - Optional

	// Lưu ý: KHÔNG cần gửi createdAt, createdBy - Backend tự động set
	// Lưu ý: Nếu ToOrgIDs rỗng hoặc null → share với tất cả organizations
}

// OrganizationShareUpdateInput dùng cho cập nhật organization share (tầng transport)
// Đây là contract/interface cho Frontend - định nghĩa cấu trúc dữ liệu cần gửi khi cập nhật share
// Lưu ý: Backend parse trực tiếp vào Model, nhưng DTO này dùng để Frontend biết cấu trúc cần gửi
// Lưu ý: OrganizationShare thường không cần update, nhưng nếu có thì chỉ update PermissionNames và Description
type OrganizationShareUpdateInput struct {
	PermissionNames []string `json:"permissionNames,omitempty"` // [] hoặc null = tất cả permissions, ["Order.Read", "Order.Create"] = chỉ share với permissions cụ thể - Optional
	Description     string   `json:"description,omitempty"`     // Mô tả về lệnh share để người dùng hiểu được mục đích - Optional

	// Lưu ý: KHÔNG thể update ownerOrganizationId, toOrgIds - Backend sẽ tự động xóa các fields này nếu có trong request (bảo mật)
	// Lưu ý: KHÔNG thể update createdAt, createdBy - Backend sẽ tự động xóa các fields này nếu có trong request
}
