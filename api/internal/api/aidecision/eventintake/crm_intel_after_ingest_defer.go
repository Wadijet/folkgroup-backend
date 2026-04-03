// Package eventintake — Trì hoãn trailing yêu cầu tính lại CRM intelligence sau ingest worker (gom theo org + unifiedId).
// Cùng mô hình bộ nhớ process-local với ScheduleDeferredSideEffect (datachanged_defer.go).
package eventintake

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CrmIntelAfterIngestFlushJob — một cặp org + unifiedId đến hạn, consumer sẽ xếp job crm_intel_compute (refresh).
type CrmIntelAfterIngestFlushJob struct {
	OrgHex        string
	UnifiedID     string
	TraceID       string
	CorrelationID string
	ParentEventID string // event crm.intelligence.recompute_requested gần nhất (để trace parentDecisionEventID)
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
	orgHex, unifiedID string
}

type crmIntelAfterIngestSlot struct {
	due           time.Time
	traceID       string
	correlationID string
	parentEventID string
}

var (
	crmIntelAfterIngestMu    sync.Mutex
	crmIntelAfterIngestSlots = make(map[crmIntelAfterIngestKey]*crmIntelAfterIngestSlot)
)

// ScheduleCrmIntelligenceRecomputeDebounce — trailing debounce sau crm.intelligence.recompute_requested: mỗi lần gọi cùng (org, unifiedId) → due = now + window.
func ScheduleCrmIntelligenceRecomputeDebounce(orgHex, unifiedID string, window time.Duration, traceID, correlationID, parentEventID string) {
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
	due := time.Now().Add(window)

	crmIntelAfterIngestMu.Lock()
	defer crmIntelAfterIngestMu.Unlock()
	slot := crmIntelAfterIngestSlots[k]
	if slot == nil {
		slot = &crmIntelAfterIngestSlot{}
		crmIntelAfterIngestSlots[k] = slot
	}
	slot.due = due
	if strings.TrimSpace(traceID) != "" {
		slot.traceID = strings.TrimSpace(traceID)
	}
	if strings.TrimSpace(correlationID) != "" {
		slot.correlationID = strings.TrimSpace(correlationID)
	}
	if strings.TrimSpace(parentEventID) != "" {
		slot.parentEventID = strings.TrimSpace(parentEventID)
	}
}

// TakeDueCrmIntelAfterIngestJobs — lấy job đã đến hạn và xóa khỏi lịch.
func TakeDueCrmIntelAfterIngestJobs(now time.Time) []CrmIntelAfterIngestFlushJob {
	crmIntelAfterIngestMu.Lock()
	defer crmIntelAfterIngestMu.Unlock()
	var out []CrmIntelAfterIngestFlushJob
	for k, slot := range crmIntelAfterIngestSlots {
		if slot == nil || now.Before(slot.due) {
			continue
		}
		out = append(out, CrmIntelAfterIngestFlushJob{
			OrgHex:        k.orgHex,
			UnifiedID:     k.unifiedID,
			TraceID:       slot.traceID,
			CorrelationID: slot.correlationID,
			ParentEventID: slot.parentEventID,
		})
		delete(crmIntelAfterIngestSlots, k)
	}
	return out
}
