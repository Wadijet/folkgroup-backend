package pcdto

// PcPosShopCreateInput dữ liệu đầu vào khi tạo hoặc cập nhật shop từ Pancake POS API
type PcPosShopCreateInput struct {
	PanCakeData map[string]interface{} `json:"panCakeData" validate:"required"`
}
