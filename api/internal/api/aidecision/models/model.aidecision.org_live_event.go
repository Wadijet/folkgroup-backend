// Package models — Bản ghi live org (replay GET đa replica); mỗi Publish = một dòng (audit/UI + payload đầy đủ).
package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// AIDecisionOrgLiveEvent schema docSchemaVersion>=2: trường phẳng phục vụ query/UI; payload = JSON DecisionLiveEvent đầy đủ.
type AIDecisionOrgLiveEvent struct {
	ID                  primitive.ObjectID `bson:"_id,omitempty"`
	OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId" index:"compound:decision_org_live_org_created"`
	CreatedAt           int64              `bson:"createdAt" index:"compound:decision_org_live_org_created,order:-1"`
	DocSchemaVersion    int                `bson:"docSchemaVersion,omitempty"`
	TraceID             string             `bson:"traceId,omitempty" index:"single:1,sparse"`
	W3CTraceID          string             `bson:"w3cTraceId,omitempty" index:"single:1,sparse"`
	SpanID              string             `bson:"spanId,omitempty"`
	ParentSpanID        string             `bson:"parentSpanId,omitempty"`
	CorrelationID       string             `bson:"correlationId,omitempty"`
	DecisionCaseID      string             `bson:"decisionCaseId,omitempty" index:"single:1,sparse"`
	Phase               string             `bson:"phase,omitempty"`
	UITitle             string             `bson:"uiTitle,omitempty"`
	Payload             []byte             `bson:"payload"`
}
