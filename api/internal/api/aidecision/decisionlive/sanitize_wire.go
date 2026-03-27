package decisionlive

import "math"

// SanitizeDecisionLiveEventJSON chuẩn hóa field trước khi json.Marshal (tránh lỗi NaN/Inf đóng WS).
func SanitizeDecisionLiveEventJSON(ev *DecisionLiveEvent) {
	if ev == nil {
		return
	}
	if math.IsNaN(ev.Confidence) || math.IsInf(ev.Confidence, 0) {
		ev.Confidence = 0
	}
}
