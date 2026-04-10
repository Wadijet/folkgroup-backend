package decisionlive

import "math"

// SanitizeDecisionLiveEventJSON — Trước khi gửi từng DecisionLiveEvent qua WS (replay hoặc stream): chuẩn hóa số (tránh NaN/Inf làm hỏng JSON).
func SanitizeDecisionLiveEventJSON(ev *DecisionLiveEvent) {
	if ev == nil {
		return
	}
	if math.IsNaN(ev.Confidence) || math.IsInf(ev.Confidence, 0) {
		ev.Confidence = 0
	}
}
