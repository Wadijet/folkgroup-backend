// Package decisionlive — Gắn W3C Trace Context (trace-id / span-id) trước khi buffer và persist.
package decisionlive

import (
	"strings"

	"meta_commerce/internal/traceutil"
)

// enrichW3CTraceContext — Điền w3cTraceId (32 hex) + spanId (16 hex) theo chuẩn W3C / OpenTelemetry.
// routeTraceKey là khóa luồng trong hệ (vd. trace_xxx hoặc đã là 32 hex) — dùng để neo ổn định tới trace-id chuẩn.
// parentSpanId: khi live bật, traceStore.append gán = spanId mốc trước cùng trace trước khi gọi hàm này; caller cũng có thể set sẵn (phân nhánh).
func enrichW3CTraceContext(ev *DecisionLiveEvent, routeTraceKey string) {
	if ev == nil {
		return
	}
	if strings.TrimSpace(ev.W3CTraceID) == "" {
		ev.W3CTraceID = traceutil.W3CTraceIDFromKey(routeTraceKey)
	}
	if strings.TrimSpace(ev.SpanID) == "" {
		ev.SpanID = traceutil.NewSpanID()
	}
}

// BackfillW3CTraceIDOnly chỉ điền w3cTraceId khi replay payload cũ (không sinh spanId mới — tránh đổi ID mỗi lần đọc).
func BackfillW3CTraceIDOnly(ev *DecisionLiveEvent) {
	if ev == nil {
		return
	}
	if strings.TrimSpace(ev.W3CTraceID) == "" && strings.TrimSpace(ev.TraceID) != "" {
		ev.W3CTraceID = traceutil.W3CTraceIDFromKey(ev.TraceID)
	}
}
