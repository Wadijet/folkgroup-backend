// Package eventtypes — Tham chiếu luồng E2E (G1–G6) cho trace và Publish.
//
// Nguồn tài liệu: docs/flows/bang-pha-buoc-event-e2e.md — bảng chi tiết phải khớp logic map dưới đây khi đổi tên bước.
// Pha ghi thô G1: CIO (S01–S04) — không debounce CIO trong catalog. Job message.batch_ready (debounce emit) không còn dòng catalog pha G4; ResolveE2EForQueueEnvelope neo G2-S02 (consumer processEvent).
// Pha merge G2: G2-S01 = consumer lease một job queue — trên trục merge điển hình l1_datachanged (sau G1-S04); cùng lease cho mọi job khác. G2-S02: processEvent (gom/gấp, side-effect…). Worker miền (G2-S03–S04) merge L1→L2; G2-S05-E01 enqueue lại — l2_datachanged + <prefix>.changed + after_l2_merge (đối chiếu G1-S04); bản ghi cũ crm_merge_queue + recompute_requested. Gom debounce; gấp chỉ bỏ/rút ngắn gom.
// Pha intel G3: G3-S01 nhận job queue (điển hình l2_datachanged); G3-S02 AID xếp job *_intel_compute; G3-S03…S05 worker miền; G3-S06 catalog — miền phát <domain>_intel_recomputed lên queue.
// Pha quyết định G4: envelope decision_events_queue loại *_intel_recomputed → G4-S01 (AID nhận — ResolveOrCreate / điều phối case), LabelVi theo eventType.
// Hai chế độ:
//   - Envelope queue: vị trí nghiệp vụ theo eventType (G1 enqueue, G3 intel, G4 quyết định…).
//   - Timeline consumer: mốc vòng đời worker — Stage G2; mốc sau lease đều neo G2-S02 (trừ processing_start = G2-S01).
package eventtypes

import "strings"

// Giai đoạn tham chiếu E2E (một chữ G + số) — khớp bảng docs G1–G6.
const (
	E2EStageG1 = "G1"
	E2EStageG2 = "G2"
	E2EStageG3 = "G3"
	E2EStageG4 = "G4"
	E2EStageG5 = "G5"
	E2EStageG6 = "G6"
)

// Mốc vòng đời consumer (timeline live) — truyền vào ResolveE2EForQueueConsumerMilestone.
const (
	E2EQueueMilestoneProcessingStart = "processing_start"
	E2EQueueMilestoneDatachangedDone = "datachanged_done"
	E2EQueueMilestoneHandlerDone     = "handler_done"
	E2EQueueMilestoneHandlerError    = "handler_error"
	E2EQueueMilestoneRoutingSkipped  = "routing_skipped"
	E2EQueueMilestoneNoHandler       = "no_handler"
)

// E2ERef — một điểm trên luồng tham chiếu (đủ để hiển thị / lọc / audit).
type E2ERef struct {
	Stage   string // G1…G6 (rỗng = chưa gán)
	StepID  string // Ví dụ G1-S04, G4-S01
	LabelVi string // Mô tả ngắn tiếng Việt
}

