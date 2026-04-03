// Package models — Job trong collection cix_intel_compute (Raw→L1→L2→L3→Flag→Action, worker domain CIX).
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CixIntelComputeJob document trong cix_intel_compute — cùng quy ước {domain}_intel_compute với CRM/Ads/Order.
type CixIntelComputeJob struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	ConversationID      string             `json:"conversationId" bson:"conversationId" index:"single:1"`
	CustomerID          string             `json:"customerId" bson:"customerId"`
	Channel             string             `json:"channel" bson:"channel"`
	CioEventUid         string             `json:"cioEventUid" bson:"cioEventUid"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:cix_intel_compute_poll"`
	ProcessedAt         *int64             `json:"processedAt,omitempty" bson:"processedAt,omitempty" index:"single:1,compound:cix_intel_compute_poll"`
	ProcessError        string             `json:"processError,omitempty" bson:"processError,omitempty"`
	RetryCount          int                `json:"retryCount" bson:"retryCount"`
	CreatedAt           int64              `json:"createdAt" bson:"createdAt" index:"single:-1,compound:cix_intel_compute_poll,order:1"`
}
