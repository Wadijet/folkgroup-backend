package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"meta_commerce/internal/utility/identity"
)

// PcPosShop lưu thông tin cửa hàng từ Pancake POS API
type PcPosShop struct {
	ID          primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`                                                       // ID của shop trong MongoDB
	// ===== IDENTITY 4 LỚP =====
	Uid          string                       `json:"uid" bson:"uid" index:"single:1"`
	SourceIds    map[string]string            `json:"sourceIds,omitempty" bson:"sourceIds,omitempty"`
	SourceIdsPos string                       `json:"-" bson:"sourceIds.pos,omitempty" index:"single:1,sparse"`
	Links        map[string]identity.LinkItem `json:"links,omitempty" bson:"links,omitempty"`
	ShopId      int64                  `json:"shopId" bson:"shopId" index:"unique;text" extract:"PanCakeData\\.id,converter=int64"`     // ID của shop trên Pancake POS (extract từ PanCakeData["id"], convert sang int64)
	Name        string                 `json:"name" bson:"name" index:"text" extract:"PanCakeData\\.name,converter=string,optional"`    // Tên cửa hàng (extract từ PanCakeData["name"])
	AvatarUrl   string                 `json:"avatarUrl" bson:"avatarUrl" extract:"PanCakeData\\.avatar_url,converter=string,optional"` // Link hình đại diện (extract từ PanCakeData["avatar_url"])
	Pages       []interface{}          `json:"pages" bson:"pages" extract:"PanCakeData\\.pages,optional"`                               // Thông tin các pages được gộp trong shop (extract từ PanCakeData["pages"])
	PanCakeData map[string]interface{} `json:"panCakeData" bson:"panCakeData"` // Dữ liệu gốc từ Pancake POS API

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu dữ liệu (phân quyền)

	// ===== METADATA =====
	PanCakeUpdatedAt int64 `json:"panCakeUpdatedAt" bson:"panCakeUpdatedAt" extract:"PanCakeData\\.updated_at,converter=time,format=2006-01-02T15:04:05Z,optional"` // Thời gian cập nhật từ Pancake (extract từ PanCakeData["updated_at"])
	CreatedAt        int64 `json:"createdAt" bson:"createdAt"`                                                                                                    // Thời gian tạo
	UpdatedAt        int64 `json:"updatedAt" bson:"updatedAt"`                                                                                                    // Thời gian cập nhật
}
