// Package eventtypes — Tham chiếu luồng E2E (G1–G6) cho trace và Publish.
//
// Nguồn tài liệu: docs/flows/bang-pha-buoc-event-e2e.md — bảng chi tiết phải khớp logic map dưới đây khi đổi tên bước.
// Pha ghi thô G1: CIO (S01–S04) — không debounce CIO trong catalog. Job message.batch_ready (debounce emit) không còn dòng catalog pha G4; ResolveE2EForQueueEnvelope neo G2-S02 (consumer processEvent).
// Pha merge G2: G2-S01 = consumer lease một job queue — trên trục merge điển hình l1_datachanged (sau G1-S04); cùng lease cho mọi job khác. G2-S02: processEvent (gom/gấp, side-effect…). Worker miền (G2-S03–S04) merge L1→L2; G2-S05-E01 enqueue lại — l2_datachanged + <prefix>.changed + after_l2_merge (đối chiếu G1-S04); bản ghi cũ crm_merge_queue + recompute_requested. Gom debounce; gấp chỉ bỏ/rút ngắn gom.
// Pha intel G3: G3-S01 nhận job queue (điển hình l2_datachanged); G3-S02 AID xếp job *_intel_compute; G3-S03…S05 worker miền; G3-S06 catalog — miền phát <domain>_intel_recomputed lên queue.
// Pha quyết định G4: envelope decision_events_queue loại *_intel_recomputed → G4-S01 (AID nhận — ResolveOrCreate / điều phối case).
// LabelVi (e2eStepLabelVi) lấy từ E2EStepCatalog.descriptionUserVi — một nguồn với §5.3 / JSON `steps` (E2ECatalogDescriptionUserViForStep).
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
	LabelVi string // Theo §5.3: descriptionUserVi catalog cho stepId (trừ nhánh lỗi/thiếu map tùy chỉnh)
}

// e2eRefCatalog — stage + stepId máy; LabelVi = descriptionUserVi trong E2EStepCatalog (§5.3).
func e2eRefCatalog(stage, stepID string) E2ERef {
	return E2ERef{
		Stage:   stage,
		StepID:  stepID,
		LabelVi: E2ECatalogDescriptionUserViForStep(stepID),
	}
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
		return e2eRefCatalog(E2EStageG1, "G1-S04")
	}

	// --- G2 pha merge: sau merge L2 (wire mới l2_datachanged + <prefix>.changed; legacy crm_merge_queue + recompute_requested) ---
	if ps == PipelineStageAfterL2Merge {
		if es == EventSourceL2Datachanged {
			etTrim := strings.TrimSpace(et)
			if etTrim == CrmIntelligenceRecomputeRequested || (strings.LastIndexByte(etTrim, '.') > 0 && strings.HasSuffix(etTrim, ".changed")) {
				return e2eRefCatalog(E2EStageG2, "G2-S05-E01")
			}
		}
		if et == CrmIntelligenceRecomputeRequested && es == EventSourceCrmMergeQueue {
			return e2eRefCatalog(E2EStageG2, "G2-S05-E01")
		}
	}

	// --- G3: yêu cầu intel / CIX / order recompute ---
	switch et {
	case CrmIntelligenceComputeRequested:
		return e2eRefCatalog(E2EStageG3, "G3-S01")
	case CrmIntelligenceRecomputeRequested:
		return e2eRefCatalog(E2EStageG3, "G3-S01")
	case AdsIntelligenceRecomputeRequested, AdsIntelligenceRecalculateAllRequested:
		return e2eRefCatalog(E2EStageG3, "G3-S01")
	case OrderRecomputeRequested:
		return e2eRefCatalog(E2EStageG3, "G3-S01")
	case CixAnalysisRequested:
		return e2eRefCatalog(E2EStageG3, "G3-S01")
	case OrderIntelligenceRequested:
		return e2eRefCatalog(E2EStageG3, "G3-S01")
	}

	// --- G4: AID nhận handoff intel trên queue + ngữ cảnh + ra quyết định (gom debounce message.batch_ready neo G2-S02) ---
	switch et {
	case CixIntelRecomputed:
		return e2eRefCatalog(E2EStageG4, "G4-S01")
	case CrmIntelRecomputed:
		return e2eRefCatalog(E2EStageG4, "G4-S01")
	case OrderIntelRecomputed:
		return e2eRefCatalog(E2EStageG4, "G4-S01")
	case CampaignIntelRecomputed:
		return e2eRefCatalog(E2EStageG4, "G4-S01")
	case CustomerContextRequested:
		return e2eRefCatalog(E2EStageG4, "G4-S02")
	case CustomerContextReady:
		return e2eRefCatalog(E2EStageG4, "G4-S02")
	case AdsContextRequested:
		return e2eRefCatalog(E2EStageG4, "G4-S02")
	case AdsContextReady:
		return e2eRefCatalog(E2EStageG4, "G4-S02")
	case MessageBatchReady:
		return e2eRefCatalog(E2EStageG2, "G2-S02")
	case AIDecisionExecuteRequested:
		return e2eRefCatalog(E2EStageG4, "G4-S03-E01")
	case ExecutorProposeRequested:
		return e2eRefCatalog(E2EStageG4, "G4-S03-E02")
	case AdsProposeRequested:
		return e2eRefCatalog(E2EStageG4, "G4-S03-E03")
	}

	// --- Prefix / l1 datachanged còn lại (an toàn) ---
	if IsL1DatachangedEventSource(es) {
		return e2eRefCatalog(E2EStageG1, "G1-S04")
	}
	if strings.HasPrefix(et, PrefixConversation) || strings.HasPrefix(et, PrefixMessage) {
		return e2eRefCatalog(E2EStageG1, "G1-S04")
	}
	if strings.HasPrefix(et, PrefixOrder) && et != OrderRecomputeRequested && et != OrderIntelligenceRequested {
		return e2eRefCatalog(E2EStageG1, "G1-S04")
	}
	if strings.HasPrefix(et, PrefixCrmDot) || strings.HasPrefix(et, "pos_") || strings.HasPrefix(et, "fb_") ||
		strings.HasPrefix(et, "meta_") {
		return e2eRefCatalog(E2EStageG1, "G1-S04")
	}

	return E2ERef{
		Stage:   "",
		StepID:  "",
		LabelVi: "Chưa map E2E — tra docs và bổ sung eventtypes.ResolveE2EForQueueEnvelope cho " + et,
	}
}

