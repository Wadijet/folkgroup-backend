// Package decisionlive — Trích xuất trường phẳng phục vụ persist Mongo (UI/audit/query).
package decisionlive

import (
	"encoding/json"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// phaseLabelVi nhãn giai đoạn thân thiện UI (Tiếng Việt).
func phaseLabelVi(phase string) string {
	switch strings.TrimSpace(phase) {
	case PhaseQueued:
		return "Đã xếp hàng"
	case PhaseConsuming:
		return "Đang xử lý quyết định"
	case PhaseSkipped:
		return "Bỏ qua bước"
	case PhaseParse:
		return "Đọc gợi ý từ tình huống"
	case PhaseLLM:
		return "Phân tích bổ sung (AI)"
	case PhaseDecision:
		return "Tổng hợp quyết định"
	case PhasePolicy:
		return "Áp dụng quy tắc duyệt"
	case PhasePropose:
		return "Tạo đề xuất / thực thi"
	case PhaseEmpty:
		return "Không có hành động"
	case PhaseDone:
		return "Hoàn tất"
	case PhaseError:
		return "Có lỗi"
	case PhaseQueueProcessing:
		return "Queue: bắt đầu"
	case PhaseQueueDone:
		return "Queue: xong"
	case PhaseQueueError:
		return "Queue: lỗi"
	case PhaseDatachangedEffects:
		return "Side-effect đồng bộ"
	case PhaseOrchestrate:
		return "Điều phối case & tác vụ"
	case PhaseCixIntegrated:
		return "Đã tích hợp phân tích CIX"
	case PhaseExecuteReady:
		return "Sẵn sàng thực thi quyết định"
	default:
		if phase == "" {
			return "Bước luồng"
		}
		return phase
	}
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

// mergeAuditRefsForPersist gộp refs từ event + trace/correlation để query/audit.
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
	return out
}

// BuildOrgLivePersistDocument tài liệu ghi Mongo: một bước một dòng, payload JSON đầy đủ + trường phẳng cho UI/audit.
func BuildOrgLivePersistDocument(ownerOrgID primitive.ObjectID, docID primitive.ObjectID, createdAt int64, ev DecisionLiveEvent) bson.M {
	payload := mustMarshalPayload(ev)
	return bson.M{
		"_id":                 docID,
		"ownerOrganizationId":   ownerOrgID,
		"createdAt":           createdAt,
		"docSchemaVersion":      2,
		"traceId":             ev.TraceID,
		"w3cTraceId":          ev.W3CTraceID,
		"spanId":              ev.SpanID,
		"parentSpanId":        ev.ParentSpanID,
		"correlationId":       ev.CorrelationID,
		"decisionCaseId":      ev.DecisionCaseID,
		"phase":               ev.Phase,
		"severity":            ev.Severity,
		"seq":                 ev.Seq,
		"feedSeq":             ev.FeedSeq,
		"stream":              ev.Stream,
		"sourceKind":          ev.SourceKind,
		"sourceTitle":         ev.SourceTitle,
		"feedSourceCategory":  ev.FeedSourceCategory,
		"feedSourceLabelVi":   ev.FeedSourceLabelVi,
		"opsTier":             ev.OpsTier,
		"opsTierLabelVi":      ev.OpsTierLabelVi,
		"decisionMode":        ev.DecisionMode,
		"uiTitle":             uiTitleForLiveEvent(ev),
		"uiSummary":           firstNonEmpty(ev.Summary, ev.ReasoningSummary),
		"phaseLabelVi":        phaseLabelVi(ev.Phase),
		"stepKind":            stepKindFromEvent(ev),
		"stepTitle":           stepTitleFromEvent(ev),
		"refs":                mergeAuditRefsForPersist(ev),
		"detailBullets":       ev.DetailBullets,
		"detailSections":      detailSectionsToBSON(ev.DetailSections),
		"payload":             payload,
	}
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

func mustMarshalPayload(ev DecisionLiveEvent) []byte {
	b, err := json.Marshal(ev)
	if err != nil {
		return []byte("{}")
	}
	return b
}
