// persist_org_audit.go — Dựng document BSON ghi collection decision_org_live_events (trường phẳng + payload JSON DecisionLiveEvent).
package decisionlive

import (
	"encoding/json"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"meta_commerce/internal/api/aidecision/eventtypes"
)

// phaseLabelVi nhãn giai đoạn thân thiện UI (Tiếng Việt).
func phaseLabelVi(phase string) string {
	return eventtypes.ResolveLivePhaseLabelVi(phase)
}

func stepKindFromEvent(ev DecisionLiveEvent) string {
	if ev.Step != nil && strings.TrimSpace(ev.Step.Kind) != "" {
		return strings.TrimSpace(ev.Step.Kind)
	}
	return ""
}

func stepTitleFromEvent(ev DecisionLiveEvent) string {
	if ev.Step != nil && strings.TrimSpace(ev.Step.Title) != "" {
		return strings.TrimSpace(ev.Step.Title)
	}
	return ""
}

// uiTitleForLiveEvent tiêu đề một dòng ưu tiên cho UI.
func uiTitleForLiveEvent(ev DecisionLiveEvent) string {
	if t := stepTitleFromEvent(ev); t != "" {
		return t
	}
	pl := phaseLabelVi(ev.Phase)
	if ev.SourceTitle != "" {
		return pl + " — " + ev.SourceTitle
	}
	return pl
}

// enrichLiveEventUIPresentation — Gắn phaseLabelVi / uiTitle / uiSummary lên payload timeline (REST/WS/Mongo JSON) để mỗi node swimlane có mô tả ngắn trước khi mở detailBullets / processTrace.
func enrichLiveEventUIPresentation(ev *DecisionLiveEvent) {
	if ev == nil {
		return
	}
	if strings.TrimSpace(ev.PhaseLabelVi) == "" {
		ev.PhaseLabelVi = phaseLabelVi(ev.Phase)
	}
	if strings.TrimSpace(ev.UiTitle) == "" {
		ev.UiTitle = uiTitleForLiveEvent(*ev)
	}
	if strings.TrimSpace(ev.UiSummary) == "" {
		ev.UiSummary = strings.TrimSpace(firstNonEmpty(ev.Summary, ev.ReasoningSummary))
	}
}

// mergeAuditRefsForPersist — Gộp ev.Refs với traceId, correlationId, decisionCaseId, w3cTraceId, spanId (lưu một map refs trên document).
func mergeAuditRefsForPersist(ev DecisionLiveEvent) map[string]string {
	out := make(map[string]string)
	for k, v := range ev.Refs {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			continue
		}
		out[k] = v
	}
	if ev.TraceID != "" {
		out["traceId"] = ev.TraceID
	}
	if ev.CorrelationID != "" {
		out["correlationId"] = ev.CorrelationID
	}
	if ev.DecisionCaseID != "" {
		out["decisionCaseId"] = ev.DecisionCaseID
	}
	if ev.W3CTraceID != "" {
		out["w3cTraceId"] = ev.W3CTraceID
	}
	if ev.SpanID != "" {
		out["spanId"] = ev.SpanID
	}
	if ev.ParentSpanID != "" {
		out["parentSpanId"] = ev.ParentSpanID
	}
	if bd := strings.TrimSpace(ev.BusinessDomain); bd != "" {
		out["businessDomain"] = bd
	}
	if lb := strings.TrimSpace(ev.BusinessDomainLabelVi); lb != "" {
		out["businessDomainLabelVi"] = lb
	}
	return out
}

