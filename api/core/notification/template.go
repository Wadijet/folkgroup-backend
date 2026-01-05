package notification

import (
	"context"
	"errors"
	"fmt"
	"strings"

	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"
	"meta_commerce/core/cta"
	"meta_commerce/core/notification/channels"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Template xử lý việc tìm và render template
type Template struct {
	templateService *services.NotificationTemplateService
	ctaRenderer     *cta.Renderer
}

// NewTemplate tạo mới Template
func NewTemplate() (*Template, error) {
	templateService, err := services.NewNotificationTemplateService()
	if err != nil {
		return nil, fmt.Errorf("failed to create template service: %w", err)
	}

	ctaRenderer, err := cta.NewRenderer()
	if err != nil {
		return nil, fmt.Errorf("failed to create CTA renderer: %w", err)
	}

	return &Template{
		templateService: templateService,
		ctaRenderer:     ctaRenderer,
	}, nil
}

// FindTemplate tìm template theo EventType, ChannelType, và OrganizationID
// Logic: Tìm team-specific trước, nếu không có → tìm system template
func (t *Template) FindTemplate(ctx context.Context, eventType string, channelType string, organizationID primitive.ObjectID) (*models.NotificationTemplate, error) {
	// 1. Tìm team-specific template
	filter := bson.M{
		"eventType":           eventType,
		"channelType":         channelType,
		"ownerOrganizationId": organizationID,
		"isActive":            true,
	}

	template, err := t.templateService.FindOne(ctx, filter, nil)
	if err == nil {
		return &template, nil
	}
	if !errors.Is(err, common.ErrNotFound) {
		return nil, fmt.Errorf("failed to find team-specific template: %w", err)
	}

	// 2. Nếu không có → Tìm system template (ownerOrganizationId = systemOrg.ID)
	// System Organization là cấp cao nhất, chứa templates mặc định cho tất cả organizations
	systemOrgID, err := cta.GetSystemOrganizationID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get system organization ID: %w", err)
	}

	filter = bson.M{
		"eventType":           eventType,
		"channelType":         channelType,
		"ownerOrganizationId": systemOrgID,
		"isActive":            true,
	}

	template, err = t.templateService.FindOne(ctx, filter, nil)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return nil, fmt.Errorf("template not found for eventType=%s, channelType=%s", eventType, channelType)
		}
		return nil, fmt.Errorf("failed to find system template: %w", err)
	}

	return &template, nil
}

// RenderedTemplate và RenderedCTA đã được di chuyển vào channels package để tránh import cycle

// Render render template với payload và organization ID
// CTAs sẽ được render từ CTACodes (nếu có) hoặc từ CTAs cũ (backward compatibility)
func (t *Template) Render(ctx context.Context, template *models.NotificationTemplate, payload map[string]interface{}, organizationID primitive.ObjectID, baseURL string) (*channels.RenderedTemplate, error) {
	// Render subject
	subject := template.Subject
	for _, variable := range template.Variables {
		value, exists := payload[variable]
		if !exists {
			value = ""
		}
		placeholder := "{{" + variable + "}}"
		subject = strings.ReplaceAll(subject, placeholder, fmt.Sprintf("%v", value))
	}

	// Render content
	content := template.Content
	for _, variable := range template.Variables {
		value, exists := payload[variable]
		if !exists {
			value = ""
		}
		placeholder := "{{" + variable + "}}"
		content = strings.ReplaceAll(content, placeholder, fmt.Sprintf("%v", value))
	}

	// Render CTAs
	renderedCTAs := []channels.RenderedCTA{}

	// Ưu tiên dùng CTACodes (mới) nếu có
	if len(template.CTACodes) > 0 {
		// Render CTAs từ CTACodes bằng CTA module
		ctaReq := cta.CTARenderRequest{
			CTACodes:        template.CTACodes,
			Variables:       payload,
			BaseURL:         baseURL,
			TrackingEnabled: false, // Tracking sẽ được thêm sau bởi delivery processor
			HistoryID:       nil,    // Chưa có history ID lúc này
			OrganizationID:  organizationID,
		}

		ctaResponse, err := t.ctaRenderer.RenderCTAs(ctx, ctaReq)
		if err != nil {
			return nil, fmt.Errorf("failed to render CTAs: %w", err)
		}

		// Convert từ cta.RenderedCTA sang channels.RenderedCTA
		for _, ctaRendered := range ctaResponse.CTAs {
			renderedCTAs = append(renderedCTAs, channels.RenderedCTA{
				Label:       ctaRendered.Label,
				Action:      ctaRendered.OriginalURL, // Dùng original URL, tracking sẽ được thêm sau
				OriginalURL: ctaRendered.OriginalURL,
				Style:       ctaRendered.Style,
			})
		}
	} else if len(template.CTAs) > 0 {
		// Backward compatibility: Render CTAs cũ (nếu không có CTACodes)
		for _, cta := range template.CTAs {
			renderedCTA := channels.RenderedCTA{
				Label:  cta.Label,
				Action: cta.Action,
				Style:  cta.Style,
			}

			// Render variables trong Action
			for _, variable := range template.Variables {
				value, exists := payload[variable]
				if !exists {
					value = ""
				}
				placeholder := "{{" + variable + "}}"
				renderedCTA.Action = strings.ReplaceAll(renderedCTA.Action, placeholder, fmt.Sprintf("%v", value))
			}

			// Render {{baseUrl}} đặc biệt
			if baseUrl, exists := payload["baseUrl"]; exists {
				renderedCTA.Action = strings.ReplaceAll(renderedCTA.Action, "{{baseUrl}}", fmt.Sprintf("%v", baseUrl))
			}

			renderedCTA.OriginalURL = renderedCTA.Action
			renderedCTAs = append(renderedCTAs, renderedCTA)
		}
	}

	return &channels.RenderedTemplate{
		Subject: subject,
		Content: content,
		CTAs:    renderedCTAs,
	}, nil
}

