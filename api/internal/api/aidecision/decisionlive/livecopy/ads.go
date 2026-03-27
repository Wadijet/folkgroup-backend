package livecopy

import (
	"strings"

	"meta_commerce/internal/api/aidecision/decisionlive"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
)

// BuildAdsOptimizationLiveEvent dựng DecisionLiveEvent cho pipeline Ads (ads.context_ready → ACTION_RULE → propose).
// campaignId/adAccountId: tham chiếu hiển thị; có thể rỗng nếu đọc từ refs sau merge.
func BuildAdsOptimizationLiveEvent(
	caseDoc *aidecisionmodels.DecisionCase,
	queueEvt *aidecisionmodels.DecisionEvent,
	phase, severity, summary string,
	detailBullets []string,
	stepTitle string,
	extraRefs map[string]string,
	campaignID, adAccountID string,
) decisionlive.DecisionLiveEvent {
	adsSections := []decisionlive.DecisionLiveDetailSection{
		{Title: "Diễn giải nghiệp vụ (mốc Ads này — luồng đầy đủ là nhiều live_event)", Items: []string{
			"Ngữ cảnh chiến dịch thường qua các mốc queue trước (mỗi mốc một live_event).",
			"Tại mốc đánh giá: đối chiếu số liệu với quy tắc (ACTION_RULE / metrics).",
			"Gợi ý / duyệt: có thể là mốc propose hoặc queue khác — xem timeline.",
		}},
	}
	ev := decisionlive.DecisionLiveEvent{
		Phase:          phase,
		Severity:       severity,
		Summary:        summary,
		SourceKind:     decisionlive.FeedSourceAds,
		SourceTitle:    "Chiến dịch Meta Ads",
		DetailBullets:  adsMergeStructuredBullets(campaignID, adAccountID, queueEvt, detailBullets),
		DetailSections: adsSections,
	}
	if caseDoc != nil {
		ev.DecisionCaseID = caseDoc.DecisionCaseID
		ev.CorrelationID = caseDoc.CorrelationID
		decisionlive.EnrichLiveEventFromCase(caseDoc.DecisionCaseID, caseDoc.TraceID, &ev)
	}
	rs := "Đánh giá chiến dịch theo số liệu và quy tắc; có thể tạo gợi ý chờ duyệt."
	if strings.TrimSpace(ev.ReasoningSummary) == "" {
		ev.ReasoningSummary = rs
	}
	decisionlive.MergeRefsFromDecisionEnvelope(&ev, queueEvt)
	if queueEvt != nil && strings.TrimSpace(ev.CorrelationID) == "" {
		ev.CorrelationID = strings.TrimSpace(queueEvt.CorrelationID)
	}
	mergeExtraRefsInto(&ev, extraRefs)
	title := strings.TrimSpace(stepTitle)
	if title == "" {
		title = "Tối ưu quảng cáo"
	}
	ev.Step = &decisionlive.TraceStep{
		Kind:      "ads_rule",
		Title:     title,
		Reasoning: ev.ReasoningSummary,
	}
	return ev
}

func adsMergeStructuredBullets(campaignID, adAccountID string, queueEvt *aidecisionmodels.DecisionEvent, detail []string) []string {
	in := "Dựa trên số liệu chiến dịch đã lưu và nội dung tác vụ."
	if cid := firstNonEmpty(campaignID, refVal(queueEvt, "campaignId")); cid != "" {
		in = "Chiến dịch: " + cid
		if aa := firstNonEmpty(adAccountID, refVal(queueEvt, "adAccountId")); aa != "" {
			in += " — Tài khoản quảng cáo: " + aa
		}
		in += "."
	}
	mech := "Hệ thống đối chiếu số liệu với quy tắc; nếu có gợi ý sẽ đưa vào bước duyệt."
	out := "Xem tóm tắt phía trên cho kết quả từng bước."
	core := []string{in, mech, out}
	if len(detail) == 0 {
		return core
	}
	return append(core, detail...)
}

func refVal(queueEvt *aidecisionmodels.DecisionEvent, key string) string {
	if queueEvt == nil {
		return ""
	}
	m := decisionlive.RefsFromDecisionEventEnvelope(queueEvt)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[key])
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return strings.TrimSpace(b)
}

func mergeExtraRefsInto(ev *decisionlive.DecisionLiveEvent, extra map[string]string) {
	if ev == nil || len(extra) == 0 {
		return
	}
	if ev.Refs == nil {
		ev.Refs = make(map[string]string)
	}
	for k, v := range extra {
		v = strings.TrimSpace(v)
		if k == "" || v == "" {
			continue
		}
		if _, ok := ev.Refs[k]; !ok || ev.Refs[k] == "" {
			ev.Refs[k] = v
		}
	}
}
