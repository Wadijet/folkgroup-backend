// queue_process_trace — Dựng cây processTrace cho mốc timeline consumer (tiếng Việt ngắn gọn; khung Mục đích/Đầu vào/… ở mức event — xem livecopy.BuildQueueConsumerEvent / TraceStep.reasoning).
package worker

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"meta_commerce/internal/api/aidecision/crmqueue"
	"meta_commerce/internal/api/aidecision/datachangedsidefx"
	"meta_commerce/internal/api/aidecision/decisionlive"
	"meta_commerce/internal/api/aidecision/decisionlive/livecopy"
	"meta_commerce/internal/api/aidecision/eventtypes"
	"meta_commerce/internal/api/aidecision/intelrecomputed"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	"meta_commerce/internal/global"
)

const (
	maxProcessTraceErrRunes    = 400
	maxProcessTraceDetailRunes = 520
)

func truncateProcessTraceErr(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxProcessTraceErrRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxProcessTraceErrRunes]) + "…"
}

func truncateProcessTraceDetail(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxProcessTraceDetailRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxProcessTraceDetailRunes]) + "…"
}

func formatDeferWindowVi(d time.Duration) string {
	if d <= 0 {
		return "chạy ngay"
	}
	sec := int(d.Round(time.Second) / time.Second)
	if sec < 60 {
		if sec <= 0 {
			sec = 1
		}
		return fmt.Sprintf("trì hoãn ~%d giây", sec)
	}
	min := int(d / time.Minute)
	if d%time.Minute == 0 {
		return fmt.Sprintf("trì hoãn ~%d phút", min)
	}
	return fmt.Sprintf("trì hoãn ~%d phút %d giây", min, sec%60)
}

func friendlyDataOpVi(op string) string {
	switch strings.ToLower(strings.TrimSpace(op)) {
	case "insert", "inserted":
		return "thêm mới"
	case "update", "updated":
		return "cập nhật"
	case "delete", "deleted":
		return "xoá"
	default:
		return "thay đổi"
	}
}

// friendlyCollectionVi — Tên nguồn dễ hiểu (không dùng tên collection kỹ thuật trên UI processTrace).
func friendlyCollectionVi(src string) string {
	c := strings.TrimSpace(src)
	if c == "" {
		return "dữ liệu của bạn"
	}
	g := global.MongoDB_ColNames
	switch c {
	case g.FbMessageItems:
		return "tin nhắn"
	case g.FbConvesations:
		return "hội thoại"
	case g.FbCustomers, g.PcPosCustomers, g.CustomerCustomers:
		return "khách hàng"
	case g.PcPosOrders, g.OrderCanonical:
		return "đơn hàng"
	case g.MetaCampaigns, g.MetaAds, g.MetaAdSets:
		return "quảng cáo / chiến dịch"
	case g.CustomerNotes:
		return "ghi chú khách"
	default:
		return "dữ liệu nguồn"
	}
}

func friendlyRuleSummaryVi(ruleID string) string {
	switch strings.TrimSpace(ruleID) {
	case "crm_l1_merge_sources":
		return "hồ sơ khách (nhiều nguồn)"
	case "cix_fb_message_items":
		return "tin nhắn — phân tích hội thoại"
	case "meta_ads_synced_family":
		return "quảng cáo Meta"
	case "generic_source":
		return "dữ liệu chung"
	case "empty_collection":
		return "chưa xác định nguồn"
	default:
		return "theo cấu hình hệ thống"
	}
}

// datachangedTraceSourceUnreadable — Không đọc được document từ Mongo (trace cho vận hành).
func datachangedTraceSourceUnreadable(src, idHex string) []decisionlive.DecisionLiveProcessNode {
	_ = idHex
	return []decisionlive.DecisionLiveProcessNode{
		{
			Kind:     decisionlive.ProcessTraceKindOutcome,
			Key:      "dc_source_unreadable",
			LabelVi:  "Không tìm thấy dữ liệu gốc để xử lý",
			DetailVi: truncateProcessTraceDetail(fmt.Sprintf("Nguồn: %s. Có thể bản ghi đã đổi hoặc bị xoá trước khi hệ thống kịp đọc.", friendlyCollectionVi(src))),
		},
	}
}

