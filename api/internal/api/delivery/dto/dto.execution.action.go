// Package deliverydto — DTO cho domain Delivery.
//
// dto.execution.action.go: Action Contract — input chuẩn cho Execution Engine.
// Theo docs-shared/architecture/foundational/data-contract.md §3.
// AI Decision Engine tạo → Execution Engine consume.
package deliverydto

// ExecutionActionTarget đích thực thi (customer, channel, ...).
type ExecutionActionTarget struct {
	CustomerID string `json:"customerId"` // cust_xxx — ID chuẩn customer
	Channel    string `json:"channel"`   // zalo | messenger | sms | website_chat | telegram
	// Mở rộng: OrderID, CampaignID, AdsetID, ... tùy action_type
}

// ExecutionActionInput input chuẩn cho Execution Engine.
// Contract: AI Decision Engine → Execution Engine.
type ExecutionActionInput struct {
	ActionID       string                 `json:"actionId"`       // act_xxx — định danh action
	ActionType     string                 `json:"actionType"`     // SEND_MESSAGE | PAUSE_ADSET | UPDATE_AD | TAG_CUSTOMER | ASSIGN_TO_AGENT | ...
	Target         ExecutionActionTarget  `json:"target"`
	Payload        map[string]interface{} `json:"payload,omitempty"`
	Priority       string                 `json:"priority,omitempty"`       // high | medium | low
	IdempotencyKey string                 `json:"idempotencyKey,omitempty"` // act_xxx_20260312_001 — tránh duplicate
	Source         string                 `json:"source,omitempty"`        // APPROVAL_GATE | AI_DECISION_ENGINE | MANUAL | RULE_ENGINE
	TraceID        string                 `json:"traceId,omitempty"`       // trace_xxx — trace end-to-end
	CorrelationID  string                 `json:"correlationId,omitempty"`  // corr_xxx — liên session + customer + order
	DecisionID     string                 `json:"decisionId,omitempty"`     // dec_xxx — decision đã ra
}

// ActionType constants — các loại action Execution Engine hỗ trợ (vision).
const (
	ActionTypeSendMessage    = "SEND_MESSAGE"
	ActionTypePauseAdset     = "PAUSE_ADSET"
	ActionTypeUpdateAd       = "UPDATE_AD"
	ActionTypeCreateOrder    = "CREATE_ORDER"
	ActionTypeAssignToAgent  = "ASSIGN_TO_AGENT"
	ActionTypeTagCustomer    = "TAG_CUSTOMER"
	ActionTypePublishContent = "PUBLISH_CONTENT"
	ActionTypeScheduleTask   = "SCHEDULE_TASK"
)

// SourceApprovalGate — action phải có source này khi gọi Delivery qua HTTP (Vision 08).
const SourceApprovalGate = "APPROVAL_GATE"
