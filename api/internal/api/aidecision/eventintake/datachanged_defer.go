// Package eventintake — Hàng đợi trì hoãn side-effect datachanged (trailing debounce).
//
// Khi RULE_DATACHANGED_SIDE_EFFECT_POLICY chạy thành công: số giây do script + PARAM_DATACHANGED_SIDE_EFFECT_POLICY quyết định (ưu tiên hơn env tier).
// Fallback: phân tầng datachanged_business.go + env:
//   - AI_DECISION_BUSINESS_DEFER_OPERATIONAL_SEC — cửa sổ gom cho mức “vận hành” (mặc định 90).
//   - AI_DECISION_BUSINESS_DEFER_BACKGROUND_SEC — cửa sổ gom cho mức “nền” (mặc định 300).
//   - AI_DECISION_DEFER_REPORT_SEC / DEFER_CRM_REFRESH_SEC / DEFER_CRM_INGEST_SEC — nếu **đặt trong env**
//     (kể cả =0) thì **ghi đè** chỉ kênh đó theo từng giá trị; không đặt thì dùng số theo mức nghiệp vụ.
//
// Mỗi lần có datachanged mới cho cùng (org, collection, id), deadline trailing = now + window.
package eventintake

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DeferChannel kênh side-effect (mỗi kênh có thể có override env riêng).
type DeferChannel int

const (
	DeferChannelCRMIngest DeferChannel = iota
	DeferChannelReport
	DeferChannelCRMRefresh
)

// DeferredSideEffectKind loại side-effect được lên lịch flush sau.
type DeferredSideEffectKind string

const (
	DeferredKindReport     DeferredSideEffectKind = "report"
	DeferredKindCrmRefresh DeferredSideEffectKind = "crm_refresh"
	DeferredKindCrmIngest  DeferredSideEffectKind = "crm_ingest"
)

// DeferredSideEffectFlushJob một việc đến hạn cần chạy trong worker (đọc lại Mongo rồi gọi report/ingest/refresh).
type DeferredSideEffectFlushJob struct {
	Kind   DeferredSideEffectKind
	OrgHex string
	Coll   string
	IDHex  string
}

type deferEntityKey struct {
	orgHex, coll, idHex string
}

var (
	deferSchedMu   sync.Mutex
	reportDeferDue map[deferEntityKey]time.Time
	crmRefDeferDue map[deferEntityKey]time.Time
	crmIngDeferDue map[deferEntityKey]time.Time
)

func init() {
	reportDeferDue = make(map[deferEntityKey]time.Time)
	crmRefDeferDue = make(map[deferEntityKey]time.Time)
	crmIngDeferDue = make(map[deferEntityKey]time.Time)
}

// deferSecForChannel: nếu env key có trong môi trường → dùng giá trị (0 = không trì hoãn kênh này).
// Nếu không khai báo env → fallbackSec (theo mức nghiệp vụ).
func deferSecForChannel(envKey string, fallbackSec int) int {
	raw, ok := os.LookupEnv(envKey)
	if !ok {
		if fallbackSec < 0 {
			return 0
		}
		return fallbackSec
	}
	s := strings.TrimSpace(raw)
	if s == "" {
		return fallbackSec
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return fallbackSec
	}
	return n
}

// DeferWindowFor trả về thời lượng trì hoãn trailing cho kênh, theo mức nghiệp vụ + ghi đè env (LookupEnv).
func DeferWindowFor(u BusinessSideEffectUrgency, ch DeferChannel) time.Duration {
	if u == UrgencyRealtime {
		return 0
	}
	opSec := envInt("AI_DECISION_BUSINESS_DEFER_OPERATIONAL_SEC", 90)
	bgSec := envInt("AI_DECISION_BUSINESS_DEFER_BACKGROUND_SEC", 300)
	fallback := opSec
	if u == UrgencyBackground {
		fallback = bgSec
	}
	var key string
	switch ch {
	case DeferChannelReport:
		key = "AI_DECISION_DEFER_REPORT_SEC"
	case DeferChannelCRMRefresh:
		key = "AI_DECISION_DEFER_CRM_REFRESH_SEC"
	case DeferChannelCRMIngest:
		key = "AI_DECISION_DEFER_CRM_INGEST_SEC"
	default:
		return 0
	}
	sec := deferSecForChannel(key, fallback)
	if sec <= 0 {
		return 0
	}
	return time.Duration(sec) * time.Second
}

// ScheduleDeferredSideEffect đặt deadline trailing: mỗi lần gọi → due = now + window.
func ScheduleDeferredSideEffect(kind DeferredSideEffectKind, orgHex, coll, idHex string, window time.Duration) {
	if window <= 0 {
		return
	}
	orgHex = strings.TrimSpace(orgHex)
	coll = strings.TrimSpace(coll)
	idHex = strings.TrimSpace(idHex)
	if orgHex == "" || coll == "" || idHex == "" {
		return
	}
	k := deferEntityKey{orgHex: orgHex, coll: coll, idHex: idHex}
	due := time.Now().Add(window)

	deferSchedMu.Lock()
	defer deferSchedMu.Unlock()
	switch kind {
	case DeferredKindReport:
		reportDeferDue[k] = due
	case DeferredKindCrmRefresh:
		crmRefDeferDue[k] = due
	case DeferredKindCrmIngest:
		crmIngDeferDue[k] = due
	}
}

// TakeDueDeferredSideEffectJobs lấy mọi job đã đến hạn và xóa khỏi lịch (gọi từ worker mỗi tick).
func TakeDueDeferredSideEffectJobs(now time.Time) []DeferredSideEffectFlushJob {
	deferSchedMu.Lock()
	defer deferSchedMu.Unlock()
	var out []DeferredSideEffectFlushJob
	collect := func(m map[deferEntityKey]time.Time, kind DeferredSideEffectKind) {
		for ek, due := range m {
			if !now.Before(due) {
				out = append(out, DeferredSideEffectFlushJob{
					Kind:   kind,
					OrgHex: ek.orgHex,
					Coll:   ek.coll,
					IDHex:  ek.idHex,
				})
				delete(m, ek)
			}
		}
	}
	collect(reportDeferDue, DeferredKindReport)
	collect(crmRefDeferDue, DeferredKindCrmRefresh)
	collect(crmIngDeferDue, DeferredKindCrmIngest)
	return out
}
