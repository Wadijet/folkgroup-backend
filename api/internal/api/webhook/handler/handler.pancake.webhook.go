// Package webhookhdl - handler webhook Pancake (conversation, message, order, customer).
package webhookhdl

import (
	"context"
	"fmt"
	"time"

	basehdl "meta_commerce/internal/api/base/handler"
	webhookdto "meta_commerce/internal/api/webhook/dto"
	fbsvc "meta_commerce/internal/api/fb/service"
	pcsvc "meta_commerce/internal/api/pc/service"
	webhookmodels "meta_commerce/internal/api/webhook/models"
	webhooksvc "meta_commerce/internal/api/webhook/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/logger"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// PancakeWebhookHandler xử lý các webhook từ Pancake API
type PancakeWebhookHandler struct {
	pcOrderService        *pcsvc.PcOrderService
	fbConversationService *fbsvc.FbConversationService
	fbMessageService      *fbsvc.FbMessageService
	fbCustomerService     *fbsvc.FbCustomerService
	webhookLogService     *webhooksvc.WebhookLogService
}

// NewPancakeWebhookHandler tạo mới PancakeWebhookHandler
func NewPancakeWebhookHandler() (*PancakeWebhookHandler, error) {
	pcOrderService, err := pcsvc.NewPcOrderService()
	if err != nil {
		return nil, fmt.Errorf("failed to create pc order service: %v", err)
	}
	fbConversationService, err := fbsvc.NewFbConversationService()
	if err != nil {
		return nil, fmt.Errorf("failed to create fb conversation service: %v", err)
	}
	fbMessageService, err := fbsvc.NewFbMessageService()
	if err != nil {
		return nil, fmt.Errorf("failed to create fb message service: %v", err)
	}
	fbCustomerService, err := fbsvc.NewFbCustomerService()
	if err != nil {
		return nil, fmt.Errorf("failed to create fb customer service: %v", err)
	}
	webhookLogService, err := webhooksvc.NewWebhookLogService()
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook log service: %v", err)
	}
	return &PancakeWebhookHandler{
		pcOrderService:        pcOrderService,
		fbConversationService: fbConversationService,
		fbMessageService:      fbMessageService,
		fbCustomerService:     fbCustomerService,
		webhookLogService:     webhookLogService,
	}, nil
}

// HandlePancakeWebhook xử lý webhook từ Pancake
func (h *PancakeWebhookHandler) HandlePancakeWebhook(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		log := logger.GetAppLogger()
		rawBody := string(c.Body())
		ctx := c.Context()
		var req webhookdto.PancakeWebhookRequest
		parseErr := c.Bind().Body(&req)

		webhookLog, logErr := h.saveWebhookLog(ctx, c, "pancake", req, rawBody, parseErr)
		if logErr != nil {
			log.WithError(logErr).Warn("🔔 [PANCAKE WEBHOOK] Không thể lưu webhook log")
		}

		if parseErr != nil {
			c.Status(common.StatusOK).JSON(fiber.Map{
				"code": common.StatusOK, "message": "Webhook đã được nhận và lưu log", "status": "success",
			})
			return nil
		}

		var processErr error
		if req.Payload.EventType != "" {
			switch req.Payload.EventType {
			case "order_created", "order_updated":
				processErr = h.handleOrderEvent(ctx, req.Payload)
			case "conversation_updated":
				processErr = h.handleConversationEvent(ctx, req.Payload)
			case "message_received":
				processErr = h.handleMessageEvent(ctx, req.Payload)
			case "customer_updated":
				processErr = h.handleCustomerEvent(ctx, req.Payload)
			default:
				log.WithField("eventType", req.Payload.EventType).Warn("🔔 [PANCAKE WEBHOOK] Event type chưa được xử lý")
			}
			if webhookLog != nil {
				errorMsg := ""
				if processErr != nil {
					errorMsg = processErr.Error()
				}
				_ = h.webhookLogService.UpdateProcessedStatus(ctx, webhookLog.ID, processErr == nil, errorMsg)
			}
			if processErr != nil {
				log.WithError(processErr).WithField("eventType", req.Payload.EventType).Error("🔔 [PANCAKE WEBHOOK] Lỗi khi xử lý webhook")
			}
		} else {
			log.Warn("🔔 [PANCAKE WEBHOOK] Không có eventType, chỉ lưu log")
		}

		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Webhook đã được nhận và lưu log", "status": "success",
		})
		return nil
	})
}

