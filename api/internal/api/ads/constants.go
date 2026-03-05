// Package ads — Module cơ chế duyệt đề xuất hành động quảng cáo.
// Có thể phát triển và tách thành service độc lập.
package ads

// EventType cho notification (domain ads).
const (
	EventTypeActionPendingApproval = "ads_action_pending_approval" // Khi có đề xuất cần duyệt
	EventTypeActionExecuted       = "ads_action_executed"         // Sau khi thực thi thành công
	EventTypeActionExecutedFailed = "ads_action_executed_failed"  // Sau khi thực thi thất bại
	EventTypeActionRejected       = "ads_action_rejected"         // Khi human reject
)

// ActionType loại hành động đề xuất.
const (
	ActionTypeKill         = "KILL"
	ActionTypeIncrease     = "INCREASE"
	ActionTypeDecrease     = "DECREASE"
	ActionTypePause        = "PAUSE"
	ActionTypeResume       = "RESUME"
	ActionTypeSetBudget    = "SET_BUDGET"
	ActionTypeCircuitBreak = "CIRCUIT_BREAK_PAUSE"
)

// Status trạng thái đề xuất trong queue.
const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusRejected = "rejected"
	StatusExecuted = "executed"
	StatusFailed   = "failed"
)
