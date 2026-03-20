// Package approval — ApprovalModeConfig model cho Vision 08.
package approval

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ApprovalModeConfig cấu hình mode duyệt theo domain/scope (Vision 08).
// Thay thế logic rải rác trong ads_meta_config, CIX_APPROVAL_ACTIONS env.
type ApprovalModeConfig struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1;compound:approval_mode_lookup"`
	Domain              string             `json:"domain" bson:"domain" index:"single:1;compound:approval_mode_lookup"`   // ads | cix | cio
	ScopeKey            string             `json:"scopeKey" bson:"scopeKey" index:"single:1;compound:approval_mode_lookup"` // adAccountId, planId, "" (default)
	Mode                string             `json:"mode" bson:"mode"`                                                       // manual_required | auto_by_rule | fully_auto
	ActionOverrides     map[string]string  `json:"actionOverrides,omitempty" bson:"actionOverrides,omitempty"`           // actionType -> mode
}

// Mode constants — Approval modes theo Vision 08.
const (
	ApprovalModeManualRequired = "manual_required"
	ApprovalModeAutoByRule     = "auto_by_rule"
	ApprovalModeFullyAuto      = "fully_auto"
)
