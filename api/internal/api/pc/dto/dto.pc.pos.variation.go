package pcdto

// PcPosVariationCreateInput dữ liệu đầu vào khi tạo hoặc cập nhật variation từ Pancake POS API
type PcPosVariationCreateInput struct {
	PosData map[string]interface{} `json:"posData" validate:"required"`
}
