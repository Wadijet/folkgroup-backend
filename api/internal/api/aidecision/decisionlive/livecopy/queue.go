package livecopy

import (
	"fmt"
	"strings"

	"meta_commerce/internal/api/aidecision/decisionlive"
	"meta_commerce/internal/api/aidecision/eventtypes"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
)

const maxQueueErrRunes = 400

// traceConcreteObservabilityHint — mã hỗ trợ (đội kỹ thuật); người dùng chỉ cần chụp màn hình hoặc copy refs.
func traceConcreteObservabilityHint(evt *aidecisionmodels.DecisionEvent) string {
	if evt == nil {
		return "Nếu cần hỗ trợ: trong phần tham chiếu của sự kiện thường có mã luồng và mã việc."
	}
	var parts []string
	if s := strings.TrimSpace(evt.EventID); s != "" {
		parts = append(parts, "mã việc "+s)
	}
	if s := strings.TrimSpace(evt.TraceID); s != "" {
		parts = append(parts, "mã luồng "+s)
	}
	if s := strings.TrimSpace(evt.CorrelationID); s != "" {
		parts = append(parts, "mã liên kết "+s)
	}
	if len(parts) == 0 {
		return "Nếu cần hỗ trợ, hãy gửi kèm ảnh chụp dòng thời gian hoặc mã trong phần tham chiếu."
	}
	return "Gửi hỗ trợ kèm: " + strings.Join(parts, ", ") + "."
}

// queueDetailSectionTraceConcrete — neo ngắn gọn với mã nội bộ (chủ yếu cho đội kỹ thuật).
func queueDetailSectionTraceConcrete(evt *aidecisionmodels.DecisionEvent) string {
	if evt == nil {
		return "Việc này có bản ghi trong hàng đợi xử lý nội bộ — đội kỹ thuật tra theo mã luồng trong tham chiếu."
	}
	eid := strings.TrimSpace(evt.EventID)
	if eid == "" {
		return "Việc này có bản ghi trong hàng đợi xử lý nội bộ — tra theo mã luồng trong tham chiếu."
	}
	return fmt.Sprintf("Mã việc trong hệ thống: %s (đội kỹ thuật dùng để tra cứu).", eid)
}

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

// BuildQueueConsumerEvent — Timeline consumer G2: summary + ít bullet (nghiệp vụ); chi tiết/audit trong một section mở rộng; processTrace = cây nhánh logic (tùy worker điền). TraceStep (Phương án B): inputRef / reasoning / outputRef có cấu trúc. Dòng «Trong quy trình:» do Publish (enrichPublishE2ERef) chèn — không lặp ở đây.
func BuildQueueConsumerEvent(evt *aidecisionmodels.DecisionEvent, ms QueueMilestone, processErr error, extraBullets []string, processTrace []decisionlive.DecisionLiveProcessNode) decisionlive.DecisionLiveEvent {
	dn := DomainNarrativeFromQueueEvent(evt)
	bullets := queueStructuredBullets(evt, ms, dn, processErr)
	bullets = append(bullets, extraBullets...)
	bullets = capDetailBullets(bullets, 5)
	sections := queueDetailSections(evt, ms, processErr)
	phase, severity, summary, rs := queueSummaryForMilestone(ms, dn, evt, processErr)
	e2eRef := eventtypes.ResolveE2EForQueueConsumerMilestone(evt.EventType, evt.EventSource, evt.PipelineStage, queueMilestoneToE2EKey(ms))
	outcomeKind := queueOutcomeKindForMilestone(ms, evt)
	ev := decisionlive.DecisionLiveEvent{
		Phase:            phase,
		Severity:         severity,
		OutcomeKind:      outcomeKind,
		Summary:          summary,
		ReasoningSummary: rs,
		E2EStage:         e2eRef.Stage,
		E2EStepID:        e2eRef.StepID,
		E2EStepLabelVi:   e2eRef.LabelVi,
		SourceKind:       decisionlive.SourceQueue,
		SourceTitle:      queueFriendlyEventLabel(evt),
		CorrelationID:    evt.CorrelationID,
		DecisionCaseID:   decisionCaseIDFromQueuePayload(evt),
		Refs:             mergeRefsE2E(decisionlive.RefsFromDecisionEventEnvelope(evt), e2eRef),
		DetailBullets:    bullets,
		DetailSections:   sections,
		Step:             buildQueueConsumerTraceStep(evt, ms, dn, processErr),
		ProcessTrace:     processTrace,
	}
	return ev
}

