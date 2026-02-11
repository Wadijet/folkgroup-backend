package pcdto

// PcPosCustomerCreateInput dữ liệu đầu vào khi tạo/update POS customer
type PcPosCustomerCreateInput struct {
	PosData map[string]interface{} `json:"posData" validate:"required"`
}
