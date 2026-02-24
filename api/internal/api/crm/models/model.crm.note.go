// Package models - CrmNote thuộc domain CRM (crm_notes).
// Ghi chú khách hàng (soft delete).
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CrmNote lưu ghi chú khách (crm_notes).
type CrmNote struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`

	CustomerId          string             `json:"customerId" bson:"customerId" index:"single:1,compound:crm_note_org_customer_created"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:crm_note_org_customer_created"`
	NoteText            string             `json:"noteText" bson:"noteText"`
	NextAction          string             `json:"nextAction,omitempty" bson:"nextAction,omitempty"`
	NextActionDate      int64              `json:"nextActionDate,omitempty" bson:"nextActionDate,omitempty"`
	CreatedBy           primitive.ObjectID `json:"createdBy" bson:"createdBy"`
	IsDeleted           bool               `json:"isDeleted" bson:"isDeleted"`

	CreatedAt int64 `json:"createdAt" bson:"createdAt" index:"single:-1,compound:crm_note_org_customer_created"`
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"`
}
