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

// PancakePosWebhookHandler xử lý các webhook từ Pancake POS API
type PancakePosWebhookHandler struct {
	pcPosOrderService    *services.PcPosOrderService
	pcPosProductService  *services.PcPosProductService
	pcPosCustomerService *services.PcPosCustomerService
	webhookLogService    *services.WebhookLogService
}

// NewPancakePosWebhookHandler tạo mới PancakePosWebhookHandler
// Returns:
//   - *PancakePosWebhookHandler: Instance mới của PancakePosWebhookHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewPancakePosWebhookHandler() (*PancakePosWebhookHandler, error) {
	pcPosOrderService, err := services.NewPcPosOrderService()
	if err != nil {
		return nil, fmt.Errorf("failed to create pc pos order service: %v", err)
	}

	pcPosProductService, err := services.NewPcPosProductService()
	if err != nil {
		return nil, fmt.Errorf("failed to create pc pos product service: %v", err)
	}

	pcPosCustomerService, err := services.NewPcPosCustomerService()
	if err != nil {
		return nil, fmt.Errorf("failed to create pc pos customer service: %v", err)
	}

	webhookLogService, err := services.NewWebhookLogService()
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook log service: %v", err)
	}

	return &PancakePosWebhookHandler{
		pcPosOrderService:    pcPosOrderService,
		pcPosProductService:  pcPosProductService,
		pcPosCustomerService: pcPosCustomerService,
		webhookLogService:    webhookLogService,
	}, nil
}

// HandlePancakePosWebhook xử lý webhook từ Pancake POS
// Endpoint này nhận webhook từ Pancake POS về các events như:
// - order_created: Đơn hàng mới được tạo
// - order_updated: Đơn hàng được cập nhật
// - order_status_changed: Trạng thái đơn hàng thay đổi
// - product_created: Sản phẩm mới được tạo
// - product_updated: Sản phẩm được cập nhật
// - customer_created: Khách hàng mới được tạo
// - customer_updated: Khách hàng được cập nhật
// - inventory_updated: Tồn kho được cập nhật
// - etc.
//
// Tham số:
//   - c: Fiber context chứa request body từ Pancake POS
//
// Trả về:
//   - error: Lỗi nếu có trong quá trình xử lý
//
// Lưu ý:
//   - Endpoint này KHÔNG cần authentication middleware (Pancake POS gọi trực tiếp)
//   - Có thể cần verify API key từ query parameter hoặc header (tùy cấu hình Pancake POS)
//   - Webhook sẽ trigger notification hoặc lưu dữ liệu vào database
func (h *PancakePosWebhookHandler) HandlePancakePosWebhook(c fiber.Ctx) error {
	return SafeHandlerWrapper(c, func() error {
		log := logger.GetAppLogger()

		// Lưu raw body trước khi parse (để lưu vào webhook log)
		rawBody := string(c.Body())

		// Parse request body
		var req dto.PancakePosWebhookRequest
		if err := c.Bind().Body(&req); err != nil {
			log.WithError(err).Warn("🔔 [PANCAKE POS WEBHOOK] Không thể parse request body")
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

		if req.Payload.ShopID == 0 {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": "shopId không được để trống",
				"status":  "error",
			})
			return nil
		}

		// Lưu webhook log để debug (trước khi xử lý)
		ctx := c.Context()
		webhookLog, logErr := h.saveWebhookLog(ctx, c, "pancake_pos", req, rawBody)
		if logErr != nil {
			log.WithError(logErr).Warn("🔔 [PANCAKE POS WEBHOOK] Không thể lưu webhook log")
		}

		// TODO: Verify API key từ query parameter hoặc header (nếu Pancake POS yêu cầu)
		// apiKey := c.Query("api_key")
		// if apiKey == "" {
		//     apiKey = c.Get("X-API-Key")
		// }
		// if !verifyPancakePosAPIKey(apiKey) {
		//     c.Status(common.StatusUnauthorized).JSON(fiber.Map{
		//         "code":    common.ErrCodeAuth.Code,
		//         "message": "API key không hợp lệ",
		//         "status":  "error",
		//     })
		//     return nil
		// }

		// Log webhook received
		log.WithFields(map[string]interface{}{
			"eventType": req.Payload.EventType,
			"shopId":    req.Payload.ShopID,
			"timestamp": req.Payload.Timestamp,
		}).Info("🔔 [PANCAKE POS WEBHOOK] Nhận webhook từ Pancake POS")

		// Xử lý webhook dựa trên eventType
		var processErr error
		switch req.Payload.EventType {
		case "order_created", "order_updated", "order_status_changed":
			processErr = h.handleOrderEvent(ctx, req.Payload)
		case "product_created", "product_updated":
			processErr = h.handleProductEvent(ctx, req.Payload)
		case "customer_created", "customer_updated":
			processErr = h.handleCustomerEvent(ctx, req.Payload)
		case "inventory_updated":
			processErr = h.handleInventoryEvent(ctx, req.Payload)
		default:
			log.WithField("eventType", req.Payload.EventType).Warn("🔔 [PANCAKE POS WEBHOOK] Event type chưa được xử lý")
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
			log.WithError(processErr).WithField("eventType", req.Payload.EventType).Error("🔔 [PANCAKE POS WEBHOOK] Lỗi khi xử lý webhook")
			// Vẫn trả về 200 OK để Pancake POS không retry
		}

		// Trả về success response
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code":    common.StatusOK,
			"message": "Webhook đã được nhận và xử lý thành công",
			"data": fiber.Map{
				"eventType": req.Payload.EventType,
				"shopId":    req.Payload.ShopID,
			},
			"status": "success",
		})

		return nil
	})
}

