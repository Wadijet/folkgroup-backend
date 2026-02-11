package pcdto

// PcPosCategoryCreateInput dữ liệu đầu vào khi tạo hoặc cập nhật category từ Pancake POS API
type PcPosCategoryCreateInput struct {
	PosData map[string]interface{} `json:"posData" validate:"required"`
}
