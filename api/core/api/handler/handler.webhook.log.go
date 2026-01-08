package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
)

// WebhookLogHandler xử lý các route CRUD cho webhook log
// Kế thừa từ BaseHandler để có sẵn các method CRUD
type WebhookLogHandler struct {
	*BaseHandler[models.WebhookLog, dto.WebhookLogCreateInput, dto.WebhookLogUpdateInput]
}

// NewWebhookLogHandler tạo mới WebhookLogHandler
// Returns:
//   - *WebhookLogHandler: Instance mới của WebhookLogHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewWebhookLogHandler() (*WebhookLogHandler, error) {
	webhookLogService, err := services.NewWebhookLogService()
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook log service: %w", err)
	}

	return &WebhookLogHandler{
		BaseHandler: NewBaseHandler[models.WebhookLog, dto.WebhookLogCreateInput, dto.WebhookLogUpdateInput](webhookLogService.BaseServiceMongoImpl),
	}, nil
}
