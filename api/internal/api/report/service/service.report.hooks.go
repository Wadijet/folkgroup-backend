// Package reportsvc — Hỗ trợ MarkDirty theo nhiều period; luồng datachanged ghi touch trong RAM (xem service.report.redis_touch.go).
package reportsvc

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// markDirtyForPeriods gọi MarkDirty cho từng (reportKey, periodKey).
// Chu kỳ tắt được chặn trong MarkDirty — không cần lọc ở đây.
func markDirtyForPeriods(ctx context.Context, reportSvc *ReportService, periodKeys map[string]string, ownerOrgID primitive.ObjectID) {
	for reportKey, periodKey := range periodKeys {
		_ = reportSvc.MarkDirty(ctx, reportKey, periodKey, ownerOrgID)
	}
}
