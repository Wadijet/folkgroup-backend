package cta

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RenderedCTA là CTA đã được render với tracking URL
type RenderedCTA struct {
	Code        string `json:"code"`        // Mã CTA
	Label       string `json:"label"`       // Label đã render
	URL         string `json:"url"`         // Tracking URL (redirect qua tracking endpoint)
	OriginalURL string `json:"originalUrl"` // URL gốc (sau khi render variables)
	Style       string `json:"style"`      // Style: "primary", "success", "secondary", "danger"
	Index       int    `json:"index"`       // Index trong mảng CTAs (0-based)
}

// CTARenderRequest là request để render CTAs
type CTARenderRequest struct {
	CTACodes        []string               `json:"ctaCodes"`        // Danh sách mã CTA cần render
	Variables       map[string]interface{} `json:"variables"`       // Variables để render vào CTA
	BaseURL         string                 `json:"baseURL"`         // Base URL cho tracking endpoint
	TrackingEnabled bool                   `json:"trackingEnabled"` // Có bật tracking không
	HistoryID       *primitive.ObjectID    `json:"historyId"`       // ID của DeliveryHistory (nếu có)
	OrganizationID  primitive.ObjectID     `json:"organizationId"` // Organization ID để tìm CTA (organization-specific → system)
}

// CTARenderResponse là response sau khi render CTAs
type CTARenderResponse struct {
	CTAs []RenderedCTA `json:"ctas"`
}

// Renderer xử lý việc render CTA
type Renderer struct {
	ctaLibraryService *services.CTALibraryService
}

// NewRenderer tạo mới Renderer
func NewRenderer() (*Renderer, error) {
	ctaLibraryService, err := services.NewCTALibraryService()
	if err != nil {
		return nil, fmt.Errorf("failed to create CTA library service: %w", err)
	}

	return &Renderer{
		ctaLibraryService: ctaLibraryService,
	}, nil
}

// FindCTA tìm CTA theo code và organization ID
// Logic: Tìm organization-specific trước, nếu không có → tìm System CTA
func (r *Renderer) FindCTA(ctx context.Context, code string, organizationID primitive.ObjectID) (*models.CTALibrary, error) {
	// 1. Tìm organization-specific CTA
	filter := bson.M{
		"code":                code,
		"ownerOrganizationId": organizationID,
		"isActive":            true,
	}

	cta, err := r.ctaLibraryService.FindOne(ctx, filter, nil)
	if err == nil {
		return &cta, nil
	}
	if !errors.Is(err, common.ErrNotFound) {
		return nil, fmt.Errorf("failed to find organization-specific CTA: %w", err)
	}

	// 2. Nếu không có → Tìm System CTA
	systemOrgID, err := GetSystemOrganizationID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get System Organization ID: %w", err)
	}

	filter = bson.M{
		"code":                code,
		"ownerOrganizationId": systemOrgID,
		"isActive":            true,
	}

	cta, err = r.ctaLibraryService.FindOne(ctx, filter, nil)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return nil, fmt.Errorf("CTA not found for code=%s", code)
		}
		return nil, fmt.Errorf("failed to find system CTA: %w", err)
	}

	return &cta, nil
}

// RenderCTA render một CTA với variables
func (r *Renderer) RenderCTA(cta *models.CTALibrary, variables map[string]interface{}) (string, string, error) {
	// Render label
	label := cta.Label
	for _, variable := range cta.Variables {
		value, exists := variables[variable]
		if !exists {
			value = ""
		}
		placeholder := "{{" + variable + "}}"
		label = strings.ReplaceAll(label, placeholder, fmt.Sprintf("%v", value))
	}

	// Render action URL
	action := cta.Action
	for _, variable := range cta.Variables {
		value, exists := variables[variable]
		if !exists {
			value = ""
		}
		placeholder := "{{" + variable + "}}"
		action = strings.ReplaceAll(action, placeholder, fmt.Sprintf("%v", value))
	}

	// Render {{baseUrl}} đặc biệt (nếu có trong variables)
	if baseUrl, exists := variables["baseUrl"]; exists {
		action = strings.ReplaceAll(action, "{{baseUrl}}", fmt.Sprintf("%v", baseUrl))
	}

	return label, action, nil
}

