package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PcPosOrder lưu thông tin đơn hàng từ Pancake POS API
type PcPosOrder struct {
	ID              primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`                                                                                   // ID của order trong MongoDB
	OrderId         int64                  `json:"orderId" bson:"orderId" index:"text" extract:"PosData\\.id,converter=int64"`                                          // ID của order trên Pancake POS (extract từ PosData["id"], convert sang int64)
	SystemId        int64                  `json:"systemId" bson:"systemId" extract:"PosData\\.system_id,converter=int64,optional"`                                     // System ID (extract từ PosData["system_id"])
	ShopId          int64                  `json:"shopId" bson:"shopId" index:"text" extract:"PosData\\.shop_id,converter=int64,optional"`                              // ID của shop (extract từ PosData["shop_id"])
	Status          int                    `json:"status" bson:"status" extract:"PosData\\.status,converter=int,optional"`                                              // Trạng thái đơn hàng (extract từ PosData["status"])
	StatusName      string                 `json:"statusName" bson:"statusName" extract:"PosData\\.status_name,converter=string,optional"`                              // Tên trạng thái (extract từ PosData["status_name"])
	BillFullName    string                 `json:"billFullName" bson:"billFullName" index:"text" extract:"PosData\\.bill_full_name,converter=string,optional"`          // Tên người thanh toán (extract từ PosData["bill_full_name"])
	BillPhoneNumber string                 `json:"billPhoneNumber" bson:"billPhoneNumber" index:"text" extract:"PosData\\.bill_phone_number,converter=string,optional"` // Số điện thoại người thanh toán (extract từ PosData["bill_phone_number"])
	BillEmail       string                 `json:"billEmail" bson:"billEmail" index:"text" extract:"PosData\\.bill_email,converter=string,optional"`                    // Email người thanh toán (extract từ PosData["bill_email"])
	CustomerId      string                 `json:"customerId" bson:"customerId" index:"text" extract:"PosData\\.customer\\.id,converter=string,optional"`               // ID khách hàng (extract từ PosData["customer"]["id"], có thể là UUID string)
	WarehouseId     string                 `json:"warehouseId" bson:"warehouseId" index:"text" extract:"PosData\\.warehouse_id,converter=string,optional"`              // ID kho hàng (extract từ PosData["warehouse_id"], UUID string)
	ShippingFee     float64                `json:"shippingFee" bson:"shippingFee" extract:"PosData\\.shipping_fee,converter=number,optional"`                           // Phí vận chuyển (extract từ PosData["shipping_fee"])
	TotalDiscount   float64                `json:"totalDiscount" bson:"totalDiscount" extract:"PosData\\.total_discount,converter=number,optional"`                     // Tổng giảm giá (extract từ PosData["total_discount"])
	Note            string                 `json:"note" bson:"note" extract:"PosData\\.note,converter=string,optional"`                                                 // Ghi chú đơn hàng (extract từ PosData["note"])
	PageId          string                 `json:"pageId" bson:"pageId" index:"text" extract:"PosData\\.page_id,converter=string,optional"`                             // Facebook Page ID (extract từ PosData["page_id"])
	PostId          string                 `json:"postId" bson:"postId" index:"text" extract:"PosData\\.post_id,converter=string,optional"`                             // Facebook Post ID (extract từ PosData["post_id"])
	InsertedAt      int64                  `json:"insertedAt" bson:"insertedAt" extract:"PosData\\.inserted_at,converter=time,format=2006-01-02T15:04:05Z,optional"`    // Thời gian tạo đơn hàng (extract từ PosData["inserted_at"])
	PosCreatedAt    int64                  `json:"posCreatedAt" bson:"posCreatedAt" extract:"PosData\\.inserted_at,converter=time,format=2006-01-02T15:04:05Z,optional"` // Thời gian tạo đơn từ POS (extract từ PosData["inserted_at"])
	PosUpdatedAt    int64                  `json:"posUpdatedAt" bson:"posUpdatedAt" extract:"PosData\\.updated_at,converter=time,format=2006-01-02T15:04:05Z,optional"` // Thời gian cập nhật từ POS (extract từ PosData["updated_at"])
	PaidAt          int64                  `json:"paidAt" bson:"paidAt" extract:"PosData\\.paid_at,converter=time,format=2006-01-02T15:04:05Z,optional"`                // Thời gian thanh toán (extract từ PosData["paid_at"])
	OrderItems      []interface{}          `json:"orderItems" bson:"orderItems" extract:"PosData\\.items,converter=array|PosData\\.order_items,converter=array,optional"` // Danh sách sản phẩm (converter=array giữ nguyên slice, không convert sang string)
	ShippingAddress map[string]interface{} `json:"shippingAddress" bson:"shippingAddress" extract:"PosData\\.shipping_address,optional"`                                // Địa chỉ giao hàng (extract từ PosData["shipping_address"])
	WarehouseInfo   map[string]interface{} `json:"warehouseInfo" bson:"warehouseInfo" extract:"PosData\\.warehouse_info,optional"`                                      // Thông tin kho hàng (extract từ PosData["warehouse_info"])
	CustomerInfo    map[string]interface{} `json:"customerInfo" bson:"customerInfo" extract:"PosData\\.customer,optional"`                                              // Thông tin khách hàng (extract từ PosData["customer"])
	PosData         map[string]interface{} `json:"posData" bson:"posData"`                                                                                              // Dữ liệu gốc từ Pancake POS API

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:idx_backfill_orders"` // Tổ chức sở hữu dữ liệu

	// Index backfill: sort theo thời gian gốc trong posData (dùng cho Find phân trang CRUD)
	posDataInsertedAt int64 `bson:"posData.inserted_at,omitempty" index:"compound:idx_backfill_orders"`
	posDataUpdatedAt  int64 `bson:"posData.updated_at,omitempty" index:"compound:idx_backfill_orders"`

	CreatedAt int64 `json:"createdAt" bson:"createdAt"` // Thời gian tạo
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"` // Thời gian cập nhật
}
