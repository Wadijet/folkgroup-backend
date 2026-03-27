// Package decisionlive — Gắn thông tin audit lên DecisionLiveEvent trước khi Publish/persist.
package decisionlive

// EnrichLiveEventFromCase gắn decisionCaseId và refs (không ghi đè khóa đã có).
func EnrichLiveEventFromCase(decisionCaseID, caseTraceID string, ev *DecisionLiveEvent) {
	if ev == nil {
		return
	}
	if decisionCaseID != "" && ev.DecisionCaseID == "" {
		ev.DecisionCaseID = decisionCaseID
	}
	if decisionCaseID == "" && caseTraceID == "" {
		return
	}
	if ev.Refs == nil {
		ev.Refs = make(map[string]string)
	}
	if decisionCaseID != "" && ev.Refs["decisionCaseId"] == "" {
		ev.Refs["decisionCaseId"] = decisionCaseID
	}
	if caseTraceID != "" && ev.Refs["caseTraceId"] == "" {
		ev.Refs["caseTraceId"] = caseTraceID
	}
}
