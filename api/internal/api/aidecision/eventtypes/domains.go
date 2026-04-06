// Package eventtypes — bổ sung hằng domain (miền nghiệp vụ) tách ads_meta / ads_google và cross-domain.
package eventtypes

// Định danh miền quảng cáo & tương thích Mongo (rule, approval).
//
// Quy ước:
//   - DomainAdsMeta: canonical trong contract mới, activity history (Meta), cross-domain EntityRef.
//   - DomainAdsGoogle: dành cho Google Ads (chưa triển khai).
//   - DomainAdsRuleIntel: giá trị domain trong rule_definitions / rule_param_sets cho RULE_ADS_* (Meta).
//     Giữ "ads" để không phá dữ liệu đã seed; migration sau có thể đổi Mongo → ads_meta nếu cần.
//   - DomainAdsApproval: approval_mode_config, action_pending_approval, executor Propose (Meta).
//     Giữ "ads" để khớp document hiện có trong DB.
const (
	DomainAdsMeta       = "ads_meta"
	DomainAdsGoogle     = "ads_google"
	DomainAdsRuleIntel  = "ads"
	DomainAdsApproval   = "ads"
	DomainCrossIntel    = "cross_intel"
	DomainAdsIntelQueue = "ads_intel"
)
