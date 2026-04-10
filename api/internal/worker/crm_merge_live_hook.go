// Package worker — Hook timeline live cho crm_pending_merge (triển khai gắn từ cmd/server qua livehooks, tránh import cycle worker↔decisionlive).
package worker

import (
	crmmodels "meta_commerce/internal/api/crm/models"
)

var (
	crmPendingMergeLiveStart func(*crmmodels.CrmPendingMerge)
	crmPendingMergeLiveDone  func(*crmmodels.CrmPendingMerge)
	crmPendingMergeLiveErr   func(*crmmodels.CrmPendingMerge, error)
)

// SetCrmPendingMergeLiveHooks đăng ký publish timeline (nil = bỏ qua).
func SetCrmPendingMergeLiveHooks(start func(*crmmodels.CrmPendingMerge), done func(*crmmodels.CrmPendingMerge), onErr func(*crmmodels.CrmPendingMerge, error)) {
	crmPendingMergeLiveStart = start
	crmPendingMergeLiveDone = done
	crmPendingMergeLiveErr = onErr
}

func notifyCrmPendingMergeLiveStart(item *crmmodels.CrmPendingMerge) {
	if crmPendingMergeLiveStart != nil && item != nil {
		crmPendingMergeLiveStart(item)
	}
}

func notifyCrmPendingMergeLiveDone(item *crmmodels.CrmPendingMerge) {
	if crmPendingMergeLiveDone != nil && item != nil {
		crmPendingMergeLiveDone(item)
	}
}

func notifyCrmPendingMergeLiveError(item *crmmodels.CrmPendingMerge, err error) {
	if crmPendingMergeLiveErr != nil && item != nil && err != nil {
		crmPendingMergeLiveErr(item, err)
	}
}
