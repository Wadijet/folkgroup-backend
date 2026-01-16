package handler

import (
	"encoding/base64"
	"fmt"
	"meta_commerce/core/api/dto"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"
	"meta_commerce/core/cta"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson"
)

// TrackingHandler xử lý tất cả các action tracking (open, click, confirm, cta)
type TrackingHandler struct {
	historyService *services.DeliveryHistoryService
}

// NewTrackingHandler tạo mới TrackingHandler
func NewTrackingHandler() (*TrackingHandler, error) {
	historyService, err := services.NewDeliveryHistoryService()
	if err != nil {
		return nil, fmt.Errorf("failed to create delivery history service: %v", err)
	}

	return &TrackingHandler{
		historyService: historyService,
	}, nil
}

// HandleAction xử lý các action tracking (public endpoint, không cần auth)
//
// Endpoint: GET /api/v1/track/:action/:historyId?ctaIndex=...
// Actions:
//   - "open": Track email open (trả về 1x1 PNG pixel)
//   - "click": Track notification click (redirect về original URL, cần ctaIndex trong query)
//   - "confirm": Track notification confirm (trả về JSON)
//   - "cta": Track CTA click (redirect về original URL, cần ctaIndex trong query)
//
// Params:
//   - action: Action type (open, click, confirm, cta) - từ URL path
//   - historyId: DeliveryHistory ID (bắt buộc) - từ URL path
//   - ctaIndex: CTA index (bắt buộc cho "click" và "cta", không cần cho "open" và "confirm") - từ query param
//
// LÝ DO PHẢI TẠO ENDPOINT ĐẶC BIỆT (không thể dùng CRUD chuẩn):
// 1. Public endpoint (không cần authentication):
//    - User click/open email → tracking URL
//    - Không có authentication token, chỉ có historyId trong URL
// 2. Response format đặc biệt:
//    - "open": Trả về 1x1 transparent PNG image
//    - "click", "cta": HTTP redirect (302) về original URL
//    - "confirm": JSON response
//    - Không phải format CRUD chuẩn
// 3. Logic nghiệp vụ phức tạp:
//    - Update MongoDB với $inc operators
//    - Decode tracking URL từ query params
//    - Lấy IP address và User Agent từ request
//
// KẾT LUẬN: Cần giữ endpoint đặc biệt vì đây là public endpoint với nhiều response format khác nhau
//           Sử dụng action param để gộp tất cả tracking actions vào 1 endpoint
func (h *TrackingHandler) HandleAction(c fiber.Ctx) error {
	return SafeHandlerWrapper(c, func() error {
		// Parse và validate URL params
		var params dto.TrackingActionParams
		if err := h.ParseRequestParams(c, &params); err != nil {
			// Tùy theo action, trả về response format phù hợp
			return h.handleErrorByAction(c, params.Action, err)
		}

	// Validate ctaIndex cho các action cần thiết
	if (params.Action == "click" || params.Action == "cta") && params.CTAIndex < 0 {
		return c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code":    common.ErrCodeValidationFormat.Code,
			"message": fmt.Sprintf("ctaIndex is required for action '%s'", params.Action),
			"status":  "error",
		})
	}

	// Xử lý theo action
	switch params.Action {
	case "open":
		return h.handleOpenAction(c, params)
	case "click":
		return h.handleClickAction(c, params)
	case "confirm":
		return h.handleConfirmAction(c, params)
	case "cta":
		return h.handleCTAAction(c, params)
	default:
		return c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code":    common.ErrCodeValidationFormat.Code,
			"message": "Invalid action. Supported actions: open, click, confirm, cta",
			"status":  "error",
		})
	}
	})
}

// handleOpenAction xử lý action "open" - track email open và trả về 1x1 PNG pixel
func (h *TrackingHandler) handleOpenAction(c fiber.Ctx, params dto.TrackingActionParams) error {
	now := time.Now().Unix()

	// Lấy history hiện tại
	history, err := h.historyService.FindOneById(c.Context(), params.HistoryID)
	if err != nil {
		// Return pixel nếu không tìm thấy (luôn return pixel để email client không bị lỗi)
		return h.returnPixel(c)
	}

	// Update history với open tracking
	updateDoc := bson.M{
		"$inc": bson.M{"openCount": 1},
	}

	// Nếu chưa có OpenedAt, set nó
	if history.OpenedAt == nil {
		updateDoc["$set"] = bson.M{"openedAt": now}
	}

	_, err = h.historyService.UpdateOne(c.Context(), bson.M{"_id": params.HistoryID}, updateDoc, nil)
	if err != nil {
		// Log error nhưng vẫn return pixel
	}

	// Return 1x1 transparent pixel
	return h.returnPixel(c)
}

