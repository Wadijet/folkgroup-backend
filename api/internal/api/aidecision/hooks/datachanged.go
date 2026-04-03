// Package hooks — Đăng ký events.OnDataChanged → emit queue AI Decision theo collection nguồn.
//
// Hai lớp tách biệt:
//   - Lớp 1 — DoSyncUpsert: so updated_at nguồn (posData/panCakeData) để giảm lượt ghi Mongo khi đồng bộ từ ngoài.
//   - Lớp 2 — hook này: cổng enqueue (org, registry, bỏ delete) — không lặp lại so sánh updated_at nguồn (đã thuộc lớp 1).
//
// Payload queue tối giản: sourceCollection, normalizedRecordUid, dataChangeOperation — consumer hydrate từ Mongo.
// event_type = <prefix>.inserted|.updated theo source_sync_registry.
// Ghi queue thực tế qua ShouldEmitDatachangedToDecisionQueue (registry không xóa — chỉ lọc emit).
package hooks

import (
	"context"

	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	"meta_commerce/internal/api/events"
	"meta_commerce/internal/utility"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RegisterAIDecisionOnDataChanged đăng ký handler toàn cục (gọi một lần từ init.registry).
// Luồng duy nhất: EmitEvent → decision_events_queue.
func RegisterAIDecisionOnDataChanged(decSvc *aidecisionsvc.AIDecisionService) {
	events.OnDataChanged(func(ctx context.Context, e events.DataChangeEvent) {
		if e.Document == nil {
			return
		}
		if e.Operation == events.OpDelete {
			return
		}
		ownerOrgID := events.GetOwnerOrganizationIDFromDocument(e.Document)
		if ownerOrgID.IsZero() {
			return
		}

		prefix, ok := sourceSyncPrefixesMap()[e.CollectionName]
		if !ok {
			return
		}
		if !ShouldEmitDatachangedToDecisionQueue(e.CollectionName) {
			return
		}
		emitUnifiedSourceDataChanged(ctx, decSvc, e, ownerOrgID, prefix)
	})
}

func docToMap(doc interface{}) map[string]interface{} {
	data, err := bson.Marshal(doc)
	if err != nil {
		return nil
	}
	var m map[string]interface{}
	if err := bson.Unmarshal(data, &m); err != nil {
		return nil
	}
	return m
}

func idHexFromDoc(m map[string]interface{}) string {
	if m == nil {
		return ""
	}
	if v, ok := m["_id"]; ok && v != nil {
		switch t := v.(type) {
		case primitive.ObjectID:
			return t.Hex()
		}
	}
	return ""
}

// eventTypeForSourceSync: insert → *.inserted; update | upsert → *.updated.
func eventTypeForSourceSync(entityPrefix string, op string) string {
	suffix := "updated"
	switch op {
	case events.OpInsert:
		suffix = "inserted"
	case events.OpUpdate, events.OpUpsert:
		suffix = "updated"
	default:
		suffix = "updated"
	}
	return entityPrefix + "." + suffix
}

// emitUnifiedSourceDataChanged — payload tối giản; consumer gọi HydrateDatachangedPayload.
func emitUnifiedSourceDataChanged(ctx context.Context, decSvc *aidecisionsvc.AIDecisionService, e events.DataChangeEvent, ownerOrgID primitive.ObjectID, entityPrefix string) {
	m := docToMap(e.Document)
	if m == nil {
		return
	}
	idHex := idHexFromDoc(m)
	if idHex == "" {
		return
	}
	payload := map[string]interface{}{
		"sourceCollection":    e.CollectionName,
		"normalizedRecordUid": idHex,
		"dataChangeOperation": e.Operation,
	}
	// Roll-up Ads Intelligence chỉ cập nhật currentMetrics — không kích hoạt lại pipeline campaign (xem ProcessMetaCampaignDataChanged).
	if events.IsAdsIntelligenceRollupContext(ctx) {
		payload["adsIntelligenceRollupOnly"] = true
	}
	eventType := eventTypeForSourceSync(entityPrefix, e.Operation)
	// Một traceId / correlationId gốc cho toàn chuỗi queue → orchestrate → CIX / execute (không ghi đè khi caller đã set).
	traceID := utility.GenerateUID(utility.UIDPrefixTrace)
	correlationID := utility.GenerateUID(utility.UIDPrefixCorrelation)
	_, _ = decSvc.EmitEvent(ctx, &aidecisionsvc.EmitEventInput{
		EventType:     eventType,
		EventSource:   "datachanged",
		EntityType:    entityPrefix,
		EntityID:      idHex,
		OrgID:         ownerOrgID.Hex(),
		OwnerOrgID:    ownerOrgID,
		Priority:      "high",
		Lane:          "fast",
		TraceID:       traceID,
		CorrelationID: correlationID,
		Payload:       payload,
	})
}
