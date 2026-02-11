package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PcPosCategory lưu thông tin danh mục sản phẩm từ Pancake POS API
type PcPosCategory struct {
	ID         primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`                                                      // ID của category trong MongoDB
	CategoryId int64                  `json:"categoryId" bson:"categoryId" index:"text" extract:"PosData\\.id,converter=int64"`       // ID của category trên Pancake POS (extract từ PosData["id"], convert sang int64)
	ShopId     int64                  `json:"shopId" bson:"shopId" index:"text" extract:"PosData\\.shop_id,converter=int64,optional"` // ID của shop (extract từ PosData["shop_id"])
	Name       string                 `json:"name" bson:"name" index:"text" extract:"PosData\\.name,converter=string,optional"`       // Tên danh mục (extract từ PosData["name"])
	PosData    map[string]interface{} `json:"posData" bson:"posData"`                                                                 // Dữ liệu gốc từ Pancake POS API

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu dữ liệu (phân quyền)

	CreatedAt int64 `json:"createdAt" bson:"createdAt"` // Thời gian tạo
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"` // Thời gian cập nhật
}
