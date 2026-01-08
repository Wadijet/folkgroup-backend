package handler

import (
	"context"
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"
	"meta_commerce/core/logger"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// PancakeWebhookHandler xử lý các webhook từ Pancake API
type PancakeWebhookHandler struct {
	pcOrderService        *services.PcOrderService
	fbConversationService *services.FbConversationService
	fbMessageService      *services.FbMessageService
	fbCustomerService     *services.FbCustomerService
	webhookLogService     *services.WebhookLogService
}

// NewPancakeWebhookHandler tạo mới PancakeWebhookHandler
// Returns:
//   - *PancakeWebhookHandler: Instance mới của PancakeWebhookHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewPancakeWebhookHandler() (*PancakeWebhookHandler, error) {
	pcOrderService, err := services.NewPcOrderService()
	if err != nil {
		return nil, fmt.Errorf("failed to create pc order service: %v", err)
	}

	fbConversationService, err := services.NewFbConversationService()
	if err != nil {
		return nil, fmt.Errorf("failed to create fb conversation service: %v", err)
	}

	fbMessageService, err := services.NewFbMessageService()
	if err != nil {
		return nil, fmt.Errorf("failed to create fb message service: %v", err)
	}

	fbCustomerService, err := services.NewFbCustomerService()
	if err != nil {
		return nil, fmt.Errorf("failed to create fb customer service: %v", err)
	}

	webhookLogService, err := services.NewWebhookLogService()
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
// Endpoint này nhận webhook từ Pancake về các events như:
// - conversation_updated: Cuộc hội thoại được cập nhật
// - message_received: Nhận tin nhắn mới
// - order_created: Đơn hàng mới được tạo
// - order_updated: Đơn hàng được cập nhật
// - customer_updated: Khách hàng được cập nhật
// - etc.
//
// Tham số:
//   - c: Fiber context chứa request body từ Pancake
//
// Trả về:
//   - error: Lỗi nếu có trong quá trình xử lý
//
// Lưu ý:
//   - Endpoint này KHÔNG cần authentication middleware (Pancake gọi trực tiếp)
//   - Có thể cần verify signature hoặc API key từ Pancake (tùy cấu hình)
//   - Webhook sẽ trigger notification hoặc lưu dữ liệu vào database
func (h *PancakeWebhookHandler) HandlePancakeWebhook(c fiber.Ctx) error {
	return SafeHandlerWrapper(c, func() error {
		log := logger.GetAppLogger()

		// Lưu raw body trước khi parse (để lưu vào webhook log)
		rawBody := string(c.Body())

		// Parse request body
		var req dto.PancakeWebhookRequest
		if err := c.Bind().Body(&req); err != nil {
			log.WithError(err).Warn("🔔 [PANCAKE WEBHOOK] Không thể parse request body")
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": "Dữ liệu gửi lên không đúng định dạng JSON",
				"status":  "error",
			})
			return nil
		}

		// Validate
		if req.Payload.EventType == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": "eventType không được để trống",
				"status":  "error",
			})
			return nil
		}

		// Lưu webhook log để debug (trước khi xử lý)
		ctx := c.Context()
		webhookLog, logErr := h.saveWebhookLog(ctx, c, "pancake", req, rawBody)
		if logErr != nil {
			log.WithError(logErr).Warn("🔔 [PANCAKE WEBHOOK] Không thể lưu webhook log")
		}

		// TODO: Verify webhook signature (nếu Pancake hỗ trợ)
		// if req.Signature != "" {
		//     if !verifyPancakeWebhookSignature(c, req) {
		//         c.Status(common.StatusUnauthorized).JSON(fiber.Map{
		//             "code":    common.ErrCodeAuth.Code,
		//             "message": "Webhook signature không hợp lệ",
		//             "status":  "error",
		//         })
		//         return nil
		//     }
		// }

		// Log webhook received
		log.WithFields(map[string]interface{}{
			"eventType": req.Payload.EventType,
			"pageId":    req.Payload.PageID,
			"timestamp": req.Payload.Timestamp,
		}).Info("🔔 [PANCAKE WEBHOOK] Nhận webhook từ Pancake")

		// Xử lý webhook dựa trên eventType
		var processErr error
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

		// Cập nhật trạng thái xử lý trong webhook log
		if webhookLog != nil {
			errorMsg := ""
			if processErr != nil {
				errorMsg = processErr.Error()
			}
			_ = h.webhookLogService.UpdateProcessedStatus(ctx, webhookLog.ID, processErr == nil, errorMsg)
		}

		if processErr != nil {
			log.WithError(processErr).WithField("eventType", req.Payload.EventType).Error("🔔 [PANCAKE WEBHOOK] Lỗi khi xử lý webhook")
			// Vẫn trả về 200 OK để Pancake không retry
		}

		// Trả về success response
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code":    common.StatusOK,
			"message": "Webhook đã được nhận và xử lý thành công",
			"data": fiber.Map{
				"eventType": req.Payload.EventType,
				"pageId":    req.Payload.PageID,
			},
			"status": "success",
		})

		return nil
	})
}

