// Package ads — Executor cho domain ads. Đăng ký với approval package.
package ads

import (
	"context"

	adssvc "meta_commerce/internal/api/ads/service"
	"meta_commerce/internal/approval"
	pkgapproval "meta_commerce/pkg/approval"
)

func init() {
	approval.RegisterExecutor(DomainAds, pkgapproval.ExecutorFunc(executeAdsAction))
	approval.RegisterEventTypes(DomainAds, map[string]string{
		"executed": "ads_action_executed",
		"rejected": "ads_action_rejected",
	})
}

const DomainAds = "ads"

func executeAdsAction(ctx context.Context, doc *pkgapproval.ActionPending) (map[string]interface{}, error) {
	return adssvc.ExecuteAdsAction(ctx, doc)
}