// queueOutcomeKindForMilestone — Phân loại kết quả consumer queue (bất thường vs bình thường).
func queueOutcomeKindForMilestone(ms QueueMilestone, evt *aidecisionmodels.DecisionEvent) string {
	switch ms {
	case QueueMilestoneHandlerError:
		return decisionlive.OutcomeProcessingError
	case QueueMilestoneRoutingSkipped:
		return decisionlive.OutcomePolicySkipped
	case QueueMilestoneNoHandler:
		if isDatachangedCustomerMirrorOnly(evt) {
			return decisionlive.OutcomeNominal
		}
		return decisionlive.OutcomeUnsupported
	case QueueMilestoneHandlerDone:
		return decisionlive.OutcomeSuccess
	default:
		return decisionlive.OutcomeNominal
	}
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
	if evt == nil || !eventtypes.IsL1DatachangedEventSource(evt.EventSource) {
		return false
	}
	et := strings.TrimSpace(evt.EventType)
	for _, p := range []string{"pos_customer.", "fb_customer.", "crm_customer.", "customer_customer."} {
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
		// Không tiền tố G2 ở summary: neo G2-Sxx nằm ở DetailBullets (Publish/enrichPublishE2ERef) + e2e*.
		summary = fmt.Sprintf("Đang bắt đầu: %s", dn.StepTitle)
		reasoningSummary = dn.BusinessOneLine
	case QueueMilestoneDatachangedDone:
		phase = decisionlive.PhaseDatachangedEffects
		severity = decisionlive.SeverityInfo
		if isDatachangedCustomerMirrorOnly(evt) {
			summary = fmt.Sprintf("Đã đồng bộ %s — không cần bước tự động thêm trên trợ lý.", queueFriendlyEventLabel(evt))
			reasoningSummary = dn.BusinessOneLine
		} else {
			summary = fmt.Sprintf("Đã đồng bộ sau khi lưu dữ liệu — tiếp tục: %s", dn.StepTitle)
			reasoningSummary = dn.BusinessOneLine
		}
	case QueueMilestoneHandlerDone:
		phase = decisionlive.PhaseQueueDone
		severity = decisionlive.SeverityInfo
		summary = fmt.Sprintf("Đã xử lý xong: %s", dn.StepTitle)
		reasoningSummary = dn.BusinessOneLine
	case QueueMilestoneHandlerError:
		phase = decisionlive.PhaseQueueError
		severity = decisionlive.SeverityError
		errStr := ""
		if processErr != nil {
			errStr = truncateRunes(processErr.Error(), maxQueueErrRunes)
		}
		summary = fmt.Sprintf("Không xử lý được «%s»: %s", dn.StepTitle, errStr)
		reasoningSummary = dn.BusinessOneLine
	case QueueMilestoneRoutingSkipped:
		phase = decisionlive.PhaseSkipped
		severity = decisionlive.SeverityInfo
		summary = fmt.Sprintf("Đã bỏ qua theo cài đặt — «%s»", dn.StepTitle)
		reasoningSummary = dn.BusinessOneLine
	case QueueMilestoneNoHandler:
		phase = decisionlive.PhaseSkipped
		if isDatachangedCustomerMirrorOnly(evt) {
			severity = decisionlive.SeverityInfo
			summary = fmt.Sprintf("Chỉ cần đồng bộ — %s", queueFriendlyEventLabel(evt))
			reasoningSummary = dn.BusinessOneLine
		} else {
			severity = decisionlive.SeverityWarn
			summary = fmt.Sprintf("Chưa hỗ trợ loại cập nhật này — %s", queueFriendlyEventLabel(evt))
			reasoningSummary = dn.BusinessOneLine
		}
	default:
		phase = decisionlive.PhaseQueueProcessing
		severity = decisionlive.SeverityInfo
		summary = dn.StepTitle
	}
	return phase, severity, summary, reasoningSummary
}

// queueFriendlyEventLabel — nhãn hiển thị ngắn (tiếng Việt); mã kỹ thuật vẫn nằm trong refs khi cần tra cứu.
// QueueFriendlyEventLabel — nhãn ngắn tiếng Việt cho loại cập nhật (UI / processTrace / timeline).
func QueueFriendlyEventLabel(evt *aidecisionmodels.DecisionEvent) string {
	return queueFriendlyEventLabel(evt)
}

