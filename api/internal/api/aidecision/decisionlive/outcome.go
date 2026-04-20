package decisionlive

import (
	"strings"

	"meta_commerce/internal/api/aidecision/eventtypes"
)

// OutcomeKind — Phân loại kết quả mốc timeline (ổn định cho lọc UI / cảnh báo / audit).
// Bất thường (outcomeAbnormal=true): lỗi, thiếu hỗ trợ, thiếu dữ liệu, không có hành động, đề xuất thất bại, một phần thất bại, bỏ qua theo policy.
const (
	OutcomeNominal         = "nominal"          // Tiến trình bình thường (mốc trung gian)
	OutcomeSuccess         = "success"          // Hoàn tất mong đợi
	OutcomeProcessingError = "processing_error" // Lỗi kỹ thuật / handler / queue
	OutcomePolicySkipped   = "policy_skipped"   // Bỏ qua theo quy tắc routing (noop có chủ đích)
	OutcomeUnsupported     = "unsupported"      // Chưa có xử lý cho loại sự kiện
	OutcomeDataIncomplete  = "data_incomplete"  // Thiếu dữ liệu đầu vào (vd. chưa có phân tích hội thoại)
	OutcomeNoActions       = "no_actions"       // Sau phân tích không còn hành động phù hợp
	OutcomeProposalFailed  = "proposal_failed"  // Không tạo được đề xuất / việc cần làm
	OutcomePartialFailure  = "partial_failure"  // Một phần luồng lỗi (vd. không xếp hàng intel đơn)
	// OutcomeQueueSkippedUnspecified — queue PhaseSkipped nhưng không suy ra được policy vs unsupported (infer dự phòng).
	OutcomeQueueSkippedUnspecified = "queue_skipped_unspecified"
)

// IsAbnormalOutcomeKind — true nếu cần hiển thị / lọc như trường hợp cần chú ý (khác nominal/success).
func IsAbnormalOutcomeKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case OutcomeNominal, OutcomeSuccess, "":
		return false
	default:
		return true
	}
}

// OutcomeLabelViForKind — Nhãn ngắn cho chip/filter người dùng.
func OutcomeLabelViForKind(kind string) string {
	return eventtypes.ResolveLiveOutcomeLabelVi(kind)
}

// inferOutcomeKindFromPhaseSeverity — Khi builder chưa gán OutcomeKind, suy ra từ phase/severity/nguồn.
func inferOutcomeKindFromPhaseSeverity(ev *DecisionLiveEvent) string {
	if ev == nil {
		return OutcomeNominal
	}
	if ev.Severity == SeverityError || ev.Phase == PhaseQueueError || ev.Phase == PhaseError {
		return OutcomeProcessingError
	}
	switch ev.Phase {
	case PhaseAdsEvaluate:
		if ev.Severity == SeverityError {
			return OutcomeProcessingError
		}
		if ev.Severity == SeverityWarn {
			return OutcomeNoActions
		}
		return OutcomeNominal
	case PhaseEmpty:
		return OutcomeNoActions
	case PhaseSkipped:
		if ev.SourceKind == SourceQueue {
			return OutcomeQueueSkippedUnspecified
		}
		return OutcomeDataIncomplete
	case PhasePropose:
		if ev.Severity == SeverityWarn {
			return OutcomeProposalFailed
		}
	}
	if ev.Severity == SeverityWarn && ev.Phase == PhaseEmpty {
		return OutcomeNoActions
	}
	return OutcomeNominal
}

// EnrichLiveOutcomeMetadata — Gắn outcomeKind (nếu thiếu), outcomeAbnormal, outcomeLabelVi trước khi đẩy live/persist.
func EnrichLiveOutcomeMetadata(ev *DecisionLiveEvent) {
	if ev == nil {
		return
	}
	if strings.TrimSpace(ev.OutcomeKind) == "" {
		ev.OutcomeKind = inferOutcomeKindFromPhaseSeverity(ev)
	}
	if strings.TrimSpace(ev.OutcomeLabelVi) == "" {
		ev.OutcomeLabelVi = OutcomeLabelViForKind(ev.OutcomeKind)
	}
	ev.OutcomeAbnormal = IsAbnormalOutcomeKind(ev.OutcomeKind)
}
