package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PcPosShop lưu thông tin cửa hàng từ Pancake POS API
type PcPosShop struct {
	ID          primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`                                                       // ID của shop trong MongoDB
	ShopId      int64                  `json:"shopId" bson:"shopId" index:"unique;text" extract:"PanCakeData\\.id,converter=int64"`     // ID của shop trên Pancake POS (extract từ PanCakeData["id"], convert sang int64)
	Name        string                 `json:"name" bson:"name" index:"text" extract:"PanCakeData\\.name,converter=string,optional"`    // Tên cửa hàng (extract từ PanCakeData["name"])
	AvatarUrl   string                 `json:"avatarUrl" bson:"avatarUrl" extract:"PanCakeData\\.avatar_url,converter=string,optional"` // Link hình đại diện (extract từ PanCakeData["avatar_url"])
	Pages       []interface{}          `json:"pages" bson:"pages" extract:"PanCakeData\\.pages,optional"`                               // Thông tin các pages được gộp trong shop (extract từ PanCakeData["pages"])
	PanCakeData map[string]interface{} `json:"panCakeData" bson:"panCakeData"`                                                          // Dữ liệu gốc từ Pancake POS API

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu dữ liệu (phân quyền)

	CreatedAt int64 `json:"createdAt" bson:"createdAt"` // Thời gian tạo
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"` // Thời gian cập nhật
}
