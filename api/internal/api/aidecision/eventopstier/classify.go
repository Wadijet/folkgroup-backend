// Package eventopstier — phân loại eventType theo góc nhìn quản lý / vận hành (backend là nguồn sự thật).
// Không import aidecisionsvc / worker để tránh import cycle (decisionlive → eventopstier → …).
package eventopstier

import "strings"

// Tier — mức “đáng xem” cho dashboard vận hành (ổn định cho API / JSON).
const (
	TierDecision    = "decision"      // Thực thi / đề xuất quyết định trực tiếp
	TierPipeline    = "pipeline"      // Chuẩn bị ngữ cảnh, dẫn vào quyết định
	TierOperational = "operational"   // Đồng bộ, batch, metrics — ưu tiên thấp khi soi feed
	TierUnknown     = "unknown"       // Chưa ánh xạ (event tùy biến / legacy)
)

var labelVi = map[string]string{
	TierDecision:    "Quyết định",
	TierPipeline:    "Chuẩn bị quyết định",
	TierOperational: "Vận hành",
	TierUnknown:     "Chưa phân loại",
}

// LabelVi nhãn hiển thị tiếng Việt cho tier.
func LabelVi(tier string) string {
	if s, ok := labelVi[tier]; ok {
		return s
	}
	return labelVi[TierUnknown]
}

// pipelineEntityPrefixes — tiền tố entity (datachanged: *.inserted|*.updated) dẫn vào intel / ngữ cảnh quyết định.
// Đồng bộ khái niệm với hooks/source_sync_registry.go: CRM + đơn + hội thoại + CIX; không gồm POS catalog / Meta sync / webhook thô.
var pipelineEntityPrefixes = map[string]struct{}{
	"conversation":        {},
	"message":             {},
	"order":               {},
	"pc_order":            {}, // đơn từ nguồn PC (song song order POS)
	"cix_analysis_result": {},
	"crm_customer":        {},
	"crm_note":            {},
	"crm_activity":        {},
	"fb_customer":         {}, // hồ sơ khách FB — ngữ cảnh CI / bán hàng
}

// exactTier — ánh xạ tường minh (đồng bộ với consumer / emit; chuỗi literal tránh import cycle).
var exactTier = map[string]string{
	// Quyết định trực tiếp
	"aidecision.execute_requested": TierDecision,
	"executor.propose_requested":   TierDecision,
	"ads.propose_requested":        TierDecision,

	// Chuẩn bị / luồng vào quyết định
	"conversation.intelligence_requested": TierPipeline,
	"cix.analysis_requested":              TierPipeline,
	"cix.analysis_completed":              TierPipeline,
	"cix_analysis_result.inserted":          TierPipeline,
	"cix_analysis_result.updated":           TierPipeline,
	"customer.context_requested":            TierPipeline,
	"customer.context_ready":                TierPipeline,
	"order.flags_emitted":                   TierPipeline,
	"order.intelligence_requested":          TierPipeline,
	"order.recompute_requested":             TierPipeline,
	"commerce.order_completed":              TierPipeline,
	"conversation.message_inserted":         TierPipeline,
	"message.batch_ready":                   TierPipeline,
	"ads.context_requested":                 TierPipeline,
	"ads.context_ready":                     TierPipeline,
	// ads.updated — đổi cấu hình ads trong DB (thường batch) nhưng dẫn vào pipeline ads context / intel
	"ads.updated": TierPipeline,

	// Vận hành / nền
	"crm.intelligence.compute_requested":            TierOperational,
	"ads.intelligence.recompute_requested":           TierOperational,
	"ads.intelligence.recalculate_all_requested":      TierOperational,
	"meta_campaign.inserted": TierOperational,
	"meta_campaign.updated":  TierOperational,
	// Các prefix meta_*, pos_*, fb_page, fb_post, fb_message_item, webhook_log (*.inserted|*.updated)
	// không nằm trong pipelineEntityPrefixes → ClassifyEventType xếp operational qua nhánh suffix (không cần lặp từng dòng).
}

// ClassifyEventType trả tier + nhãn tiếng Việt cho eventType (chuỗi queue / ingest).
func ClassifyEventType(eventType string) (tier string, labelViOut string) {
	et := strings.TrimSpace(eventType)
	if et == "" {
		return TierUnknown, LabelVi(TierUnknown)
	}
	if t, ok := exactTier[et]; ok {
		return t, LabelVi(t)
	}
	if i := strings.LastIndexByte(et, '.'); i > 0 {
		sfx := et[i+1:]
		if sfx == "inserted" || sfx == "updated" {
			pfx := et[:i]
			if _, ok := pipelineEntityPrefixes[pfx]; ok {
				return TierPipeline, LabelVi(TierPipeline)
			}
			return TierOperational, LabelVi(TierOperational)
		}
	}
	// event_type tùy biến / tương lai — không đoán “vận hành” để tránh hiển thị sai tier.
	return TierUnknown, LabelVi(TierUnknown)
}
