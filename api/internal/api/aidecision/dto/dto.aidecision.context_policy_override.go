// Package dto — Override Context Policy Matrix theo org.
package dto

// ContextPolicyOverrideUpsertRequest body POST /ai-decision/context-policy-overrides
type ContextPolicyOverrideUpsertRequest struct {
	CaseType         string   `json:"caseType"`
	Enabled          bool     `json:"enabled"`
	RequiredContexts []string `json:"requiredContexts"`
	OptionalContexts []string `json:"optionalContexts,omitempty"`
	Note             string   `json:"note,omitempty"`
}

// ContextPolicyOverrideItem phần tử danh sách
type ContextPolicyOverrideItem struct {
	ID                  string   `json:"id"`
	OwnerOrganizationID string   `json:"ownerOrganizationId"`
	CaseType            string   `json:"caseType"`
	Enabled             bool     `json:"enabled"`
	RequiredContexts    []string `json:"requiredContexts"`
	OptionalContexts    []string `json:"optionalContexts,omitempty"`
	Note                string   `json:"note,omitempty"`
	UpdatedAt           int64    `json:"updatedAt"`
	CreatedAt           int64    `json:"createdAt"`
}
