// Package eventintake — CRM intel sau ingest: debounce theo org + unifiedId (trailing), lưu MongoDB decision_trailing_debounce.
package eventintake

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"
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

// ScheduleCrmIntelligenceRecomputeDebounce — trailing debounce sau crm.intelligence.recompute_requested: mỗi lần gọi cùng (org, unifiedId) → due = now + window.
// causalOrderingAtMs: gộp theo max trong cửa sổ (thứ tự nghiệp vụ); 0 = bỏ qua.
func ScheduleCrmIntelligenceRecomputeDebounce(ctx context.Context, orgHex, unifiedID string, window time.Duration, traceID, correlationID, parentEventID string, causalOrderingAtMs int64) error {
	return upsertTrailingCrmIntelAfterIngest(ctx, orgHex, unifiedID, window, traceID, correlationID, parentEventID, causalOrderingAtMs)
}

// TakeDueCrmIntelAfterIngestJobs — lấy job đã đến hạn (document đã xóa trong Mongo).
func TakeDueCrmIntelAfterIngestJobs(ctx context.Context, now time.Time) ([]CrmIntelAfterIngestFlushJob, error) {
	docs, err := takeDueTrailingDocs(ctx, now, trailingBucketCrmIntelAfterIngest)
	if err != nil || len(docs) == 0 {
		return nil, err
	}
	out := make([]CrmIntelAfterIngestFlushJob, 0, len(docs))
	for _, d := range docs {
		out = append(out, CrmIntelAfterIngestFlushJob{
			OrgHex:             strings.TrimSpace(d.OrgHex),
			UnifiedID:          strings.TrimSpace(d.UnifiedID),
			TraceID:            strings.TrimSpace(d.TraceID),
			CorrelationID:      strings.TrimSpace(d.CorrelationID),
			ParentEventID:      strings.TrimSpace(d.ParentEventID),
			CausalOrderingAtMs: d.CausalMs,
		})
	}
	return out, nil
}
