package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PcPosVariation lưu thông tin biến thể sản phẩm từ Pancake POS API
type PcPosVariation struct {
	ID             primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`                                                                   // ID của variation trong MongoDB
	VariationId    string                 `json:"variationId" bson:"variationId" index:"text,unique" extract:"PosData\\.id,converter=string"`          // ID của variation trên Pancake POS (extract từ PosData["id"], UUID string)
	ProductId      string                 `json:"productId" bson:"productId" index:"text" extract:"PosData\\.product_id,converter=string,optional"`    // ID của product (extract từ PosData["product_id"], UUID string)
	ShopId         int64                  `json:"shopId" bson:"shopId" index:"text" extract:"PosData\\.shop_id,converter=int64,optional"`              // ID của shop (extract từ PosData["shop_id"])
	Sku            string                 `json:"sku" bson:"sku" index:"text" extract:"PosData\\.sku,converter=string,optional"`                       // Mã SKU (extract từ PosData["sku"])
	RetailPrice    float64                `json:"retailPrice" bson:"retailPrice" extract:"PosData\\.retail_price,converter=number,optional"`           // Giá bán lẻ (extract từ PosData["retail_price"])
	PriceAtCounter float64                `json:"priceAtCounter" bson:"priceAtCounter" extract:"PosData\\.price_at_counter,converter=number,optional"` // Giá tại quầy (extract từ PosData["price_at_counter"])
	Quantity       int64                  `json:"quantity" bson:"quantity" extract:"PosData\\.quantity,converter=int64,optional"`                      // Số lượng tồn kho (extract từ PosData["quantity"])
	Weight         float64                `json:"weight" bson:"weight" extract:"PosData\\.weight,converter=number,optional"`                           // Trọng lượng (extract từ PosData["weight"])
	Fields         []interface{}          `json:"fields" bson:"fields" extract:"PosData\\.fields,optional"`                                            // Các trường thuộc tính (extract từ PosData["fields"])
	Images         []string               `json:"images" bson:"images" extract:"PosData\\.images,optional"`                                            // Danh sách hình ảnh (extract từ PosData["images"])
	PosData        map[string]interface{} `json:"posData" bson:"posData"`                                                                              // Dữ liệu gốc từ Pancake POS API

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu dữ liệu (phân quyền)

	CreatedAt int64 `json:"createdAt" bson:"createdAt"` // Thời gian tạo
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"` // Thời gian cập nhật
}
