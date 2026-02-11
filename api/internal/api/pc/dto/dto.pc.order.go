package pcdto

// PcOrderCreateInput dữ liệu đầu vào khi tạo đơn hàng
type PcOrderCreateInput struct {
	PanCakeData map[string]interface{} `json:"panCakeData" validate:"required"`
}
