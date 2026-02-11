package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AccessToken lưu các access tokens để truy cập vào các hệ thống khác nhau
type AccessToken struct {
	ID            primitive.ObjectID   `json:"id,omitempty" bson:"_id,omitempty"`  // ID của access token
	Name          string               `json:"name" bson:"name" index:"unique"`    // Tên của access token
	Describe      string               `json:"describe" bson:"describe"`           // Mô tả access token
	System        string               `json:"system" bson:"system"`               // Hệ thống của access token
	Value         string               `json:"value" bson:"value"`                 // Giá trị của access token
	AssignedUsers []primitive.ObjectID `json:"assignedUsers" bson:"assignedUsers"` // Danh sách người dùng được gán access token
	Status        byte                 `json:"status" bson:"status"`               // Trạng thái của access token (0 = active, 1 = inactive)

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu dữ liệu (phân quyền)

	CreatedAt int64 `json:"createdAt" bson:"createdAt"` // Thời gian tạo access token
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"` // Thời gian cập nhật access token
}