// ResolveE2EForQueueEnvelope map envelope decision_events_queue → tham chiếu E2E (vị trí nghiệp vụ).
func ResolveE2EForQueueEnvelope(eventType, eventSource, pipelineStage string) E2ERef {
	et := strings.TrimSpace(eventType)
	es := strings.TrimSpace(eventSource)
	ps := strings.TrimSpace(pipelineStage)
	if et == "" {
		return E2ERef{LabelVi: "Thiếu eventType — chưa gán bước chuẩn"}
	}

	// --- G1 pha ghi thô: l1_datachanged (tương thích datachanged) → queue (G1-S04) ---
	if IsL1DatachangedEventSource(es) && IsPipelineStageAfterL1Change(ps) {
		return E2ERef{Stage: E2EStageG1, StepID: "G1-S04", LabelVi: "Enqueue sau CRUD / hook datachanged"}
	}

	// --- G2 pha merge: sau merge L2 (wire mới l2_datachanged + <prefix>.changed; legacy crm_merge_queue + recompute_requested) ---
	if ps == PipelineStageAfterL2Merge {
		if es == EventSourceL2Datachanged {
			etTrim := strings.TrimSpace(et)
			if etTrim == CrmIntelligenceRecomputeRequested || (strings.LastIndexByte(etTrim, '.') > 0 && strings.HasSuffix(etTrim, ".changed")) {
				return E2ERef{Stage: E2EStageG2, StepID: "G2-S05-E01", LabelVi: "Báo cập nhật sau L2 (canonical) — enqueue AID"}
			}
		}
		if et == CrmIntelligenceRecomputeRequested && es == EventSourceCrmMergeQueue {
			return E2ERef{Stage: E2EStageG2, StepID: "G2-S05-E01", LabelVi: "Yêu cầu tính lại CRM intel sau merge L2 (bản ghi cũ)"}
		}
	}

	// --- G3: yêu cầu intel / CIX / order recompute ---
	switch et {
	case CrmIntelligenceComputeRequested:
		return E2ERef{Stage: E2EStageG3, StepID: "G3-S01", LabelVi: "Yêu cầu compute CRM intel"}
	case CrmIntelligenceRecomputeRequested:
		return E2ERef{Stage: E2EStageG3, StepID: "G3-S01", LabelVi: "Yêu cầu recompute CRM intel"}
	case AdsIntelligenceRecomputeRequested, AdsIntelligenceRecalculateAllRequested:
		return E2ERef{Stage: E2EStageG3, StepID: "G3-S01", LabelVi: "Yêu cầu recompute Ads intel"}
	case OrderRecomputeRequested:
		return E2ERef{Stage: E2EStageG3, StepID: "G3-S01", LabelVi: "Yêu cầu recompute Order intel"}
	case CixAnalysisRequested:
		return E2ERef{Stage: E2EStageG3, StepID: "G3-S01", LabelVi: "Yêu cầu phân tích CIX"}
	case OrderIntelligenceRequested:
		return E2ERef{Stage: E2EStageG3, StepID: "G3-S01", LabelVi: "Order intelligence (legacy)"}
	}

	// --- G4: AID nhận handoff intel trên queue + ngữ cảnh + ra quyết định (gom debounce message.batch_ready neo G2-S02) ---
	switch et {
	case CixIntelRecomputed:
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S01", LabelVi: "AID nhận cix_intel_recomputed — ResolveOrCreate / điều phối case"}
	case CrmIntelRecomputed:
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S01", LabelVi: "AID nhận crm_intel_recomputed — ResolveOrCreate / điều phối case"}
	case OrderIntelRecomputed:
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S01", LabelVi: "AID nhận order_intel_recomputed — ResolveOrCreate / điều phối case"}
	case CampaignIntelRecomputed:
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S01", LabelVi: "AID nhận campaign_intel_recomputed — ResolveOrCreate / điều phối case"}
	case CustomerContextRequested:
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S02", LabelVi: "Yêu cầu context khách hàng"}
	case CustomerContextReady:
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S02", LabelVi: "Context khách hàng sẵn sàng"}
	case AdsContextRequested:
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S02", LabelVi: "Yêu cầu context ads"}
	case AdsContextReady:
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S02", LabelVi: "Context ads sẵn sàng"}
	case MessageBatchReady:
		return E2ERef{Stage: E2EStageG2, StepID: "G2-S02", LabelVi: "Job message.batch_ready — xử lý trên consumer (gom debounce)"}
	case AIDecisionExecuteRequested:
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S03-E01", LabelVi: "Phát lệnh thực thi AID"}
	case ExecutorProposeRequested:
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S03-E02", LabelVi: "Yêu cầu đề xuất Executor"}
	case AdsProposeRequested:
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S03-E03", LabelVi: "Đề xuất Ads (legacy)"}
	}

	// --- Prefix / l1 datachanged còn lại (an toàn) ---
	if IsL1DatachangedEventSource(es) {
		return E2ERef{Stage: E2EStageG1, StepID: "G1-S04", LabelVi: "Enqueue từ L1 datachanged (chi tiết bước xem eventType)"}
	}
	if strings.HasPrefix(et, PrefixConversation) || strings.HasPrefix(et, PrefixMessage) {
		return E2ERef{Stage: E2EStageG1, StepID: "G1-S04", LabelVi: "Sự kiện domain → queue (conversation/message)"}
	}
	if strings.HasPrefix(et, PrefixOrder) && et != OrderRecomputeRequested && et != OrderIntelligenceRequested {
		return E2ERef{Stage: E2EStageG1, StepID: "G1-S04", LabelVi: "Sự kiện domain → queue (order)"}
	}
	if strings.HasPrefix(et, PrefixCrmDot) || strings.HasPrefix(et, "pos_") || strings.HasPrefix(et, "fb_") ||
		strings.HasPrefix(et, "meta_") {
		return E2ERef{Stage: E2EStageG1, StepID: "G1-S04", LabelVi: "Sự kiện domain / Meta / POS → queue"}
	}

	return E2ERef{
		Stage:   "",
		StepID:  "",
		LabelVi: "Chưa map E2E — tra docs và bổ sung eventtypes.ResolveE2EForQueueEnvelope cho " + et,
	}
}