// handleClickAction xử lý action "click" - track notification click và redirect
func (h *TrackingHandler) handleClickAction(c fiber.Ctx, params dto.TrackingActionParams) error {
	// ctaIndex đã được validate ở trên (phải >= 0 cho action "click")

	// Lấy original URL từ query params
	originalURL := c.Query("url", "")
	if originalURL == "" {
		return c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code":    common.ErrCodeValidationFormat.Code,
			"message": "url parameter is required",
			"status":  "error",
		})
	}

	// Decode base64 URL nếu cần
	if decoded, err := base64.URLEncoding.DecodeString(originalURL); err == nil {
		originalURL = string(decoded)
	}

	now := time.Now().Unix()

	// Lấy history hiện tại
	history, err := h.historyService.FindOneById(c.Context(), params.HistoryID)
	if err != nil {
		// Redirect về original URL nếu không tìm thấy history
		c.Redirect().To(originalURL)
		return nil
	}

	// Update click tracking (tổng và CTA riêng)
	updateDoc := bson.M{
		"$inc": bson.M{
			"clickCount": 1,
		},
		"$set": bson.M{
			"lastClickAt": now,
		},
	}

	if history.ClickedAt == nil {
		updateDoc["$set"].(bson.M)["clickedAt"] = now
	}

	// Update CTA click tracking (riêng từng CTA)
	if params.CTAIndex >= 0 && params.CTAIndex < len(history.CTAClicks) {
		// Increment click count cho CTA này
		ctaPath := fmt.Sprintf("ctaClicks.%d", params.CTAIndex)
		updateDoc["$inc"].(bson.M)[fmt.Sprintf("%s.clickCount", ctaPath)] = 1
		updateDoc["$set"].(bson.M)[fmt.Sprintf("%s.lastClickAt", ctaPath)] = now

		// Set firstClickAt nếu chưa có
		if history.CTAClicks[params.CTAIndex].FirstClickAt == nil {
			updateDoc["$set"].(bson.M)[fmt.Sprintf("%s.firstClickAt", ctaPath)] = now
		}
	}

	_, err = h.historyService.UpdateOne(c.Context(), bson.M{"_id": params.HistoryID}, updateDoc, nil)
	if err != nil {
		// Log error nhưng vẫn redirect
	}

	// Redirect về original URL
	c.Redirect().To(originalURL)
	return nil
}

// handleConfirmAction xử lý action "confirm" - track notification confirm và trả về JSON
func (h *TrackingHandler) handleConfirmAction(c fiber.Ctx, params dto.TrackingActionParams) error {
	now := time.Now().Unix()

	// Update confirmedAt
	update := services.UpdateData{
		Set: bson.M{
			"confirmedAt": now,
		},
	}

	_, err := h.historyService.UpdateOne(c.Context(), bson.M{"_id": params.HistoryID}, update, nil)
	if err != nil {
		return c.Status(common.StatusInternalServerError).JSON(fiber.Map{
			"code":    common.ErrCodeDatabase.Code,
			"message": err.Error(),
			"status":  "error",
		})
	}

	return c.Status(common.StatusOK).JSON(fiber.Map{
		"code":    common.StatusOK,
		"message": "Notification đã được xác nhận",
		"data": map[string]interface{}{
			"confirmedAt": now,
		},
		"status": "success",
	})
}

