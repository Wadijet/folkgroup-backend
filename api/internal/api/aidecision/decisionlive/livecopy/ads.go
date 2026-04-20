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
	frame := PublishCatalogUserViForLivePhase(strings.TrimSpace(phase))
	if frame == "" {
		frame = PublishCatalogUserViForLivePhase(decisionlive.PhaseAdsEvaluate)
	}
	adsSections := []decisionlive.DecisionLiveDetailSection{
		{Title: "Tham chiếu", Items: []string{
			"Neo catalog: " + frame,
		}},
	}
	outcomeK := decisionlive.OutcomeNominal
	switch strings.TrimSpace(severity) {
	case decisionlive.SeverityError:
		outcomeK = decisionlive.OutcomeProcessingError
	case decisionlive.SeverityWarn:
		outcomeK = decisionlive.OutcomeNoActions
	}
	sumOut := PublishWithSituation(frame, strings.TrimSpace(summary))
	if strings.TrimSpace(summary) == "" {
		sumOut = frame
	}
	ev := decisionlive.DecisionLiveEvent{
		Phase:          phase,
		Severity:       severity,
		OutcomeKind:    outcomeK,
		Summary:        sumOut,
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
	if strings.TrimSpace(ev.ReasoningSummary) == "" {
		ev.ReasoningSummary = frame
	}
	decisionlive.MergeRefsFromDecisionEnvelope(&ev, queueEvt)
	if queueEvt != nil && strings.TrimSpace(ev.CorrelationID) == "" {
		ev.CorrelationID = strings.TrimSpace(queueEvt.CorrelationID)
	}
	mergeExtraRefsInto(&ev, extraRefs)
	title := strings.TrimSpace(stepTitle)
	if title == "" {
		title = frame
	} else {
		title = PublishWithSituation(frame, title)
	}
	ev.Step = &decisionlive.TraceStep{
		Kind:      "ads_rule",
		Title:     title,
		Reasoning: ev.ReasoningSummary,
	}
	return ev
}

func adsMergeStructuredBullets(campaignID, adAccountID string, queueEvt *aidecisionmodels.DecisionEvent, detail []string) []string {
	var core []string
	if cid := firstNonEmpty(campaignID, refVal(queueEvt, "campaignId")); cid != "" {
		line := "Chiến dịch: " + cid
		if aa := firstNonEmpty(adAccountID, refVal(queueEvt, "adAccountId")); aa != "" {
			line += " · Tài khoản QC: " + aa
		}
		core = append(core, line)
	}
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
