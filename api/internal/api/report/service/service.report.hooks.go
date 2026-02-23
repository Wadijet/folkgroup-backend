// Package reportsvc - hook đăng ký xử lý event thay đổi dữ liệu để MarkDirty báo cáo.
package reportsvc

import (
	"context"

	"meta_commerce/internal/api/events"
	"meta_commerce/internal/global"
)

func init() {
	events.OnDataChanged(handleReportDataChange)
}

// handleReportDataChange xử lý event thay đổi dữ liệu: đánh dấu dirty các báo cáo theo chu kỳ.
// Chỉ xử lý collection pc_pos_orders (có thể mở rộng cho collection khác sau).
func handleReportDataChange(ctx context.Context, e events.DataChangeEvent) {
	if e.CollectionName != global.MongoDB_ColNames.PcPosOrders {
		return
	}
	if e.Document == nil {
		return
	}
	// Lấy timestamp: posCreatedAt (fallback insertedAt, createdAt). Đơn vị giây.
	ts := events.GetInt64Field(e.Document, "PosCreatedAt")
	if ts == 0 {
		ts = events.GetInt64Field(e.Document, "InsertedAt")
	}
	if ts == 0 {
		ts = events.GetInt64Field(e.Document, "CreatedAt")
	}
	if ts == 0 {
		return
	}
	// Nếu là millisecond (> 1e12), chuyển sang giây
	if ts > 1e12 {
		ts = ts / 1000
	}
	ownerOrgID := events.GetOwnerOrganizationIDFromDocument(e.Document)
	if ownerOrgID.IsZero() {
		return
	}
	reportSvc, err := NewReportService()
	if err != nil {
		return
	}
	periodKeys, err := reportSvc.GetDirtyPeriodKeysForCollection(ctx, e.CollectionName, ts)
	if err != nil || len(periodKeys) == 0 {
		return
	}
	for reportKey, periodKey := range periodKeys {
		_ = reportSvc.MarkDirty(ctx, reportKey, periodKey, ownerOrgID)
	}
}
