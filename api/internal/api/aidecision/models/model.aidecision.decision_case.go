// Package models — DecisionCase cho runtime AI Decision.
//
// Theo PLATFORM_L1_EVENT_DECISION_SUPPLEMENT §4.4 Schema chuẩn.
// Decision Case = đơn vị vận hành từ trigger đến outcome.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// DecisionCaseEntityRefs tham chiếu entity chính.
type DecisionCaseEntityRefs struct {
	CustomerID     string `json:"customerId,omitempty" bson:"customerId,omitempty"`
	ConversationID string `json:"conversationId,omitempty" bson:"conversationId,omitempty"`
	OrderID        string `json:"orderId,omitempty" bson:"orderId,omitempty"`
	CampaignID     string `json:"campaignId,omitempty" bson:"campaignId,omitempty"`
}

// DecisionCase document trong decision_cases_runtime.
type DecisionCase struct {
	ID                  primitive.ObjectID       `json:"id,omitempty" bson:"_id,omitempty"`
	DecisionCaseID      string                   `json:"decisionCaseId" bson:"decisionCaseId" index:"unique:1"` // dcs_xxx
	OrgID               string                   `json:"orgId" bson:"orgId" index:"single:1"`
	OwnerOrganizationID primitive.ObjectID       `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`

	RootEventID     string   `json:"rootEventId" bson:"rootEventId"`
	TriggerEventIDs []string `json:"triggerEventIds" bson:"triggerEventIds"`
	LatestEventID   string   `json:"latestEventId" bson:"latestEventId"`

	EntityRefs DecisionCaseEntityRefs `json:"entityRefs" bson:"entityRefs"`

	CaseType string `json:"caseType" bson:"caseType" index:"single:1"`
	Priority string `json:"priority" bson:"priority"` // high | normal | low
	Urgency  string `json:"urgency" bson:"urgency"`   // realtime | near_realtime | deferred

	// TraceID / CorrelationID — neo case ↔ luồng queue / rule logs; merge chỉ ghi khi field đang trống.
	TraceID       string `json:"traceId,omitempty" bson:"traceId,omitempty" index:"single:1,sparse"`
	CorrelationID string `json:"correlationId,omitempty" bson:"correlationId,omitempty" index:"single:1,sparse"`

	Status string `json:"status" bson:"status" index:"single:1"`

	RequiredContexts []string               `json:"requiredContexts" bson:"requiredContexts"`
	ReceivedContexts []string               `json:"receivedContexts" bson:"receivedContexts"`
	ContextPackets   map[string]interface{}  `json:"contextPackets" bson:"contextPackets"`

	DecisionPacket interface{} `json:"decisionPacket,omitempty" bson:"decisionPacket,omitempty"`

	ActionIDs    []string `json:"actionIds" bson:"actionIds"`
	ExecutionIDs []string `json:"executionIds" bson:"executionIds"`

	OutcomeSummary interface{} `json:"outcomeSummary,omitempty" bson:"outcomeSummary,omitempty"`
	ClosureType   string      `json:"closureType,omitempty" bson:"closureType,omitempty"` // closed_* — xem hằng Closure*

	OpenedAt int64  `json:"openedAt" bson:"openedAt" index:"single:1"`
	ClosedAt *int64 `json:"closedAt,omitempty" bson:"closedAt,omitempty"`

	CreatedAt int64 `json:"createdAt" bson:"createdAt"`
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"`

	// LastAdsContextRequestedAt — ms, lần cuối emit ads.context_requested; dùng cooldown tránh nhân job queue khi campaign_intel_recomputed / meta_campaign.updated dồn dập.
	LastAdsContextRequestedAt int64 `json:"lastAdsContextRequestedAt,omitempty" bson:"lastAdsContextRequestedAt,omitempty"`
}

// Case status constants
const (
	CaseStatusOpened             = "opened"
	CaseStatusContextCollecting  = "context_collecting"
	CaseStatusReadyForDecision   = "ready_for_decision"
	CaseStatusDecided            = "decided"
	CaseStatusActionsCreated     = "actions_created"
	CaseStatusExecuting          = "executing"
	CaseStatusOutcomeWaiting     = "outcome_waiting"
	CaseStatusClosed             = "closed"
)

// Closure type constants
const (
	ClosureProposed = "closed_proposed" // Case đã tạo xong proposals — Executor quản lý actions, case không chờ outcome
	ClosureComplete = "closed_complete"
	ClosureTimeout  = "closed_timeout"
	ClosureManual   = "closed_manual"
	// ClosureIncomplete — thiếu dữ liệu đầu vào hoặc không đủ điều kiện đánh giá (không phải lỗi rule).
	ClosureIncomplete = "closed_incomplete"
	// ClosureNoAction — đã đánh giá rule nhưng không có hành động đề xuất (nghiệp vụ: không đạt ngưỡng / không có cờ).
	ClosureNoAction = "closed_no_action"
	// ClosureFailed — lỗi kỹ thuật hoặc pipeline không hoàn tất (persist/emit/đọc DB).
	ClosureFailed = "closed_failed"
)

// Case type constants
const (
	CaseTypeConversationResponse = "conversation_response_decision"
	CaseTypeCustomerState        = "customer_state_decision"
	CaseTypeOrderRisk            = "order_risk_decision"
	CaseTypeAdsOptimization      = "ads_optimization_decision"
	CaseTypeExecutionRecovery   = "execution_recovery_decision"
)
