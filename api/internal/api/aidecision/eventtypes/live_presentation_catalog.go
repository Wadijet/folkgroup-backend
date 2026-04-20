package eventtypes

import "strings"

// LivePresentationCatalogSchemaVersion — tăng khi thay đổi dữ liệu hiển thị UI trong catalog tập trung.
const LivePresentationCatalogSchemaVersion = 1

const (
	liveCatalogDefaultKey = "__default__"
	liveNarrativeE2EVi    = "Trong quy trình"
	liveNarrativeHandoff  = "Bước chuyển:"
)

var livePhaseLabelViCatalog = map[string]string{
	"queued":                     "Đang chờ tới lượt xử lý",
	"consuming":                  "Đang phân tích và đưa ra gợi ý",
	"skipped":                    "Không cần xử lý thêm",
	"parse":                      "Đang đọc gợi ý từ hội thoại",
	"llm":                        "Đang tinh chỉnh bằng AI",
	"decision":                   "Đang chọn hướng xử lý",
	"policy":                     "Đang phân loại cần duyệt hay tự động",
	"propose":                    "Đang tạo đề xuất hoặc việc cần làm",
	"empty":                      "Không có việc cần làm tiếp",
	"done":                       "Đã xong",
	"error":                      "Có lỗi xảy ra",
	"queue_processing":           "Hệ thống vừa nhận việc",
	"queue_done":                 "Đã xử lý xong việc này",
	"queue_error":                "Xử lý gặp lỗi",
	"datachanged_effects":        "Đang đồng bộ sau khi bạn lưu dữ liệu",
	"orchestrate":                "Đang sắp xếp hồ sơ xử lý",
	"cix_integrated":             "Đã có phân tích hội thoại mới",
	"execute_ready":              "Đã đủ thông tin để đưa ra gợi ý",
	"ads_evaluate":               "Đang đánh giá quy tắc / tối ưu quảng cáo",
	"intel_domain_compute_start": "Domain bắt đầu chạy phân tích",
	"intel_domain_compute_done":  "Domain đã lưu kết quả phân tích",
	"intel_domain_compute_error": "Có lỗi khi Domain chạy phân tích",
	liveCatalogDefaultKey:        "Bước luồng",
}

var liveOutcomeLabelViCatalog = map[string]string{
	"nominal":                   "Đang xử lý",
	"success":                   "Hoàn tất",
	"processing_error":          "Lỗi xử lý",
	"policy_skipped":            "Bỏ qua theo cài đặt",
	"unsupported":               "Chưa hỗ trợ loại này",
	"data_incomplete":           "Thiếu dữ liệu",
	"no_actions":                "Không có việc đề xuất",
	"proposal_failed":           "Không tạo được đề xuất",
	"partial_failure":           "Một phần không thành công",
	"queue_skipped_unspecified": "Cần xem chi tiết mốc",
	liveCatalogDefaultKey:       "Khác",
}

var liveFeedSourceLabelViCatalog = map[string]string{
	"conversation":        "Hội thoại",
	"order":               "Đơn hàng",
	"decision":            "Quyết định",
	"intel":               "Chuẩn bị intel",
	"ads":                 "Ads",
	"meta_sync":           "Đồng bộ Meta",
	"pos_sync":            "Đồng bộ POS",
	"crm":                 "CRM",
	"webhook":             "Webhook",
	"queue":               "Hàng đợi",
	"other":               "Khác",
	liveCatalogDefaultKey: "Khác",
}

var liveBusinessDomainLabelViCatalog = map[string]string{
	"cio":          "CIO",
	"pc":           "Pancake",
	"fb":           "Facebook",
	"webhook":      "Webhook",
	"meta":         "Meta",
	"ads":          "Ads",
	"crm":          "CRM",
	"order":        "Đơn hàng",
	"conversation": "Hội thoại",
	"cix":          "CIX",
	"report":       "Báo cáo",
	"notification": "Thông báo",
	"aidecision":   "AI Decision",
	"executor":     "Executor",
	"learning":     "Learning",
	"unknown":      "Chưa rõ module",
}

