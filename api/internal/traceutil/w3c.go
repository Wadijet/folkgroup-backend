// Package traceutil — Định danh trace theo W3C Trace Context / OpenTelemetry (độ dài cố định).
//
// Tham chiếu: https://www.w3.org/TR/trace-context/ — trace-id 32 ký tự hex (128-bit), span-id 16 ký tự hex (64-bit).
package traceutil

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const (
	// TraceIDHexLen độ dài trace-id chuẩn W3C (ký tự hex).
	TraceIDHexLen = 32
	// SpanIDHexLen độ dài span-id chuẩn W3C.
	SpanIDHexLen = 16
)

// NewTraceID sinh trace-id ngẫu nhiên 32 ký tự hex (chữ thường).
func NewTraceID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback: deterministic từ thời gian + entropy yếu — không nên xảy ra thường xuyên.
		s := sha256.Sum256([]byte("traceutil.NewTraceID.fallback"))
		copy(b[:], s[:16])
	}
	return hex.EncodeToString(b[:])
}

// NewSpanID sinh span-id ngẫu nhiên 16 ký tự hex (chữ thường).
func NewSpanID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		s := sha256.Sum256([]byte("traceutil.NewSpanID.fallback"))
		copy(b[:], s[8:16])
	}
	return hex.EncodeToString(b[:])
}

// IsValidTraceID kiểm tra định dạng trace-id W3C (đúng 32 hex, không rỗng).
func IsValidTraceID(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	if len(s) != TraceIDHexLen {
		return false
	}
	for _, c := range s {
		if c >= '0' && c <= '9' || c >= 'a' && c <= 'f' {
			continue
		}
		return false
	}
	return true
}

// IsValidSpanID kiểm tra định dạng span-id W3C (đúng 16 hex).
func IsValidSpanID(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	if len(s) != SpanIDHexLen {
		return false
	}
	for _, c := range s {
		if c >= '0' && c <= '9' || c >= 'a' && c <= 'f' {
			continue
		}
		return false
	}
	return true
}

// TraceParentValue giá trị header `traceparent` (W3C) — dạng: {version}-{trace-id}-{span-id}-{flags}.
// version = 00; flags = 00 (không sample) hoặc 01 (sampled) theo quy ước đơn giản.
func TraceParentValue(w3cTraceID, spanID string, sampled bool) string {
	const ver = "00"
	tid := strings.TrimSpace(strings.ToLower(w3cTraceID))
	sid := strings.TrimSpace(strings.ToLower(spanID))
	flags := "00"
	if sampled {
		flags = "01"
	}
	return ver + "-" + tid + "-" + sid + "-" + flags
}

// W3CTraceIDFromKey neo ổn định từ khóa nội bộ (vd. trace_xxx của data contract) sang trace-id 32 hex.
// Nếu key đã là 32 hex hợp lệ — trả về chữ thường; nếu không — băm SHA-256 rồi lấy 16 byte đầu (32 hex).
func W3CTraceIDFromKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return NewTraceID()
	}
	if IsValidTraceID(key) {
		return strings.ToLower(key)
	}
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:16])
}
