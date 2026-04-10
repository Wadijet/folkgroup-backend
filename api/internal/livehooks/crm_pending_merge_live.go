// Package livehooks — Gắn publish timeline vào worker nền (tránh import cycle internal/worker ↔ decisionlive).
package livehooks

import (
	"strings"

	"meta_commerce/internal/api/aidecision/decisionlive"
	crmmodels "meta_commerce/internal/api/crm/models"
	"meta_commerce/internal/worker"
)

func crmPendingMergeExtra(item *crmmodels.CrmPendingMerge) map[string]string {
	if item == nil {
		return nil
	}
	m := map[string]string{"jobId": item.ID.Hex(), "collection": item.CollectionName}
	if item.CoalesceKey != "" {
		m["coalesceKey"] = item.CoalesceKey
	}
	return m
}

// RegisterCrmPendingMergeLiveHooks đăng ký mốc live cho CrmPendingMergeWorker.
func RegisterCrmPendingMergeLiveHooks() {
	worker.SetCrmPendingMergeLiveHooks(
		func(item *crmmodels.CrmPendingMerge) {
			if item == nil || strings.TrimSpace(item.TraceID) == "" {
				return
			}
			decisionlive.PublishIntelDomainMilestone(item.OwnerOrganizationID, strings.TrimSpace(item.TraceID), item.CorrelationID, decisionlive.IntelDomainCrmPendingMerge, decisionlive.IntelMilestoneStart,
				"Worker CRM merge queue: bắt đầu gộp L1→L2 (crm_pending_merge).",
				[]string{"Đang áp dữ liệu nguồn vào hồ sơ canonical CRM — sau đó có thể emit yêu cầu intel (debounce)."},
				crmPendingMergeExtra(item))
		},
		func(item *crmmodels.CrmPendingMerge) {
			if item == nil || strings.TrimSpace(item.TraceID) == "" {
				return
			}
			decisionlive.PublishIntelDomainMilestone(item.OwnerOrganizationID, strings.TrimSpace(item.TraceID), item.CorrelationID, decisionlive.IntelDomainCrmPendingMerge, decisionlive.IntelMilestoneDone,
				"Worker CRM merge queue: hoàn tất merge L1→L2.",
				[]string{"Đã cập nhật CRM L2; nếu có điều kiện, hệ thống sẽ lên lịch recompute intel (cùng trace khi đã gắn từ đầu)."},
				crmPendingMergeExtra(item))
		},
		func(item *crmmodels.CrmPendingMerge, err error) {
			if item == nil || err == nil || strings.TrimSpace(item.TraceID) == "" {
				return
			}
			msg := strings.TrimSpace(err.Error())
			if len(msg) > 400 {
				msg = msg[:400] + "…"
			}
			decisionlive.PublishIntelDomainMilestone(item.OwnerOrganizationID, strings.TrimSpace(item.TraceID), item.CorrelationID, decisionlive.IntelDomainCrmPendingMerge, decisionlive.IntelMilestoneError,
				"Worker CRM merge queue: lỗi khi merge L1→L2.",
				[]string{"Chi tiết (rút gọn): " + msg},
				crmPendingMergeExtra(item))
		},
	)
}