// BuildOrgLivePersistDocument dựng một document InsertOne vào decision_org_live_events (một mốc org-live).
//
// Thứ tự nội dung (bám code):
//
//	Bước A — payload = json.Marshal(DecisionLiveEvent) đầy đủ (nguồn sự thật cho replay; lỗi marshal → "{}").
//	Bước B — Khóa & thời gian: _id mới, ownerOrganizationId, createdAt (server ms), docSchemaVersion = 2.
//	Bước C — Định danh trace/case: traceId, w3cTraceId, spanId, parentSpanId, correlationId, decisionCaseId (từ ev).
//	Bước D — Pipeline & feed: phase, severity, seq, feedSeq, stream, sourceKind, sourceTitle, feedSource*, opsTier*, decisionMode, businessDomain, businessDomainLabelVi (module xử lý mốc §1.2 doc).
//	Bước E — UI suy ra: uiTitle (step title hoặc phase + source), uiSummary (summary hoặc reasoningSummary), phaseLabelVi, stepKind, stepTitle.
//	Bước E2 — Tham chiếu E2E: e2eStage, e2eStepId, e2eStepLabelVi (luồng chuẩn G1–G6).
//	Bước E3 — Kết quả: outcomeKind, outcomeAbnormal, outcomeLabelVi (phân loại bình thường / bất thường).
//	Bước F — refs = mergeAuditRefsForPersist(ev); detailBullets; detailSections; processTrace (BSON, tùy có).
//	Bước G — payload (byte) — client/API đọc lại DecisionLiveEvent qua Unmarshal(payload).
func BuildOrgLivePersistDocument(ownerOrgID primitive.ObjectID, docID primitive.ObjectID, createdAt int64, ev DecisionLiveEvent) bson.M {
	payload := mustMarshalPayload(ev)
	return bson.M{
		"_id":                   docID,
		"ownerOrganizationId":   ownerOrgID,
		"createdAt":             createdAt,
		"docSchemaVersion":      2,
		"traceId":               ev.TraceID,
		"w3cTraceId":            ev.W3CTraceID,
		"spanId":                ev.SpanID,
		"parentSpanId":          ev.ParentSpanID,
		"correlationId":         ev.CorrelationID,
		"decisionCaseId":        ev.DecisionCaseID,
		"phase":                 ev.Phase,
		"phaseLabelVi":          firstNonEmpty(strings.TrimSpace(ev.PhaseLabelVi), phaseLabelVi(ev.Phase)),
		"severity":              ev.Severity,
		"seq":                   ev.Seq,
		"feedSeq":               ev.FeedSeq,
		"stream":                ev.Stream,
		"sourceKind":            ev.SourceKind,
		"sourceTitle":           ev.SourceTitle,
		"feedSourceCategory":    ev.FeedSourceCategory,
		"feedSourceLabelVi":     ev.FeedSourceLabelVi,
		"businessDomain":        strings.TrimSpace(ev.BusinessDomain),
		"businessDomainLabelVi": strings.TrimSpace(ev.BusinessDomainLabelVi),
		"opsTier":               ev.OpsTier,
		"opsTierLabelVi":        ev.OpsTierLabelVi,
		"decisionMode":          ev.DecisionMode,
		"uiTitle":               firstNonEmpty(strings.TrimSpace(ev.UiTitle), uiTitleForLiveEvent(ev)),
		"uiSummary":             firstNonEmpty(strings.TrimSpace(ev.UiSummary), strings.TrimSpace(firstNonEmpty(ev.Summary, ev.ReasoningSummary))),
		"stepKind":              stepKindFromEvent(ev),
		"stepTitle":             stepTitleFromEvent(ev),
		"e2eStage":              strings.TrimSpace(ev.E2EStage),
		"e2eStepId":             strings.TrimSpace(ev.E2EStepID),
		"e2eStepLabelVi":        strings.TrimSpace(ev.E2EStepLabelVi),
		"outcomeKind":           strings.TrimSpace(ev.OutcomeKind),
		"outcomeAbnormal":       ev.OutcomeAbnormal,
		"outcomeLabelVi":        strings.TrimSpace(ev.OutcomeLabelVi),
		"refs":                  mergeAuditRefsForPersist(ev),
		"detailBullets":         ev.DetailBullets,
		"detailSections":        detailSectionsToBSON(ev.DetailSections),
		"processTrace":          processTraceToBSON(ev.ProcessTrace),
		"payload":               payload,
	}
}

func processTraceToBSON(nodes []DecisionLiveProcessNode) interface{} {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]bson.M, 0, len(nodes))
	for _, n := range nodes {
		m := bson.M{
			"kind":    n.Kind,
			"labelVi": n.LabelVi,
		}
		if strings.TrimSpace(n.Key) != "" {
			m["key"] = strings.TrimSpace(n.Key)
		}
		if strings.TrimSpace(n.DetailVi) != "" {
			m["detailVi"] = strings.TrimSpace(n.DetailVi)
		}
		if child := processTraceToBSON(n.Children); child != nil {
			m["children"] = child
		}
		out = append(out, m)
	}
	return out
}

func detailSectionsToBSON(sections []DecisionLiveDetailSection) []bson.M {
	if len(sections) == 0 {
		return nil
	}
	out := make([]bson.M, 0, len(sections))
	for _, s := range sections {
		title := strings.TrimSpace(s.Title)
		if title == "" && len(s.Items) == 0 {
			continue
		}
		out = append(out, bson.M{
			"title": title,
			"items": s.Items,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

// mustMarshalPayload — Bước A của BuildOrgLivePersistDocument: toàn bộ DecisionLiveEvent → JSON bytes.
func mustMarshalPayload(ev DecisionLiveEvent) []byte {
	b, err := json.Marshal(ev)
	if err != nil {
		return []byte("{}")
	}
	return b
}
