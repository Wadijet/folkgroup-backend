// Package datachanged — Miền Meta Ads: phản ứng thay đổi dữ liệu cho hồ sơ / debounce quảng cáo.
package datachanged

import (
	"context"

	"meta_commerce/internal/api/events"
	metahooks "meta_commerce/internal/api/meta/hooks"
)

// ProcessAdsProfileFromDataChange kích hoạt xử lý profile ads (debounce nội bộ meta hooks).
func ProcessAdsProfileFromDataChange(ctx context.Context, e events.DataChangeEvent) {
	metahooks.ProcessDataChangeForAdsProfile(ctx, e)
}
