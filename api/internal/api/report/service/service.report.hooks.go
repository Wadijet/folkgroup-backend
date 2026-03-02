// Package reportsvc - hook đăng ký xử lý event thay đổi dữ liệu để MarkDirty báo cáo.
package reportsvc

import (
	"context"
	"time"

	"meta_commerce/internal/api/events"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func init() {
	events.OnDataChanged(handleReportDataChange)
}

// markDirtyForPeriods gọi MarkDirty cho từng (reportKey, periodKey).
// Chu kỳ tắt được chặn trong MarkDirty — không cần lọc ở đây.
func markDirtyForPeriods(ctx context.Context, reportSvc *ReportService, periodKeys map[string]string, ownerOrgID primitive.ObjectID) {
	for reportKey, periodKey := range periodKeys {
		_ = reportSvc.MarkDirty(ctx, reportKey, periodKey, ownerOrgID)
	}
}

// handleReportDataChange xử lý event thay đổi dữ liệu: đánh dấu dirty các báo cáo theo chu kỳ.
// Xử lý: pc_pos_orders (order_*), pc_pos_customers (customer_*), crm_activity_history (customer_* — snapshot tính từ đây).
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
		// Chỉ MarkDirty khi field ảnh hưởng period thay đổi (OpUpdate + PreviousDocument)
		if e.Operation == events.OpUpdate && e.PreviousDocument != nil {
			tsNew := events.GetPeriodTimestamp(e.Document, e.CollectionName)
			tsPrev := events.GetPeriodTimestamp(e.PreviousDocument, e.CollectionName)
			if tsNew == tsPrev && tsNew != 0 {
				return
			}
		}
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
		// Chỉ MarkDirty order_daily; weekly/monthly/yearly tính on-demand khi xem (giống customer).
		orderReportKeys := GetActiveOrderReportKeys()
		periodKeys, err := reportSvc.GetDirtyPeriodKeysForReportKeys(ctx, orderReportKeys, ts)
		if err != nil || len(periodKeys) == 0 {
			return
		}
		markDirtyForPeriods(ctx, reportSvc, periodKeys, ownerOrgID)
	case global.MongoDB_ColNames.PcPosCustomers:
		if e.Operation == events.OpUpdate && e.PreviousDocument != nil {
			tsNew := events.GetPeriodTimestamp(e.Document, e.CollectionName)
			tsPrev := events.GetPeriodTimestamp(e.PreviousDocument, e.CollectionName)
			if tsNew == tsPrev && tsNew != 0 {
				return
			}
		}
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
		customerReportKeys := GetActiveCustomerReportKeys()
		periodKeys, err := reportSvc.GetDirtyPeriodKeysForReportKeys(ctx, customerReportKeys, ts)
		if err != nil || len(periodKeys) == 0 {
			return
		}
		markDirtyForPeriods(ctx, reportSvc, periodKeys, ownerOrgID)
	case global.MongoDB_ColNames.CrmActivityHistory:
		if e.Operation == events.OpUpdate && e.PreviousDocument != nil {
			tsNew := events.GetPeriodTimestamp(e.Document, e.CollectionName)
			tsPrev := events.GetPeriodTimestamp(e.PreviousDocument, e.CollectionName)
			if tsNew == tsPrev && tsNew != 0 {
				return
			}
		}
		// Snapshot customer tính từ crm_activity_history — khi activity mới/đổi cần recompute.
		ts = events.GetInt64Field(e.Document, "ActivityAt")
		if ts == 0 {
			ts = events.GetInt64Field(e.Document, "CreatedAt")
		}
		if ts == 0 {
			ts = time.Now().Unix()
		}
		if ts > 1e12 {
			ts = ts / 1000
		}
		customerReportKeys := GetActiveCustomerReportKeys()
		periodKeys, err := reportSvc.GetDirtyPeriodKeysForReportKeys(ctx, customerReportKeys, ts)
		if err != nil || len(periodKeys) == 0 {
			return
		}
		markDirtyForPeriods(ctx, reportSvc, periodKeys, ownerOrgID)
	default:
		return
	}
}
