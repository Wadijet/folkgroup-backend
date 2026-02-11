package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PcPosProduct lưu thông tin sản phẩm từ Pancake POS API
type PcPosProduct struct {
	ID                primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`                                                          // ID của product trong MongoDB
	ProductId         string                 `json:"productId" bson:"productId" index:"text" extract:"PosData\\.id,converter=string"`            // ID của product trên Pancake POS (extract từ PosData["id"], UUID string)
	ShopId            int64                  `json:"shopId" bson:"shopId" index:"text" extract:"PosData\\.shop_id,converter=int64,optional"`     // ID của shop (extract từ PosData["shop_id"])
	Name              string                 `json:"name" bson:"name" index:"text" extract:"PosData\\.name,converter=string,optional"`           // Tên sản phẩm (extract từ PosData["name"])
	CategoryIds       []int64                `json:"categoryIds" bson:"categoryIds" extract:"PosData\\.category_ids,optional"`                   // Danh sách ID danh mục (extract từ PosData["category_ids"])
	TagIds            []int64                `json:"tagIds" bson:"tagIds" extract:"PosData\\.tags,optional"`                                     // Danh sách ID tags (extract từ PosData["tags"])
	IsHide            bool                   `json:"isHide" bson:"isHide" extract:"PosData\\.is_hide,converter=bool,optional"`                   // Trạng thái ẩn/hiện sản phẩm (extract từ PosData["is_hide"])
	NoteProduct       string                 `json:"noteProduct" bson:"noteProduct" extract:"PosData\\.note_product,converter=string,optional"`  // Ghi chú sản phẩm (extract từ PosData["note_product"])
	ProductAttributes []interface{}          `json:"productAttributes" bson:"productAttributes" extract:"PosData\\.product_attributes,optional"` // Thuộc tính sản phẩm (extract từ PosData["product_attributes"])
	PosData           map[string]interface{} `json:"posData" bson:"posData"`                                                                     // Dữ liệu gốc từ Pancake POS API

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu dữ liệu (phân quyền)

	CreatedAt int64 `json:"createdAt" bson:"createdAt"` // Thời gian tạo
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"` // Thời gian cập nhật
}