// ResolveLivePhaseLabelVi — trả nhãn tiếng Việt cho phase timeline (ưu tiên map E2E step nếu resolve được).
func ResolveLivePhaseLabelVi(phase string) string {
	p := strings.TrimSpace(phase)
	if p == "" {
		return livePhaseLabelViCatalog[liveCatalogDefaultKey]
	}
	switch p {
	case "ads_evaluate":
		if s := strings.TrimSpace(E2ECatalogDescriptionUserViForStep("G4-S03")); s != "" {
			return s
		}
	case "intel_domain_compute_start":
		if s := strings.TrimSpace(E2ECatalogDescriptionUserViForStep("G3-S03")); s != "" {
			return s
		}
	case "intel_domain_compute_done":
		if s := strings.TrimSpace(E2ECatalogDescriptionUserViForStep("G3-S05")); s != "" {
			return s
		}
	}
	if s, ok := livePhaseLabelViCatalog[p]; ok {
		return s
	}
	return p
}

// ResolveLiveOutcomeLabelVi — trả nhãn tiếng Việt cho outcomeKind.
func ResolveLiveOutcomeLabelVi(kind string) string {
	k := strings.TrimSpace(kind)
	if s, ok := liveOutcomeLabelViCatalog[k]; ok {
		return s
	}
	return liveOutcomeLabelViCatalog[liveCatalogDefaultKey]
}

// ResolveLiveFeedSourceLabelVi — trả nhãn tiếng Việt cho feedSourceCategory.
func ResolveLiveFeedSourceLabelVi(category string) string {
	c := strings.TrimSpace(category)
	if s, ok := liveFeedSourceLabelViCatalog[c]; ok {
		return s
	}
	return liveFeedSourceLabelViCatalog[liveCatalogDefaultKey]
}

// ResolveLiveBusinessDomainLabelVi — trả nhãn hiển thị cho businessDomain.
func ResolveLiveBusinessDomainLabelVi(code string) string {
	c := strings.ToLower(strings.TrimSpace(code))
	if c == "" {
		return liveBusinessDomainLabelViCatalog["unknown"]
	}
	if s, ok := liveBusinessDomainLabelViCatalog[c]; ok {
		return s
	}
	return c
}

// IsLiveDetailBulletE2ENarrative — nhận diện dòng mô tả neo E2E đã chèn vào detailBullets.
func IsLiveDetailBulletE2ENarrative(line string) bool {
	s := strings.TrimSpace(line)
	return strings.HasPrefix(s, liveNarrativeE2EVi+":") ||
		strings.HasPrefix(s, "E2E:") ||
		strings.HasPrefix(s, "E2E ") ||
		strings.Contains(s, "Tham chiếu E2E")
}

// IsLiveDetailBulletHandoffNarrative — nhận diện dòng mô tả bước chuyển miền trong detailBullets.
func IsLiveDetailBulletHandoffNarrative(line string) bool {
	s := strings.TrimSpace(line)
	return strings.Contains(s, liveNarrativeHandoff) || strings.Contains(s, "Miền chuyển giao:")
}

// ResolveLiveE2EPublishNarrative — tạo một dòng mô tả ngắn từ tham chiếu E2E (Gx-Syy + nhãn).
func ResolveLiveE2EPublishNarrative(ref E2ERef) string {
	line := liveNarrativeE2EVi
	if strings.TrimSpace(ref.StepID) != "" {
		line += ": " + strings.TrimSpace(ref.StepID)
	}
	if strings.TrimSpace(ref.LabelVi) != "" {
		line += " — " + strings.TrimSpace(ref.LabelVi)
	}
	line += "."
	return line
}

// ResolveLiveHandoffLineFromDomainVi — chuẩn hóa dòng handoff khi emitter chỉ gửi tên miền.
func ResolveLiveHandoffLineFromDomainVi(domainVi string) string {
	v := strings.TrimSpace(domainVi)
	if v == "" {
		return ""
	}
	return liveNarrativeHandoff + " " + v + "."
}

// ResolveLiveHandoffLineFromAIDEvent — tạo dòng handoff từ event AI Decision đã phát sang miền khác.
func ResolveLiveHandoffLineFromAIDEvent(eventSource, eventType string) string {
	es := strings.TrimSpace(eventSource)
	et := strings.TrimSpace(eventType)
	if et == "" || es != "aidecision" {
		return ""
	}
	label := ResolveLiveDomainLabelViFromEventType(et)
	if label == "" {
		return ""
	}
	return liveNarrativeHandoff + " AI Decision đã tạo việc cho miền «" + label + "» — loại sự kiện: " + et + "."
}

