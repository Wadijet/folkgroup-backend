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
		{Title: "Luồng tối ưu quảng cáo (mỗi bước có thể là một mốc timeline)", Items: []string{
			"Trước bước này thường đã có các mốc trên hàng đợi (chuẩn bị ngữ cảnh, v.v.) — mỗi mốc một dòng timeline.",
			"Ở bước đánh giá: hệ thống đối chiếu số liệu chiến dịch với quy tắc hành động.",
			"Sau đó có thể là mốc đề xuất hoặc job khác — theo dõi tiếp trên cùng một trace.",
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
	rs := "Đối chiếu số liệu chiến dịch với quy tắc; nếu đủ điều kiện sẽ tạo gợi ý chờ duyệt."
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
	in := "Dựa trên số liệu chiến dịch đã lưu trên hệ thống và nội dung job hiện tại."
	if cid := firstNonEmpty(campaignID, refVal(queueEvt, "campaignId")); cid != "" {
		in = "Chiến dịch: " + cid
		if aa := firstNonEmpty(adAccountID, refVal(queueEvt, "adAccountId")); aa != "" {
			in += " — Tài khoản quảng cáo: " + aa
		}
		in += "."
	}
	mech := "Đối chiếu số liệu với quy tắc; nếu có hành động phù hợp sẽ chuyển sang bước duyệt / thực thi."
	out := "Kết luận từng bước nằm ở dòng tóm tắt phía trên."
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
