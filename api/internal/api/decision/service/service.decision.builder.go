// Package decisionsvc — Builder chuyển entity nguồn thành DecisionCase.
//
// Builder đọc dữ liệu entity, extract field liên quan, tạo summary và AI text,
// rồi tạo decision case. Chỉ gọi khi entity đã đóng vòng đời.
package decisionsvc

import (
	"fmt"
	"time"

	pkgapproval "meta_commerce/pkg/approval"

	"meta_commerce/internal/api/decision/models"
)

// BuildDecisionCaseFromAction chuyển ActionPending (executed/rejected/failed) thành DecisionCase.
//
// ActionPending là entity nguồn cho action (pause campaign, reduce budget, ...) và governance (approval/rejection).
// Chỉ gọi khi status = executed | rejected | failed.
func BuildDecisionCaseFromAction(ap *pkgapproval.ActionPending) (*models.DecisionCase, error) {
	if ap == nil {
		return nil, fmt.Errorf("ActionPending không được nil")
	}
	// Chỉ build khi lifecycle đã đóng
	switch ap.Status {
	case pkgapproval.StatusExecuted, pkgapproval.StatusRejected, pkgapproval.StatusFailed:
		// OK
	default:
		return nil, fmt.Errorf("ActionPending chưa đóng vòng đời (status=%s), không tạo decision case", ap.Status)
	}

	caseType := models.CaseTypeAction
	if ap.Status == pkgapproval.StatusRejected {
		caseType = models.CaseTypeApproval
	}

	result := models.DecisionResultSuccess
	if ap.Status == pkgapproval.StatusRejected {
		result = models.DecisionResultRejected
	} else if ap.Status == pkgapproval.StatusFailed {
		result = models.DecisionResultFailed
	}

	sourceClosedAt := ap.ExecutedAt
	if ap.Status == pkgapproval.StatusRejected {
		sourceClosedAt = ap.RejectedAt
	}
	if sourceClosedAt == 0 {
		sourceClosedAt = time.Now().Unix()
	}

	// Extract target từ payload nếu có
	targetType := ""
	targetId := ""
	if ap.Payload != nil {
		if t, ok := ap.Payload["targetType"].(string); ok {
			targetType = t
		}
		if t, ok := ap.Payload["targetId"].(string); ok {
			targetId = t
		}
		if targetType == "" && ap.Domain == "ads" {
			targetType = "campaign"
			if cid, ok := ap.Payload["campaignId"].(string); ok {
				targetId = cid
			}
		}
	}

	caseId := fmt.Sprintf("dc_%s_%d", ap.ID.Hex()[:8], sourceClosedAt)

	dc := &models.DecisionCase{
		CaseId:              caseId,
		CaseType:             caseType,
		CaseCategory:         ap.Domain,
		Domain:               ap.Domain,
		TargetType:           targetType,
		TargetId:             targetId,
		SourceRef:            models.SourceRef{RefType: "action_pending", RefId: ap.ID.Hex()},
		GoalCode:             ap.ActionType,
		Result:               result,
		OwnerOrganizationID:  ap.OwnerOrganizationID,
		SourceClosedAt:       sourceClosedAt * 1000, // chuyển sang ms
		Text: models.DecisionCaseText{
			SystemSummary: models.DecisionCaseSystemSummary{
				Title:        fmt.Sprintf("%s - %s", ap.ActionType, ap.Domain),
				ShortSummary: ap.Reason,
			},
			AIText: models.DecisionCaseAIText{
				Situation:         ap.Reason,
				DecisionRationale: extractString(ap.Payload, "reason", "rationale"),
				IntendedGoal:      ap.ActionType,
				ExpectedOutcome:   extractString(ap.Payload, "expectedOutcome", "expected"),
				ActualOutcome:    buildActualOutcome(ap),
				Lesson:           buildLesson(ap),
			},
			HumanNotes: models.DecisionCaseHumanNotes{
				DecisionNote: ap.DecisionNote,
			},
		},
		Tags: []string{ap.Domain, ap.ActionType, ap.Status},
	}

	// Summary từ ExecuteResponse nếu có
	if ap.ExecuteResponse != nil {
		if pm, ok := ap.ExecuteResponse["primaryMetric"].(string); ok {
			dc.Summary.PrimaryMetric = pm
		}
		if bv, ok := toFloat64(ap.ExecuteResponse["baselineValue"]); ok {
			dc.Summary.BaselineValue = bv
		}
		if fv, ok := toFloat64(ap.ExecuteResponse["finalValue"]); ok {
			dc.Summary.FinalValue = fv
		}
		if dc.Summary.BaselineValue != 0 || dc.Summary.FinalValue != 0 {
			dc.Summary.Delta = dc.Summary.FinalValue - dc.Summary.BaselineValue
		}
	}

	return dc, nil
}

// BuildDecisionCaseFromCIOChoice placeholder — CIO choice entity chưa có.
func BuildDecisionCaseFromCIOChoice(_ interface{}) (*models.DecisionCase, error) {
	return nil, fmt.Errorf("BuildDecisionCaseFromCIOChoice: entity CIO choice chưa triển khai")
}

// BuildDecisionCaseFromContentChoice placeholder — Content choice entity chưa có.
func BuildDecisionCaseFromContentChoice(_ interface{}) (*models.DecisionCase, error) {
	return nil, fmt.Errorf("BuildDecisionCaseFromContentChoice: entity Content choice chưa triển khai")
}

// BuildDecisionCaseFromApproval alias cho BuildDecisionCaseFromAction khi focus vào governance.
// ActionPending với status rejected/approved (sau execute) đều dùng BuildDecisionCaseFromAction.
func BuildDecisionCaseFromApproval(ap *pkgapproval.ActionPending) (*models.DecisionCase, error) {
	return BuildDecisionCaseFromAction(ap)
}

func extractString(m map[string]interface{}, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func toFloat64(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	default:
		return 0, false
	}
}

func buildActualOutcome(ap *pkgapproval.ActionPending) string {
	if ap.Status == pkgapproval.StatusRejected {
		return "Từ chối: " + ap.DecisionNote
	}
	if ap.Status == pkgapproval.StatusFailed {
		return "Thất bại: " + ap.ExecuteError
	}
	if ap.ExecuteResponse != nil {
		if msg, ok := ap.ExecuteResponse["message"].(string); ok {
			return msg
		}
	}
	return "Đã thực thi thành công"
}

func buildLesson(ap *pkgapproval.ActionPending) string {
	if ap.Status == pkgapproval.StatusRejected && ap.DecisionNote != "" {
		return "Lý do từ chối: " + ap.DecisionNote
	}
	if ap.Status == pkgapproval.StatusFailed && ap.ExecuteError != "" {
		return "Lỗi cần tránh: " + ap.ExecuteError
	}
	return ""
}
