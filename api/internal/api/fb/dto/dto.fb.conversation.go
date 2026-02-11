package fbdto

// FbConversationCreateInput dữ liệu đầu vào khi tạo conversation
type FbConversationCreateInput struct {
	PageId       string                 `json:"pageId" validate:"required"`
	PageUsername string                 `json:"pageUsername"` // Có thể rỗng; extract từ PanCakeData nếu có
	PanCakeData  map[string]interface{} `json:"panCakeData"`
}
