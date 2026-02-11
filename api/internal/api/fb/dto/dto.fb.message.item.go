package fbdto

// FbMessageItemCreateInput dữ liệu đầu vào khi tạo message item
type FbMessageItemCreateInput struct {
	ConversationId string                 `json:"conversationId" validate:"required"`
	MessageId      string                 `json:"messageId" validate:"required"`
	MessageData    map[string]interface{} `json:"messageData" validate:"required"`
	InsertedAt     int64                  `json:"insertedAt"`
}

// FbMessageItemUpdateInput dữ liệu đầu vào khi cập nhật message item
type FbMessageItemUpdateInput struct {
	ConversationId string                 `json:"conversationId" validate:"required"`
	MessageId      string                 `json:"messageId" validate:"required"`
	MessageData    map[string]interface{} `json:"messageData" validate:"required"`
	InsertedAt     int64                  `json:"insertedAt"`
}
