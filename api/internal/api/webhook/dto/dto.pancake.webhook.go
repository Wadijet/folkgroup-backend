package webhookdto

// PancakeWebhookPayload là payload nhận được từ Pancake webhook
type PancakeWebhookPayload struct {
	EventType string                 `json:"eventType"`
	PageID    string                 `json:"pageId"`
	Data      map[string]interface{} `json:"data"`
	Timestamp int64                  `json:"timestamp"`
}

// PancakeWebhookRequest là request body từ Pancake webhook
type PancakeWebhookRequest struct {
	Payload   PancakeWebhookPayload `json:"payload" validate:"required"`
	Signature string               `json:"signature,omitempty"`
}