// handleOrderEvent xử lý webhook events liên quan đến đơn hàng (order_created, order_updated)
func (h *PancakeWebhookHandler) handleOrderEvent(ctx context.Context, payload dto.PancakeWebhookPayload) error {
	log := logger.GetAppLogger()

	// Lấy dữ liệu order từ payload.data
	orderData, ok := payload.Data["order"].(map[string]interface{})
	if !ok {
		// Nếu không có field "order", thử lấy trực tiếp từ data
		orderData = payload.Data
	}

	if orderData == nil {
		return fmt.Errorf("không tìm thấy dữ liệu order trong payload")
	}

	// Extract pancakeOrderId từ orderData
	pancakeOrderId, ok := orderData["id"].(string)
	if !ok {
		// Thử convert từ số sang string
		if idNum, ok := orderData["id"].(float64); ok {
			pancakeOrderId = fmt.Sprintf("%.0f", idNum)
		} else {
			return fmt.Errorf("không tìm thấy order ID trong dữ liệu")
		}
	}

	// Tạo filter để tìm order theo pancakeOrderId
	filter := bson.M{"pancakeOrderId": pancakeOrderId}

	// Tạo update document
	now := time.Now().UnixMilli()
	update := bson.M{
		"$set": bson.M{
			"panCakeData": orderData,
			"updatedAt":   now,
		},
		"$setOnInsert": bson.M{
			"pancakeOrderId": pancakeOrderId,
			"status":         0, // 0 = active
			"createdAt":      now,
		},
	}

	// Upsert order
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	_, err := h.pcOrderService.BaseServiceMongoImpl.FindOneAndUpdate(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to upsert order: %v", err)
	}

	log.WithFields(map[string]interface{}{
		"pancakeOrderId": pancakeOrderId,
		"eventType":      payload.EventType,
	}).Info("🔔 [PANCAKE WEBHOOK] Đã lưu order vào database")

	return nil
}

// handleConversationEvent xử lý webhook events liên quan đến conversation (conversation_updated)
func (h *PancakeWebhookHandler) handleConversationEvent(ctx context.Context, payload dto.PancakeWebhookPayload) error {
	log := logger.GetAppLogger()

	// Lấy dữ liệu conversation từ payload.data
	conversationData, ok := payload.Data["conversation"].(map[string]interface{})
	if !ok {
		// Nếu không có field "conversation", thử lấy trực tiếp từ data
		conversationData = payload.Data
	}

	if conversationData == nil {
		return fmt.Errorf("không tìm thấy dữ liệu conversation trong payload")
	}

	// Extract conversationId từ conversationData
	conversationId, ok := conversationData["id"].(string)
	if !ok {
		return fmt.Errorf("không tìm thấy conversation ID trong dữ liệu")
	}

	// Extract pageId từ conversationData hoặc payload
	pageId := payload.PageID
	if pageId == "" {
		if pageIdFromData, ok := conversationData["page_uid"].(string); ok {
			pageId = pageIdFromData
		}
	}

	// Tạo filter để tìm conversation theo conversationId
	filter := bson.M{"conversationId": conversationId}

	// Tạo update document
	now := time.Now().UnixMilli()
	update := bson.M{
		"$set": bson.M{
			"panCakeData":      conversationData,
			"pageId":           pageId,
			"panCakeUpdatedAt": payload.Timestamp,
			"updatedAt":        now,
		},
		"$setOnInsert": bson.M{
			"conversationId": conversationId,
			"pageUsername":    "", // Có thể extract từ conversationData nếu có
			"createdAt":      now,
		},
	}

	// Upsert conversation
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	_, err := h.fbConversationService.BaseServiceMongoImpl.FindOneAndUpdate(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to upsert conversation: %v", err)
	}

	log.WithFields(map[string]interface{}{
		"conversationId": conversationId,
		"pageId":         pageId,
		"eventType":      payload.EventType,
	}).Info("🔔 [PANCAKE WEBHOOK] Đã lưu conversation vào database")

	return nil
}

