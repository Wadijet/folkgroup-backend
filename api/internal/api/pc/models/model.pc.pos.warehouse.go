package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PcPosWarehouse lưu thông tin kho hàng từ Pancake POS API
type PcPosWarehouse struct {
	ID          primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`                                                             // ID của warehouse trong MongoDB
	WarehouseId string                 `json:"warehouseId" bson:"warehouseId" index:"text" extract:"PanCakeData\\.id,converter=string"`       // ID của warehouse trên Pancake POS (extract từ PanCakeData["id"], convert sang string - UUID)
	ShopId      int64                  `json:"shopId" bson:"shopId" index:"text" extract:"PanCakeData\\.shop_id,converter=int64,optional"`    // ID của shop (extract từ PanCakeData["shop_id"])
	Name        string                 `json:"name" bson:"name" index:"text" extract:"PanCakeData\\.name,converter=string,optional"`          // Tên kho hàng (extract từ PanCakeData["name"])
	PhoneNumber string                 `json:"phoneNumber" bson:"phoneNumber" extract:"PanCakeData\\.phone_number,converter=string,optional"` // Số điện thoại kho hàng (extract từ PanCakeData["phone_number"])
	FullAddress string                 `json:"fullAddress" bson:"fullAddress" extract:"PanCakeData\\.full_address,converter=string,optional"` // Địa chỉ đầy đủ (extract từ PanCakeData["full_address"])
	ProvinceId  string                 `json:"provinceId" bson:"provinceId" extract:"PanCakeData\\.province_id,converter=string,optional"`    // ID tỉnh/thành phố (extract từ PanCakeData["province_id"])
	DistrictId  string                 `json:"districtId" bson:"districtId" extract:"PanCakeData\\.district_id,converter=string,optional"`    // ID quận/huyện (extract từ PanCakeData["district_id"])
	CommuneId   string                 `json:"communeId" bson:"communeId" extract:"PanCakeData\\.commune_id,converter=string,optional"`       // ID phường/xã (extract từ PanCakeData["commune_id"])
	PanCakeData map[string]interface{} `json:"panCakeData" bson:"panCakeData"`                                                                // Dữ liệu gốc từ Pancake POS API

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu dữ liệu (phân quyền)

	CreatedAt int64 `json:"createdAt" bson:"createdAt"` // Thời gian tạo
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"` // Thời gian cập nhật
}
