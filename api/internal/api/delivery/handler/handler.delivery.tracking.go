// Package deliveryhdl - TrackingHandler (xem basehdl.delivery.send.go cho package doc).
// File: basehdl.delivery.tracking.go - giữ tên cấu trúc cũ (basehdl.<domain>.<entity>.go).
package deliveryhdl

import (
	"encoding/base64"
	"fmt"
	"time"

	deliverydto "meta_commerce/internal/api/delivery/dto"
	deliverysvc "meta_commerce/internal/api/delivery/service"
	basehdl "meta_commerce/internal/api/base/handler"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/cta"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson"
)

// TrackingHandler xử lý tất cả các action tracking (open, click, confirm, cta)
type TrackingHandler struct {
	historyService *deliverysvc.DeliveryHistoryService
}

// NewTrackingHandler tạo mới TrackingHandler
func NewTrackingHandler() (*TrackingHandler, error) {
	historyService, err := deliverysvc.NewDeliveryHistoryService()
	if err != nil {
		return nil, fmt.Errorf("failed to create delivery history service: %v", err)
	}
	return &TrackingHandler{historyService: historyService}, nil
}

// HandleAction xử lý các action tracking (public endpoint)
func (h *TrackingHandler) HandleAction(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var params deliverydto.TrackingActionParams
		if err := h.parseRequestParams(c, &params); err != nil {
			return h.handleErrorByAction(c, params.Action, err)
		}
		if (params.Action == "click" || params.Action == "cta") && params.CTAIndex < 0 {
			return c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": fmt.Sprintf("ctaIndex is required for action '%s'", params.Action),
				"status":  "error",
			})
		}
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

func (h *TrackingHandler) handleOpenAction(c fiber.Ctx, params deliverydto.TrackingActionParams) error {
	now := time.Now().Unix()
	history, err := h.historyService.FindOneById(c.Context(), params.HistoryID)
	if err != nil {
		return h.returnPixel(c)
	}
	updateDoc := bson.M{"$inc": bson.M{"openCount": 1}}
	if history.OpenedAt == nil {
		updateDoc["$set"] = bson.M{"openedAt": now}
	}
	_, _ = h.historyService.UpdateOne(c.Context(), bson.M{"_id": params.HistoryID}, updateDoc, nil)
	return h.returnPixel(c)
}

func (h *TrackingHandler) handleClickAction(c fiber.Ctx, params deliverydto.TrackingActionParams) error {
	originalURL := c.Query("url", "")
	if originalURL == "" {
		return c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code": common.ErrCodeValidationFormat.Code, "message": "url parameter is required", "status": "error",
		})
	}
	if decoded, err := base64.URLEncoding.DecodeString(originalURL); err == nil {
		originalURL = string(decoded)
	}
	now := time.Now().Unix()
	history, err := h.historyService.FindOneById(c.Context(), params.HistoryID)
	if err != nil {
		c.Redirect().To(originalURL)
		return nil
	}
	updateDoc := bson.M{
		"$inc": bson.M{"clickCount": 1},
		"$set": bson.M{"lastClickAt": now},
	}
	if history.ClickedAt == nil {
		updateDoc["$set"].(bson.M)["clickedAt"] = now
	}
	if params.CTAIndex >= 0 && params.CTAIndex < len(history.CTAClicks) {
		ctaPath := fmt.Sprintf("ctaClicks.%d", params.CTAIndex)
		updateDoc["$inc"].(bson.M)[fmt.Sprintf("%s.clickCount", ctaPath)] = 1
		updateDoc["$set"].(bson.M)[fmt.Sprintf("%s.lastClickAt", ctaPath)] = now
		if history.CTAClicks[params.CTAIndex].FirstClickAt == nil {
			updateDoc["$set"].(bson.M)[fmt.Sprintf("%s.firstClickAt", ctaPath)] = now
		}
	}
	_, _ = h.historyService.UpdateOne(c.Context(), bson.M{"_id": params.HistoryID}, updateDoc, nil)
	c.Redirect().To(originalURL)
	return nil
}

