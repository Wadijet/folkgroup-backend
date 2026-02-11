package pcdto

// PcPosWarehouseCreateInput dữ liệu đầu vào khi tạo hoặc cập nhật warehouse từ Pancake POS API
type PcPosWarehouseCreateInput struct {
	PanCakeData map[string]interface{} `json:"panCakeData" validate:"required"`
}
