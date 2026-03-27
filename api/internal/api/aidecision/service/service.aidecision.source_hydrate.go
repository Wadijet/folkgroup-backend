// Package aidecisionsvc — bổ sung payload từ Mongo cho event datachanged (payload tối giản từ hook).
//
// Hook chỉ gửi sourceCollection + normalizedRecordUid + dataChangeOperation; mọi ref nghiệp vụ
// (conversationId, customerId, channel, lastMessage debounce, orderId, …) được hydrate tại đây.
package aidecisionsvc

import (
	"context"
	"strings"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// HydrateDatachangedPayload đọc bản ghi nguồn và merge vào evt.Payload (không ghi đè field đã có và khác rỗng).
// Luôn trả về nil — không làm fail consumer khi doc không còn.
func (s *AIDecisionService) HydrateDatachangedPayload(ctx context.Context, evt *aidecisionmodels.DecisionEvent) {
	if evt == nil || evt.EventSource != "datachanged" || evt.Payload == nil {
		return
	}
	p := evt.Payload
	src, _ := p["sourceCollection"].(string)
	idHex := stringFromPayloadID(p, evt.EntityID)
	if src == "" || idHex == "" {
		return
	}
	oid, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		return
	}
	coll, ok := global.RegistryCollections.Get(src)
	if !ok {
		return
	}
	var raw bson.M
	if err := coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&raw); err != nil {
		return
	}

	switch src {
	case global.MongoDB_ColNames.FbConvesations, global.MongoDB_ColNames.FbMessages:
		if _, has := p["channel"]; !has {
			p["channel"] = "messenger"
		}
		mergeStringIfEmpty(p, raw, "conversationId", "conversationId", "conversation_id")
		mergeStringIfEmpty(p, raw, "customerId", "customerId", "customer_id")
		if src == global.MongoDB_ColNames.FbMessages {
			if lm := extractMessageTextForDebounce(raw); lm != "" {
				if _, has := p["lastMessage"]; !has {
					p["lastMessage"] = lm
				}
			}
		}
	case global.MongoDB_ColNames.PcPosOrders:
		hydratePcPosOrderFromRaw(p, raw)
	case global.MongoDB_ColNames.CrmCustomers:
		mergeStringIfEmpty(p, raw, "unifiedId", "unifiedId")
	}
	// Mọi collection: ref phẳng thường gặp (Meta/CRM/FB…) — chỉ điền khi payload chưa có.
	hydrateGenericRefsFromRaw(p, raw)
}

func stringFromPayloadID(p map[string]interface{}, entityID string) string {
	if u, ok := p["normalizedRecordUid"].(string); ok && strings.TrimSpace(u) != "" {
		return strings.TrimSpace(u)
	}
	return strings.TrimSpace(entityID)
}

func mergeStringIfEmpty(p map[string]interface{}, raw bson.M, payloadKey string, rawKeys ...string) {
	if strFromPayload(p, payloadKey) != "" {
		return
	}
	for _, rk := range rawKeys {
		if s := stringFromBSONValue(raw[rk]); s != "" {
			p[payloadKey] = s
			return
		}
	}
}

func strFromPayload(p map[string]interface{}, key string) string {
	v, ok := p[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func stringFromBSONValue(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case primitive.ObjectID:
		return t.Hex()
	default:
		return ""
	}
}

func int64FromBSONValue(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch t := v.(type) {
	case int64:
		return t
	case int32:
		return int64(t)
	case int:
		return int64(t)
	case float64:
		return int64(t)
	default:
		return 0
	}
}

func bsonSubMap(v interface{}) map[string]interface{} {
	if v == nil {
		return nil
	}
	var m map[string]interface{}
	b, err := bson.Marshal(v)
	if err != nil {
		return nil
	}
	if err := bson.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}

func extractMessageTextForDebounce(raw bson.M) string {
	keys := []string{"text", "message", "body", "content", "lastMessage", "snippet"}
	for _, k := range keys {
		if s := stringFromBSONValue(raw[k]); s != "" {
			return s
		}
	}
	if pd := bsonSubMap(raw["panCakeData"]); pd != nil {
		for _, k := range keys {
			if s := stringFromBSONValue(pd[k]); s != "" {
				return s
			}
		}
	}
	return ""
}

func hydrateGenericRefsFromRaw(p map[string]interface{}, raw bson.M) {
	mergeStringIfEmpty(p, raw, "customerId", "customerId", "customer_id")
	mergeStringIfEmpty(p, raw, "conversationId", "conversationId", "conversation_id")
	mergeStringIfEmpty(p, raw, "pageId", "pageId")
	mergeStringIfEmpty(p, raw, "shopId", "shopId")
	mergeStringIfEmpty(p, raw, "campaignId", "campaignId", "campaign_id")
	mergeStringIfEmpty(p, raw, "adSetId", "adSetId", "adset_id")
	mergeStringIfEmpty(p, raw, "adsetId", "adsetId")
	mergeStringIfEmpty(p, raw, "adId", "adId", "ad_id")
	mergeStringIfEmpty(p, raw, "adAccountId", "adAccountId", "ad_account_id")
	mergeStringIfEmpty(p, raw, "accountId", "accountId")
	mergeStringIfEmpty(p, raw, "externalId", "externalId")
	mergeStringIfEmpty(p, raw, "sourceId", "sourceId")
	mergeStringIfEmpty(p, raw, "uid", "uid")
	mergeIntIfEmpty(p, raw, "orderId", "orderId")
}

func mergeIntIfEmpty(p map[string]interface{}, raw bson.M, payloadKey string, rawKeys ...string) {
	if _, ok := p[payloadKey]; ok {
		return
	}
	for _, rk := range rawKeys {
		if n := int64FromBSONValue(raw[rk]); n != 0 {
			p[payloadKey] = n
			return
		}
	}
}

func hydratePcPosOrderFromRaw(p map[string]interface{}, raw bson.M) {
	pos := bsonSubMap(raw["posData"])
	mergeStringIfEmpty(p, raw, "customerId", "customerId", "customer_id")
	if strFromPayload(p, "customerId") == "" && pos != nil {
		if cm := bsonSubMap(pos["customer"]); cm != nil {
			if s := stringFromBSONValue(cm["id"]); s != "" {
				p["customerId"] = s
			}
		}
		if strFromPayload(p, "customerId") == "" {
			if s := stringFromBSONValue(pos["customer_id"]); s != "" {
				p["customerId"] = s
			}
		}
	}
	if _, ok := p["orderId"]; !ok {
		oid := int64FromBSONValue(raw["orderId"])
		if oid == 0 && pos != nil {
			oid = int64FromBSONValue(pos["id"])
		}
		if oid != 0 {
			p["orderId"] = oid
		}
	}
	if strFromPayload(p, "orderUid") == "" {
		if s := stringFromBSONValue(raw["uid"]); s != "" {
			p["orderUid"] = s
		}
	}
}
