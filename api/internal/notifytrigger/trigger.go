// Package notifytrigger — Trigger programmatic gửi thông báo qua hệ thống routing/template.
// Tách riêng để tránh import cycle (delivery ↔ notification).
package notifytrigger

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	deliverymodels "meta_commerce/internal/api/delivery/models"
	notifmodels "meta_commerce/internal/api/notification/models"
	notifsvc "meta_commerce/internal/api/notification/service"
	"meta_commerce/internal/cta"
	"meta_commerce/internal/delivery"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/notification"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TriggerProgrammatic gửi thông báo qua hệ thống routing/template (không qua HTTP).
// Dùng cho system alert, cron job, v.v.
//
// Tham số:
//   - ctx: context
//   - eventType: loại sự kiện (ví dụ: system_resource_overload)
//   - payload: biến để render template (ví dụ: cpuPercent, ramPercent, state, timestamp)
//   - organizationID: org nhận thông báo (ví dụ: System Organization)
//   - baseURL: base URL cho CTA (có thể rỗng nếu không dùng CTA)
//
// Trả về: số item đã enqueue, error
func TriggerProgrammatic(ctx context.Context, eventType string, payload map[string]interface{}, organizationID primitive.ObjectID, baseURL string) (int, error) {
	if payload == nil {
		payload = make(map[string]interface{})
	}
	if baseURL == "" {
		baseURL = "https://localhost"
	}
	if _, exists := payload["baseUrl"]; !exists {
		payload["baseUrl"] = baseURL
	}

	router, err := notification.NewRouter()
	if err != nil {
		return 0, fmt.Errorf("tạo router: %w", err)
	}
	template, err := notification.NewTemplate()
	if err != nil {
		return 0, fmt.Errorf("tạo template: %w", err)
	}
	queue, err := delivery.NewQueue()
	if err != nil {
		return 0, fmt.Errorf("tạo delivery queue: %w", err)
	}
	channelService, err := notifsvc.NewNotificationChannelService()
	if err != nil {
		return 0, fmt.Errorf("tạo channel service: %w", err)
	}
	senderService, err := notifsvc.NewNotificationSenderService()
	if err != nil {
		return 0, fmt.Errorf("tạo sender service: %w", err)
	}

	domain := notification.GetDomainFromEventType(eventType)
	severity := notification.GetSeverityFromEventType(eventType)
	orgIDPtr := &organizationID

	routes, err := router.FindRoutes(ctx, eventType, domain, severity, orgIDPtr)
	if err != nil {
		return 0, fmt.Errorf("tìm routes: %w", err)
	}
	if len(routes) == 0 {
		logger.GetAppLogger().WithField("eventType", eventType).Warn("🔔 [NOTIFICATION] Không có routes cho eventType")
		return 0, nil
	}

	queueItems := make([]*deliverymodels.DeliveryQueueItem, 0)
	now := time.Now().Unix()

	for _, route := range routes {
		channel, err := channelService.FindOneById(ctx, route.ChannelID)
		if err != nil {
			logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
				"channelId": route.ChannelID.Hex(),
				"eventType": eventType,
			}).Warn("🔔 [NOTIFICATION] Không tìm thấy channel, bỏ qua route")
			continue
		}
		if !channel.IsActive {
			continue
		}

		tpl, err := template.FindTemplate(ctx, eventType, channel.ChannelType, route.OrganizationID)
		if err != nil {
			logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
				"eventType":   eventType,
				"channelType": channel.ChannelType,
			}).Warn("🔔 [NOTIFICATION] Không tìm thấy template, bỏ qua route")
			continue
		}

		rendered, err := template.Render(ctx, tpl, payload, route.OrganizationID, baseURL)
		if err != nil {
			logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
				"eventType":   eventType,
				"channelType": channel.ChannelType,
			}).Error("🔔 [NOTIFICATION] Lỗi render template")
			continue
		}

		ctaJSONs := make([]string, 0, len(rendered.CTAs))
		for _, cta := range rendered.CTAs {
			if j, err := json.Marshal(cta); err == nil {
				ctaJSONs = append(ctaJSONs, string(j))
			}
		}

		var recipients []string
		switch channel.ChannelType {
		case "email":
			recipients = channel.Recipients
		case "telegram":
			recipients = channel.ChatIDs
		case "webhook":
			if channel.WebhookURL != "" {
				recipients = []string{channel.WebhookURL}
			}
		default:
			continue
		}
		if len(recipients) == 0 {
			continue
		}

		sender, senderID, err := findSenderForChannel(ctx, senderService, &channel, route.OrganizationID)
		if err != nil {
			logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
				"channelId":   channel.ID.Hex(),
				"channelType": channel.ChannelType,
			}).Warn("🔔 [NOTIFICATION] Không tìm thấy sender, bỏ qua route")
			continue
		}

		var encryptedSenderConfig string
		if sender != nil {
			if j, err := json.Marshal(sender); err == nil {
				encryptedSenderConfig, _ = delivery.EncryptSenderConfig(j)
			}
		}

		priority := notification.GetPriorityFromSeverity(severity)
		maxRetries := notification.GetMaxRetriesFromSeverity(severity)

		for _, recipient := range recipients {
			queueItems = append(queueItems, &deliverymodels.DeliveryQueueItem{
				ID:                  primitive.NewObjectID(),
				EventType:           eventType,
				OwnerOrganizationID: route.OrganizationID,
				SenderID:            senderID,
				SenderConfig:        encryptedSenderConfig,
				ChannelType:         channel.ChannelType,
				Recipient:           recipient,
				Subject:             rendered.Subject,
				Content:             rendered.Content,
				CTAs:                ctaJSONs,
				Payload:             payload,
				Status:              "pending",
				RetryCount:          0,
				MaxRetries:          maxRetries,
				Priority:            priority,
				CreatedAt:           now,
				UpdatedAt:           now,
			})
		}
	}

	if len(queueItems) == 0 {
		return 0, nil
	}

	if err := queue.Enqueue(ctx, queueItems); err != nil {
		return 0, fmt.Errorf("enqueue: %w", err)
	}
	return len(queueItems), nil
}