// handleCTAAction xử lý action "cta" - track CTA click và redirect
func (h *TrackingHandler) handleCTAAction(c fiber.Ctx, params dto.TrackingActionParams) error {
	// ctaIndex đã được validate ở trên (phải >= 0 cho action "cta")

	// Lấy original URL từ query param
	encodedURL := c.Query("url")
	if encodedURL == "" {
		return c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code":    common.ErrCodeValidationFormat.Code,
			"message": "url parameter is required",
			"status":  "error",
		})
	}

	// Decode URL
	decodedURL, err := cta.DecodeTrackingURL(encodedURL)
	if err != nil {
		return c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code":    common.ErrCodeValidationFormat.Code,
			"message": "Invalid encoded URL",
			"status":  "error",
		})
	}

	// Lấy IP address và User Agent
	ipAddress := c.IP()
	userAgent := c.Get("User-Agent")

	// Lấy DeliveryHistory để lấy ownerOrganizationID và CTA code
	ctx := c.Context()
	history, err := h.historyService.FindOneById(ctx, params.HistoryID)
	if err != nil {
		return c.Status(common.StatusNotFound).JSON(fiber.Map{
			"code":    common.ErrCodeDatabaseQuery.Code,
			"message": "DeliveryHistory not found",
			"status":  "error",
		})
	}

	// Lấy ownerOrganizationID từ DeliveryHistory
	ownerOrgID := history.OwnerOrganizationID

	// Lấy CTA code từ DeliveryHistory (nếu có CTA tại index này)
	ctaCode := ""
	if params.CTAIndex >= 0 && params.CTAIndex < len(history.CTAClicks) {
		// CTA code không được lưu trực tiếp trong CTAClick
		// Có thể lấy từ CTALibrary dựa trên Label hoặc cần thêm field Code vào CTAClick trong tương lai
		// Hiện tại dùng empty string, có thể implement sau nếu cần
		ctaCode = "" // TODO: Lấy CTA code từ CTALibrary dựa trên Label hoặc index
	}

	// Ghi lại click
	err = cta.TrackCTAClick(ctx, params.HistoryID, params.CTAIndex, ctaCode, ownerOrgID, ipAddress, userAgent)
	if err != nil {
		return c.Status(common.StatusInternalServerError).JSON(fiber.Map{
			"code":    common.ErrCodeInternalServer.Code,
			"message": "Failed to track CTA click",
			"details": err.Error(),
			"status":  "error",
		})
	}

	// Redirect về original URL
	c.Redirect().To(decodedURL)
	return nil
}

// ParseRequestParams parse và validate URL params
func (h *TrackingHandler) ParseRequestParams(c fiber.Ctx, params *dto.TrackingActionParams) error {
	// Parse action từ URL path
	params.Action = c.Params("action")
	if params.Action == "" {
		return common.NewError(
			common.ErrCodeValidationFormat,
			"action is required",
			common.StatusBadRequest,
			nil,
		)
	}

	// Parse historyId từ URL path
	historyIDStr := c.Params("historyId")
	if historyIDStr == "" {
		return common.NewError(
			common.ErrCodeValidationFormat,
			"historyId is required",
			common.StatusBadRequest,
			nil,
		)
	}

	// Validate và convert historyId
	var err error
	params.HistoryID, err = params.ParseHistoryID(historyIDStr)
	if err != nil {
		return err
	}

	// Parse ctaIndex từ query param (optional cho một số actions)
	ctaIndexStr := c.Query("ctaIndex")
	if ctaIndexStr != "" {
		params.CTAIndex, err = params.ParseCTAIndex(ctaIndexStr)
		if err != nil {
			return err
		}
	} else {
		// Set -1 nếu không có ctaIndex (để phân biệt với 0)
		params.CTAIndex = -1
	}

	return nil
}

// returnPixel trả về 1x1 transparent PNG pixel
func (h *TrackingHandler) returnPixel(c fiber.Ctx) error {
	c.Type("image/png")
	c.Send([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82})
	return nil
}

// handleErrorByAction xử lý lỗi theo response format của từng action
func (h *TrackingHandler) handleErrorByAction(c fiber.Ctx, action string, err error) error {
	switch action {
	case "open":
		// Luôn return pixel cho action "open" (kể cả khi có lỗi)
		return h.returnPixel(c)
	case "click", "cta":
		// Redirect về URL nếu có, hoặc trả về JSON error
		originalURL := c.Query("url", "")
		if originalURL != "" {
			// Decode base64 URL nếu cần
			if decoded, decodeErr := base64.URLEncoding.DecodeString(originalURL); decodeErr == nil {
				originalURL = string(decoded)
			}
			c.Redirect().To(originalURL)
			return nil
		}
		return c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code":    common.ErrCodeValidationFormat.Code,
			"message": err.Error(),
			"status":  "error",
		})
	default:
		// "confirm" và các action khác: trả về JSON error
		return c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code":    common.ErrCodeValidationFormat.Code,
			"message": err.Error(),
			"status":  "error",
		})
	}
}
