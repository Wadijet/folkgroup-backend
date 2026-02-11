// Package webhookhdl chứa HTTP handler cho domain Webhook (log).
// File: basehdl.webhook.log.go
package webhookhdl

import (
	"fmt"

	basehdl "meta_commerce/internal/api/base/handler"
	webhookdto "meta_commerce/internal/api/webhook/dto"
	webhookmodels "meta_commerce/internal/api/webhook/models"
	webhooksvc "meta_commerce/internal/api/webhook/service"
)

// WebhookLogHandler xử lý các route CRUD cho webhook log
type WebhookLogHandler struct {
	*basehdl.BaseHandler[webhookmodels.WebhookLog, webhookdto.WebhookLogCreateInput, webhookdto.WebhookLogUpdateInput]
}

// NewWebhookLogHandler tạo mới WebhookLogHandler
func NewWebhookLogHandler() (*WebhookLogHandler, error) {
	webhookLogService, err := webhooksvc.NewWebhookLogService()
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook log service: %w", err)
	}

	return &WebhookLogHandler{
		BaseHandler: basehdl.NewBaseHandler[webhookmodels.WebhookLog, webhookdto.WebhookLogCreateInput, webhookdto.WebhookLogUpdateInput](webhookLogService.BaseServiceMongoImpl),
	}, nil
}
