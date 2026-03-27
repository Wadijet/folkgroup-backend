package livecopy

import (
	"fmt"
	"strings"

	"meta_commerce/internal/api/aidecision/decisionlive"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
)

const maxQueueErrRunes = 400

// QueueMilestone — mốc vòng đời consumer cho một job queue (mỗi lần publish queue = một DecisionLiveEvent trên timeline).
type QueueMilestone int

const (
	QueueMilestoneProcessingStart QueueMilestone = iota // lease → sắp dispatch
	QueueMilestoneDatachangedDone                       // applyDatachangedSideEffects xong
	QueueMilestoneHandlerDone                           // processEvent thành công
	QueueMilestoneHandlerError                          // processEvent lỗi
	QueueMilestoneRoutingSkipped                        // routing noop
	QueueMilestoneNoHandler                             // chưa đăng ký handler
)

// BuildQueueConsumerEvent dựng DecisionLiveEvent: tóm tắt cho vận hành (tiêu đề + gạch đầu dòng ngắn).
func BuildQueueConsumerEvent(evt *aidecisionmodels.DecisionEvent, ms QueueMilestone, processErr error, extraBullets []string) decisionlive.DecisionLiveEvent {
	dn := DomainNarrativeFromQueueEvent(evt)
	bullets := queueStructuredBullets(evt, ms, dn, processErr)
	bullets = append(bullets, extraBullets...)
	if eid := strings.TrimSpace(evt.EventID); eid != "" {
		bullets = append([]string{
			"Mã tác vụ: " + eid + " — cùng một luồng theo dõi có thể có nhiều lần xử lý (nhiều đợt cập nhật).",
		}, bullets...)
	}
	sections := queueDetailSections(evt, ms, processErr)
	phase, severity, summary, rs := queueSummaryForMilestone(ms, dn, evt, processErr)
	ev := decisionlive.DecisionLiveEvent{
		Phase:            phase,
		Severity:         severity,
		Summary:          summary,
		ReasoningSummary: rs,
		SourceKind:       decisionlive.SourceQueue,
		SourceTitle:      evt.EventType,
		CorrelationID:    evt.CorrelationID,
		DecisionCaseID:   decisionCaseIDFromQueuePayload(evt),
		Refs:             decisionlive.RefsFromDecisionEventEnvelope(evt),
		DetailBullets:    bullets,
		DetailSections:   sections,
		Step: &decisionlive.TraceStep{
			Kind:      "queue",
			Title:     dn.StepTitle,
			Reasoning: dn.BusinessOneLine,
		},
	}
	return ev
}

func decisionCaseIDFromQueuePayload(evt *aidecisionmodels.DecisionEvent) string {
	if evt == nil || evt.Payload == nil {
		return ""
	}
	for _, key := range []string{"decisionCaseId", "decisionCaseID"} {
		if v, ok := evt.Payload[key].(string); ok {
			if s := strings.TrimSpace(v); s != "" {
				return s
			}
		}
	}
	return ""
}

func queueSummaryForMilestone(ms QueueMilestone, dn DomainNarrative, evt *aidecisionmodels.DecisionEvent, processErr error) (phase, severity, summary, reasoningSummary string) {
	reasoningSummary = dn.BusinessOneLine
	switch ms {
	case QueueMilestoneProcessingStart:
		phase = decisionlive.PhaseQueueProcessing
		severity = decisionlive.SeverityInfo
		summary = fmt.Sprintf("Đang xử lý: %s — %s", dn.StepTitle, truncateOneLine(dn.BusinessOneLine, 160))
	case QueueMilestoneDatachangedDone:
		phase = decisionlive.PhaseDatachangedEffects
		severity = decisionlive.SeverityInfo
		summary = fmt.Sprintf("Đã đồng bộ phần phụ sau khi dữ liệu đổi — %s — tiếp theo là bước chính.", dn.StepTitle)
	case QueueMilestoneHandlerDone:
		phase = decisionlive.PhaseQueueDone
		severity = decisionlive.SeverityInfo
		summary = fmt.Sprintf("Đã xử lý xong: %s", dn.StepTitle)
	case QueueMilestoneHandlerError:
		phase = decisionlive.PhaseQueueError
		severity = decisionlive.SeverityError
		errStr := ""
		if processErr != nil {
			errStr = truncateRunes(processErr.Error(), maxQueueErrRunes)
		}
		summary = fmt.Sprintf("Lỗi: %s — %s (%s)", errStr, dn.StepTitle, truncateOneLine(dn.BusinessOneLine, 120))
	case QueueMilestoneRoutingSkipped:
		phase = decisionlive.PhaseSkipped
		severity = decisionlive.SeverityInfo
		summary = fmt.Sprintf("Bỏ qua theo cấu hình — %s — loại việc này không cần xử lý sâu.", dn.StepTitle)
	case QueueMilestoneNoHandler:
		phase = decisionlive.PhaseSkipped
		severity = decisionlive.SeverityWarn
		summary = fmt.Sprintf("Chưa hỗ trợ loại «%s» — %s. Liên hệ kỹ thuật nếu cần bật.", evt.EventType, dn.StepTitle)
	default:
		phase = decisionlive.PhaseQueueProcessing
		severity = decisionlive.SeverityInfo
		summary = dn.StepTitle
	}
	return phase, severity, summary, reasoningSummary
}

