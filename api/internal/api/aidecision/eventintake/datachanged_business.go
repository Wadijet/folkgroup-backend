// Package eventintake — Phân tầng độ “gấp” side-effect datachanged theo nghiệp vụ (fallback khi rule JS tắt/lỗi).
// Khi đổi logic classify: cập nhật đồng thời script trong ruleintel/migration/seed_rule_aidecision_side_effect_policy.go (LOGIC_DATACHANGED_SIDE_EFFECT_POLICY).
//
// Ba mức:
//   - Realtime — tương tác khách / tiền / ghi chú vận hành: không gom, chạy ngay (trừ dedupe CRM ingest nếu bật).
//   - Operational — đồng bộ mirror thường (hội thoại cập nhật, đơn sửa…): gom theo cửa sổ “vận hành”.
//   - Background — catalog, cấu trúc Meta Ads, log: gom cửa sổ dài hơn.
//
// Ghi đè thủ công (API / vận hành): payload immediateSideEffects | forceImmediateSideEffects | urgentSideEffects = true → luôn Realtime.
package eventintake

import (
	"strings"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	"meta_commerce/internal/api/events"
	"meta_commerce/internal/global"
)

// BusinessSideEffectUrgency mức ưu tiên side-effect sau thay đổi nguồn.
type BusinessSideEffectUrgency int

const (
	// UrgencyUnknown — giá trị 0, coi như Operational khi tính defer (an toàn).
	UrgencyUnknown BusinessSideEffectUrgency = iota
	UrgencyRealtime
	UrgencyOperational
	UrgencyBackground
)

// ClassifyDatachangedBusinessUrgency suy ra mức nghiệp vụ từ collection + thao tác + eventType.
func ClassifyDatachangedBusinessUrgency(evt *aidecisionmodels.DecisionEvent, sourceCollection, operation string) BusinessSideEffectUrgency {
	if evt != nil && evt.Payload != nil {
		if payloadBoolTrue(evt.Payload, "immediateSideEffects") ||
			payloadBoolTrue(evt.Payload, "forceImmediateSideEffects") ||
			payloadBoolTrue(evt.Payload, "urgentSideEffects") {
			return UrgencyRealtime
		}
	}

	src := strings.TrimSpace(sourceCollection)
	op := strings.ToLower(strings.TrimSpace(operation))
	if op == "" {
		op = events.OpUpdate
	}
	c := global.MongoDB_ColNames

	// --- Background: dữ liệu cấu trúc / marketing / catalog — chậm vài phút không làm hỏng SLA hội thoại ---
	switch src {
	case c.MetaCampaigns, c.MetaAdSets, c.MetaAds, c.MetaAdInsights, c.MetaAdInsightsDailySnapshots, c.MetaAdAccounts,
		c.PcPosProducts, c.PcPosVariations, c.PcPosCategories, c.PcPosShops, c.PcPosWarehouses,
		c.FbPages, c.FbPosts, c.WebhookLogs:
		return UrgencyBackground
	case c.PcOrders:
		// Đơn legacy Pancake (không phải POS API) — ưu tiên thấp hơn luồng đơn POS chính.
		return UrgencyBackground
	}

	// --- Realtime: chạm trực tiếp khách, tiền, hoặc thao tác nhân viên trên hồ sơ ---
	switch src {
	case c.CrmNotes, c.CrmCustomers, c.CrmActivityHistory:
		return UrgencyRealtime
	case c.FbMessages:
		// Tin nhắn mới = cần pipeline hội thoại / CRM kịp thời.
		if op == events.OpInsert {
			return UrgencyRealtime
		}
		return UrgencyOperational
	case c.FbMessageItems:
		// Chi tiết từng tin (CIO) — ưu tiên side-effect / queue giống message.inserted.
		if op == events.OpInsert {
			return UrgencyRealtime
		}
		return UrgencyOperational
	case c.FbConvesations:
		// Thread mới: gần với “có tương tác mới”; cập nhật sync thường chỉ metadata.
		if op == events.OpInsert {
			return UrgencyRealtime
		}
		return UrgencyOperational
	case c.PcPosOrders:
		// Đơn mới: risk / báo cáo doanh thu cần gần real-time; sửa đơn vẫn quan trọng nhưng cho phép gom ngắn.
		if op == events.OpInsert || op == events.OpUpsert {
			return UrgencyRealtime
		}
		return UrgencyOperational
	case c.FbCustomers, c.PcPosCustomers:
		// Lead / khách mới: Realtime; cập nhật sync từ ngoài: Operational.
		if op == events.OpInsert || op == events.OpUpsert {
			return UrgencyRealtime
		}
		return UrgencyOperational
	}

	// --- eventType khi collection không khớp hoặc bổ sung nghĩa ---
	if evt != nil {
		et := strings.TrimSpace(evt.EventType)
		switch {
		case strings.HasPrefix(et, "crm_note."), strings.HasPrefix(et, "crm_customer."), strings.HasPrefix(et, "crm_activity."):
			return UrgencyRealtime
		case strings.HasPrefix(et, "message.") && strings.HasSuffix(et, ".inserted"):
			return UrgencyRealtime
		case strings.HasPrefix(et, "order.") && strings.HasSuffix(et, ".inserted"):
			return UrgencyRealtime
		case strings.HasPrefix(et, "meta_"), strings.HasPrefix(et, "fb_page."), strings.HasPrefix(et, "fb_post."),
			strings.HasPrefix(et, "pos_product."), strings.HasPrefix(et, "pos_shop."), strings.HasPrefix(et, "webhook_log."):
			return UrgencyBackground
		}
	}

	// Mặc định: không chắc → gom “vận hành” thay vì coi là nền (tránh bỏ sót CRM).
	return UrgencyOperational
}

// payloadBoolTrue chấp nhận bool, chuỗi "true"/"1", số khác 0.
func payloadBoolTrue(m map[string]interface{}, key string) bool {
	if m == nil {
		return false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		return s == "true" || s == "1" || s == "yes"
	case float64:
		return t != 0
	case float32:
		return t != 0
	case int:
		return t != 0
	case int64:
		return t != 0
	default:
		return false
	}
}
