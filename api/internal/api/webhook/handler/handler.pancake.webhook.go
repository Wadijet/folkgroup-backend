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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// PancakeWebhookHandler xử lý các webhook từ Pancake API
type PancakeWebhookHandler struct {
	pcOrderService        *pcsvc.PcOrderService
	fbConversationService *fbsvc.FbConversationService
	fbMessageService      *fbsvc.FbMessageService
	fbCustomerService     *fbsvc.FbCustomerService
	fbPageService         *fbsvc.FbPageService
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
	fbPageService, err := fbsvc.NewFbPageService()
	if err != nil {
		return nil, fmt.Errorf("failed to create fb page service: %v", err)
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
		fbPageService:         fbPageService,
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

// handleConversationEvent xử lý webhook conversation_updated.
// Quan trọng: phải set ownerOrganizationId từ fb_pages để hook CRM có thể xử lý và tạo activity conversation_started.
// Upsert customer từ payload vào fb_customers trước (nếu có) để IngestConversationTouchpoint resolve được.
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
	// Upsert customer từ conversation payload vào fb_customers (nếu có) để MergeFromFbCustomer tìm thấy khi hook chạy.
	// Nhiều khách chỉ có hội thoại, chưa nhận webhook customer_updated — cần tạo fb_customer từ conversation.
	if pageId != "" && h.fbPageService != nil && h.fbCustomerService != nil {
		if page, err := h.fbPageService.FindOneByPageID(ctx, pageId); err == nil && !page.OwnerOrganizationID.IsZero() {
			upsertCustomerFromConversation(ctx, h.fbCustomerService, conversationData, pageId, page.OwnerOrganizationID)
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
	// Lấy ownerOrganizationId từ fb_pages để hook CRM có thể xử lý và tạo activity conversation_started.
	// Nếu thiếu ownerOrganizationId, hook sẽ bỏ qua (ownerOrgID.IsZero()) và lịch sử hội thoại không hiện.
	if pageId != "" && h.fbPageService != nil {
		if page, err := h.fbPageService.FindOneByPageID(ctx, pageId); err == nil && !page.OwnerOrganizationID.IsZero() {
			setFields["ownerOrganizationId"] = page.OwnerOrganizationID
		}
	}
	update := bson.M{
		"$set":         setFields,
		"$setOnInsert": bson.M{"conversationId": conversationId, "pageUsername": "", "createdAt": now},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	_, err := h.fbConversationService.BaseServiceMongoImpl.FindOneAndUpdate(ctx, filter, update, opts)
	return err
}

// upsertCustomerFromConversation upsert customer từ conversation payload vào fb_customers.
// Giúp IngestConversationTouchpoint resolve customerId khi khách chưa nhận webhook customer_updated.
// Nếu customer object không có inserted_at/created_at, merge thời gian từ conversation (khi hội thoại bắt đầu)
// để activity "Khởi tạo khách từ Facebook" dùng đúng thời điểm nguồn sự kiện.
func upsertCustomerFromConversation(ctx context.Context, fbCustomerSvc *fbsvc.FbCustomerService, conversationData map[string]interface{}, pageId string, ownerOrgID primitive.ObjectID) {
	customerData := extractCustomerObjectFromConversation(conversationData)
	if customerData == nil {
		return
	}
	customerId, ok := customerData["id"].(string)
	if !ok {
		if n, ok := customerData["id"].(float64); ok {
			customerId = fmt.Sprintf("%.0f", n)
		} else {
			return
		}
	}
	if customerId == "" {
		return
	}
	// Merge thời gian nguồn từ conversation khi customer object chưa có — để activityAt dùng đúng inserted_at của sự kiện.
	panCakeDataToStore := mergeSourceTimestampIntoCustomerData(customerData, conversationData)
	now := time.Now().UnixMilli()
	filter := bson.M{"customerId": customerId}
	update := bson.M{
		"$set": bson.M{
			"panCakeData": panCakeDataToStore, "pageId": pageId,
			"ownerOrganizationId": ownerOrgID, "updatedAt": now,
		},
		"$setOnInsert": bson.M{"customerId": customerId, "createdAt": now},
	}
	opts := options.Update().SetUpsert(true)
	_, _ = fbCustomerSvc.Collection().UpdateOne(ctx, filter, update, opts)
}

// mergeSourceTimestampIntoCustomerData merge inserted_at/created_at từ conversation vào customerData nếu customer chưa có.
// Trả về bản copy đã merge (không mutate customerData gốc).
func mergeSourceTimestampIntoCustomerData(customerData, conversationData map[string]interface{}) map[string]interface{} {
	if customerData == nil {
		return nil
	}
	// Kiểm tra customer đã có thời gian chưa (created_at, inserted_at)
	if _, hasCreated := customerData["created_at"]; hasCreated {
		return customerData
	}
	if _, hasInserted := customerData["inserted_at"]; hasInserted {
		return customerData
	}
	// Lấy inserted_at hoặc created_at từ conversation (thời điểm hội thoại bắt đầu)
	sourceTs := extractTimestampFromMap(conversationData, "inserted_at", "created_at")
	if sourceTs == nil {
		return customerData
	}
	// Copy và thêm thời gian nguồn
	out := make(map[string]interface{}, len(customerData)+1)
	for k, v := range customerData {
		out[k] = v
	}
	out["inserted_at"] = sourceTs
	return out
}

// extractTimestampFromMap lấy giá trị timestamp từ map (string hoặc số) — dùng cho merge vào customerData.
func extractTimestampFromMap(m map[string]interface{}, keys ...string) interface{} {
	if m == nil {
		return nil
	}
	for _, key := range keys {
		if v, ok := m[key]; ok && v != nil {
			return v
		}
	}
	return nil
}

// extractCustomerObjectFromConversation lấy customer object từ conversation data.
func extractCustomerObjectFromConversation(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}
	if cust, ok := data["customer"].(map[string]interface{}); ok && cust != nil {
		return cust
	}
	if arr, ok := data["customers"].([]interface{}); ok && len(arr) > 0 {
		if m, ok := arr[0].(map[string]interface{}); ok {
			return m
		}
	}
	return nil
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
