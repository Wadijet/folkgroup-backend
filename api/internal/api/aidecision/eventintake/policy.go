// Package eventintake — policy bổ sung sau khi event datachanged đã vào decision_events_queue (side-effect consumer).
//
// Lớp 1: DoSyncUpsert — giảm ghi DB khi đồng bộ ngoài.
// Lớp 2: hook aidecision (OnDataChanged) — cổng enqueue (org, registry, không delete); so updated_at nguồn thuộc lớp 1.
// Sau khi vào queue: worker applyDatachangedSideEffects (một cửa) quyết định enqueue crm_pending_merge / report / ads — intel CRM sau merge queue + recompute_requested.
// Policy (đây): dedupe xếp job merge queue theo cửa sổ (AI_DECISION_EVENTINTAKE_CRM_DEDUPE_SEC).
// Trì hoãn trailing + mức nghiệp vụ: datachanged_defer.go, datachanged_business.go — BUSINESS_DEFER_* và DEFER_* (ghi đè theo kênh nếu set trong env).
package eventintake

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"meta_commerce/internal/api/aidecision/eventtypes"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
)

// SideEffectDecision quyết định nhánh side-effect nào chạy sau datachanged.
type SideEffectDecision struct {
	AllowCrmMergeQueue bool
	AllowReport        bool
	AllowAds           bool
	ReasonsSkipped     []string
}

var (
	crmMergeQueueDedupeMu   sync.Mutex
	crmMergeQueueDedupeLast map[string]time.Time // key → lần gần nhất cho phép xếp job crm_pending_merge
)

func init() {
	crmMergeQueueDedupeLast = make(map[string]time.Time)
}

// EvaluateDatachangedSideEffects áp dụng policy chung; có thể mở rộng theo EventType / Org / collection.
func EvaluateDatachangedSideEffects(evt *aidecisionmodels.DecisionEvent, sourceCollection, idHex, orgHex string) SideEffectDecision {
	d := SideEffectDecision{
		AllowCrmMergeQueue: true,
		AllowReport:        true,
		AllowAds:           true,
	}
	if evt == nil || evt.EventSource != eventtypes.EventSourceDatachanged {
		return d
	}
	sec := envInt("AI_DECISION_EVENTINTAKE_CRM_DEDUPE_SEC", 0)
	if sec > 0 && !allowCrmMergeQueueDedupe(orgHex, sourceCollection, idHex, time.Duration(sec)*time.Second) {
		d.AllowCrmMergeQueue = false
		d.ReasonsSkipped = append(d.ReasonsSkipped, "crm_dedupe_window")
	}
	// Báo cáo: có thể thêm REPORT_MARKDIRTY_MIN_INTERVAL — mặc định luôn chạy.
	// Ads: metahooks đã debounce; có thể thêm cờ tắt lớp 2 sau này.
	return d
}

func allowCrmMergeQueueDedupe(orgHex, collection, idHex string, window time.Duration) bool {
	orgHex = strings.TrimSpace(orgHex)
	collection = strings.TrimSpace(collection)
	idHex = strings.TrimSpace(idHex)
	if orgHex == "" || collection == "" || idHex == "" {
		return true
	}
	key := orgHex + "|" + collection + "|" + idHex
	now := time.Now()
	crmMergeQueueDedupeMu.Lock()
	defer crmMergeQueueDedupeMu.Unlock()
	if t, ok := crmMergeQueueDedupeLast[key]; ok && now.Sub(t) < window {
		return false
	}
	crmMergeQueueDedupeLast[key] = now
	if len(crmMergeQueueDedupeLast) > 50000 {
		cutoff := now.Add(-2 * window)
		for k, t0 := range crmMergeQueueDedupeLast {
			if t0.Before(cutoff) {
				delete(crmMergeQueueDedupeLast, k)
			}
		}
	}
	return true
}

func envInt(key string, defaultVal int) int {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return defaultVal
	}
	return n
}
