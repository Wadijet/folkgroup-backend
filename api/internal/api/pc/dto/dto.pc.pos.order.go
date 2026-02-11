package pcdto

// PcPosOrderCreateInput dữ liệu đầu vào khi tạo hoặc cập nhật order từ Pancake POS API
type PcPosOrderCreateInput struct {
	PosData map[string]interface{} `json:"posData" validate:"required"`
}