func queueFriendlyEventLabel(evt *aidecisionmodels.DecisionEvent) string {
	if evt == nil {
		return ""
	}
	et := strings.TrimSpace(evt.EventType)
	switch et {
	case eventtypes.OrderChanged, eventtypes.OrderInserted, eventtypes.OrderUpdated:
		return "Đơn hàng"
	case eventtypes.OrderIntelRecomputed:
		return "Phân tích đơn"
	case eventtypes.ConversationChanged, eventtypes.MessageChanged,
		eventtypes.ConversationInserted, eventtypes.ConversationUpdated, eventtypes.MessageInserted, eventtypes.MessageUpdated:
		return "Hội thoại / tin nhắn"
	case eventtypes.ConversationMessageInserted, eventtypes.MessageBatchReady:
		return "Tin nhắn (gom lô)"
	case eventtypes.CustomerContextReady:
		return "Thông tin khách"
	case eventtypes.CrmIntelRecomputed:
		return "Phân tích khách"
	case eventtypes.CixIntelRecomputed:
		return "Phân tích hội thoại"
	case eventtypes.CampaignIntelRecomputed, eventtypes.MetaCampaignChanged, eventtypes.MetaCampaignInserted, eventtypes.MetaCampaignUpdated:
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
		return "Khách hàng (đã gộp dữ liệu)"
	default:
		if et != "" {
			return "Cập nhật tự động"
		}
		return "Cập nhật hệ thống"
	}
}

func queueStructuredBullets(evt *aidecisionmodels.DecisionEvent, ms QueueMilestone, dn DomainNarrative, processErr error) []string {
	var parts []string
	switch ms {
	case QueueMilestoneProcessingStart:
		if evt != nil && eventtypes.IsL1DatachangedEventSource(evt.EventSource) {
			parts = append(parts, fmt.Sprintf("«%s» — sau khi lưu dữ liệu, hệ thống đồng bộ rồi chuyển bước tiếp.", dn.StepTitle))
		} else {
			parts = append(parts, fmt.Sprintf("«%s» — hệ thống đang chuyển tới bước xử lý phù hợp.", dn.StepTitle))
		}
	case QueueMilestoneDatachangedDone:
		if isDatachangedCustomerMirrorOnly(evt) {
			parts = append(parts, fmt.Sprintf("Đã đồng bộ %s — không cần thêm bước tự động trên trợ lý.", queueFriendlyEventLabel(evt)))
		} else {
			parts = append(parts, fmt.Sprintf("Đã đồng bộ xong — tiếp tục với «%s».", dn.StepTitle))
		}
	default:
		switch ms {
		case QueueMilestoneHandlerDone:
			parts = append(parts, fmt.Sprintf("Đã hoàn tất «%s».", dn.StepTitle))
		case QueueMilestoneHandlerError:
			line := fmt.Sprintf("Gặp sự cố khi xử lý «%s».", dn.StepTitle)
			if processErr != nil {
				line += " " + truncateRunes(processErr.Error(), 180)
			}
			parts = append(parts, line)
		case QueueMilestoneRoutingSkipped:
			parts = append(parts, fmt.Sprintf("Theo cài đặt, bỏ qua bước tự động cho «%s».", dn.StepTitle))
		case QueueMilestoneNoHandler:
			if isDatachangedCustomerMirrorOnly(evt) {
				parts = append(parts, fmt.Sprintf("Với %s chỉ cần đồng bộ dữ liệu — không có thêm bước trợ lý.", queueFriendlyEventLabel(evt)))
			} else {
				parts = append(parts, fmt.Sprintf("Loại cập nhật «%s» chưa được hỗ trợ tự động.", queueFriendlyEventLabel(evt)))
			}
		default:
			parts = append(parts, dn.StepTitle)
		}
	}
	if len(dn.EntityBullets) > 0 {
		parts = append(parts, dn.EntityBullets...)
	}
	return parts
}

// queueE2EPositionAuditLine — mốc timeline này map bước E2E nào (để audit đối chiếu bang-pha-buoc-event-e2e).
func queueE2EPositionAuditLine(ms QueueMilestone, evt *aidecisionmodels.DecisionEvent) string {
	// Một dòng neo với luồng G2 trong docs/flows/bang-pha-buoc-event-e2e (consumer queue).
	switch ms {
	case QueueMilestoneProcessingStart:
		return "G2 — nhận job từ hàng đợi nội bộ; bắt đầu xử lý."
	case QueueMilestoneDatachangedDone:
		if evt != nil && isDatachangedCustomerMirrorOnly(evt) {
			return "G2 — đồng bộ mirror khách xong; thường không còn handler trên consumer."
		}
		return "G2 — đồng bộ sau lưu xong; tiếp routing/handler nếu có."
	case QueueMilestoneHandlerDone:
		return "G2 — handler nghiệp vụ chạy xong."
	case QueueMilestoneHandlerError:
		return "G2 — lỗi handler; có thể retry; gửi hỗ trợ kèm mã việc/trace."
	case QueueMilestoneRoutingSkipped:
		return "G2 — routing bỏ qua handler (theo cấu hình)."
	case QueueMilestoneNoHandler:
		return "G2 — chưa có handler cho loại sự kiện (có thể bổ sung sau)."
	default:
		return "Các mốc cùng traceId xếp theo thời gian."
	}
}