// ResolveLiveHandoffLineFromJobType — mô tả handoff theo jobType enqueue tới worker miền.
func ResolveLiveHandoffLineFromJobType(jobType string) string {
	jt := strings.ToLower(strings.TrimSpace(jobType))
	if jt == "" {
		return ""
	}
	switch {
	case strings.Contains(jt, "crm_intel"):
		return liveNarrativeHandoff + " đã xếp hàng việc cho worker miền CRM (intel)."
	case strings.Contains(jt, "cix_intel"):
		return liveNarrativeHandoff + " đã xếp hàng việc cho worker miền CIX (intel hội thoại)."
	case strings.Contains(jt, "order_intel"):
		return liveNarrativeHandoff + " đã xếp hàng việc cho worker miền Đơn hàng (intel)."
	case strings.Contains(jt, "ads_intel"):
		return liveNarrativeHandoff + " đã xếp hàng việc cho worker miền Quảng cáo (intel)."
	default:
		return ""
	}
}

// ResolveLiveDomainLabelViFromEventType — nhãn miền nghiệp vụ cho mô tả handoff theo eventType.
func ResolveLiveDomainLabelViFromEventType(eventType string) string {
	et := strings.TrimSpace(eventType)
	if et == "" {
		return ""
	}
	low := strings.ToLower(et)
	switch {
	case strings.HasPrefix(low, "cix.") || strings.HasPrefix(low, "cix_intel"):
		return "CIX (hội thoại)"
	case strings.HasPrefix(low, "crm.") || strings.HasPrefix(low, "crm_"):
		return "CRM / khách hàng"
	case strings.HasPrefix(low, "customer."):
		return "Khách hàng (CRM)"
	case strings.HasPrefix(low, "order.") || strings.HasPrefix(low, "order_intel"):
		return "Đơn hàng"
	case strings.HasPrefix(low, "ads.") || strings.HasPrefix(low, "campaign_intel") || strings.HasPrefix(low, "meta_ad") || strings.HasPrefix(low, "meta_campaign"):
		return "Quảng cáo / Meta"
	case strings.HasPrefix(low, "aidecision.execute_requested"):
		return "Thực thi (Executor)"
	case strings.HasPrefix(low, "executor."):
		return "Executor"
	case strings.HasPrefix(low, "conversation.") || strings.HasPrefix(low, "message."):
		return "Hội thoại / tin nhắn"
	default:
		return ""
	}
}

// ResolveLiveQueueEventTypeLabelVi — nhãn tiếng Việt ngắn cho eventType trên queue/timeline.
func ResolveLiveQueueEventTypeLabelVi(eventType string) string {
	et := strings.TrimSpace(eventType)
	switch et {
	case OrderChanged, OrderInserted, OrderUpdated:
		return "Đơn hàng"
	case OrderIntelRecomputed:
		return "Phân tích đơn"
	case ConversationChanged, MessageChanged, ConversationInserted, ConversationUpdated, MessageInserted, MessageUpdated:
		return "Hội thoại / tin nhắn"
	case ConversationMessageInserted, MessageBatchReady:
		return "Tin nhắn (gom lô)"
	case CustomerContextReady:
		return "Thông tin khách"
	case CrmIntelRecomputed:
		return "Phân tích khách"
	case CixIntelRecomputed:
		return "Phân tích hội thoại"
	case CampaignIntelRecomputed, MetaCampaignChanged, MetaCampaignInserted, MetaCampaignUpdated:
		return "Quảng cáo / chiến dịch"
	case AdsContextRequested, AdsContextReady:
		return "Ngữ cảnh quảng cáo"
	case CrmIntelligenceComputeRequested, CrmIntelligenceRecomputeRequested:
		return "Cập nhật chỉ số khách"
	case PosCustomerInserted, PosCustomerUpdated:
		return "Khách hàng POS"
	case FbCustomerInserted, FbCustomerUpdated:
		return "Khách hàng Facebook"
	case CrmCustomerInserted, CrmCustomerUpdated:
		return "Khách hàng (đã gộp dữ liệu)"
	default:
		if et != "" {
			return "Cập nhật tự động"
		}
		return "Cập nhật hệ thống"
	}
}
