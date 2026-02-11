package fbdto

// FbPostCreateInput dữ liệu đầu vào khi tạo post
type FbPostCreateInput struct {
	PanCakeData map[string]interface{} `json:"panCakeData" validate:"required"`
}

// FbPostUpdateTokenInput dữ liệu đầu vào khi cập nhật token
type FbPostUpdateTokenInput struct {
	PostId      string                 `json:"postId" validate:"required"`
	PanCakeData map[string]interface{} `json:"panCakeData" validate:"required"`
}
