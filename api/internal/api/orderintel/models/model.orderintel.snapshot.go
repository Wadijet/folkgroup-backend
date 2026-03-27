// Package models — Snapshot Order Intelligence (Vision 07): Raw → L1 → L2 → L3 → Flags.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// OrderIntelligenceSnapshot bản ghi tính toán theo đơn (upsert theo orderUid + org).
type OrderIntelligenceSnapshot struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	OrderUid            string             `json:"orderUid" bson:"orderUid" index:"compound:order_intel_uid_org"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"compound:order_intel_uid_org"`
	OrderID             int64              `json:"orderId,omitempty" bson:"orderId,omitempty"` // Pancake POS id
	Layer1              OrderLayer1        `json:"layer1" bson:"layer1"`
	Layer2              OrderLayer2        `json:"layer2" bson:"layer2"`
	Layer3              OrderLayer3        `json:"layer3" bson:"layer3"`
	Flags               []string           `json:"flags" bson:"flags"`
	Trace               OrderIntelTrace    `json:"trace" bson:"trace"`
	UpdatedAt           int64              `json:"updatedAt" bson:"updatedAt" index:"single:-1"`
	CreatedAt           int64              `json:"createdAt" bson:"createdAt"`
}

// OrderLayer1 giai đoạn đơn (Vision §3.1).
type OrderLayer1 struct {
	Stage string `json:"stage" bson:"stage"` // new | processing | fulfilled | completed | canceled | returned | unknown
}

// OrderLayer2 sức khỏe & giá trị (Vision §3.2) — Phase 1: heuristic, Phase 2: Rule Engine DB.
type OrderLayer2 struct {
	AOVTier              string `json:"aovTier" bson:"aovTier"`                           // low | medium | high | premium
	ConversionQuality    string `json:"conversionQuality" bson:"conversionQuality"`       // strong | normal | weak
	FulfillmentLatency   string `json:"fulfillmentLatency" bson:"fulfillmentLatency"`     // on_time | delayed | critical | unknown
	ReturnRisk           string `json:"returnRisk" bson:"returnRisk"`                     // low | medium | high
	TotalAfterDiscountVND float64 `json:"totalAfterDiscountVnd" bson:"totalAfterDiscountVnd"` // để audit
}

// OrderLayer3 tín hiệu vi mô (Vision §3.3).
type OrderLayer3 struct {
	SourceAttribution string `json:"sourceAttribution" bson:"sourceAttribution"` // ads | organic | unknown
	DelayPattern      string `json:"delayPattern" bson:"delayPattern"`           // none | mild | severe
	HighValueSignal   bool   `json:"highValueSignal" bson:"highValueSignal"`
	AtRiskReturn      string `json:"atRiskReturn" bson:"atRiskReturn"` // low | medium | high
}

// OrderIntelTrace khóa trace cross-module (Vision §5).
type OrderIntelTrace struct {
	AdID             string `json:"adId,omitempty" bson:"adId,omitempty"`
	PostID           string `json:"postId,omitempty" bson:"postId,omitempty"`
	ConversationID   string `json:"conversationId,omitempty" bson:"conversationId,omitempty"`
	CustomerID       string `json:"customerId,omitempty" bson:"customerId,omitempty"`
}
