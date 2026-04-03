// Package datachanged — Miền báo cáo: chạm dirty sau thay đổi nguồn (Redis touch / MarkDirty qua luồng worker flush).
package datachanged

import (
	"context"

	"meta_commerce/internal/api/events"
	reportsvc "meta_commerce/internal/api/report/service"
)

// RecordTouchFromDataChange ghi nhận thay đổi để báo cáo theo chu kỳ tính lại (không MarkDirty trực tiếp tại đây).
func RecordTouchFromDataChange(ctx context.Context, e events.DataChangeEvent) {
	reportsvc.RecordReportTouchFromDataChange(ctx, e)
}
