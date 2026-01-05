package handler

import (
	"context"
	"meta_commerce/core/common"
	"meta_commerce/core/cta"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CTATrackHandler xử lý tracking CTA clicks
type CTATrackHandler struct{}

// NewCTATrackHandler tạo mới CTATrackHandler
func NewCTATrackHandler() *CTATrackHandler {
	return &CTATrackHandler{}
}

// TrackCTAClick xử lý click vào CTA (public endpoint, không cần auth)
func (h *CTATrackHandler) TrackCTAClick(c fiber.Ctx) error {
	// Lấy historyId và ctaIndex từ params
	historyIDStr := c.Params("historyId")
	ctaIndexStr := c.Params("ctaIndex")

	if historyIDStr == "" || ctaIndexStr == "" {
		return c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code":    common.ErrCodeValidationFormat.Code,
			"message": "historyId and ctaIndex are required",
			"status":  "error",
		})
	}

	historyID, err := primitive.ObjectIDFromHex(historyIDStr)
	if err != nil {
		return c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code":    common.ErrCodeValidationFormat.Code,
			"message": "Invalid historyId",
			"status":  "error",
		})
	}

	ctaIndex, err := strconv.Atoi(ctaIndexStr)
	if err != nil {
		return c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code":    common.ErrCodeValidationFormat.Code,
			"message": "Invalid ctaIndex",
			"status":  "error",
		})
	}

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

	// TODO: Lấy ownerOrganizationID từ DeliveryHistory
	// Tạm thời dùng System Organization ID
	ctx := context.Background()
	systemOrgID, err := cta.GetSystemOrganizationID(ctx)
	if err != nil {
		return c.Status(common.StatusInternalServerError).JSON(fiber.Map{
			"code":    common.ErrCodeInternalServer.Code,
			"message": "Failed to get system organization",
			"status":  "error",
		})
	}

	// TODO: Lấy CTA code từ DeliveryHistory
	// Tạm thời dùng empty string
	ctaCode := ""

	// Ghi lại click
	err = cta.TrackCTAClick(ctx, historyID, ctaIndex, ctaCode, systemOrgID, ipAddress, userAgent)
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