// buildDatachangedSideEffectTraceNodes — Các bước con sau khi đã chạy datachangedsidefx.Run (đúng thứ tự thực tế trong worker).
func buildDatachangedSideEffectTraceNodes(ac *datachangedsidefx.ApplyContext, pancakeLine string) []decisionlive.DecisionLiveProcessNode {
	if ac == nil {
		return nil
	}
	out := make([]decisionlive.DecisionLiveProcessNode, 0, 8)

	out = append(out, decisionlive.DecisionLiveProcessNode{
		Kind:     decisionlive.ProcessTraceKindStep,
		Key:      "dc_read_doc",
		LabelVi:  "Đọc lại thông tin bạn vừa thay đổi",
		DetailVi: truncateProcessTraceDetail(fmt.Sprintf("Loại: %s · Thao tác: %s.", friendlyCollectionVi(ac.Src), friendlyDataOpVi(ac.Op))),
	})

	if pl := strings.TrimSpace(pancakeLine); pl != "" {
		kind := decisionlive.ProcessTraceKindStep
		if strings.HasPrefix(pl, "Không cập nhật được đơn từ Pancake") {
			kind = decisionlive.ProcessTraceKindError
		}
		out = append(out, decisionlive.DecisionLiveProcessNode{
			Kind:     kind,
			Key:      "dc_pancake_order_sync",
			LabelVi:  "Đồng bộ đơn hàng từ Pancake",
			DetailVi: truncateProcessTraceDetail(pl),
		})
	}

	policyParts := []string{
		fmt.Sprintf("Gộp / chuẩn bị hồ sơ khách: %s", ternaryVi(ac.Dec.AllowCrmMergeQueue, "có", "tạm bỏ qua (vừa xử lý gần đây hoặc theo cài đặt)")),
		fmt.Sprintf("Cập nhật báo cáo: %s", ternaryVi(ac.Dec.AllowReport, "có", "không")),
		fmt.Sprintf("Phần liên quan quảng cáo: %s", ternaryVi(ac.Dec.AllowAds, "có", "không")),
	}
	if len(ac.Dec.ReasonsSkipped) > 0 {
		rv := make([]string, 0, len(ac.Dec.ReasonsSkipped))
		for _, r := range ac.Dec.ReasonsSkipped {
			switch r {
			case "crm_dedupe_window":
				rv = append(rv, "trùng lặp trong thời gian ngắn — hệ thống gom một lần")
			default:
				rv = append(rv, r)
			}
		}
		policyParts = append(policyParts, "Ghi chú: "+strings.Join(rv, ", "))
	}
	out = append(out, decisionlive.DecisionLiveProcessNode{
		Kind:     decisionlive.ProcessTraceKindStep,
		Key:      "dc_policy_intake",
		LabelVi:  "Kiểm tra: có cần gộp khách, báo cáo hay quảng cáo không?",
		DetailVi: truncateProcessTraceDetail(strings.Join(policyParts, " · ")),
	})

	routeParts := []string{fmt.Sprintf("Nhóm xử lý: %s", friendlyRuleSummaryVi(ac.Route.RuleID))}
	var pipes []string
	if ac.Route.CustomerPendingMergeCollection {
		pipes = append(pipes, "chuẩn bị gộp thông tin khách")
	}
	if ac.Route.ReportTouchPipeline {
		pipes = append(pipes, "làm mới báo cáo")
	}
	if ac.Route.AdsProfilePipeline {
		pipes = append(pipes, "cập nhật phần liên quan quảng cáo")
	}
	if ac.Route.CixIntelPipeline {
		pipes = append(pipes, "có thể phân tích tin nhắn sau (nếu bật)")
	}
	if ac.Route.OrderIntelPipeline {
		pipes = append(pipes, "có thể phân tích đơn hàng sau (nếu bật)")
	}
	if ac.Route.CustomerIntelRefreshDeferPipeline {
		pipes = append(pipes, "làm mới phân tích khách (có thể chờ thêm)")
	}
	if len(pipes) == 0 {
		routeParts = append(routeParts, "không bật thêm bước tự động cho loại này")
	} else {
		routeParts = append(routeParts, "Các bước có thể chạy: "+strings.Join(pipes, "; ")+".")
	}
	routeParts = append(routeParts, ternaryVi(ac.Route.EmitToDecisionQueue, "Cho phép tiếp tục xử lý trên hàng chờ nội bộ.", "Không tạo thêm việc trên hàng chờ cho bước phản chiếu này."))
	out = append(out, decisionlive.DecisionLiveProcessNode{
		Kind:     decisionlive.ProcessTraceKindStep,
		Key:      "dc_route_collection",
		LabelVi:  "Chọn cách xử lý phù hợp với loại dữ liệu",
		DetailVi: truncateProcessTraceDetail(strings.Join(routeParts, " ")),
	})

	deferLine := fmt.Sprintf("Gộp khách: %s · Báo cáo: %s · Làm mới phân tích khách: %s · Phân tích tin nhắn: %s · Phân tích đơn: %s",
		formatDeferWindowVi(ac.IngestWin),
		formatDeferWindowVi(ac.ReportWin),
		formatDeferWindowVi(ac.RefreshWin),
		formatDeferWindowVi(ac.CixIntelDefer),
		formatDeferWindowVi(ac.OrderIntelDefer),
	)
	out = append(out, decisionlive.DecisionLiveProcessNode{
		Kind:     decisionlive.ProcessTraceKindStep,
		Key:      "dc_defer_windows",
		LabelVi:  "Lịch xử lý tiếp theo (có thể gom hoặc chờ một lúc)",
		DetailVi: truncateProcessTraceDetail(deferLine),
	})

	out = append(out, decisionlive.DecisionLiveProcessNode{
		Kind:     decisionlive.ProcessTraceKindStep,
		Key:      "dc_run_sidefx",
		LabelVi:  "Thực hiện các bước đồng bộ đã bật",
		DetailVi: truncateProcessTraceDetail("Ví dụ: cập nhật hàng chờ gộp khách, báo cáo, phân tích tin nhắn hoặc đơn — tùy loại dữ liệu và cài đặt tài khoản bạn."),
	})

	return out
}