// handleOrderEvent xử lý webhook events liên quan đến đơn hàng (order_created, order_updated, order_status_changed)
func (h *PancakePosWebhookHandler) handleOrderEvent(ctx context.Context, payload dto.PancakePosWebhookPayload) error {
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

	// Extract orderId từ orderData
	var orderId int64
	if idFloat, ok := orderData["id"].(float64); ok {
		orderId = int64(idFloat)
	} else if idInt, ok := orderData["id"].(int64); ok {
		orderId = idInt
	} else {
		return fmt.Errorf("không tìm thấy order ID trong dữ liệu")
	}

	// Tạo filter để tìm order theo orderId
	filter := bson.M{"orderId": orderId}

	// Tạo update document
	now := time.Now().UnixMilli()
	update := bson.M{
		"$set": bson.M{
			"posData":   orderData,
			"updatedAt": now,
		},
		"$setOnInsert": bson.M{
			"orderId": orderId,
			"shopId":  payload.ShopID,
			"createdAt": now,
		},
	}

	// Upsert order
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	_, err := h.pcPosOrderService.BaseServiceMongoImpl.FindOneAndUpdate(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to upsert order: %v", err)
	}

	log.WithFields(map[string]interface{}{
		"orderId":   orderId,
		"shopId":    payload.ShopID,
		"eventType": payload.EventType,
	}).Info("🔔 [PANCAKE POS WEBHOOK] Đã lưu order vào database")

	return nil
}

// handleProductEvent xử lý webhook events liên quan đến sản phẩm (product_created, product_updated)
func (h *PancakePosWebhookHandler) handleProductEvent(ctx context.Context, payload dto.PancakePosWebhookPayload) error {
	log := logger.GetAppLogger()

	// Lấy dữ liệu product từ payload.data
	productData, ok := payload.Data["product"].(map[string]interface{})
	if !ok {
		// Nếu không có field "product", thử lấy trực tiếp từ data
		productData = payload.Data
	}

	if productData == nil {
		return fmt.Errorf("không tìm thấy dữ liệu product trong payload")
	}

	// Extract productId từ productData (UUID string)
	productId, ok := productData["id"].(string)
	if !ok {
		// Thử convert từ số sang string (nếu Pancake POS gửi số)
		if idNum, ok := productData["id"].(float64); ok {
			productId = fmt.Sprintf("%.0f", idNum)
		} else {
			return fmt.Errorf("không tìm thấy product ID trong dữ liệu")
		}
	}

	// Tạo filter để tìm product theo productId
	filter := bson.M{"productId": productId}

	// Tạo update document
	now := time.Now().UnixMilli()
	update := bson.M{
		"$set": bson.M{
			"posData":   productData,
			"shopId":    payload.ShopID,
			"updatedAt": now,
		},
		"$setOnInsert": bson.M{
			"productId": productId,
			"createdAt": now,
		},
	}

	// Upsert product
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	_, err := h.pcPosProductService.BaseServiceMongoImpl.FindOneAndUpdate(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to upsert product: %v", err)
	}

	log.WithFields(map[string]interface{}{
		"productId": productId,
		"shopId":    payload.ShopID,
		"eventType": payload.EventType,
	}).Info("🔔 [PANCAKE POS WEBHOOK] Đã lưu product vào database")

	return nil
}

// handleCustomerEvent xử lý webhook events liên quan đến khách hàng (customer_created, customer_updated)
func (h *PancakePosWebhookHandler) handleCustomerEvent(ctx context.Context, payload dto.PancakePosWebhookPayload) error {
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
			"posData":      customerData,
			"shopId":       payload.ShopID,
			"posUpdatedAt": payload.Timestamp,
			"updatedAt":    now,
		},
		"$setOnInsert": bson.M{
			"customerId": customerId,
			"createdAt":  now,
		},
	}

	// Upsert customer
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	_, err := h.pcPosCustomerService.BaseServiceMongoImpl.FindOneAndUpdate(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to upsert customer: %v", err)
	}

	log.WithFields(map[string]interface{}{
		"customerId": customerId,
		"shopId":     payload.ShopID,
		"eventType":  payload.EventType,
	}).Info("🔔 [PANCAKE POS WEBHOOK] Đã lưu customer vào database")

	return nil
}

// handleInventoryEvent xử lý webhook events liên quan đến tồn kho (inventory_updated)
func (h *PancakePosWebhookHandler) handleInventoryEvent(ctx context.Context, payload dto.PancakePosWebhookPayload) error {
	log := logger.GetAppLogger()

	// Lấy dữ liệu inventory từ payload.data
	inventoryData, ok := payload.Data["inventory"].(map[string]interface{})
	if !ok {
		// Nếu không có field "inventory", thử lấy trực tiếp từ data
		inventoryData = payload.Data
	}

	if inventoryData == nil {
		return fmt.Errorf("không tìm thấy dữ liệu inventory trong payload")
	}

	// TODO: Xử lý inventory update
	// Inventory có thể liên quan đến variation, cần xử lý theo variation_id
	// Hiện tại chỉ log, chưa implement chi tiết vì cần xem cấu trúc dữ liệu thực tế từ Pancake POS

	log.WithFields(map[string]interface{}{
		"shopId":    payload.ShopID,
		"eventType": payload.EventType,
	}).Info("🔔 [PANCAKE POS WEBHOOK] Nhận inventory update (chưa xử lý chi tiết)")

	return nil
}

// saveWebhookLog lưu webhook log vào database để debug
func (h *PancakePosWebhookHandler) saveWebhookLog(ctx context.Context, c fiber.Ctx, source string, req dto.PancakePosWebhookRequest, rawBody string) (*models.WebhookLog, error) {
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
		ShopID:         int64(req.Payload.ShopID),
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