// handleMessageEvent xử lý webhook events liên quan đến message (message_received)
func (h *PancakeWebhookHandler) handleMessageEvent(ctx context.Context, payload dto.PancakeWebhookPayload) error {
	log := logger.GetAppLogger()

	// Lấy dữ liệu message từ payload.data
	messageData, ok := payload.Data["message"].(map[string]interface{})
	if !ok {
		// Nếu không có field "message", thử lấy trực tiếp từ data
		messageData = payload.Data
	}

	if messageData == nil {
		return fmt.Errorf("không tìm thấy dữ liệu message trong payload")
	}

	// Extract conversationId từ messageData
	conversationId, ok := messageData["conversation_id"].(string)
	if !ok {
		return fmt.Errorf("không tìm thấy conversation_id trong dữ liệu message")
	}

	// Extract pageId từ messageData hoặc payload
	pageId := payload.PageID
	if pageId == "" {
		if pageIdFromData, ok := messageData["page_id"].(string); ok {
			pageId = pageIdFromData
		}
	}

	// Sử dụng UpsertMessages để xử lý message (tương tự như endpoint upsert-messages)
	// Tạo panCakeData với messages array
	panCakeData := make(map[string]interface{})
	for k, v := range messageData {
		if k != "messages" {
			panCakeData[k] = v
		}
	}

	// Nếu có messages array, thêm vào
	if messages, ok := messageData["messages"].([]interface{}); ok {
		panCakeData["messages"] = messages
	} else {
		// Nếu không có messages array, tạo array với message hiện tại
		panCakeData["messages"] = []interface{}{messageData}
	}

	// Gọi UpsertMessages để xử lý
	_, err := h.fbMessageService.UpsertMessages(
		ctx,
		conversationId,
		pageId,
		"", // pageUsername - có thể extract từ messageData nếu có
		"", // customerId - có thể extract từ messageData nếu có
		panCakeData,
		false, // hasMore
	)
	if err != nil {
		return fmt.Errorf("failed to upsert message: %v", err)
	}

	log.WithFields(map[string]interface{}{
		"conversationId": conversationId,
		"pageId":         pageId,
		"eventType":      payload.EventType,
	}).Info("🔔 [PANCAKE WEBHOOK] Đã lưu message vào database")

	return nil
}

// handleCustomerEvent xử lý webhook events liên quan đến customer (customer_updated)
func (h *PancakeWebhookHandler) handleCustomerEvent(ctx context.Context, payload dto.PancakeWebhookPayload) error {
	log := logger.GetAppLogger()

	// Lấy dữ liệu customer từ payload.data
	customerData, ok := payload.Data["customer"].(map[string]interface{})
	if !ok {
		// Nếu không có field "customer", thử lấy trực tiếp từ data
		customerData = payload.Data
	}

	if customerData == nil {
		return fmt.Errorf("không tìm thấy dữ liệu customer trong payload")
	}

	// Extract customerId từ customerData
	customerId, ok := customerData["id"].(string)
	if !ok {
		// Thử convert từ số sang string
		if idNum, ok := customerData["id"].(float64); ok {
			customerId = fmt.Sprintf("%.0f", idNum)
		} else {
			return fmt.Errorf("không tìm thấy customer ID trong dữ liệu")
		}
	}

	// Tạo filter để tìm customer theo customerId
	filter := bson.M{"customerId": customerId}

	// Tạo update document
	now := time.Now().UnixMilli()
	update := bson.M{
		"$set": bson.M{
			"panCakeData":      customerData,
			"panCakeUpdatedAt": payload.Timestamp,
			"updatedAt":        now,
		},
		"$setOnInsert": bson.M{
			"customerId": customerId,
			"createdAt":  now,
		},
	}

	// Upsert customer
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	_, err := h.fbCustomerService.BaseServiceMongoImpl.FindOneAndUpdate(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to upsert customer: %v", err)
	}

	log.WithFields(map[string]interface{}{
		"customerId": customerId,
		"eventType":  payload.EventType,
	}).Info("🔔 [PANCAKE WEBHOOK] Đã lưu customer vào database")

	return nil
}

// saveWebhookLog lưu webhook log vào database để debug
func (h *PancakeWebhookHandler) saveWebhookLog(ctx context.Context, c fiber.Ctx, source string, req dto.PancakeWebhookRequest, rawBody string) (*models.WebhookLog, error) {
	now := time.Now().UnixMilli()

	// Lấy request headers
	requestHeaders := make(map[string]string)
	c.Request().Header.VisitAll(func(key, value []byte) {
		requestHeaders[string(key)] = string(value)
	})

	// Tạo webhook log
	webhookLog := models.WebhookLog{
		Source:         source,
		EventType:      req.Payload.EventType,
		PageID:         req.Payload.PageID,
		RequestHeaders: requestHeaders,
		RequestBody: map[string]interface{}{
			"payload": req.Payload,
		},
		RawBody:    rawBody,
		Processed:  false,
		IPAddress:  c.IP(),
		UserAgent:  c.Get("User-Agent"),
		ReceivedAt: now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Lưu vào database
	result, err := h.webhookLogService.CreateWebhookLog(ctx, webhookLog)
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook log: %v", err)
	}

	return result, nil
}
