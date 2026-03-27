// Package dto — Quy tắc routing decision_events_queue (noop | pass_through).
package dto

// RoutingRuleUpsertRequest body POST /ai-decision/routing-rules
type RoutingRuleUpsertRequest struct {
	EventType string `json:"eventType"`
	Behavior  string `json:"behavior"` // noop | pass_through
	Enabled   bool   `json:"enabled"`
	Note      string `json:"note,omitempty"`
}

// RoutingRuleItem phần tử danh sách
type RoutingRuleItem struct {
	ID                  string `json:"id"`
	OwnerOrganizationID string `json:"ownerOrganizationId"`
	EventType           string `json:"eventType"`
	Behavior            string `json:"behavior"`
	Enabled             bool   `json:"enabled"`
	Note                string `json:"note,omitempty"`
	UpdatedAt           int64  `json:"updatedAt"`
	CreatedAt           int64  `json:"createdAt"`
}
