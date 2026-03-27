package events

import "context"

// ctxKeyAdsIntelligenceRollup — đánh dấu CRUD phát sinh từ roll-up Ads Intelligence (chỉ đổi currentMetrics).
type ctxKeyAdsIntelligenceRollup struct{}

// WithAdsIntelligenceRollupContext gắn cờ vào context: payload queue sẽ có adsIntelligenceRollupOnly
// để ProcessMetaCampaignDataChanged không kích hoạt lại ads.context_requested (tránh vòng lặp).
func WithAdsIntelligenceRollupContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeyAdsIntelligenceRollup{}, true)
}

// IsAdsIntelligenceRollupContext trả về true nếu context được gắn bởi WithAdsIntelligenceRollupContext.
func IsAdsIntelligenceRollupContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	v, ok := ctx.Value(ctxKeyAdsIntelligenceRollup{}).(bool)
	return ok && v
}
