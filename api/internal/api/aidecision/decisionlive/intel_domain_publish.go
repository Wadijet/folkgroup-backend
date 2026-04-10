// Package decisionlive — Publish mốc worker domain (intel / context) lên cùng timeline trace AID.
package decisionlive

import (
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Giá trị intelDomain trên Refs (ổn định cho lọc / tài liệu).
const (
	IntelDomainCIX       = "cix"
	IntelDomainCRMIntel  = "customer_intel" // timeline / lọc domain (đồng bộ tiền tố customer_*)
	IntelDomainOrderIntel = "order_intel"
	IntelDomainAdsIntel  = "ads_intel"
	IntelDomainCrmContext = "customer_context"
	// IntelDomainCrmPendingMerge — worker merge L1→L2 (customer_pending_merge), sau datachanged.
	IntelDomainCrmPendingMerge = "customer_pending_merge"
)

// IntelDomainMilestoneKind — bắt đầu / xong / lỗi (worker domain).
type IntelDomainMilestoneKind int

const (
	IntelMilestoneStart IntelDomainMilestoneKind = iota
	IntelMilestoneDone
	IntelMilestoneError
)

// PublishIntelDomainMilestone đẩy một mốc timeline khi worker nghiệp vụ (CRM, Order, CIX, Ads…) chạy job hoặc xử lý context.
// Bỏ qua nếu thiếu traceId hoặc org — cùng quy ước với Publish.
func PublishIntelDomainMilestone(ownerOrgID primitive.ObjectID, traceID, correlationID, domain string, kind IntelDomainMilestoneKind, summaryVi string, detailBullets []string, extraRefs map[string]string) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" || ownerOrgID.IsZero() {
		return
	}
	var phase string
	var sev string
	var outcome string
	switch kind {
	case IntelMilestoneStart:
		phase = PhaseIntelDomainComputeStart
		sev = SeverityInfo
		outcome = OutcomeNominal
	case IntelMilestoneDone:
		phase = PhaseIntelDomainComputeDone
		sev = SeverityInfo
		outcome = OutcomeSuccess
	case IntelMilestoneError:
		phase = PhaseIntelDomainComputeError
		sev = SeverityError
		outcome = OutcomeProcessingError
	default:
		phase = PhaseIntelDomainComputeStart
		sev = SeverityInfo
		outcome = OutcomeNominal
	}
	refs := map[string]string{
		"intelDomain": strings.TrimSpace(domain),
		"workerLane":  "domain_worker",
	}
	for k, v := range extraRefs {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		refs[k] = v
	}
	bullets := detailBullets
	if len(bullets) == 0 && strings.TrimSpace(summaryVi) != "" {
		bullets = []string{strings.TrimSpace(summaryVi)}
	}
	ev := DecisionLiveEvent{
		Phase:              phase,
		Severity:           sev,
		Summary:            strings.TrimSpace(summaryVi),
		CorrelationID:      strings.TrimSpace(correlationID),
		SourceKind:         FeedSourceIntel,
		FeedSourceCategory: FeedSourceIntel,
		FeedSourceLabelVi:  labelFeedSource(FeedSourceIntel),
		Refs:               refs,
		DetailBullets:      bullets,
		OutcomeKind:        outcome,
	}
	Publish(ownerOrgID, traceID, ev)
}
