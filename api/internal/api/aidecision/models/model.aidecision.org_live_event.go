// Package models — Schema index cho collection Mongo decision_org_live_events (AI Decision org-live persist).
//
// Khi nào có bản ghi: chỉ sau decisionlive.Publish khi (1) AI_DECISION_LIVE_ENABLED bật (có bước 6b append org)
// và (2) org persist bật (AIDecisionLiveOrgPersist / AI_DECISION_LIVE_ORG_PERSIST). Mỗi lần Publish đủ điều kiện
// → tối đa một InsertOne bất đồng bộ (persistOrgLiveEventAsync) — một document = một mốc timeline (orgEv).
//
// Nội dung lưu: BSON do BuildOrgLivePersistDocument (decisionlive/persist_org_audit.go) — có nhiều trường phẳng
// (lọc, UI, audit) và payload (JSON nguyên vẹn DecisionLiveEvent). Struct dưới đây dùng cho CreateIndexes;
// các trường phẳng khác vẫn được ghi cùng document khi InsertOne.
package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// AIDecisionOrgLiveEvent — Trường tối thiểu + index; khớp subset BSON thực tế (docSchemaVersion >= 2).
//
// Nhóm nội dung trên document (đối chiếu code dựng doc):
//
//	Hệ thống: _id, ownerOrganizationId, createdAt (ms), docSchemaVersion (=2).
//	Định danh / trace: traceId, w3cTraceId, spanId, parentSpanId, correlationId, decisionCaseId (lọc persisted-events).
//	Pipeline & nguồn: phase, severity, seq, feedSeq, stream, sourceKind, sourceTitle, feedSource*, businessDomain, businessDomainLabelVi, opsTier*, decisionMode.
//	Tham chiếu E2E: e2eStage, e2eStepId, e2eStepLabelVi (luồng chuẩn G1–G6 — docs/flows/bang-pha-buoc-event-e2e.md).
//	Kết quả: outcomeKind, outcomeAbnormal, outcomeLabelVi (phân loại bình thường / bất thường).
//	UI suy ra: uiTitle, uiSummary, phaseLabelVi, stepKind, stepTitle.
//	Refs & chi tiết: refs (map), detailBullets, detailSections, processTrace (cây bước xử lý — tùy mốc).
//	Nguồn sự thật đầy đủ: payload ([]byte JSON của toàn bộ DecisionLiveEvent — đọc replay qua json.Unmarshal).
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
	BusinessDomain      string             `bson:"businessDomain,omitempty" index:"single:1,sparse"`
	E2EStage            string             `bson:"e2eStage,omitempty" index:"single:1,sparse"`
	E2EStepID           string             `bson:"e2eStepId,omitempty"`
	E2EStepLabelVi      string             `bson:"e2eStepLabelVi,omitempty"`
	UITitle             string             `bson:"uiTitle,omitempty"`
	Payload             []byte             `bson:"payload"`
}