func (h *TrackingHandler) handleConfirmAction(c fiber.Ctx, params deliverydto.TrackingActionParams) error {
	now := time.Now().Unix()
	update := basesvc.UpdateData{Set: bson.M{"confirmedAt": now}}
	_, err := h.historyService.UpdateOne(c.Context(), bson.M{"_id": params.HistoryID}, update, nil)
	if err != nil {
		return c.Status(common.StatusInternalServerError).JSON(fiber.Map{
			"code": common.ErrCodeDatabase.Code, "message": err.Error(), "status": "error",
		})
	}
	return c.Status(common.StatusOK).JSON(fiber.Map{
		"code": common.StatusOK, "message": "Notification đã được xác nhận",
		"data": map[string]interface{}{"confirmedAt": now}, "status": "success",
	})
}

func (h *TrackingHandler) handleCTAAction(c fiber.Ctx, params deliverydto.TrackingActionParams) error {
	encodedURL := c.Query("url")
	if encodedURL == "" {
		return c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code": common.ErrCodeValidationFormat.Code, "message": "url parameter is required", "status": "error",
		})
	}
	decodedURL, err := cta.DecodeTrackingURL(encodedURL)
	if err != nil {
		return c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code": common.ErrCodeValidationFormat.Code, "message": "Invalid encoded URL", "status": "error",
		})
	}
	ctx := c.Context()
	history, err := h.historyService.FindOneById(ctx, params.HistoryID)
	if err != nil {
		return c.Status(common.StatusNotFound).JSON(fiber.Map{
			"code": common.ErrCodeDatabaseQuery.Code, "message": "DeliveryHistory not found", "status": "error",
		})
	}
	ownerOrgID := history.OwnerOrganizationID
	ctaCode := ""
	if params.CTAIndex >= 0 && params.CTAIndex < len(history.CTAClicks) {
		ctaCode = ""
	}
	if err := cta.TrackCTAClick(ctx, params.HistoryID, params.CTAIndex, ctaCode, ownerOrgID, c.IP(), c.Get("User-Agent")); err != nil {
		return c.Status(common.StatusInternalServerError).JSON(fiber.Map{
			"code": common.ErrCodeInternalServer.Code, "message": "Failed to track CTA click", "details": err.Error(), "status": "error",
		})
	}
	c.Redirect().To(decodedURL)
	return nil
}

func (h *TrackingHandler) parseRequestParams(c fiber.Ctx, params *deliverydto.TrackingActionParams) error {
	params.Action = c.Params("action")
	if params.Action == "" {
		return common.NewError(common.ErrCodeValidationFormat, "action is required", common.StatusBadRequest, nil)
	}
	historyIDStr := c.Params("historyId")
	if historyIDStr == "" {
		return common.NewError(common.ErrCodeValidationFormat, "historyId is required", common.StatusBadRequest, nil)
	}
	var err error
	params.HistoryID, err = params.ParseHistoryID(historyIDStr)
	if err != nil {
		return err
	}
	ctaIndexStr := c.Query("ctaIndex")
	if ctaIndexStr != "" {
		params.CTAIndex, err = params.ParseCTAIndex(ctaIndexStr)
		if err != nil {
			return err
		}
	} else {
		params.CTAIndex = -1
	}
	return nil
}

func (h *TrackingHandler) returnPixel(c fiber.Ctx) error {
	c.Type("image/png")
	c.Send([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82})
	return nil
}

func (h *TrackingHandler) handleErrorByAction(c fiber.Ctx, action string, err error) error {
	switch action {
	case "open":
		return h.returnPixel(c)
	case "click", "cta":
		originalURL := c.Query("url", "")
		if originalURL != "" {
			if decoded, decodeErr := base64.URLEncoding.DecodeString(originalURL); decodeErr == nil {
				originalURL = string(decoded)
			}
			c.Redirect().To(originalURL)
			return nil
		}
		return c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code": common.ErrCodeValidationFormat.Code, "message": err.Error(), "status": "error",
		})
	default:
		return c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code": common.ErrCodeValidationFormat.Code, "message": err.Error(), "status": "error",
		})
	}
}
