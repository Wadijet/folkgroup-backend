package pcdto

// PcPosProductCreateInput dữ liệu đầu vào khi tạo hoặc cập nhật product từ Pancake POS API
type PcPosProductCreateInput struct {
	PosData map[string]interface{} `json:"posData" validate:"required"`
}