func queueStructuredBullets(evt *aidecisionmodels.DecisionEvent, ms QueueMilestone, dn DomainNarrative, processErr error) []string {
	srcVi := "khác"
	switch evt.EventSource {
	case "datachanged":
		srcVi = "sau khi dữ liệu vừa cập nhật"
	case "aidecision":
		srcVi = "từ luồng AI Decision"
	}
	in := fmt.Sprintf("Loại việc: %s — nguồn: %s", evt.EventType, srcVi)
	if evt.EntityType != "" || evt.EntityID != "" {
		in += fmt.Sprintf(" — đối tượng: %s %s", evt.EntityType, evt.EntityID)
	}
	mech := "Trình tự: hệ thống nhận việc vào hàng đợi"
	if evt.EventSource == "datachanged" && (ms == QueueMilestoneProcessingStart || ms == QueueMilestoneDatachangedDone) {
		mech += " → đồng bộ các phần phụ (nếu có) → thực hiện đúng loại việc."
	} else {
		mech += " → đọc nội dung → thực hiện đúng loại việc."
	}
	var out string
	switch ms {
	case QueueMilestoneProcessingStart:
		out = "Giai đoạn này: bắt đầu xử lý."
	case QueueMilestoneDatachangedDone:
		out = "Giai đoạn này: đã xong bước đồng bộ phụ; sắp tới là bước chính."
	case QueueMilestoneHandlerDone:
		out = "Giai đoạn này: đã xử lý xong tác vụ này."
	case QueueMilestoneHandlerError:
		out = "Giai đoạn này: có lỗi — hệ thống có thể thử lại."
		if processErr != nil {
			out += " " + truncateRunes(processErr.Error(), 200)
		}
	case QueueMilestoneRoutingSkipped:
		out = "Giai đoạn này: dừng sớm theo cấu hình, không làm thêm."
	case QueueMilestoneNoHandler:
		out = "Giai đoạn này: chưa có bước xử lý tương ứng."
	default:
		out = "Xem tóm tắt phía trên."
	}
	base := []string{in, mech, out}
	if len(dn.EntityBullets) > 0 {
		base = append(base, dn.EntityBullets...)
	}
	return base
}

// queueDetailSections — diễn giải nội dung trong cùng một mốc queue (không tạo thêm mốc timeline).
func queueDetailSections(evt *aidecisionmodels.DecisionEvent, ms QueueMilestone, processErr error) []decisionlive.DecisionLiveDetailSection {
	var steps []string
	switch ms {
	case QueueMilestoneProcessingStart:
		steps = []string{
			"Nhận job từ hàng đợi và gán phiên xử lý.",
			"Xác định loại việc và đối tượng theo envelope.",
		}
		if evt != nil && evt.EventSource == "datachanged" {
			steps = append(steps, "Chạy phần phụ sau khi dữ liệu đổi (nếu có), trước bước chính.")
		} else {
			steps = append(steps, "Đọc payload và chuẩn bị handler tương ứng loại việc.")
		}
	case QueueMilestoneDatachangedDone:
		steps = []string{
			"Đã xong bước đồng bộ phụ sau khi dữ liệu đổi.",
			"Chuyển sang xử lý chính theo đúng loại việc.",
		}
	case QueueMilestoneHandlerDone:
		steps = []string{
			"Handler đã chạy xử lý nghiệp vụ theo loại việc.",
			"Hoàn tất thành công — kết thúc lượt queue này.",
		}
	case QueueMilestoneHandlerError:
		steps = []string{
			"Có lỗi trong lúc xử lý (chi tiết ở tóm tắt phía trên).",
			"Có thể thử lại hoặc cần tra log / hỗ trợ kỹ thuật.",
		}
		if processErr != nil {
			steps = append(steps, "Gợi ý: kiểm tra thông báo lỗi ngắn gọn đi kèm mốc này.")
		}
	case QueueMilestoneRoutingSkipped:
		steps = []string{
			"Routing xác định không cần xử lý sâu cho loại việc này.",
			"Dừng tại đây — không chạy handler nghiệp vụ đầy đủ.",
		}
	case QueueMilestoneNoHandler:
		steps = []string{
			"Loại việc chưa có handler trên môi trường này.",
			"Cần bật cấu hình hoặc cập nhật phiên bản nếu đây là tính năng mong đợi.",
		}
	default:
		steps = []string{
			"Theo dõi tóm tắt phía trên — mỗi mốc trên timeline là một live_event riêng.",
		}
	}
	return []decisionlive.DecisionLiveDetailSection{
		{Title: "Diễn giải trong mốc này", Items: steps},
	}
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func truncateOneLine(s string, max int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	return truncateRunes(s, max)
}
