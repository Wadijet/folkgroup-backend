// Package models — Model cho module Decision Brain.
//
// Decision Brain là bộ nhớ học tập (learning memory) cho hệ thống AI Commerce.
// Lưu trữ các decision case đã hoàn thành — không phải Activity Log hay event stream.
// Case chỉ được tạo khi entity nguồn đã đóng vòng đời (completed, reviewed, executed, rejected, failed).
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// DecisionCaseSummary dùng cho query nhanh, dashboard, AI clustering.
// Outcome chi tiết nằm ở entity nguồn.
type DecisionCaseSummary struct {
	PrimaryMetric string  `json:"primaryMetric,omitempty" bson:"primaryMetric,omitempty"`
	BaselineValue float64 `json:"baselineValue,omitempty" bson:"baselineValue,omitempty"`
	FinalValue    float64 `json:"finalValue,omitempty" bson:"finalValue,omitempty"`
	Delta         float64 `json:"delta,omitempty" bson:"delta,omitempty"`
}

// DecisionCaseSystemSummary tóm tắt ngắn cho system.
type DecisionCaseSystemSummary struct {
	Title        string `json:"title,omitempty" bson:"title,omitempty"`
	ShortSummary string `json:"shortSummary,omitempty" bson:"shortSummary,omitempty"`
}

// DecisionCaseAIText cấu trúc văn bản phục vụ AI learning.
// Các field này quan trọng cho retrieval, clustering và fine-tuning.
type DecisionCaseAIText struct {
	Situation         string `json:"situation,omitempty" bson:"situation,omitempty"`
	DecisionRationale string `json:"decisionRationale,omitempty" bson:"decisionRationale,omitempty"`
	IntendedGoal      string `json:"intendedGoal,omitempty" bson:"intendedGoal,omitempty"`
	ExpectedOutcome   string `json:"expectedOutcome,omitempty" bson:"expectedOutcome,omitempty"`
	ActualOutcome     string `json:"actualOutcome,omitempty" bson:"actualOutcome,omitempty"`
	Lesson            string `json:"lesson,omitempty" bson:"lesson,omitempty"`
	NextSuggestion    string `json:"nextSuggestion,omitempty" bson:"nextSuggestion,omitempty"`
}

// DecisionCaseHumanNotes ghi chú từ con người (review, override).
type DecisionCaseHumanNotes struct {
	DecisionNote   string `json:"decisionNote,omitempty" bson:"decisionNote,omitempty"`
	ReviewNote     string `json:"reviewNote,omitempty" bson:"reviewNote,omitempty"`
	OverrideReason string `json:"overrideReason,omitempty" bson:"overrideReason,omitempty"`
	FreeNote       string `json:"freeNote,omitempty" bson:"freeNote,omitempty"`
}

// DecisionCaseText cấu trúc văn bản đầy đủ cho mỗi case.
type DecisionCaseText struct {
	SystemSummary DecisionCaseSystemSummary `json:"systemSummary,omitempty" bson:"systemSummary,omitempty"`
	AIText        DecisionCaseAIText       `json:"aiText,omitempty" bson:"aiText,omitempty"`
	HumanNotes    DecisionCaseHumanNotes   `json:"humanNotes,omitempty" bson:"humanNotes,omitempty"`
}

// SourceRef tham chiếu đến entity nguồn.
type SourceRef struct {
	RefType string `json:"refType" bson:"refType"`
	RefId   string `json:"refId" bson:"refId"`
}

// DecisionCase document lưu trong collection decision_cases.
//
// Decision Brain KHÔNG lưu: webhook events, order events, message events,
// layer updates, score updates, flag creation. Những thứ đó thuộc Activity Log,
// State Engine, Operational Entities.
//
// Các entity có thể tạo case: Action (pause campaign, reduce budget, ...),
// CIO choice (chọn kênh, lên lịch touchpoint), Content choice (chọn creative),
// Governance (approval, rejection, override).
type DecisionCase struct {
	ID                   primitive.ObjectID  `json:"id,omitempty" bson:"_id,omitempty"`
	CaseId               string              `json:"caseId" bson:"caseId" index:"unique:1"`
	CaseType             string              `json:"caseType" bson:"caseType" index:"single:1"`
	CaseCategory         string              `json:"caseCategory" bson:"caseCategory" index:"single:1"`
	Domain               string              `json:"domain" bson:"domain" index:"single:1"`
	TargetType           string              `json:"targetType" bson:"targetType" index:"single:1"`
	TargetId             string              `json:"targetId" bson:"targetId" index:"single:1"`
	SourceRef            SourceRef           `json:"sourceRef" bson:"sourceRef"`
	GoalCode             string              `json:"goalCode" bson:"goalCode" index:"single:1"`
	Result               string              `json:"result" bson:"result" index:"single:1"`
	Summary              DecisionCaseSummary  `json:"summary,omitempty" bson:"summary,omitempty"`
	Text                 DecisionCaseText    `json:"text,omitempty" bson:"text,omitempty"`
	Tags                 []string            `json:"tags,omitempty" bson:"tags,omitempty"`
	OwnerOrganizationID  primitive.ObjectID  `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`
	SourceClosedAt       int64               `json:"sourceClosedAt" bson:"sourceClosedAt" index:"single:-1"`
	CreatedAt            int64               `json:"createdAt" bson:"createdAt"`
	UpdatedAt            int64               `json:"updatedAt" bson:"updatedAt"`
}

// Kết quả case
const (
	DecisionResultSuccess  = "success"
	DecisionResultPartial  = "partial"
	DecisionResultFailed   = "failed"
	DecisionResultRejected = "rejected"
)

// Loại case
const (
	CaseTypeAction         = "action"
	CaseTypeCIOChoice      = "cio_choice"
	CaseTypeContentChoice  = "content_choice"
	CaseTypeApproval       = "approval"
)