// findSenderForChannel tìm sender cho channel.
func findSenderForChannel(ctx context.Context, senderService *notifsvc.NotificationSenderService, channel *notifmodels.NotificationChannel, organizationID primitive.ObjectID) (*notifmodels.NotificationChannelSender, primitive.ObjectID, error) {
	if len(channel.SenderIDs) > 0 {
		for _, senderID := range channel.SenderIDs {
			sender, err := senderService.FindOneById(ctx, senderID)
			if err == nil && sender.IsActive && sender.ChannelType == channel.ChannelType {
				return &sender, senderID, nil
			}
		}
	}

	systemOrgID, err := cta.GetSystemOrganizationID(ctx)
	if err != nil {
		return nil, primitive.NilObjectID, fmt.Errorf("lấy system org ID: %w", err)
	}

	filter := bson.M{
		"channelType": channel.ChannelType,
		"isActive":    true,
		"$or": []bson.M{
			{"ownerOrganizationId": organizationID},
			{"ownerOrganizationId": systemOrgID},
		},
	}

	senders, err := senderService.Find(ctx, filter, nil)
	if err != nil || len(senders) == 0 {
		return nil, primitive.NilObjectID, fmt.Errorf("không tìm thấy sender cho %s", channel.ChannelType)
	}

	for _, s := range senders {
		if s.OwnerOrganizationID != nil && s.OwnerOrganizationID.Hex() == organizationID.Hex() {
			return &s, s.ID, nil
		}
	}
	for _, s := range senders {
		if s.OwnerOrganizationID != nil && s.OwnerOrganizationID.Hex() == systemOrgID.Hex() {
			return &s, s.ID, nil
		}
	}
	return &senders[0], senders[0].ID, nil
}
