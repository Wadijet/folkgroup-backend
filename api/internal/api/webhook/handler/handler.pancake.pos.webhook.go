// Package webhookhdl - handler webhook Pancake POS (chỉ lưu webhook gửi đến).
package webhookhdl

import (
	"context"
	"time"

	basehdl "meta_commerce/internal/api/base/handler"
	webhookdto "meta_commerce/internal/api/webhook/dto"
	webhookmodels "meta_commerce/internal/api/webhook/models"
	webhooksvc "meta_commerce/internal/api/webhook/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/logger"

	"github.com/gofiber/fiber/v3"
)

// PancakePosWebhookHandler chỉ lưu webhook Pancake POS vào webhook_log, không xử lý order/product/customer.
type PancakePosWebhookHandler struct {
	webhookLogService *webhooksvc.WebhookLogService
}

// NewPancakePosWebhookHandler tạo mới PancakePosWebhookHandler
func NewPancakePosWebhookHandler() (*PancakePosWebhookHandler, error) {
	webhookLogService, err := webhooksvc.NewWebhookLogService()
	if err != nil {
		return nil, err
	}
	return &PancakePosWebhookHandler{
		webhookLogService: webhookLogService,
	}, nil
}

// HandlePancakePosWebhook nhận webhook từ Pancake POS, lưu vào webhook_log và trả 200 OK.
func (h *PancakePosWebhookHandler) HandlePancakePosWebhook(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		log := logger.GetAppLogger()
		rawBody := string(c.Body())
		ctx := c.Context()
		var req webhookdto.PancakePosWebhookRequest
		parseErr := c.Bind().Body(&req)

		webhookLog, logErr := h.saveWebhookLog(ctx, c, "pancake_pos", req, rawBody, parseErr)
		if logErr != nil {
			log.WithError(logErr).Warn("🔔 [PANCAKE POS WEBHOOK] Không thể lưu webhook log")
		}

		if webhookLog != nil && parseErr == nil {
			_ = h.webhookLogService.UpdateProcessedStatus(ctx, webhookLog.ID, true, "")
		}

		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Webhook đã được nhận và lưu log", "status": "success",
		})
		return nil
	})
}

func (h *PancakePosWebhookHandler) saveWebhookLog(ctx context.Context, c fiber.Ctx, source string, req webhookdto.PancakePosWebhookRequest, rawBody string, parseErr error) (*webhookmodels.WebhookLog, error) {
	now := time.Now().UnixMilli()
	requestHeaders := make(map[string]string)
	c.Request().Header.VisitAll(func(key, value []byte) {
		requestHeaders[string(key)] = string(value)
	})
	requestBody := make(map[string]interface{})
	if parseErr == nil && req.Payload.EventType != "" {
		requestBody = map[string]interface{}{"payload": req.Payload}
	} else {
		parseErrStr := ""
		if parseErr != nil {
			parseErrStr = parseErr.Error()
		}
		requestBody = map[string]interface{}{"raw": rawBody, "parseError": parseErrStr}
	}
	webhookLog := webhookmodels.WebhookLog{
		Source: source, EventType: req.Payload.EventType, ShopID: int64(req.Payload.ShopID),
		RequestHeaders: requestHeaders, RequestBody: requestBody, RawBody: rawBody,
		Processed: false,
		ProcessError: func() string {
			if parseErr != nil {
				return "Parse error: " + parseErr.Error()
			}
			return ""
		}(),
		IPAddress: c.IP(), UserAgent: c.Get("User-Agent"), ReceivedAt: now, CreatedAt: now, UpdatedAt: now,
	}
	return h.webhookLogService.CreateWebhookLog(ctx, webhookLog)
}
