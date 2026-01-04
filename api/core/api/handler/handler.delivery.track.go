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

// DeliveryTrackHandler xử lý tracking opens và clicks (Hệ thống 1)
// Alias của NotificationTrackHandler để dùng trong delivery namespace
type DeliveryTrackHandler struct {
	historyService *services.NotificationHistoryService
}

// NewDeliveryTrackHandler tạo mới DeliveryTrackHandler
func NewDeliveryTrackHandler() (*DeliveryTrackHandler, error) {
	historyService, err := services.NewNotificationHistoryService()
	if err != nil {
		return nil, fmt.Errorf("failed to create notification history service: %v", err)
	}

	return &DeliveryTrackHandler{
		historyService: historyService,
	}, nil
}

// HandleTrackOpen xử lý tracking khi email được mở (tracking pixel)
func (h *DeliveryTrackHandler) HandleTrackOpen(c fiber.Ctx) error {
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

		// Update history với open tracking
		updateDoc := bson.M{
			"$inc": bson.M{"openCount": 1},
		}

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
func (h *DeliveryTrackHandler) HandleTrackClick(c fiber.Ctx) error {
	return SafeHandlerWrapper(c, func() error {
		historyIDStr := c.Params("historyId")
		ctaIndexStr := c.Params("ctaIndex")

		if historyIDStr == "" || ctaIndexStr == "" {
			c.Status(400)
			c.JSON(map[string]string{"error": "historyId và ctaIndex là bắt buộc"})
			return nil
		}

		if !primitive.IsValidObjectID(historyIDStr) {
			c.Status(400)
			c.JSON(map[string]string{"error": "historyId không hợp lệ"})
			return nil
		}

		historyID := utility.String2ObjectID(historyIDStr)
		ctaIndex := 0
		if _, err := fmt.Sscanf(ctaIndexStr, "%d", &ctaIndex); err != nil {
			c.Status(400)
			c.JSON(map[string]string{"error": "ctaIndex phải là số"})
			return nil
		}

		// Lấy original URL từ query params
		originalURL := c.Query("url", "")
		if originalURL == "" {
			c.Status(400)
			c.JSON(map[string]string{"error": "url là bắt buộc"})
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

		// Update click tracking
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

		// Update CTA click tracking
		if ctaIndex >= 0 && ctaIndex < len(history.CTAClicks) {
			ctaPath := fmt.Sprintf("ctaClicks.%d", ctaIndex)
			updateDoc["$inc"].(bson.M)[fmt.Sprintf("%s.clickCount", ctaPath)] = 1
			updateDoc["$set"].(bson.M)[fmt.Sprintf("%s.lastClickAt", ctaPath)] = now

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

// HandleTrackConfirm xử lý tracking khi notification được confirm
func (h *DeliveryTrackHandler) HandleTrackConfirm(c fiber.Ctx) error {
	return SafeHandlerWrapper(c, func() error {
		historyIDStr := c.Params("historyId")
		if historyIDStr == "" {
			c.Status(400)
			c.JSON(map[string]string{"error": "historyId là bắt buộc"})
			return nil
		}

		if !primitive.IsValidObjectID(historyIDStr) {
			c.Status(400)
			c.JSON(map[string]string{"error": "historyId không hợp lệ"})
			return nil
		}

		historyID := utility.String2ObjectID(historyIDStr)
		now := time.Now().Unix()

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

		c.JSON(map[string]interface{}{
			"message":     "Notification đã được xác nhận",
			"confirmedAt": now,
		})
		return nil
	})
}
