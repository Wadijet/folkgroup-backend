package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PcPosCustomer lưu thông tin khách hàng từ Pancake POS API
type PcPosCustomer struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"` // ID của customer trong MongoDB

	// ===== IDENTIFIERS =====
	CustomerId string `json:"customerId" bson:"customerId" index:"text,unique" extract:"PosData\\.id,converter=string"` // UUID string - POS Customer ID (extract từ PosData["id"])
	ShopId     int64  `json:"shopId" bson:"shopId" index:"text" extract:"PosData\\.shop_id,converter=int64,optional"`   // Shop ID (extract từ PosData["shop_id"])

	// ===== BASIC INFO =====
	Name         string   `json:"name" bson:"name" index:"text" extract:"PosData\\.name,converter=string,optional"`         // Tên khách hàng (extract từ PosData["name"])
	PhoneNumbers []string `json:"phoneNumbers" bson:"phoneNumbers" index:"text" extract:"PosData\\.phone_numbers,optional"` // Số điện thoại (extract từ PosData["phone_numbers"], array)
	Emails       []string `json:"emails" bson:"emails" index:"text" extract:"PosData\\.emails,optional"`                    // Email (extract từ PosData["emails"], array - POS có thể có nhiều emails)

	// ===== ADDITIONAL INFO =====
	DateOfBirth string `json:"dateOfBirth,omitempty" bson:"dateOfBirth,omitempty" extract:"PosData\\.date_of_birth,converter=string,optional"` // Ngày sinh (extract từ PosData["date_of_birth"])
	Gender      string `json:"gender,omitempty" bson:"gender,omitempty" extract:"PosData\\.gender,converter=string,optional"`                  // Giới tính (extract từ PosData["gender"])

	// ===== POS-SPECIFIC FIELDS =====
	CustomerLevelId   string        `json:"customerLevelId,omitempty" bson:"customerLevelId,omitempty" extract:"PosData\\.level_id,converter=string,optional"`                        // UUID string - Cấp độ khách hàng (extract từ PosData["level_id"])
	Point             int64         `json:"point,omitempty" bson:"point,omitempty" extract:"PosData\\.reward_point,converter=int64,optional"`                                         // Điểm tích lũy (extract từ PosData["reward_point"])
	TotalOrder        int64         `json:"totalOrder,omitempty" bson:"totalOrder,omitempty" extract:"PosData\\.order_count,converter=int64,optional"`                                // Tổng đơn hàng (extract từ PosData["order_count"])
	TotalSpent        float64       `json:"totalSpent,omitempty" bson:"totalSpent,omitempty" extract:"PosData\\.purchased_amount,converter=number,optional"`                          // Tổng tiền đã mua (extract từ PosData["purchased_amount"])
	SucceedOrderCount int64         `json:"succeedOrderCount,omitempty" bson:"succeedOrderCount,omitempty" extract:"PosData\\.succeed_order_count,converter=int64,optional"`          // Số đơn hàng thành công (extract từ PosData["succeed_order_count"])
	TagIds            []interface{} `json:"tagIds,omitempty" bson:"tagIds,omitempty" extract:"PosData\\.tags,optional"`                                                               // Tags (extract từ PosData["tags"], array)
	LastOrderAt       int64         `json:"lastOrderAt,omitempty" bson:"lastOrderAt,omitempty" extract:"PosData\\.last_order_at,converter=time,format=2006-01-02T15:04:05Z,optional"` // Thời gian đơn hàng cuối (extract từ PosData["last_order_at"])
	Addresses         []interface{} `json:"addresses,omitempty" bson:"addresses,omitempty" extract:"PosData\\.shop_customer_address,optional"`                                        // Địa chỉ (extract từ PosData["shop_customer_address"], array)
	ReferralCode      string        `json:"referralCode,omitempty" bson:"referralCode,omitempty" extract:"PosData\\.referral_code,converter=string,optional"`                         // Mã giới thiệu (extract từ PosData["referral_code"])
	IsBlock           bool          `json:"isBlock,omitempty" bson:"isBlock,omitempty" extract:"PosData\\.is_block,converter=bool,optional"`                                          // Trạng thái block (extract từ PosData["is_block"])

	// ===== SOURCE DATA =====
	PosData map[string]interface{} `json:"posData,omitempty" bson:"posData,omitempty"` // Dữ liệu gốc từ POS API

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu dữ liệu (phân quyền)

	// ===== METADATA =====
	PosUpdatedAt int64 `json:"posUpdatedAt" bson:"posUpdatedAt" extract:"PosData\\.updated_at,converter=time,format=2006-01-02T15:04:05Z,optional"` // Thời gian cập nhật từ POS (extract từ PosData["updated_at"])
	CreatedAt    int64 `json:"createdAt" bson:"createdAt"`                                                                                          // Thời gian tạo
	UpdatedAt    int64 `json:"updatedAt" bson:"updatedAt"`                                                                                          // Thời gian cập nhật
}
