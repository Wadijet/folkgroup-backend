// Package models — Model cho module Learning engine.
//
// Learning engine là bộ nhớ học tập (learning memory) cho hệ thống AI Commerce.
// Schema theo vision 11 - learning-engine.md. Case chỉ tạo khi entity đóng vòng đời.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// LearningOutcomeTechnical kết quả kỹ thuật (API, delivery).
type LearningOutcomeTechnical struct {
	Status   string `json:"status,omitempty" bson:"status,omitempty"`     // success | fail
	Delivery string `json:"delivery,omitempty" bson:"delivery,omitempty"` // delivered | failed
	LatencyMs int64 `json:"latencyMs,omitempty" bson:"latencyMs,omitempty"`
	Error    string `json:"error,omitempty" bson:"error,omitempty"`
}

// LearningOutcomeBusiness kết quả nghiệp vụ (convert, revenue).
type LearningOutcomeBusiness struct {
	CustomerPurchased bool    `json:"customerPurchased,omitempty" bson:"customerPurchased,omitempty"`
	OrderID           string  `json:"orderId,omitempty" bson:"orderId,omitempty"`
	Revenue           float64 `json:"revenue,omitempty" bson:"revenue,omitempty"`
}

// LearningOutcome outcome technical + business.
type LearningOutcome struct {
	Technical  LearningOutcomeTechnical `json:"technical,omitempty" bson:"technical,omitempty"`
	Business   LearningOutcomeBusiness `json:"business,omitempty" bson:"business,omitempty"`
	Direct     bool                    `json:"direct,omitempty" bson:"direct,omitempty"`
	RecordedAt string                  `json:"recordedAt,omitempty" bson:"recordedAt,omitempty"`
}

// LearningEvaluation tính từ outcome (Evaluation Job fill).
type LearningEvaluation struct {
	OutcomeClass     string  `json:"outcomeClass,omitempty" bson:"outcomeClass,omitempty"` // success | partial | fail | delayed
	ErrorAttribution string  `json:"errorAttribution,omitempty" bson:"errorAttribution,omitempty"`
	PrimaryMetric   string  `json:"primaryMetric,omitempty" bson:"primaryMetric,omitempty"`
	BaselineValue   float64 `json:"baselineValue,omitempty" bson:"baselineValue,omitempty"`
	FinalValue      float64 `json:"finalValue,omitempty" bson:"finalValue,omitempty"`
	Delta            float64 `json:"delta,omitempty" bson:"delta,omitempty"`
}

// LearningLearning sinh từ Evaluation (Learning Job fill).
type LearningLearning struct {
	ParamSuggestions []map[string]interface{} `json:"paramSuggestions,omitempty" bson:"paramSuggestions,omitempty"`
	RuleCandidate   interface{}              `json:"ruleCandidate,omitempty" bson:"ruleCandidate,omitempty"`
	StrategyInsight string                   `json:"strategyInsight,omitempty" bson:"strategyInsight,omitempty"`
}

// RuleAppliedEntry một rule đã chạy (từ rule_execution_logs).
type RuleAppliedEntry struct {
	RuleID       string `json:"ruleId,omitempty" bson:"ruleId,omitempty"`
	LogicVersion int    `json:"logicVersion,omitempty" bson:"logicVersion,omitempty"`
	Output       string `json:"output,omitempty" bson:"output,omitempty"`
}

// LearningCase document lưu trong learning_cases — schema vision 11.
type LearningCase struct {
	ID                   primitive.ObjectID   `json:"id,omitempty" bson:"_id,omitempty"`
	CaseId               string               `json:"caseId,omitempty" bson:"caseId,omitempty"` // dc_xxx — cho manual create, builder sinh từ sourceRefId
	DecisionID           string               `json:"decisionId,omitempty" bson:"decisionId,omitempty"`
	EntityType           string               `json:"entityType" bson:"entityType"`     // session | campaign | action_pending | touchpoint_plan
	EntityID             string               `json:"entityId" bson:"entityId"`
	ContextSnapshot      map[string]interface{} `json:"contextSnapshot,omitempty" bson:"contextSnapshot,omitempty"`
	InputSignals         map[string]interface{} `json:"inputSignals,omitempty" bson:"inputSignals,omitempty"`
	RulesApplied         []RuleAppliedEntry   `json:"rulesApplied,omitempty" bson:"rulesApplied,omitempty"`
	ParamVersion         string               `json:"paramVersion,omitempty" bson:"paramVersion,omitempty"`
	Decision             map[string]interface{} `json:"decision,omitempty" bson:"decision,omitempty"`
	ActionExecuted       map[string]interface{} `json:"actionExecuted,omitempty" bson:"actionExecuted,omitempty"`
	ExecutionTraceID     string               `json:"executionTraceId,omitempty" bson:"executionTraceId,omitempty"`
	Outcome              LearningOutcome      `json:"outcome,omitempty" bson:"outcome,omitempty"`
	Evaluation           LearningEvaluation   `json:"evaluation,omitempty" bson:"evaluation,omitempty"`
	Learning             LearningLearning     `json:"learning,omitempty" bson:"learning,omitempty"`
	OwnerOrganizationID  primitive.ObjectID   `json:"ownerOrganizationId" bson:"ownerOrganizationId"`
	SourceRefType        string               `json:"sourceRefType,omitempty" bson:"sourceRefType,omitempty"` // action_pending
	SourceRefID          string               `json:"sourceRefId,omitempty" bson:"sourceRefId,omitempty"`
	Domain               string               `json:"domain" bson:"domain"` // cix | ads | cio
	ActionType           string               `json:"actionType" bson:"actionType"`
	Result               string               `json:"result" bson:"result"` // success | partial | failed | rejected
	CreatedAt            int64                `json:"createdAt" bson:"createdAt"`
	ClosedAt             int64                `json:"closedAt" bson:"closedAt"`

	// Legacy fields cho API list/filter (map từ entityType, entityId, actionType)
	CaseType     string `json:"caseType,omitempty" bson:"caseType,omitempty"`
	CaseCategory string `json:"caseCategory,omitempty" bson:"caseCategory,omitempty"`
	GoalCode     string `json:"goalCode,omitempty" bson:"goalCode,omitempty"`
	TargetType   string `json:"targetType,omitempty" bson:"targetType,omitempty"`
	TargetId     string `json:"targetId,omitempty" bson:"targetId,omitempty"`
}

// Entity type constants
const (
	EntityTypeSession        = "session"
	EntityTypeCampaign       = "campaign"
	EntityTypeActionPending  = "action_pending"
	EntityTypeTouchpointPlan = "touchpoint_plan"
)

// Result constants
const (
	LearningResultSuccess  = "success"
	LearningResultPartial  = "partial"
	LearningResultFailed   = "failed"
	LearningResultRejected = "rejected"
)
