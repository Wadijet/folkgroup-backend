// Package models — CixPendingAnalysis cho hàng đợi phân tích.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CixPendingAnalysis document trong cix_pending_analysis — queue job phân tích.
type CixPendingAnalysis struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	ConversationID      string             `json:"conversationId" bson:"conversationId" index:"single:1"`
	CustomerID          string             `json:"customerId" bson:"customerId"`           // ID từ kênh (fb customerId)
	Channel             string             `json:"channel" bson:"channel"`                // messenger | zalo
	CioEventUid         string             `json:"cioEventUid" bson:"cioEventUid"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:cix_pending_worker"`
	ProcessedAt         *int64             `json:"processedAt,omitempty" bson:"processedAt,omitempty" index:"single:1,compound:cix_pending_worker"` // null = chưa xử lý
	ProcessError        string             `json:"processError,omitempty" bson:"processError,omitempty"`
	RetryCount          int                `json:"retryCount" bson:"retryCount"`
	CreatedAt           int64              `json:"createdAt" bson:"createdAt" index:"single:-1,compound:cix_pending_worker,order:1"`
}
