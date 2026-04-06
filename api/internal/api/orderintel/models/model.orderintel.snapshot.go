// Package models — Snapshot Order Intelligence (Vision 07): Raw → L1 → L2 → L3 → Flags.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// OrderIntelRaw — facts đầu vào pipeline intel đơn (tách khỏi layer1–3 đã suy diễn).
type OrderIntelRaw struct {
	Status                int      `json:"status" bson:"status"`
	InsertedAt            int64    `json:"insertedAt" bson:"insertedAt"`
	PosUpdatedAt          int64    `json:"posUpdatedAt" bson:"posUpdatedAt"`
	TotalAfterDiscountVND float64  `json:"totalAfterDiscountVnd" bson:"totalAfterDiscountVnd"`
	OrderSources          []string `json:"orderSources,omitempty" bson:"orderSources,omitempty"`
	AdID                  string   `json:"adId,omitempty" bson:"adId,omitempty"`
	ConversationID        string   `json:"conversationId,omitempty" bson:"conversationId,omitempty"`
	// EvaluatedAtMs — wall-clock khi worker đánh giá (layer3 phụ thuộc “bây giờ”, ví dụ fulfillment latency).
	EvaluatedAtMs int64 `json:"evaluatedAtMs" bson:"evaluatedAtMs"`
}

// OrderIntelligenceSnapshot bản ghi tính toán theo đơn — bám Unified Data Contract:
// orderUid = canonical ord_* (lớp 2), có thể rỗng; khi rỗng upsert theo orderId POS + ownerOrganizationId (lớp 3 external + tenant).
type OrderIntelligenceSnapshot struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	OrderUid            string             `json:"orderUid" bson:"orderUid" index:"compound:order_intel_uid_org"` // ord_*; để trống nếu đơn chưa gán uid chuẩn
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"compound:order_intel_uid_org"`
	OrderID             int64              `json:"orderId,omitempty" bson:"orderId,omitempty"` // ID đơn trên Pancake POS (sourceIds.pos / PosData.id)
	// Raw — facts tại lần tính gần nhất (đọc nhanh, đồng bộ với run mới nhất).
	Raw                 OrderIntelRaw      `json:"raw" bson:"raw"`
	Layer1              OrderLayer1        `json:"layer1" bson:"layer1"`
	Layer2              OrderLayer2        `json:"layer2" bson:"layer2"`
	Layer3              OrderLayer3        `json:"layer3" bson:"layer3"`
	Flags               []string           `json:"flags" bson:"flags"`
	Trace               OrderIntelTrace    `json:"trace" bson:"trace"`
	// LastIntelRunId — pointer lớp A (order_intel_runs) cho bản ghi read model B này.
	LastIntelRunId      primitive.ObjectID `json:"lastIntelRunId,omitempty" bson:"lastIntelRunId,omitempty"`
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