func (h *PancakeWebhookHandler) handleOrderEvent(ctx context.Context, payload webhookdto.PancakeWebhookPayload) error {
	orderData, ok := payload.Data["order"].(map[string]interface{})
	if !ok {
		orderData = payload.Data
	}
	if orderData == nil {
		return fmt.Errorf("không tìm thấy dữ liệu order trong payload")
	}
	pancakeOrderId, ok := orderData["id"].(string)
	if !ok {
		if idNum, ok := orderData["id"].(float64); ok {
			pancakeOrderId = fmt.Sprintf("%.0f", idNum)
		} else {
			return fmt.Errorf("không tìm thấy order ID trong dữ liệu")
		}
	}
	filter := bson.M{"pancakeOrderId": pancakeOrderId}
	now := time.Now().UnixMilli()
	update := bson.M{
		"$set": bson.M{"panCakeData": orderData, "updatedAt": now},
		"$setOnInsert": bson.M{"pancakeOrderId": pancakeOrderId, "status": 0, "createdAt": now},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	_, err := h.pcOrderService.BaseServiceMongoImpl.FindOneAndUpdate(ctx, filter, update, opts)
	return err
}

func (h *PancakeWebhookHandler) handleConversationEvent(ctx context.Context, payload webhookdto.PancakeWebhookPayload) error {
	conversationData, ok := payload.Data["conversation"].(map[string]interface{})
	if !ok {
		conversationData = payload.Data
	}
	if conversationData == nil {
		return fmt.Errorf("không tìm thấy dữ liệu conversation trong payload")
	}
	conversationId, ok := conversationData["id"].(string)
	if !ok {
		return fmt.Errorf("không tìm thấy conversation ID trong dữ liệu")
	}
	pageId := payload.PageID
	if pageId == "" {
		if pageIdFromData, ok := conversationData["page_uid"].(string); ok {
			pageId = pageIdFromData
		}
	}
	filter := bson.M{"conversationId": conversationId}
	now := time.Now().UnixMilli()
	setFields := bson.M{
		"panCakeData": conversationData, "pageId": pageId,
		"panCakeUpdatedAt": payload.Timestamp, "updatedAt": now,
	}
	// Extract customerId để checkHasConversation match được (Pancake có thể dùng customer_id, customer.id, customers[0].id)
	if cid := extractCustomerIdFromConversation(conversationData); cid != "" {
		setFields["customerId"] = cid
	}
	update := bson.M{
		"$set":         setFields,
		"$setOnInsert": bson.M{"conversationId": conversationId, "pageUsername": "", "createdAt": now},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	_, err := h.fbConversationService.BaseServiceMongoImpl.FindOneAndUpdate(ctx, filter, update, opts)
	return err
}

// extractCustomerIdFromConversation lấy customer ID từ conversation data (nhiều cấu trúc Pancake).
func extractCustomerIdFromConversation(data map[string]interface{}) string {
	if data == nil {
		return ""
	}
	if s, ok := data["customer_id"].(string); ok && s != "" {
		return s
	}
	if n, ok := data["customer_id"].(float64); ok {
		return fmt.Sprintf("%.0f", n)
	}
	if cust, ok := data["customer"].(map[string]interface{}); ok {
		if s, ok := cust["id"].(string); ok && s != "" {
			return s
		}
		if n, ok := cust["id"].(float64); ok {
			return fmt.Sprintf("%.0f", n)
		}
	}
	if arr, ok := data["customers"].([]interface{}); ok && len(arr) > 0 {
		if m, ok := arr[0].(map[string]interface{}); ok {
			if s, ok := m["id"].(string); ok && s != "" {
				return s
			}
			if n, ok := m["id"].(float64); ok {
				return fmt.Sprintf("%.0f", n)
			}
		}
	}
	return ""
}

func (h *PancakeWebhookHandler) handleMessageEvent(ctx context.Context, payload webhookdto.PancakeWebhookPayload) error {
	messageData, ok := payload.Data["message"].(map[string]interface{})
	if !ok {
		messageData = payload.Data
	}
	if messageData == nil {
		return fmt.Errorf("không tìm thấy dữ liệu message trong payload")
	}
	conversationId, ok := messageData["conversation_id"].(string)
	if !ok {
		return fmt.Errorf("không tìm thấy conversation_id trong dữ liệu message")
	}
	pageId := payload.PageID
	if pageId == "" {
		if pageIdFromData, ok := messageData["page_id"].(string); ok {
			pageId = pageIdFromData
		}
	}
	panCakeData := make(map[string]interface{})
	for k, v := range messageData {
		if k != "messages" {
			panCakeData[k] = v
		}
	}
	if messages, ok := messageData["messages"].([]interface{}); ok {
		panCakeData["messages"] = messages
	} else {
		panCakeData["messages"] = []interface{}{messageData}
	}
	// Lấy customerId từ message data hoặc conversation để fb_messages có customerId (cho checkHasConversation)
	customerId := extractCustomerIdFromConversation(messageData)
	if customerId == "" {
		if conv, err := h.fbConversationService.FindOne(ctx, bson.M{"conversationId": conversationId}, nil); err == nil && conv.CustomerId != "" {
			customerId = conv.CustomerId
		}
	}
	_, err := h.fbMessageService.UpsertMessages(ctx, conversationId, pageId, "", customerId, panCakeData, false)
	return err
}

func (h *PancakeWebhookHandler) handleCustomerEvent(ctx context.Context, payload webhookdto.PancakeWebhookPayload) error {
	customerData, ok := payload.Data["customer"].(map[string]interface{})
	if !ok {
		customerData = payload.Data
	}
	if customerData == nil {
		return fmt.Errorf("không tìm thấy dữ liệu customer trong payload")
	}
	customerId, ok := customerData["id"].(string)
	if !ok {
		if idNum, ok := customerData["id"].(float64); ok {
			customerId = fmt.Sprintf("%.0f", idNum)
		} else {
			return fmt.Errorf("không tìm thấy customer ID trong dữ liệu")
		}
	}
	filter := bson.M{"customerId": customerId}
	now := time.Now().UnixMilli()
	update := bson.M{
		"$set": bson.M{"panCakeData": customerData, "panCakeUpdatedAt": payload.Timestamp, "updatedAt": now},
		"$setOnInsert": bson.M{"customerId": customerId, "createdAt": now},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	_, err := h.fbCustomerService.BaseServiceMongoImpl.FindOneAndUpdate(ctx, filter, update, opts)
	return err
}

func (h *PancakeWebhookHandler) saveWebhookLog(ctx context.Context, c fiber.Ctx, source string, req webhookdto.PancakeWebhookRequest, rawBody string, parseErr error) (*webhookmodels.WebhookLog, error) {
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
		Source: source, EventType: req.Payload.EventType, PageID: req.Payload.PageID,
		RequestHeaders: requestHeaders, RequestBody: requestBody, RawBody: rawBody,
		Processed: false,
		ProcessError: func() string {
			if parseErr != nil {
				return fmt.Sprintf("Parse error: %v", parseErr)
			}
			return ""
		}(),
		IPAddress: c.IP(), UserAgent: c.Get("User-Agent"), ReceivedAt: now, CreatedAt: now, UpdatedAt: now,
	}
	return h.webhookLogService.CreateWebhookLog(ctx, webhookLog)
}
