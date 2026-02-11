package fbdto

// FbCustomerCreateInput dữ liệu đầu vào khi tạo/update Facebook customer
type FbCustomerCreateInput struct {
	PanCakeData map[string]interface{} `json:"panCakeData" validate:"required"`
}
