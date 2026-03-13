// Package dto — DTO cho module Decision Brain.
package dto

// DecisionCaseCreateInput input tạo decision case (thường dùng từ builder, ít khi từ API).
type DecisionCaseCreateInput struct {
	CaseId              string  `json:"caseId"`
	CaseType            string  `json:"caseType"`
	CaseCategory        string  `json:"caseCategory"`
	Domain              string  `json:"domain"`
	TargetType          string  `json:"targetType"`
	TargetId            string  `json:"targetId"`
	SourceRefType       string  `json:"sourceRefType"`
	SourceRefId         string  `json:"sourceRefId"`
	GoalCode            string  `json:"goalCode"`
	Result              string  `json:"result"`
	SummaryPrimaryMetric string  `json:"summaryPrimaryMetric,omitempty"`
	SummaryBaselineValue float64 `json:"summaryBaselineValue,omitempty"`
	SummaryFinalValue   float64 `json:"summaryFinalValue,omitempty"`
	SummaryDelta        float64 `json:"summaryDelta,omitempty"`
	TextTitle           string  `json:"textTitle,omitempty"`
	TextShortSummary    string  `json:"textShortSummary,omitempty"`
	TextSituation       string  `json:"textSituation,omitempty"`
	TextDecisionRationale string `json:"textDecisionRationale,omitempty"`
	TextIntendedGoal    string  `json:"textIntendedGoal,omitempty"`
	TextExpectedOutcome string  `json:"textExpectedOutcome,omitempty"`
	TextActualOutcome   string  `json:"textActualOutcome,omitempty"`
	TextLesson          string  `json:"textLesson,omitempty"`
	TextNextSuggestion  string  `json:"textNextSuggestion,omitempty"`
	TextDecisionNote    string  `json:"textDecisionNote,omitempty"`
	TextReviewNote      string  `json:"textReviewNote,omitempty"`
	TextOverrideReason  string  `json:"textOverrideReason,omitempty"`
	TextFreeNote        string  `json:"textFreeNote,omitempty"`
	Tags                []string `json:"tags,omitempty"`
	SourceClosedAt      int64   `json:"sourceClosedAt"`
}

// DecisionCaseListFilter filter cho ListDecisionCases.
type DecisionCaseListFilter struct {
	Domain       string `query:"domain"`
	CaseType     string `query:"caseType"`
	CaseCategory string `query:"caseCategory"`
	GoalCode     string `query:"goalCode"`
	Result       string `query:"result"`
	TargetType   string `query:"targetType"`
	TargetId     string `query:"targetId"`
	Limit        int    `query:"limit"`
	Page         int    `query:"page"`
	SortField    string `query:"sortField"`
	SortOrder    int    `query:"sortOrder"`
}
