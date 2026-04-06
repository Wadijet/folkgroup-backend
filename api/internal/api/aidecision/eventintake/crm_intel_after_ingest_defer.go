// Package eventintake — CRM intel sau ingest: debounce theo org + unifiedId (trailing, in-process).
// Cùng mô hình bộ nhớ process-local với ScheduleDeferredSideEffect (datachanged_defer.go).
// Lưu trữ: queuedebounce.MetaTable (Phase 4).
package eventintake

import (
	"os"
	"strconv"
	"strings"
	"time"

	"meta_commerce/internal/queuedebounce"
)

// CrmIntelAfterIngestFlushJob — một cặp org + unifiedId đến hạn, consumer sẽ xếp job crm_intel_compute (refresh).
type CrmIntelAfterIngestFlushJob struct {
	OrgHex        string
	UnifiedID     string
	TraceID       string
	CorrelationID string
	ParentEventID string // event crm.intelligence.recompute_requested gần nhất (để trace parentDecisionEventID)
	// CausalOrderingAtMs — max causal trong cửa sổ debounce (thứ tự nghiệp vụ khi enqueue crm_intel_compute).
	CausalOrderingAtMs int64
}

const (
	defaultCrmIntelAfterIngestDebounceSec = 90
)

// CrmIntelAfterIngestDebounceWindow — cửa sổ trailing (mỗi event mới → lùi deadline); env AI_DECISION_CRM_INTEL_AFTER_INGEST_DEBOUNCE_SEC.
func CrmIntelAfterIngestDebounceWindow() time.Duration {
	sec := defaultCrmIntelAfterIngestDebounceSec
	if s := strings.TrimSpace(os.Getenv("AI_DECISION_CRM_INTEL_AFTER_INGEST_DEBOUNCE_SEC")); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			sec = n
		}
	}
	if sec <= 0 {
		return 0
	}
	return time.Duration(sec) * time.Second
}

type crmIntelAfterIngestKey struct {
	orgHex    string
	unifiedID string
}

type crmIntelAfterIngestTrace struct {
	traceID       string
	correlationID string
	parentEventID string
	causalMs      int64
}

func mergeCausalMsDebounced(prev, next int64) int64 {
	if prev <= 0 {
		return next
	}
	if next <= 0 {
		return prev
	}
	if prev > next {
		return prev
	}
	return next
}

func mergeCrmIntelAfterIngestTrace(prev, next crmIntelAfterIngestTrace) crmIntelAfterIngestTrace {
	out := prev
	if s := strings.TrimSpace(next.traceID); s != "" {
		out.traceID = s
	}
	if s := strings.TrimSpace(next.correlationID); s != "" {
		out.correlationID = s
	}
	if s := strings.TrimSpace(next.parentEventID); s != "" {
		out.parentEventID = s
	}
	out.causalMs = mergeCausalMsDebounced(prev.causalMs, next.causalMs)
	return out
}

var crmIntelAfterIngestDebouncer = queuedebounce.NewMetaTable[crmIntelAfterIngestKey, crmIntelAfterIngestTrace](mergeCrmIntelAfterIngestTrace)

// ScheduleCrmIntelligenceRecomputeDebounce — trailing debounce sau crm.intelligence.recompute_requested: mỗi lần gọi cùng (org, unifiedId) → due = now + window.
// causalOrderingAtMs: gộp theo max trong cửa sổ (thứ tự nghiệp vụ); 0 = bỏ qua.
func ScheduleCrmIntelligenceRecomputeDebounce(orgHex, unifiedID string, window time.Duration, traceID, correlationID, parentEventID string, causalOrderingAtMs int64) {
	orgHex = strings.TrimSpace(orgHex)
	unifiedID = strings.TrimSpace(unifiedID)
	if orgHex == "" || unifiedID == "" {
		return
	}
	if window <= 0 {
		window = CrmIntelAfterIngestDebounceWindow()
	}
	if window <= 0 {
		return
	}
	k := crmIntelAfterIngestKey{orgHex: orgHex, unifiedID: unifiedID}
	meta := crmIntelAfterIngestTrace{
		traceID:       traceID,
		correlationID: correlationID,
		parentEventID: parentEventID,
		causalMs:      causalOrderingAtMs,
	}
	crmIntelAfterIngestDebouncer.Schedule(k, window, meta)
}

// TakeDueCrmIntelAfterIngestJobs — lấy job đã đến hạn và xóa khỏi lịch.
func TakeDueCrmIntelAfterIngestJobs(now time.Time) []CrmIntelAfterIngestFlushJob {
	entries := crmIntelAfterIngestDebouncer.TakeDue(now)
	if len(entries) == 0 {
		return nil
	}
	out := make([]CrmIntelAfterIngestFlushJob, 0, len(entries))
	for _, e := range entries {
		out = append(out, CrmIntelAfterIngestFlushJob{
			OrgHex:             e.Key.orgHex,
			UnifiedID:          e.Key.unifiedID,
			TraceID:            e.Meta.traceID,
			CorrelationID:      e.Meta.correlationID,
			ParentEventID:      e.Meta.parentEventID,
			CausalOrderingAtMs: e.Meta.causalMs,
		})
	}
	return out
}