// RenderCTAs render nhiều CTAs với tracking
func (r *Renderer) RenderCTAs(ctx context.Context, req CTARenderRequest) (*CTARenderResponse, error) {
	renderedCTAs := []RenderedCTA{}

	for index, code := range req.CTACodes {
		// Tìm CTA
		cta, err := r.FindCTA(ctx, code, req.OrganizationID)
		if err != nil {
			return nil, fmt.Errorf("failed to find CTA with code=%s: %w", code, err)
		}

		// Render CTA
		label, originalURL, err := r.RenderCTA(cta, req.Variables)
		if err != nil {
			return nil, fmt.Errorf("failed to render CTA with code=%s: %w", code, err)
		}

		renderedCTA := RenderedCTA{
			Code:        code,
			Label:       label,
			OriginalURL: originalURL,
			Style:       cta.Style,
			Index:       index,
		}

		// Nếu có tracking và historyID, tạo tracking URL
		if req.TrackingEnabled && req.HistoryID != nil && !req.HistoryID.IsZero() {
			trackingURL, err := r.createTrackingURL(req.BaseURL, *req.HistoryID, index, originalURL)
			if err != nil {
				return nil, fmt.Errorf("failed to create tracking URL: %w", err)
			}
			renderedCTA.URL = trackingURL
		} else {
			// Không có tracking, dùng original URL
			renderedCTA.URL = originalURL
		}

		renderedCTAs = append(renderedCTAs, renderedCTA)
	}

	return &CTARenderResponse{
		CTAs: renderedCTAs,
	}, nil
}

// createTrackingURL tạo tracking URL
// Format: /api/v1/track/:action/:historyId?ctaIndex=...&url=<base64_encoded_original_url>
// Action: "cta" (đã gộp vào unified tracking endpoint)
func (r *Renderer) createTrackingURL(baseURL string, historyID primitive.ObjectID, ctaIndex int, originalURL string) (string, error) {
	// Encode original URL thành base64
	encodedURL := base64.URLEncoding.EncodeToString([]byte(originalURL))

	// Tạo tracking URL với unified endpoint (action="cta", ctaIndex trong query param)
	trackingPath := fmt.Sprintf("/api/v1/track/cta/%s", historyID.Hex())
	trackingURL := fmt.Sprintf("%s%s?ctaIndex=%d&url=%s", baseURL, trackingPath, ctaIndex, url.QueryEscape(encodedURL))

	return trackingURL, nil
}

// DecodeTrackingURL decode tracking URL từ base64
func DecodeTrackingURL(encodedURL string) (string, error) {
	decoded, err := base64.URLEncoding.DecodeString(encodedURL)
	if err != nil {
		return "", fmt.Errorf("failed to decode URL: %w", err)
	}
	return string(decoded), nil
}

// TrackCTAClick ghi lại click vào CTA
func TrackCTAClick(ctx context.Context, historyID primitive.ObjectID, ctaIndex int, ctaCode string, ownerOrganizationID primitive.ObjectID, ipAddress, userAgent string) error {
	trackingService, err := services.NewCTATrackingService()
	if err != nil {
		return fmt.Errorf("failed to create CTA tracking service: %w", err)
	}

	tracking := models.CTATracking{
		OwnerOrganizationID: ownerOrganizationID,
		HistoryID:            historyID,
		CTAIndex:            ctaIndex,
		CTACode:              ctaCode,
		ClickedAt:             time.Now().Unix(),
		IPAddress:             ipAddress,
		UserAgent:             userAgent,
		CreatedAt:             time.Now().Unix(),
	}

	_, err = trackingService.InsertOne(ctx, tracking)
	if err != nil {
		return fmt.Errorf("failed to insert CTA tracking: %w", err)
	}

	return nil
}
