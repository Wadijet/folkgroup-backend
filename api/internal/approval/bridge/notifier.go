// Package bridge — Implementation Notifier cho pkg/approval (notifytrigger).
package bridge

import (
	"context"

	"meta_commerce/internal/logger"
	"meta_commerce/internal/notifytrigger"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NotifytriggerNotifier implement pkg/approval.Notifier.
type NotifytriggerNotifier struct{}

// NewNotifytriggerNotifier tạo notifier dùng notifytrigger.
func NewNotifytriggerNotifier() *NotifytriggerNotifier {
	return &NotifytriggerNotifier{}
}

// Notify gửi thông báo qua notifytrigger.
func (n *NotifytriggerNotifier) Notify(ctx context.Context, eventType string, payload map[string]interface{}, ownerOrgID primitive.ObjectID, baseURL string) (int, error) {
	count, err := notifytrigger.TriggerProgrammatic(ctx, eventType, payload, ownerOrgID, baseURL)
	if err != nil {
		logger.GetAppLogger().WithError(err).Warn("[APPROVAL] Trigger notification thất bại")
	}
	if count > 0 {
		logger.GetAppLogger().WithField("eventType", eventType).Info("[APPROVAL] Đã gửi thông báo đề xuất")
	}
	return count, err
}