// e2eRefFromQueueMilestoneKey — đọc stage/step từ E2EQueueMilestoneCatalog; LabelVi = descriptionUserVi §5.3 cho stepId (fallback userLabelVi / labelVi milestone).
func e2eRefFromQueueMilestoneKey(milestoneKey string) (E2ERef, bool) {
	for _, row := range E2EQueueMilestoneCatalog() {
		if row.Key == milestoneKey {
			lv := E2ECatalogDescriptionUserViForStep(row.StepID)
			if lv == "" {
				lv = strings.TrimSpace(row.UserLabelVi)
			}
			if lv == "" {
				lv = row.LabelVi
			}
			return E2ERef{Stage: row.StageID, StepID: row.StepID, LabelVi: lv}, true
		}
	}
	return E2ERef{}, false
}

// ResolveE2EForQueueConsumerMilestone — mốc timeline worker (pha merge G2 — consumer một cửa): đặt lên trên envelope cùng job.
func ResolveE2EForQueueConsumerMilestone(eventType, eventSource, pipelineStage, milestone string) E2ERef {
	ms := strings.TrimSpace(milestone)
	if ref, ok := e2eRefFromQueueMilestoneKey(ms); ok {
		return ref
	}
	return ResolveE2EForQueueEnvelope(eventType, eventSource, pipelineStage)
}

// ResolveE2EForLivePhase — khi Publish chỉ có phase (engine / orchestrate), không có envelope queue.
// phase: giá trị decisionlive.Phase* (truyền string để tránh import vòng).
// Bám docs/flows/bang-pha-buoc-event-e2e.md (G4 = pha ra quyết định).
func ResolveE2EForLivePhase(phase string) E2ERef {
	p := strings.TrimSpace(phase)
	switch p {
	case "queue_processing":
		return e2eRefCatalog(E2EStageG2, "G2-S01")
	case "datachanged_effects":
		return e2eRefCatalog(E2EStageG2, "G2-S02")
	case "queue_done", "queue_error":
		return e2eRefCatalog(E2EStageG2, "G2-S02")
	case "orchestrate":
		return e2eRefCatalog(E2EStageG4, "G4-S01")
	case "execute_ready":
		return e2eRefCatalog(E2EStageG4, "G4-S03")
	case "cix_integrated":
		return e2eRefCatalog(E2EStageG4, "G4-S02")
	case "queued":
		return e2eRefCatalog(E2EStageG4, "G4-S03-E01")
	case "consuming":
		return e2eRefCatalog(E2EStageG4, "G4-S03")
	case "parse":
		return e2eRefCatalog(E2EStageG4, "G4-S03")
	case "llm":
		return e2eRefCatalog(E2EStageG4, "G4-S03")
	case "decision":
		return e2eRefCatalog(E2EStageG4, "G4-S03")
	case "policy":
		return e2eRefCatalog(E2EStageG4, "G4-S03")
	case "propose":
		return e2eRefCatalog(E2EStageG4, "G4-S03-E02")
	case "empty":
		return e2eRefCatalog(E2EStageG4, "G4-S03")
	case "skipped":
		return e2eRefCatalog(E2EStageG4, "G4-S03")
	case "ads_evaluate":
		return e2eRefCatalog(E2EStageG4, "G4-S03")
	case "intel_domain_compute_start":
		return e2eRefCatalog(E2EStageG3, "G3-S03")
	case "intel_domain_compute_done":
		return e2eRefCatalog(E2EStageG3, "G3-S05")
	case "intel_domain_compute_error":
		return e2eRefCatalog(E2EStageG3, "G3-S04")
	case "done":
		return e2eRefCatalog(E2EStageG4, "G4-S03")
	case "error":
		return e2eRefCatalog(E2EStageG4, "G4-S03")
	default:
		if p == "" {
			return E2ERef{LabelVi: "Thiếu phase — chưa gán bước chuẩn"}
		}
		// Không gán e2eStepId kiểu "G4" (không có trong E2EStepCatalog); để trống stage/step để UI/lọc không nhầm với bước chuẩn Gx-Syy.
		return E2ERef{
			Stage:   "",
			StepID:  "",
			LabelVi: "Phase live chưa map E2E (Gx-Syy) — " + p + " — bổ sung ResolveE2EForLivePhase / constants Phase* trong decisionlive",
		}
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
