// Package decisionlive — facade đồng bộ độ sâu queue (logic trong queuedepth, tránh import cycle).
package decisionlive

import (
	"context"

	"meta_commerce/internal/api/aidecision/queuedepth"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RefreshQueueDepthForOrg đếm lại decision_events_queue cho một org từ Mongo → RAM.
func RefreshQueueDepthForOrg(ctx context.Context, ownerOrgID primitive.ObjectID) {
	_ = queuedepth.RefreshOrg(ctx, ownerOrgID)
}

// ReconcileQueueDepthFromMongo gom đếm toàn DB → RAM.
func ReconcileQueueDepthFromMongo(ctx context.Context) error {
	return queuedepth.ReconcileAllFromMongo(ctx)
}

// StartCommandCenterReconciler — reconcile lần đầu + ticker nền.
func StartCommandCenterReconciler(ctx context.Context) {
	queuedepth.StartBackground(ctx)
}

// normalizeQueueOrgHex — khóa org thống nhất với queuedepth.
func normalizeQueueOrgHex(s string) string {
	return queuedepth.NormalizeOrgHex(s)
}
