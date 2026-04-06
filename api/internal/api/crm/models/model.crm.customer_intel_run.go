// Package models — Lưu từng lần chạy intelligence khách (lớp A); crm_customers.intel trỏ bản gần nhất (lớp B).
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	// CrmCustomerIntelScopePerCustomer — một khách cụ thể.
	CrmCustomerIntelScopePerCustomer = "per_customer"
	// CrmCustomerIntelScopeBatchJob — một job xử lý nhiều khách / toàn org (không cập nhật intel trên từng customer).
	CrmCustomerIntelScopeBatchJob = "batch_job"
)

const (
	CrmCustomerIntelStatusSuccess = "success"
	CrmCustomerIntelStatusFailed  = "failed"
)

// CrmCustomerIntelRun — một lần chạy refresh/recalculate/classification batch (audit, lịch sử).
type CrmCustomerIntelRun struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`

	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:crm_cust_intel_org_uid,compound:crm_cust_intel_org_time,order:1"`

	// UnifiedId — rỗng khi scope = batch_job (tổng hợp một job).
	UnifiedId string `json:"unifiedId,omitempty" bson:"unifiedId,omitempty" index:"single:1,compound:crm_cust_intel_org_uid,order:2"`

	Scope     string `json:"scope" bson:"scope"`         // per_customer | batch_job
	Operation string `json:"operation" bson:"operation"` // refresh | recalculate_one | recalculate_all | …
	Status    string `json:"status" bson:"status"`     // success | failed

	ComputedAt int64 `json:"computedAt" bson:"computedAt" index:"single:-1,compound:crm_cust_intel_org_time,order:-1"`

	ParentJobHex          string `json:"parentJobHex,omitempty" bson:"parentJobHex,omitempty"`
	ParentDecisionEventID string `json:"parentDecisionEventId,omitempty" bson:"parentDecisionEventId,omitempty"`
	Trigger               string `json:"trigger,omitempty" bson:"trigger,omitempty"` // crm_intel_compute_job

	OutputSummary map[string]interface{} `json:"outputSummary,omitempty" bson:"outputSummary,omitempty"`
	ErrorMessage  string                 `json:"errorMessage,omitempty" bson:"errorMessage,omitempty"`
}