func ternaryVi(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func wrapQueueConsumerRoot(children []decisionlive.DecisionLiveProcessNode) []decisionlive.DecisionLiveProcessNode {
	if len(children) == 0 {
		return nil
	}
	return []decisionlive.DecisionLiveProcessNode{
		{
			Kind:     decisionlive.ProcessTraceKindBranch,
			Key:      "queue_consumer",
			LabelVi:  "Trợ lý đang xử lý từng bước",
			Children: children,
		},
	}
}

// queueTraceForProcessingStart — Mốc ngay sau khi hệ thống nhận việc.
func queueTraceForProcessingStart(evt *aidecisionmodels.DecisionEvent) []decisionlive.DecisionLiveProcessNode {
	if evt == nil {
		return nil
	}
	friendly := livecopy.QueueFriendlyEventLabel(evt)
	detail := fmt.Sprintf("Loại cập nhật: %s.", friendly)
	if eid := strings.TrimSpace(evt.EventID); eid != "" {
		detail += fmt.Sprintf(" Mã tham chiếu khi cần hỗ trợ: %s.", eid)
	}
	if tid := strings.TrimSpace(evt.TraceID); tid != "" {
		detail += fmt.Sprintf(" Mã luồng: %s.", tid)
	}
	step := decisionlive.DecisionLiveProcessNode{
		Kind:     decisionlive.ProcessTraceKindStep,
		Key:      "lease_acquired",
		LabelVi:  "Đã bắt đầu xử lý yêu cầu của bạn",
		DetailVi: truncateProcessTraceDetail(detail),
	}
	return wrapQueueConsumerRoot([]decisionlive.DecisionLiveProcessNode{step})
}

// queueProcessTracer — Gom các bước con (theo thứ tự thời gian).
type queueProcessTracer struct {
	evt      *aidecisionmodels.DecisionEvent
	children []decisionlive.DecisionLiveProcessNode
}

func newQueueProcessTracer(evt *aidecisionmodels.DecisionEvent) *queueProcessTracer {
	t := &queueProcessTracer{evt: evt}
	if evt == nil {
		return t
	}
	friendly := livecopy.QueueFriendlyEventLabel(evt)
	if friendly == "" {
		friendly = "Cập nhật hệ thống"
	}
	detail := fmt.Sprintf("Bạn vừa có: %s. Hệ thống đã xếp việc này vào hàng chờ và xử lý lần lượt để an toàn dữ liệu.", friendly)
	t.children = append(t.children, decisionlive.DecisionLiveProcessNode{
		Kind:     decisionlive.ProcessTraceKindStep,
		Key:      "queue_envelope",
		LabelVi:  fmt.Sprintf("Nhận: %s", friendly),
		DetailVi: truncateProcessTraceDetail(detail),
	})
	return t
}

func (t *queueProcessTracer) noteDatachangedSideEffects(children []decisionlive.DecisionLiveProcessNode) {
	node := decisionlive.DecisionLiveProcessNode{
		Kind:     decisionlive.ProcessTraceKindStep,
		Key:      "datachanged_side_effects",
		LabelVi:  "Sau khi bạn lưu: đồng bộ các thay đổi phát sinh",
		DetailVi: "Chi tiết từng bước nằm trong danh sách bên dưới.",
		Children: children,
	}
	if len(children) == 0 {
		node.DetailVi = "Chưa đủ thông tin để mô tả chi tiết (thiếu dữ liệu hoặc không đọc được bản ghi)."
		node.Children = nil
	}
	t.children = append(t.children, node)
}

func (t *queueProcessTracer) noteRoutingSkipped() {
	t.children = append(t.children, decisionlive.DecisionLiveProcessNode{
		Kind:     decisionlive.ProcessTraceKindDecision,
		Key:      "routing_noop",
		LabelVi:  "Theo cài đặt, lần này không chạy xử lý tự động sâu hơn",
		DetailVi: "Việc vẫn được đánh dấu xong; không có thêm hành động tự động cho loại cập nhật này.",
	})
}

func (t *queueProcessTracer) noteRoutingAllowDispatch() {
	t.children = append(t.children, decisionlive.DecisionLiveProcessNode{
		Kind:     decisionlive.ProcessTraceKindStep,
		Key:      "routing_allow",
		LabelVi:  "Tiếp tục xử lý tự động",
		DetailVi: "",
	})
}

func (t *queueProcessTracer) noteDispatchLookup() {
	et := ""
	if t.evt != nil {
		et = strings.TrimSpace(t.evt.EventType)
	}
	t.children = append(t.children, decisionlive.DecisionLiveProcessNode{
		Kind:     decisionlive.ProcessTraceKindStep,
		Key:      "dispatch_lookup",
		LabelVi:  "Xác định bước phù hợp",
		DetailVi: truncateProcessTraceDetail(queueConsumerHandlerUserExplanationVi(et)),
	})
}

func (t *queueProcessTracer) noteNoHandlerRegistered() {
	t.children = append(t.children, decisionlive.DecisionLiveProcessNode{
		Kind:     decisionlive.ProcessTraceKindOutcome,
		Key:      "no_handler",
		LabelVi:  "Chưa có bước tự động cho loại cập nhật này",
		DetailVi: "Đội vận hành có thể bật thêm sau. Dữ liệu gốc của bạn vẫn được lưu bình thường.",
	})
}

func (t *queueProcessTracer) noteHandlerInvoke() {
	et := ""
	if t.evt != nil {
		et = strings.TrimSpace(t.evt.EventType)
	}
	t.children = append(t.children, decisionlive.DecisionLiveProcessNode{
		Kind:     decisionlive.ProcessTraceKindStep,
		Key:      "handler_run",
		LabelVi:  "Đang chạy bước nghiệp vụ",
		DetailVi: truncateProcessTraceDetail(queueConsumerHandlerUserExplanationVi(et)),
	})
}

func (t *queueProcessTracer) noteHandlerSuccess() {
	t.children = append(t.children, decisionlive.DecisionLiveProcessNode{
		Kind:    decisionlive.ProcessTraceKindOutcome,
		Key:     "handler_ok",
		LabelVi: "Hoàn tất, không phát sinh lỗi",
	})
}

func (t *queueProcessTracer) noteHandlerError(err error) {
	d := truncateProcessTraceErr(err.Error())
	t.children = append(t.children, decisionlive.DecisionLiveProcessNode{
		Kind:     decisionlive.ProcessTraceKindError,
		Key:      "handler_error",
		LabelVi:  "Có lỗi trong lúc xử lý",
		DetailVi: d,
	})
}

func (t *queueProcessTracer) snapshotTree() []decisionlive.DecisionLiveProcessNode {
	if t == nil || len(t.children) == 0 {
		return nil
	}
	cp := make([]decisionlive.DecisionLiveProcessNode, len(t.children))
	copy(cp, t.children)
	return wrapQueueConsumerRoot(cp)
}

// queueConsumerHandlerUserExplanationVi — Mô tả việc đang làm bằng tiếng Việt dễ hiểu (processTrace cho người dùng cuối).
func queueConsumerHandlerUserExplanationVi(eventType string) string {
	et := strings.TrimSpace(eventType)
	switch et {
	case eventtypes.AdsIntelligenceRecomputeRequested:
		return "Sắp tính toán lại số liệu quảng cáo trên máy chủ chuyên trách (không chặn màn hình của bạn)."
	case eventtypes.AdsIntelligenceRecalculateAllRequested:
		return "Sắp làm mới hàng loạt số liệu quảng cáo theo tổ chức của bạn."
	case crmqueue.EventTypeCrmIntelligenceComputeRequested:
		return "Sắp cập nhật chỉ số / bức tranh khách hàng trên máy chủ chuyên trách."
	case crmqueue.EventTypeCrmIntelligenceRecomputeRequested:
		return "Sắp tính lại thông tin khách; có thể gom nhiều thay đổi trong thời gian ngắn."
	case eventtypes.CixAnalysisRequested:
		return "Sắp phân tích nội dung hội thoại để gợi ý việc tiếp theo phù hợp."
	case eventtypes.CustomerContextReady:
		return "Thông tin khách đã sẵn sàng — hệ thống cập nhật hồ sơ xử lý."
	case eventtypes.AdsContextRequested:
		return "Đang thu thập bức tranh quảng cáo mới nhất để đánh giá và gợi ý."
	case eventtypes.AdsContextReady:
		return "Đã có số liệu quảng cáo đủ để đánh giá và đề xuất hành động (nếu phù hợp)."
	case eventtypes.ExecutorProposeRequested, eventtypes.AdsProposeRequested:
		return "Chuẩn bị đề xuất hoặc việc cần làm theo quy tắc của bạn."
	case eventtypes.AIDecisionExecuteRequested:
		return "Điều phối thực thi lệnh đã được duyệt (chi tiết hiển thị trên luồng xử lý riêng)."
	case intelrecomputed.EventTypeOrderIntelRecomputed:
		return "Kết quả phân tích đơn hàng mới đã về — cập nhật hồ sơ xử lý."
	case intelrecomputed.EventTypeCrmIntelRecomputed:
		return "Kết quả phân tích khách mới đã về — cập nhật hồ sơ xử lý."
	case intelrecomputed.EventTypeCixIntelRecomputed:
		return "Kết quả phân tích hội thoại mới đã về — cập nhật gợi ý tiếp theo."
	case eventtypes.CampaignIntelRecomputed:
		return "Số liệu chiến dịch vừa được làm mới — có thể có bước gợi ý quảng cáo sau."
	case eventtypes.MetaCampaignChanged, eventtypes.MetaCampaignInserted, eventtypes.MetaCampaignUpdated:
		return "Đồng bộ thông tin chiến dịch từ Meta để báo cáo và gợi ý luôn mới."
	case eventtypes.MessageChanged, eventtypes.MessageInserted, eventtypes.MessageUpdated, eventtypes.ConversationMessageInserted:
		return "Cập nhật theo tin nhắn mới — có thể gom nhiều tin trong chốc lát để xử lý gọn."
	case eventtypes.ConversationChanged, eventtypes.ConversationInserted, eventtypes.ConversationUpdated:
		return "Cập nhật theo hội thoại — hệ thống sắp xếp thời điểm xử lý hợp lý."
	case eventtypes.OrderChanged, eventtypes.OrderInserted, eventtypes.OrderUpdated:
		return "Cập nhật theo đơn hàng — có thể kèm phân tích hoặc hồ sơ liên quan."
	default:
		if strings.HasPrefix(et, eventtypes.PrefixMessage) || strings.HasPrefix(et, eventtypes.PrefixConversation) {
			return "Xử lý liên quan tin nhắn hoặc hội thoại của bạn."
		}
		if strings.HasPrefix(et, eventtypes.PrefixOrder) {
			return "Xử lý liên quan đơn hàng của bạn."
		}
		return "Thực hiện bước tự động phù hợp với loại cập nhật này."
	}
}
