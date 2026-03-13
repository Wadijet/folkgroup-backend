// Package dto — DTO cho Rule Intelligence API.
package dto

// RunRuleRequest request chạy rule.
type RunRuleRequest struct {
	RuleID        string                 `json:"rule_id"`
	Domain        string                 `json:"domain"`
	EntityRef     EntityRefDTO           `json:"entity_ref"`
	Layers        map[string]interface{} `json:"layers"`
	ParamsOverride map[string]interface{} `json:"params_override,omitempty"`
}

// EntityRefDTO entity reference trong context (entity đang được rule đánh giá).
type EntityRefDTO struct {
	Domain              string `json:"domain"`
	ObjectType          string `json:"objectType"`
	ObjectID            string `json:"objectId"`
	OwnerOrganizationID string `json:"ownerOrganizationId"` // Hex ObjectID của tổ chức sở hữu entity
}
