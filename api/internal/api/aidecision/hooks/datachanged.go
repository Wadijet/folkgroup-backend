// Package hooks — Đăng ký events.OnDataChanged → emit queue AI Decision theo collection nguồn.
//
// Payload tối giản (contract): sourceCollection, normalizedRecordUid, dataChangeOperation.
// AI Decision hydrate đầy đủ ref từ Mongo — xem aidecisionsvc.HydrateDatachangedPayload.
// event_type = <prefix>.inserted|.updated theo source_sync_registry.
package hooks

import (
	"context"

	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	"meta_commerce/internal/api/events"

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
	eventType := eventTypeForSourceSync(entityPrefix, e.Operation)
	_, _ = decSvc.EmitEvent(ctx, &aidecisionsvc.EmitEventInput{
		EventType:   eventType,
		EventSource: "datachanged",
		EntityType:  entityPrefix,
		EntityID:    idHex,
		OrgID:       ownerOrgID.Hex(),
		OwnerOrgID:  ownerOrgID,
		Priority:    "high",
		Lane:        "fast",
		Payload:     payload,
	})
}
