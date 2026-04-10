package livecopy

import (
	"strings"

	"meta_commerce/internal/api/aidecision/decisionlive"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
)

// BuildAdsOptimizationLiveEvent — Dựng sự kiện timeline cho luồng tối ưu Ads: từ «ngữ cảnh sẵn sàng» → quy tắc → đề xuất.
// campaignId / adAccountId: hiển thị nhanh; có thể rỗng nếu đã có trong refs của envelope job.
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
		{Title: "Thông tin thêm", Items: []string{
			"Đối chiếu chiến dịch và tài khoản quảng cáo trong phần tham chiếu với dữ liệu đã lưu.",
			"Nếu cần hỗ trợ, gửi kèm mã luồng (trace) trong sự kiện.",
		}},
	}
	outcomeK := decisionlive.OutcomeNominal
	switch strings.TrimSpace(severity) {
	case decisionlive.SeverityError:
		outcomeK = decisionlive.OutcomeProcessingError
	case decisionlive.SeverityWarn:
		outcomeK = decisionlive.OutcomeNoActions
	}
	ev := decisionlive.DecisionLiveEvent{
		Phase:          phase,
		Severity:       severity,
		OutcomeKind:    outcomeK,
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
	rs := "Hệ thống đang so sánh số liệu chiến dịch với quy tắc đã cài; nếu phù hợp sẽ có gợi ý để bạn duyệt."
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
	line := "Đang đánh giá chiến dịch quảng cáo đã lưu theo quy tắc của bạn."
	if cid := firstNonEmpty(campaignID, refVal(queueEvt, "campaignId")); cid != "" {
		line = "Chiến dịch " + cid
		if aa := firstNonEmpty(adAccountID, refVal(queueEvt, "adAccountId")); aa != "" {
			line += " · tài khoản " + aa
		}
		line += " — kết quả tóm tắt ở dòng phía trên."
	}
	core := []string{line}
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
