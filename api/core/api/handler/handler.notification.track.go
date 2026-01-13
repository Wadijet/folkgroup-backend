package handler

import (
	"encoding/base64"
	"fmt"
	"time"

	"meta_commerce/core/api/services"
	"meta_commerce/core/common"
	"meta_commerce/core/utility"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NotificationTrackHandler xử lý tracking opens và clicks
type NotificationTrackHandler struct {
	historyService *services.DeliveryHistoryService
}

// NewNotificationTrackHandler tạo mới NotificationTrackHandler
func NewNotificationTrackHandler() (*NotificationTrackHandler, error) {
	historyService, err := services.NewDeliveryHistoryService()
	if err != nil {
		return nil, fmt.Errorf("failed to create notification history service: %v", err)
	}

	return &NotificationTrackHandler{
		historyService: historyService,
	}, nil
}

// HandleTrackOpen xử lý tracking khi email được mở (tracking pixel)
//
// LÝ DO PHẢI TẠO ENDPOINT ĐẶC BIỆT (không thể dùng CRUD chuẩn):
// 1. Public endpoint (không cần authentication):
//    - Email client mở email → load tracking pixel (1x1 transparent PNG)
//    - Không có authentication token, chỉ có historyId trong URL
//    - Phải return image/png content, không phải JSON response
// 2. Response format đặc biệt:
//    - Trả về 1x1 transparent PNG image (hardcoded bytes)
//    - Luôn return pixel ngay cả khi có lỗi (để email client không hiển thị broken image)
//    - Không phải format CRUD chuẩn (JSON response)
// 3. Update operation đặc biệt:
//    - Sử dụng MongoDB $inc operator để tăng openCount
//    - Set openedAt chỉ khi chưa có (first open)
//    - Update operation phải không block response (async-friendly)
// 4. Error handling đặc biệt:
//    - Không return error response, luôn return pixel
//    - Log error nhưng vẫn return pixel để email client không bị lỗi
//
// KẾT LUẬN: Cần giữ endpoint đặc biệt vì đây là public endpoint trả về image content,
//           không phải JSON, và có error handling đặc biệt (luôn return pixel)
func (h *NotificationTrackHandler) HandleTrackOpen(c fiber.Ctx) error {
	return SafeHandlerWrapper(c, func() error {
		historyIDStr := c.Params("historyId")
		if historyIDStr == "" {
			// Return 1x1 transparent pixel
			c.Type("image/png")
			c.Send([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82})
			return nil
		}

		if !primitive.IsValidObjectID(historyIDStr) {
			c.Type("image/png")
			c.Send([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82})
			return nil
		}

		historyID := utility.String2ObjectID(historyIDStr)
		now := time.Now().Unix()

		// Lấy history hiện tại
		history, err := h.historyService.FindOneById(c.Context(), historyID)
		if err != nil {
			// Return pixel nếu không tìm thấy
			c.Type("image/png")
			c.Send([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82})
			return nil
		}

		// Update history với open tracking (sử dụng bson.M trực tiếp cho $inc)
		updateDoc := bson.M{
			"$inc": bson.M{"openCount": 1},
		}

		// Nếu chưa có OpenedAt, set nó
		if history.OpenedAt == nil {
			updateDoc["$set"] = bson.M{"openedAt": now}
		}

		_, err = h.historyService.UpdateOne(c.Context(), bson.M{"_id": historyID}, updateDoc, nil)
		if err != nil {
			// Log error nhưng vẫn return pixel
		}

		// Return 1x1 transparent pixel
		c.Type("image/png")
		c.Send([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82})
		return nil
	})
}

// HandleTrackClick xử lý tracking khi CTA được click
//
// LÝ DO PHẢI TẠO ENDPOINT ĐẶC BIỆT (không thể dùng CRUD chuẩn):
// 1. Public endpoint (không cần authentication):
//    - User click CTA trong email → redirect qua tracking URL
//    - Không có authentication token, chỉ có historyId và ctaIndex trong URL
//    - Phải redirect về original URL sau khi track
// 2. Logic nghiệp vụ phức tạp:
//    - Parse ctaIndex từ URL params (số nguyên)
//    - Lấy original URL từ query params (encoded)
//    - Update clickCount cho CTA tại index cụ thể (array update)
//    - Set clickedAt cho CTA tại index cụ thể (array update)
//    - Redirect về original URL (HTTP redirect, không phải JSON response)
// 3. Response format đặc biệt:
//    - Trả về HTTP redirect (302) về original URL
//    - Không phải format CRUD chuẩn (JSON response)
// 4. Update operation đặc biệt:
//    - Sử dụng MongoDB array update operators ($set với dot notation)
//    - Update nested field trong array: ctaClicks[index].clickCount, ctaClicks[index].clickedAt
//
// KẾT LUẬN: Cần giữ endpoint đặc biệt vì đây là public endpoint với redirect logic,
//           update nested array fields, và response format đặc biệt (HTTP redirect)
func (h *NotificationTrackHandler) HandleTrackClick(c fiber.Ctx) error {
	return SafeHandlerWrapper(c, func() error {
		historyIDStr := c.Params("historyId")
		ctaIndexStr := c.Params("ctaIndex")

		if historyIDStr == "" || ctaIndexStr == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": "historyId và ctaIndex là bắt buộc",
				"status":  "error",
			})
			return nil
		}

		if !primitive.IsValidObjectID(historyIDStr) {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": "historyId không hợp lệ",
				"status":  "error",
			})
			return nil
		}

		historyID := utility.String2ObjectID(historyIDStr)
		ctaIndex := 0
		if _, err := fmt.Sscanf(ctaIndexStr, "%d", &ctaIndex); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": "ctaIndex phải là số",
				"status":  "error",
			})
			return nil
		}

		// Lấy original URL từ query params
		originalURL := c.Query("url", "")
		if originalURL == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": "url là bắt buộc",
				"status":  "error",
			})
			return nil
		}

		// Decode base64 URL nếu cần
		if decoded, err := base64.URLEncoding.DecodeString(originalURL); err == nil {
			originalURL = string(decoded)
		}

		now := time.Now().Unix()

		// Lấy history hiện tại
		history, err := h.historyService.FindOneById(c.Context(), historyID)
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
		if ctaIndex >= 0 && ctaIndex < len(history.CTAClicks) {
			// Increment click count cho CTA này
			ctaPath := fmt.Sprintf("ctaClicks.%d", ctaIndex)
			updateDoc["$inc"].(bson.M)[fmt.Sprintf("%s.clickCount", ctaPath)] = 1
			updateDoc["$set"].(bson.M)[fmt.Sprintf("%s.lastClickAt", ctaPath)] = now

			// Set firstClickAt nếu chưa có
			if history.CTAClicks[ctaIndex].FirstClickAt == nil {
				updateDoc["$set"].(bson.M)[fmt.Sprintf("%s.firstClickAt", ctaPath)] = now
			}
		}

		_, err = h.historyService.UpdateOne(c.Context(), bson.M{"_id": historyID}, updateDoc, nil)
		if err != nil {
			// Log error nhưng vẫn redirect
		}

		// Redirect về original URL
		c.Redirect().To(originalURL)
		return nil
	})
}

// HandleTrackConfirm xử lý tracking khi notification được confirm (CTA "Đã xem")
func (h *NotificationTrackHandler) HandleTrackConfirm(c fiber.Ctx) error {
	return SafeHandlerWrapper(c, func() error {
		historyIDStr := c.Params("historyId")
		if historyIDStr == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": "historyId là bắt buộc",
				"status":  "error",
			})
			return nil
		}

		if !primitive.IsValidObjectID(historyIDStr) {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": "historyId không hợp lệ",
				"status":  "error",
			})
			return nil
		}

		historyID := utility.String2ObjectID(historyIDStr)
		now := time.Now().Unix()

		// Update confirmedAt
		update := services.UpdateData{
			Set: bson.M{
				"confirmedAt": now,
			},
		}

		_, err := h.historyService.UpdateOne(c.Context(), bson.M{"_id": historyID}, update, nil)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code":    common.ErrCodeDatabase.Code,
				"message": err.Error(),
				"status":  "error",
			})
			return nil
		}

		c.Status(common.StatusOK).JSON(fiber.Map{
			"code":    common.StatusOK,
			"message": "Notification đã được xác nhận",
			"data": map[string]interface{}{
				"confirmedAt": now,
			},
			"status": "success",
		})
		return nil
	})
}

