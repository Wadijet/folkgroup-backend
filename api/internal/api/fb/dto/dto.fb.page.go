package fbdto

// FbPageCreateInput dữ liệu đầu vào khi tạo page
type FbPageCreateInput struct {
	AccessToken string                 `json:"accessToken" validate:"required"`
	PanCakeData map[string]interface{} `json:"panCakeData" validate:"required"`
}

// FbPageUpdateTokenInput dữ liệu đầu vào khi cập nhật token
type FbPageUpdateTokenInput struct {
	PageId          string `json:"pageId" validate:"required"`
	PageAccessToken string `json:"pageAccessToken" validate:"required"`
}
