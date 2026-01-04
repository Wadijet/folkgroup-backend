package handler

import (
	"context"
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
		return c.Status(400).JSON(fiber.Map{
			"error": "historyId and ctaIndex are required",
		})
	}

	historyID, err := primitive.ObjectIDFromHex(historyIDStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid historyId",
		})
	}

	ctaIndex, err := strconv.Atoi(ctaIndexStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid ctaIndex",
		})
	}

	// Lấy original URL từ query param
	encodedURL := c.Query("url")
	if encodedURL == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "url parameter is required",
		})
	}

	// Decode URL
	decodedURL, err := cta.DecodeTrackingURL(encodedURL)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid encoded URL",
		})
	}

	// Lấy IP address và User Agent
	ipAddress := c.IP()
	userAgent := c.Get("User-Agent")

	// TODO: Lấy ownerOrganizationID từ NotificationHistory
	// Tạm thời dùng System Organization ID
	ctx := context.Background()
	systemOrgID, err := cta.GetSystemOrganizationID(ctx)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to get system organization",
		})
	}

	// TODO: Lấy CTA code từ NotificationHistory
	// Tạm thời dùng empty string
	ctaCode := ""

	// Ghi lại click
	err = cta.TrackCTAClick(ctx, historyID, ctaIndex, ctaCode, systemOrgID, ipAddress, userAgent)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to track CTA click",
			"details": err.Error(),
		})
	}

	// Redirect về original URL
	c.Redirect().To(decodedURL)
	return nil
}