func queueDetailSectionTechnicalTitle() string {
	return "Thông tin thêm"
}

// queueDetailSections — Một section mở rộng (E2E + audit); tránh nhiều accordion chồng chữ.
func queueDetailSections(evt *aidecisionmodels.DecisionEvent, ms QueueMilestone, processErr error) []decisionlive.DecisionLiveDetailSection {
	items := []string{
		queueE2EPositionAuditLine(ms, evt),
		"Vị trí trong quy trình chuẩn (G1–G6) xem e2eStepLabelVi và doc bang-pha-buoc-event-e2e.",
	}
	switch ms {
	case QueueMilestoneProcessingStart:
		items = append(items, "Thứ tự G2: nhận job → (datachanged) đồng bộ sau lưu → routing → handler.", traceConcreteObservabilityHint(evt))
		items = append(items, queueDetailSectionTraceConcrete(evt))
	case QueueMilestoneDatachangedDone:
		if isDatachangedCustomerMirrorOnly(evt) {
			items = append(items, "Mirror khách: đồng bộ nền đã chạy; thường hết chuỗi consumer tại đây.")
		} else {
			items = append(items, "Có thể có mốc kế khi dispatch handler.")
		}
	case QueueMilestoneHandlerDone:
		items = append(items, "Job được đánh dấu hoàn tất trên hàng đợi.")
	case QueueMilestoneHandlerError:
		items = append(items, "Hỗ trợ: gửi kèm org + traceId / eventId trong refs.")
		if processErr != nil {
			items = append(items, "Chi tiết lỗi: "+truncateRunes(processErr.Error(), 280))
		}
	case QueueMilestoneRoutingSkipped:
		items = append(items, "Bỏ qua handler theo quy tắc routing — không phải lỗi.")
	case QueueMilestoneNoHandler:
		if isDatachangedCustomerMirrorOnly(evt) {
			items = append(items, "Chỉ đồng bộ mirror khách — không có handler thêm.")
		} else {
			items = append(items, "Có thể bổ sung handler trong bản sau.")
		}
	default:
		items = append(items, "Các mốc được lưu theo thời gian để bạn xem lại lịch sử xử lý.")
	}
	items = capStringSlice(items, 8)
	return []decisionlive.DecisionLiveDetailSection{{Title: queueDetailSectionTechnicalTitle(), Items: items}}
}

func capStringSlice(s []string, max int) []string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func capDetailBullets(b []string, max int) []string {
	return capStringSlice(b, max)
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

func queueMilestoneToE2EKey(ms QueueMilestone) string {
	switch ms {
	case QueueMilestoneProcessingStart:
		return eventtypes.E2EQueueMilestoneProcessingStart
	case QueueMilestoneDatachangedDone:
		return eventtypes.E2EQueueMilestoneDatachangedDone
	case QueueMilestoneHandlerDone:
		return eventtypes.E2EQueueMilestoneHandlerDone
	case QueueMilestoneHandlerError:
		return eventtypes.E2EQueueMilestoneHandlerError
	case QueueMilestoneRoutingSkipped:
		return eventtypes.E2EQueueMilestoneRoutingSkipped
	case QueueMilestoneNoHandler:
		return eventtypes.E2EQueueMilestoneNoHandler
	default:
		return ""
	}
}

func mergeRefsE2E(refs map[string]string, ref eventtypes.E2ERef) map[string]string {
	if refs == nil {
		refs = make(map[string]string)
	}
	if ref.Stage != "" {
		refs[eventtypes.E2EPayloadKeyStage] = ref.Stage
	}
	if ref.StepID != "" {
		refs[eventtypes.E2EPayloadKeyStepID] = ref.StepID
	}
	if ref.LabelVi != "" {
		refs[eventtypes.E2EPayloadKeyLabelVi] = ref.LabelVi
	}
	return refs
}
