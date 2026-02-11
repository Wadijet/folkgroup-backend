package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// FbCustomer lưu thông tin khách hàng từ Pancake API (Facebook)
type FbCustomer struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"` // ID của customer trong MongoDB

	// ===== IDENTIFIERS =====
	CustomerId string `json:"customerId" bson:"customerId" index:"text,unique" extract:"PanCakeData\\.id,converter=string"`       // Pancake Customer ID (extract từ PanCakeData["id"])
	Psid       string `json:"psid" bson:"psid" index:"text,unique,sparse" extract:"PanCakeData\\.psid,converter=string,optional"` // Page Scoped ID (Facebook) (extract từ PanCakeData["psid"])
	PageId     string `json:"pageId" bson:"pageId" index:"text" extract:"PanCakeData\\.page_id,converter=string,optional"`        // Facebook Page ID (extract từ PanCakeData["page_id"])

	// ===== BASIC INFO =====
	Name         string   `json:"name" bson:"name" index:"text" extract:"PanCakeData\\.name,converter=string,optional"`         // Tên khách hàng (extract từ PanCakeData["name"])
	PhoneNumbers []string `json:"phoneNumbers" bson:"phoneNumbers" index:"text" extract:"PanCakeData\\.phone_numbers,optional"` // Số điện thoại (extract từ PanCakeData["phone_numbers"], array)
	Email        string   `json:"email" bson:"email" index:"text" extract:"PanCakeData\\.email,converter=string,optional"`      // Email (extract từ PanCakeData["email"])

	// ===== ADDITIONAL INFO =====
	Birthday string `json:"birthday,omitempty" bson:"birthday,omitempty" extract:"PanCakeData\\.birthday,converter=string,optional"` // Ngày sinh (extract từ PanCakeData["birthday"])
	Gender   string `json:"gender,omitempty" bson:"gender,omitempty" extract:"PanCakeData\\.gender,converter=string,optional"`       // Giới tính (extract từ PanCakeData["gender"])
	LivesIn  string `json:"livesIn,omitempty" bson:"livesIn,omitempty" extract:"PanCakeData\\.lives_in,converter=string,optional"`   // Nơi ở (extract từ PanCakeData["lives_in"])

	// ===== SOURCE DATA =====
	PanCakeData map[string]interface{} `json:"panCakeData,omitempty" bson:"panCakeData,omitempty"` // Dữ liệu gốc từ Pancake API

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu dữ liệu (phân quyền)

	// ===== METADATA =====
	PanCakeUpdatedAt int64 `json:"panCakeUpdatedAt" bson:"panCakeUpdatedAt" extract:"PanCakeData\\.updated_at,converter=time,format=2006-01-02T15:04:05.000000,optional"` // Thời gian cập nhật từ Pancake (extract từ PanCakeData["updated_at"])
	CreatedAt        int64 `json:"createdAt" bson:"createdAt"`                                                                                                            // Thời gian tạo
	UpdatedAt        int64 `json:"updatedAt" bson:"updatedAt"`                                                                                                            // Thời gian cập nhật
}
