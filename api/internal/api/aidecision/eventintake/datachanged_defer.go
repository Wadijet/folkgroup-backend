// Package eventintake — Hàng đợi trì hoãn side-effect datachanged (trailing debounce).
//
// Mục tiêu debounce (gom yêu cầu tính toán / side-effect):
// Nhiều lần thay đổi dữ liệu gần nhau có thể cùng dẫn tới một loại việc (vd. tính lại intelligence khách, xếp lại job order/CIX…).
// Thay vì chuyển xuống domain từng lần, ta gom theo khóa (org + collection + id bản ghi nguồn): mỗi lần có event mới
// thì lùi deadline thêm một cửa sổ (trailing). Khi qua cửa sổ mà không còn cập nhật, chỉ flush một lần —
// tức tối đa một yêu cầu tương ứng cho khóa đó, tránh tính toán lặp sát nhau không cần thiết.
// Cửa sổ cụ thể (vd. vài phút cho CRM refresh) do hằng số / rule / env quyết định; «gấp» dùng window=0 và không ghi vào map này.
//
// Khi RULE_DATACHANGED_SIDE_EFFECT_POLICY chạy thành công: số giây do script + PARAM_DATACHANGED_SIDE_EFFECT_POLICY quyết định (ưu tiên hơn env tier).
// Fallback: phân tầng datachanged_business.go + env:
//   - AI_DECISION_BUSINESS_DEFER_OPERATIONAL_SEC — cửa sổ gom cho mức “vận hành” (mặc định 90).
//   - AI_DECISION_BUSINESS_DEFER_BACKGROUND_SEC — cửa sổ gom cho mức “nền” (mặc định 300).
//   - AI_DECISION_DEFER_REPORT_SEC / DEFER_CRM_REFRESH_SEC / DEFER_CRM_MERGE_QUEUE_SEC (hoặc legacy DEFER_CRM_INGEST_SEC) — nếu **đặt trong env**
//     (kể cả =0) thì **ghi đè** chỉ kênh đó theo từng giá trị; không đặt thì dùng số theo mức nghiệp vụ.
//
// Mỗi lần có datachanged mới cho cùng (org, collection, id), deadline trailing = now + window.
//
// Lưu trữ: MongoDB decision_trailing_debounce (bucket datachanged_defer).
package eventintake

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"
)

// DeferChannel kênh side-effect (mỗi kênh có thể có override env riêng).
type DeferChannel int

const (
	DeferChannelCrmMergeQueue DeferChannel = iota
	DeferChannelReport
	DeferChannelCRMRefresh
)

// DeferredSideEffectKind loại side-effect được lên lịch flush sau.
type DeferredSideEffectKind string

const (
	DeferredKindReport            DeferredSideEffectKind = "report"
	DeferredKindCrmRefresh        DeferredSideEffectKind = "crm_refresh"
	DeferredKindCrmMergeQueue     DeferredSideEffectKind = "crm_merge_queue"
	DeferredKindOrderIntelCompute DeferredSideEffectKind = "order_job_intel"
	DeferredKindCixIntelCompute   DeferredSideEffectKind = "cix_job_intel"
)

// DeferredSideEffectFlushJob một việc đến hạn cần chạy trong worker (đọc lại Mongo rồi gọi report/ingest/refresh).
type DeferredSideEffectFlushJob struct {
	Kind   DeferredSideEffectKind
	OrgHex string
	Coll   string
	IDHex  string
	// TraceID / CorrelationID — gom theo khóa defer (lần Schedule gần nhất có trace được giữ; flush xóa document).
	TraceID       string
	CorrelationID string
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
	case DeferChannelCrmMergeQueue:
		if raw, ok := os.LookupEnv("AI_DECISION_DEFER_CRM_MERGE_QUEUE_SEC"); ok {
			s := strings.TrimSpace(raw)
			if s == "" {
				return 0
			}
			n, err := strconv.Atoi(s)
			if err != nil || n < 0 {
				return 0
			}
			if n <= 0 {
				return 0
			}
			return time.Duration(n) * time.Second
		}
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

// ScheduleDeferredSideEffect — Ghi nhận việc cần làm sau; trailing: mỗi lần gọi lại với cùng khóa → due = now + window (lùi hạn).
// Đến hạn, consumer gọi TakeDueDeferredSideEffectJobs rồi flush đúng một job cho khóa đó (document đã xóa bởi FindOneAndDelete).
// traceID / correlationID: từ decision event datachanged — merge theo khóa (giữ trace mới nhất nếu có).
func ScheduleDeferredSideEffect(ctx context.Context, kind DeferredSideEffectKind, orgHex, coll, idHex string, window time.Duration, traceID, correlationID string) error {
	if window <= 0 {
		return nil
	}
	return upsertTrailingDatachangedDefer(ctx, kind, orgHex, coll, idHex, window, traceID, correlationID)
}

// TakeDueDeferredSideEffectJobs lấy mọi job đã đến hạn (đã xóa document trong Mongo).
func TakeDueDeferredSideEffectJobs(ctx context.Context, now time.Time) ([]DeferredSideEffectFlushJob, error) {
	docs, err := takeDueTrailingDocs(ctx, now, trailingBucketDatachangedDefer)
	if err != nil || len(docs) == 0 {
		return nil, err
	}
	out := make([]DeferredSideEffectFlushJob, 0, len(docs))
	for _, d := range docs {
		k := DeferredSideEffectKind(strings.TrimSpace(d.DeferKind))
		if k == "" {
			continue
		}
		out = append(out, DeferredSideEffectFlushJob{
			Kind:          k,
			OrgHex:        strings.TrimSpace(d.OrgHex),
			Coll:          strings.TrimSpace(d.SourceColl),
			IDHex:         strings.TrimSpace(d.IDHex),
			TraceID:       strings.TrimSpace(d.TraceID),
			CorrelationID: strings.TrimSpace(d.CorrelationID),
		})
	}
	return out, nil
}
