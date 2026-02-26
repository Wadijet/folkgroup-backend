// Package reportsvc - hook đăng ký xử lý event thay đổi dữ liệu để MarkDirty báo cáo.
package reportsvc

import (
	"context"
	"time"

	"meta_commerce/internal/api/events"
	"meta_commerce/internal/global"
)

func init() {
	events.OnDataChanged(handleReportDataChange)
}

// handleReportDataChange xử lý event thay đổi dữ liệu: đánh dấu dirty các báo cáo theo chu kỳ.
// Xử lý: pc_pos_orders (order_daily, customer_daily, ...), pc_pos_customers (customer_*).
func handleReportDataChange(ctx context.Context, e events.DataChangeEvent) {
	if e.Document == nil {
		return
	}
	ownerOrgID := events.GetOwnerOrganizationIDFromDocument(e.Document)
	if ownerOrgID.IsZero() {
		return
	}

	ts := int64(0)

	reportSvc, err := NewReportService()
	if err != nil {
		return
	}

	switch e.CollectionName {
	case global.MongoDB_ColNames.PcPosOrders:
		ts = events.GetInt64Field(e.Document, "PosCreatedAt")
		if ts == 0 {
			ts = events.GetInt64Field(e.Document, "InsertedAt")
		}
		if ts == 0 {
			ts = events.GetInt64Field(e.Document, "CreatedAt")
		}
		if ts == 0 {
			return
		}
		if ts > 1e12 {
			ts = ts / 1000
		}
		periodKeys, err := reportSvc.GetDirtyPeriodKeysForCollection(ctx, e.CollectionName, ts)
		if err != nil || len(periodKeys) == 0 {
			return
		}
		for reportKey, periodKey := range periodKeys {
			_ = reportSvc.MarkDirty(ctx, reportKey, periodKey, ownerOrgID)
		}
	case global.MongoDB_ColNames.PcPosCustomers:
		ts = events.GetInt64Field(e.Document, "UpdatedAt")
		if ts == 0 {
			ts = events.GetInt64Field(e.Document, "LastOrderAt")
		}
		if ts == 0 {
			ts = events.GetInt64Field(e.Document, "CreatedAt")
		}
		if ts == 0 {
			ts = time.Now().Unix()
		}
		if ts > 1e12 {
			ts = ts / 1000
		}
		customerReportKeys := []string{"customer_daily", "customer_weekly", "customer_monthly", "customer_yearly"}
		periodKeys, err := reportSvc.GetDirtyPeriodKeysForReportKeys(ctx, customerReportKeys, ts)
		if err != nil || len(periodKeys) == 0 {
			return
		}
		for reportKey, periodKey := range periodKeys {
			_ = reportSvc.MarkDirty(ctx, reportKey, periodKey, ownerOrgID)
		}
	default:
		return
	}
}
