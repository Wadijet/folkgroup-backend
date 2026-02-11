package webhookdto

// PancakePosWebhookPayload là payload nhận được từ Pancake POS webhook
type PancakePosWebhookPayload struct {
	EventType string                 `json:"eventType"`
	ShopID    int                    `json:"shopId"`
	Data      map[string]interface{} `json:"data"`
	Timestamp int64                  `json:"timestamp"`
}

// PancakePosWebhookRequest là request body từ Pancake POS webhook
type PancakePosWebhookRequest struct {
	Payload   PancakePosWebhookPayload `json:"payload" validate:"required"`
	Signature string                  `json:"signature,omitempty"`
}
