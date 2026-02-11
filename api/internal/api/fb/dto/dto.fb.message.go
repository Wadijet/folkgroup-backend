package fbdto

// FbMessageCreateInput dữ liệu đầu vào cho CRUD operations
type FbMessageCreateInput struct {
	PageId         string                 `json:"pageId" validate:"required"`
	PageUsername   string                 `json:"pageUsername" validate:"required"`
	ConversationId string                 `json:"conversationId" validate:"required"`
	CustomerId     string                 `json:"customerId" validate:"required"`
	PanCakeData    map[string]interface{} `json:"panCakeData" validate:"required"`
}

// FbMessageUpsertMessagesInput dữ liệu đầu vào cho endpoint upsert-messages
type FbMessageUpsertMessagesInput struct {
	PageId         string                 `json:"pageId" validate:"required"`
	PageUsername   string                 `json:"pageUsername" validate:"required"`
	ConversationId string                 `json:"conversationId" validate:"required"`
	CustomerId     string                 `json:"customerId" validate:"required"`
	PanCakeData    map[string]interface{} `json:"panCakeData" validate:"required"`
	HasMore        bool                   `json:"hasMore"`
}