// ResolveE2EForQueueConsumerMilestone — mốc timeline worker (pha merge G2 — consumer một cửa): đặt lên trên envelope cùng job.
func ResolveE2EForQueueConsumerMilestone(eventType, eventSource, pipelineStage, milestone string) E2ERef {
	ms := strings.TrimSpace(milestone)
	switch ms {
	case E2EQueueMilestoneProcessingStart:
		return E2ERef{Stage: E2EStageG2, StepID: "G2-S01", LabelVi: "Consumer lease job — điển hình l1_datachanged (trục merge); mọi job queue khác cũng qua bước này"}
	case E2EQueueMilestoneDatachangedDone:
		return E2ERef{Stage: E2EStageG2, StepID: "G2-S02", LabelVi: "Hoàn tất tác vụ sau datachanged (consumer)"}
	case E2EQueueMilestoneHandlerDone:
		return E2ERef{Stage: E2EStageG2, StepID: "G2-S02", LabelVi: "Đóng job consumer — handler hoàn tất"}
	case E2EQueueMilestoneHandlerError:
		return E2ERef{Stage: E2EStageG2, StepID: "G2-S02", LabelVi: "Lỗi xử lý trên consumer"}
	case E2EQueueMilestoneRoutingSkipped:
		return E2ERef{Stage: E2EStageG2, StepID: "G2-S02", LabelVi: "Routing bỏ qua handler (noop)"}
	case E2EQueueMilestoneNoHandler:
		return E2ERef{Stage: E2EStageG2, StepID: "G2-S02", LabelVi: "Chưa có handler đăng ký cho eventType"}
	default:
		return ResolveE2EForQueueEnvelope(eventType, eventSource, pipelineStage)
	}
}

