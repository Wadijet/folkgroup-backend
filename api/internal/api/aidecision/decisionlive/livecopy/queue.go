package livecopy

import (
	"fmt"
	"strings"

	"meta_commerce/internal/api/aidecision/decisionlive"
	"meta_commerce/internal/api/aidecision/eventtypes"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
)

const maxQueueErrRunes = 400

// QueueMilestone — Mốc trong vòng đời xử lý một job trên hàng đợi; mỗi mốc tương ứng một DecisionLiveEvent trên timeline live.
type QueueMilestone int

const (
	QueueMilestoneProcessingStart QueueMilestone = iota // Worker đã nhận job, sắp xử lý
	QueueMilestoneDatachangedDone                       // Đã xong bước chuẩn bị sau datachanged (nếu có)
	QueueMilestoneHandlerDone                           // Handler nghiệp vụ chạy xong, không lỗi
	QueueMilestoneHandlerError                          // Có lỗi khi chạy handler
	QueueMilestoneRoutingSkipped                        // Bỏ qua theo quy tắc routing (noop)
	QueueMilestoneNoHandler                             // Chưa có handler cho loại sự kiện
)

// BuildQueueConsumerEvent — Dựng DecisionLiveEvent cho timeline: tóm tắt và gạch đầu dòng ưu tiên người xem không chuyên kỹ thuật.
func BuildQueueConsumerEvent(evt *aidecisionmodels.DecisionEvent, ms QueueMilestone, processErr error, extraBullets []string) decisionlive.DecisionLiveEvent {
	dn := DomainNarrativeFromQueueEvent(evt)
	bullets := queueStructuredBullets(evt, ms, dn, processErr)
	bullets = append(bullets, extraBullets...)
	// Chỉ hiện mã theo dõi ở mốc đầu để tránh lặp lại giữa các bước trên cùng một yêu cầu.
	if eid := strings.TrimSpace(evt.EventID); eid != "" && ms == QueueMilestoneProcessingStart {
		bullets = append([]string{
			"Mã theo dõi yêu cầu: " + eid + ". (Cùng một luồng có thể có thêm yêu cầu khác nếu dữ liệu cập nhật nhiều lần.)",
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
		SourceTitle:      queueFriendlyEventLabel(evt),
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

// isDatachangedCustomerMirrorOnly — datachanged khách POS/FB/CRM: không có handler «bước chính» trên consumer;
// merge CRM / báo cáo / ads chỉ chạy trong applyDatachangedSideEffects (mốc DATACHANGED_EFFECTS).
func isDatachangedCustomerMirrorOnly(evt *aidecisionmodels.DecisionEvent) bool {
	if evt == nil || evt.EventSource != "datachanged" {
		return false
	}
	et := strings.TrimSpace(evt.EventType)
	for _, p := range []string{"pos_customer.", "fb_customer.", "crm_customer."} {
		if strings.HasPrefix(et, p) {
			return true
		}
	}
	return false
}

func queueSummaryForMilestone(ms QueueMilestone, dn DomainNarrative, evt *aidecisionmodels.DecisionEvent, processErr error) (phase, severity, summary, reasoningSummary string) {
	reasoningSummary = dn.BusinessOneLine
	switch ms {
	case QueueMilestoneProcessingStart:
		phase = decisionlive.PhaseQueueProcessing
		severity = decisionlive.SeverityInfo
		// Dòng tóm tắt ngắn; bối cảnh nằm ở ReasoningSummary / chi tiết — tránh nhồi hai câu dài giống nhau giữa các mốc.
		summary = fmt.Sprintf("Bắt đầu xử lý: %s", dn.StepTitle)
	case QueueMilestoneDatachangedDone:
		phase = decisionlive.PhaseDatachangedEffects
		severity = decisionlive.SeverityInfo
		if isDatachangedCustomerMirrorOnly(evt) {
			summary = fmt.Sprintf("Đã xong đồng bộ phụ (CRM, báo cáo, xếp hàng phân tích… nếu áp dụng). Loại «%s» không có thêm bước xử lý chính trên hàng đợi.", queueFriendlyEventLabel(evt))
		} else {
			summary = fmt.Sprintf("Đã xong phần chuẩn bị (đồng bộ phụ). Tiếp theo: %s", dn.StepTitle)
		}
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
		summary = fmt.Sprintf("Có lỗi khi xử lý «%s»: %s", dn.StepTitle, errStr)
	case QueueMilestoneRoutingSkipped:
		phase = decisionlive.PhaseSkipped
		severity = decisionlive.SeverityInfo
		summary = fmt.Sprintf("Không cần xử lý thêm cho «%s» — đúng cấu hình hệ thống.", dn.StepTitle)
	case QueueMilestoneNoHandler:
		phase = decisionlive.PhaseSkipped
		if isDatachangedCustomerMirrorOnly(evt) {
			severity = decisionlive.SeverityInfo
			summary = fmt.Sprintf("Kết thúc luồng «%s»: không có pipeline/case AI riêng — phần merge CRM và side-effect đã nằm ở bước đồng bộ phụ phía trên.", queueFriendlyEventLabel(evt))
		} else {
			severity = decisionlive.SeverityWarn
			summary = fmt.Sprintf("Chưa hỗ trợ kiểu việc này («%s»). Liên hệ kỹ thuật nếu bạn kỳ vọng có xử lý.", queueFriendlyEventLabel(evt))
		}
	default:
		phase = decisionlive.PhaseQueueProcessing
		severity = decisionlive.SeverityInfo
		summary = dn.StepTitle
	}
	return phase, severity, summary, reasoningSummary
}

// queueFriendlyEventLabel — nhãn hiển thị ngắn (tiếng Việt); mã kỹ thuật vẫn nằm trong refs khi cần tra cứu.
func queueFriendlyEventLabel(evt *aidecisionmodels.DecisionEvent) string {
	if evt == nil {
		return ""
	}
	et := strings.TrimSpace(evt.EventType)
	switch et {
	case eventtypes.OrderInserted, eventtypes.OrderUpdated:
		return "Đơn hàng"
	case eventtypes.OrderIntelRecomputed:
		return "Phân tích đơn"
	case eventtypes.ConversationInserted, eventtypes.ConversationUpdated, eventtypes.MessageInserted, eventtypes.MessageUpdated:
		return "Hội thoại / tin nhắn"
	case eventtypes.ConversationMessageInserted, eventtypes.MessageBatchReady:
		return "Tin nhắn (gom lô)"
	case eventtypes.CustomerContextReady:
		return "Thông tin khách"
	case eventtypes.CrmIntelRecomputed:
		return "Phân tích khách"
	case eventtypes.CixIntelRecomputed:
		return "Phân tích hội thoại"
	case eventtypes.CampaignIntelRecomputed, eventtypes.MetaCampaignInserted, eventtypes.MetaCampaignUpdated:
		return "Quảng cáo / chiến dịch"
	case eventtypes.AdsContextRequested, eventtypes.AdsContextReady:
		return "Ngữ cảnh quảng cáo"
	case eventtypes.CrmIntelligenceComputeRequested, eventtypes.CrmIntelligenceRecomputeRequested:
		return "Cập nhật chỉ số khách"
	case eventtypes.PosCustomerInserted, eventtypes.PosCustomerUpdated:
		return "Khách hàng POS"
	case eventtypes.FbCustomerInserted, eventtypes.FbCustomerUpdated:
		return "Khách hàng Facebook"
	case eventtypes.CrmCustomerInserted, eventtypes.CrmCustomerUpdated:
		return "Khách CRM (đã merge)"
	default:
		if et != "" {
			return et
		}
		return "AI Decision"
	}
}

func queueStructuredBullets(evt *aidecisionmodels.DecisionEvent, ms QueueMilestone, dn DomainNarrative, processErr error) []string {
	lineViệc := fmt.Sprintf("Việc đang làm: %s.", dn.StepTitle)
	var lineNguồn string
	switch evt.EventSource {
	case "datachanged":
		lineNguồn = "Có thay đổi dữ liệu trên hệ thống vừa được ghi lại — các bước sau sẽ dựa trên bản mới nhất."
	case "aidecision":
		lineNguồn = "Đây là bước tiếp theo trong luồng xử lý tự động (sau khi một bước trước hoàn tất)."
	case "debounce":
		lineNguồn = "Sau khi gom các cập nhật tin nhắn trong một khoảng thời gian ngắn."
	case "orderintel", "cix_intel", "crm", "meta_ads_intel":
		lineNguồn = "Kết quả từ bước phân tích / đồng bộ chuyên sâu vừa sẵn sàng."
	default:
		if strings.TrimSpace(evt.EventSource) != "" {
			lineNguồn = "Yêu cầu từ hệ thống (bước nối tiếp trong quy trình)."
		}
	}
	var lineMốc string
	switch ms {
	case QueueMilestoneProcessingStart:
		lineMốc = "Ở bước này: hệ thống vừa nhận yêu cầu và bắt đầu xử lý."
	case QueueMilestoneDatachangedDone:
		if isDatachangedCustomerMirrorOnly(evt) {
			lineMốc = "Ở bước này: side-effect sau datachanged (merge CRM, báo cáo, ads…) đã chạy hoặc đã lên lịch; với loại khách POS/FB/CRM không còn handler «chính» sau đó."
		} else {
			lineMốc = "Ở bước này: đã xong phần đồng bộ phụ (CRM, báo cáo, xếp hàng phân tích… nếu áp dụng). Sắp chạy bước chính."
		}
	case QueueMilestoneHandlerDone:
		lineMốc = "Ở bước này: đã hoàn tất toàn bộ xử lý cho yêu cầu này."
	case QueueMilestoneHandlerError:
		lineMốc = "Ở bước này: xử lý không thành công — có thể được thử lại."
		if processErr != nil {
			lineMốc += " Thông báo: " + truncateRunes(processErr.Error(), 200)
		}
	case QueueMilestoneRoutingSkipped:
		lineMốc = "Ở bước này: hệ thống quyết định không cần làm thêm — không phải lỗi."
	case QueueMilestoneNoHandler:
		if isDatachangedCustomerMirrorOnly(evt) {
			lineMốc = "Ở bước này: đúng thiết kế — consumer không đăng ký bước case/pipeline sau cho mirror khách; không phải lỗi hay thiếu tính năng merge CRM."
		} else {
			lineMốc = "Ở bước này: chưa có quy trình xử lý tương ứng trên phiên bản hiện tại."
		}
	default:
		lineMốc = "Xem dòng tóm tắt phía trên."
	}
	base := []string{lineViệc}
	if lineNguồn != "" {
		base = append(base, lineNguồn)
	}
	base = append(base, lineMốc)
	if len(dn.EntityBullets) > 0 {
		base = append(base, dn.EntityBullets...)
	}
	return base
}

// queueDetailSections — Phần chi tiết trong cùng một mốc timeline (không tạo thêm live_event).
func queueDetailSections(evt *aidecisionmodels.DecisionEvent, ms QueueMilestone, processErr error) []decisionlive.DecisionLiveDetailSection {
	var steps []string
	switch ms {
	case QueueMilestoneProcessingStart:
		steps = []string{
			"Hệ thống đã lấy yêu cầu từ hàng chờ và bắt đầu xử lý.",
			"Thông tin kèm theo (đơn, khách, chiến dịch…) được đọc để làm đúng việc.",
		}
		if evt != nil && evt.EventSource == "datachanged" {
			steps = append(steps, "Nếu có thay đổi dữ liệu nguồn: có thể chạy trước các bước đồng bộ phụ (CRM, báo cáo, xếp hàng phân tích), rồi mới tới bước chính.")
		} else {
			steps = append(steps, "Tiếp theo là bước xử lý đúng với loại việc này.")
		}
	case QueueMilestoneDatachangedDone:
		if isDatachangedCustomerMirrorOnly(evt) {
			steps = []string{
				"Với cập nhật khách POS / Facebook / CRM chỉ mirror: consumer xếp job vào crm_pending_ingest (CrmIngestWorker mới merge vào crm_customers).",
				"Sau đó consumer đóng job ở trạng thái «không có handler chính» — không phải lỗi.",
			}
		} else {
			steps = []string{
				"Các bước chuẩn bị sau khi dữ liệu đổi (nếu có) đã xong.",
				"Chuyển sang bước chính: ví dụ cập nhật hồ sơ rủi ro đơn, xếp hàng phân tích, v.v.",
			}
		}
	case QueueMilestoneHandlerDone:
		steps = []string{
			"Toàn bộ xử lý nghiệp vụ cho yêu cầu này đã hoàn tất.",
			"Không còn bước nào trên hàng chờ cho mã yêu cầu này.",
		}
	case QueueMilestoneHandlerError:
		steps = []string{
			"Có lỗi trong lúc xử lý — phần tóm tắt phía trên có thể có thông báo ngắn.",
			"Hệ thống có thể tự thử lại; nếu lỗi lặp lại, cần xem nhật ký hoặc liên hệ kỹ thuật.",
		}
		if processErr != nil {
			steps = append(steps, "Chi tiết kỹ thuật có thể xuất hiện trong ô lỗi kèm theo.")
		}
	case QueueMilestoneRoutingSkipped:
		steps = []string{
			"Theo cấu hình, loại việc này không cần chạy thêm bước sâu.",
			"Dừng tại đây là đúng thiết kế, không phải sự cố.",
		}
	case QueueMilestoneNoHandler:
		if isDatachangedCustomerMirrorOnly(evt) {
			steps = []string{
				"consumer_dispatch không đăng ký handler cho pos_customer.* / fb_customer.* / crm_customer.* — enqueue CRM ingest trong applyDatachangedSideEffects, merge thực tế ở CrmIngestWorker.",
				"Nếu crm_customers vẫn trống: kiểm tra CrmIngestWorker và backlog crm_pending_ingest, log [CRM], ownerOrganizationId / customerId trên bản ghi nguồn.",
			}
		} else {
			steps = []string{
				"Phiên bản hệ thống hiện tại chưa có quy trình xử lý cho kiểu việc này.",
				"Nếu bạn cần tính năng này: kiểm tra cập nhật phần mềm hoặc cấu hình.",
			}
		}
	default:
		steps = []string{
			"Mỗi mốc trên dòng thời gian là một bước riêng — đọc theo thứ tự từ trên xuống.",
		}
	}
	return []decisionlive.DecisionLiveDetailSection{
		{Title: "Giải thích thêm cho bước này", Items: steps},
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
