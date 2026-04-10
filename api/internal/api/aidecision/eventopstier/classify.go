// Package eventopstier — phân loại eventType theo góc nhìn quản lý / vận hành (backend là nguồn sự thật).
// Không import aidecisionsvc / worker để tránh import cycle (decisionlive → eventopstier → …).
package eventopstier

import (
	"strings"

	"meta_commerce/internal/api/aidecision/eventtypes"
)

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
	"cix_analysis_result": {},
	"customer_customer":   {},
	"customer_note":       {},
	"customer_activity":   {},
	"crm_customer":        {}, // legacy datachanged prefix
	"crm_note":            {},
	"crm_activity":        {},
	"fb_customer":         {}, // hồ sơ khách FB — ngữ cảnh CI / bán hàng
}

// exactTier — ánh xạ tường minh (đồng bộ với eventtypes + consumer / emit).
var exactTier = map[string]string{
	// Quyết định trực tiếp
	eventtypes.AIDecisionExecuteRequested: TierDecision,
	eventtypes.ExecutorProposeRequested:   TierDecision,
	eventtypes.AdsProposeRequested:        TierDecision,

	// Chuẩn bị / luồng vào quyết định
	eventtypes.CixAnalysisRequested:         TierPipeline,
	eventtypes.CustomerContextRequested:     TierPipeline,
	eventtypes.CustomerContextReady:         TierPipeline,
	eventtypes.OrderIntelligenceRequested:   TierPipeline,
	eventtypes.OrderRecomputeRequested:      TierPipeline,
	eventtypes.OrderIntelRecomputed:         TierPipeline,
	eventtypes.ConversationMessageInserted:  TierPipeline,
	eventtypes.MessageBatchReady:            TierPipeline,
	eventtypes.AdsContextRequested:          TierPipeline,
	eventtypes.AdsContextReady:              TierPipeline,
	// ads.updated — đổi cấu hình ads trong DB (thường batch) nhưng dẫn vào pipeline ads context / intel
	eventtypes.AdsUpdated: TierPipeline,

	// Vận hành / nền
	eventtypes.CrmIntelligenceComputeRequested:       TierOperational,
	eventtypes.CrmIntelligenceRecomputeRequested:     TierOperational,
	eventtypes.AdsIntelligenceRecomputeRequested:     TierOperational,
	eventtypes.AdsIntelligenceRecalculateAllRequested: TierOperational,
	eventtypes.MetaCampaignInserted:     TierOperational,
	eventtypes.MetaCampaignUpdated:      TierOperational,
	eventtypes.CampaignIntelRecomputed:  TierOperational,
	eventtypes.CrmIntelRecomputed:       TierOperational,
	eventtypes.CixIntelRecomputed:       TierOperational,
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
