// Package models — Override matrix Context Policy theo org (bổ sung RULE_CONTEXT_POLICY_RESOLVE).
package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// DecisionContextPolicyOverride thay thế danh sách required/context cho một caseType trong tổ chức.
type DecisionContextPolicyOverride struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"compound:ctx_policy_org_case"`
	CaseType            string             `json:"caseType" bson:"caseType" index:"compound:ctx_policy_org_case"`
	Enabled             bool               `json:"enabled" bson:"enabled"`
	// RequiredContexts khi có phần tử — thay thế required từ rule seed (không rỗng mới áp dụng).
	RequiredContexts []string `json:"requiredContexts" bson:"requiredContexts"`
	OptionalContexts []string `json:"optionalContexts,omitempty" bson:"optionalContexts,omitempty"`
	Note             string   `json:"note,omitempty" bson:"note,omitempty"`
	UpdatedAt        int64    `json:"updatedAt" bson:"updatedAt"`
	CreatedAt        int64    `json:"createdAt" bson:"createdAt"`
}