// ResolveE2EForLivePhase — khi Publish chỉ có phase (engine / orchestrate), không có envelope queue.
// phase: giá trị decisionlive.Phase* (truyền string để tránh import vòng).
// Bám docs/flows/bang-pha-buoc-event-e2e.md (G4 = pha ra quyết định).
func ResolveE2EForLivePhase(phase string) E2ERef {
	p := strings.TrimSpace(phase)
	switch p {
	case "queue_processing":
		return E2ERef{Stage: E2EStageG2, StepID: "G2-S01", LabelVi: "Consumer đang xử lý job queue (thường l1_datachanged trên trục merge)"}
	case "datachanged_effects":
		return E2ERef{Stage: E2EStageG2, StepID: "G2-S02", LabelVi: "Tác vụ sau datachanged"}
	case "queue_done", "queue_error":
		return E2ERef{Stage: E2EStageG2, StepID: "G2-S02", LabelVi: "Kết thúc xử lý job trên consumer"}
	case "orchestrate":
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S01", LabelVi: "ResolveOrCreate case — điều phối (orchestrate)"}
	case "execute_ready":
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S03", LabelVi: "Đủ ngữ cảnh — chuẩn bị execute (gate)"}
	case "cix_integrated":
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S02", LabelVi: "Đã tích hợp phân tích CIX vào case"}
	case "queued":
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S03-E01", LabelVi: "Đã xếp hàng thực thi (execute_requested)"}
	case "consuming":
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S03", LabelVi: "Engine đang chạy (ExecuteWithCase)"}
	case "parse":
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S03", LabelVi: "Đọc gợi ý từ tình huống (parse)"}
	case "llm":
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S03", LabelVi: "LLM / tinh chỉnh gợi ý"}
	case "decision":
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S03", LabelVi: "Tổng hợp quyết định"}
	case "policy":
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S03", LabelVi: "Áp policy duyệt / tự động"}
	case "propose":
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S03-E02", LabelVi: "Đề xuất vào Executor (propose)"}
	case "empty":
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S03", LabelVi: "Không có hành động sau quyết định"}
	case "skipped":
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S03", LabelVi: "Bỏ qua bước (ví dụ thiếu CIX)"}
	case "ads_evaluate":
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S03", LabelVi: "Đánh giá quy tắc Ads / tối ưu chiến dịch"}
	case "intel_domain_compute_start":
		return E2ERef{Stage: E2EStageG3, StepID: "G3-S03", LabelVi: "Worker domain — bắt đầu tính Intelligence / ngữ cảnh"}
	case "intel_domain_compute_done":
		return E2ERef{Stage: E2EStageG3, StepID: "G3-S05", LabelVi: "Worker domain — hoàn tất bước Intelligence / ngữ cảnh (sau persist read model)"}
	case "intel_domain_compute_error":
		return E2ERef{Stage: E2EStageG3, StepID: "G3-S04", LabelVi: "Worker domain — lỗi khi chạy Intelligence / ngữ cảnh"}
	case "done":
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S03", LabelVi: "Hoàn tất pipeline engine (mốc live)"}
	case "error":
		return E2ERef{Stage: E2EStageG4, StepID: "G4-S03", LabelVi: "Lỗi trong pipeline live"}
	default:
		if p == "" {
			return E2ERef{LabelVi: "Thiếu phase — chưa gán bước chuẩn"}
		}
		return E2ERef{Stage: E2EStageG4, StepID: "G4", LabelVi: "Luồng live AID — phase " + p}
	}
}

// E2EKeysPayload — khóa lưu trong payload queue (và đồng bộ Refs) để consumer đọc được.
const (
	E2EPayloadKeyStage   = "e2eStage"
	E2EPayloadKeyStepID  = "e2eStepId"
	E2EPayloadKeyLabelVi = "e2eStepLabelVi"
)

// MergePayloadE2E ghi tham chiếu E2E vào payload (nếu có giai đoạn).
func MergePayloadE2E(payload map[string]interface{}, ref E2ERef) {
	if payload == nil {
		return
	}
	if ref.Stage != "" {
		payload[E2EPayloadKeyStage] = ref.Stage
	}
	if ref.StepID != "" {
		payload[E2EPayloadKeyStepID] = ref.StepID
	}
	if ref.LabelVi != "" {
		payload[E2EPayloadKeyLabelVi] = ref.LabelVi
	}
}
