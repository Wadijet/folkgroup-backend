// Package config — Metadata cho frontend hiển thị config UI/UX (FolkForm v4.1).
package config

import (
	adsmodels "meta_commerce/internal/api/ads/models"
)

// Operator chuẩn (theo Meta Ad Rules, json-rules-engine). Dùng trong FlagConditionItem.
const (
	OpGreaterThan        = "GREATER_THAN"         // fact > value
	OpLessThan           = "LESS_THAN"             // fact < value
	OpGreaterThanOrEqual = "GREATER_THAN_OR_EQUAL" // fact >= value
	OpLessThanOrEqual    = "LESS_THAN_OR_EQUAL"   // fact <= value
	OpEqual              = "EQUAL"                // fact = value (số hoặc chuỗi)
	OpNotEqual           = "NOT_EQUAL"             // fact != value
	OpIn                 = "IN"                   // fact in [value1, value2, ...] (ValueStr: "a,b,c")
	OpNotIn              = "NOT_IN"               // fact not in list
)

// AllOperators danh sách operator hợp lệ cho frontend/validation.
var AllOperators = []string{
	OpGreaterThan, OpLessThan, OpGreaterThanOrEqual, OpLessThanOrEqual,
	OpEqual, OpNotEqual, OpIn, OpNotIn,
}

// ThresholdMetadata mô tả một ngưỡng (tham số trong điều kiện set cờ). Chỉ mô tả metric + so sánh, KHÔNG mô tả action.
type ThresholdMetadata struct {
	Key          string  `json:"key"`
	Label        string  `json:"label"`
	Description  string  `json:"description"`  // Điều kiện: metric so sánh với X (vd: "cpa_mess > X"), không đề cập Kill/Decrease
	Unit         string  `json:"unit"`        // VND, %, phút, số
	Min          float64 `json:"min,omitempty"`  // Giá trị tối thiểu (validation)
	Max          float64 `json:"max,omitempty"`  // Giá trị tối đa (validation)
	Step         float64 `json:"step,omitempty"` // Bước nhảy cho slider/input
	DefaultValue float64 `json:"defaultValue"`   // Giá trị mặc định (FolkForm v4.1) — nguồn cho InitDefaultConfig
	Group        string  `json:"group"`           // stop_loss, kill_off, mess_trap, trim, chs, base
	Order        int     `json:"order"`           // Thứ tự hiển thị trong nhóm
}

// MetricMetadata định nghĩa một chỉ số (metric) — nguồn dữ liệu, công thức tính, đơn vị.
// Metrics là đầu vào cho điều kiện set cờ; không phải ngưỡng hay action.
type MetricMetadata struct {
	Key         string `json:"key"`                   // Mã metric: cpa_mess, cpm, ctr, mess, orders, ...
	Label       string `json:"label"`                  // Label hiển thị
	Description string `json:"description"`            // Công thức / nguồn dữ liệu (vd: "spend / mess")
	Unit        string `json:"unit"`                   // VND, %, số, phút
	Source      string `json:"source,omitempty"`        // meta, pancake/pos, layer1, layer3
	Order       int    `json:"order"`                  // Thứ tự hiển thị
}

// FlagConditionItem — alias adsmodels.FlagConditionItem.
type FlagConditionItem = adsmodels.FlagConditionItem

// FlagDefinition — alias adsmodels.FlagDefinition. Cấu trúc duy nhất cho định nghĩa cờ (config + API metadata).
type FlagDefinition = adsmodels.FlagDefinition

// CommonFieldMetadata mô tả một field trong CommonConfig cho UI.
type CommonFieldMetadata struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Unit        string `json:"unit,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	InputType   string `json:"inputType,omitempty"` // text, number, select
	Order       int    `json:"order"`
}

// RuleCodeMetadata mô tả ACTION_RULE: cờ → hành động (PAUSE, DECREASE). Dùng cho actionRuleConfig.
// Khác với FlagMetadata: đây là flag → action, không phải metrics → flag.
type RuleCodeMetadata struct {
	Code             string `json:"code"`
	Label            string `json:"label"`
	ShortLabel       string `json:"shortLabel"`
	Category         string `json:"category"`   // stop_loss, kill_off, mess_trap, trim, chs
	ActionType       string `json:"actionType"` // PAUSE, DECREASE
	Description      string `json:"description"` // Mô tả hành động khi cờ trigger
	Order            int    `json:"order"`
	AutoProposeDefault bool `json:"autoProposeDefault"` // Mặc định: đề xuất khi rule trigger
	AutoApproveDefault bool `json:"autoApproveDefault"` // Mặc định: tự động phê duyệt (mess_trap_suspect, trim_eligible_decrease)
}

// AutomationFieldMetadata mô tả field automation cho UI.
type AutomationFieldMetadata struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Options     []string `json:"options,omitempty"` // Danh sách rule codes có thể chọn
}

// DynamicFlagMetadata mô tả cờ động — code = prefix + giá trị từ mảng. VD: diagnosis_xxx từ layer3.diagnoses[].
type DynamicFlagMetadata struct {
	CodePrefix    string `json:"codePrefix"`    // Tiền tố: "diagnosis_"
	SourceArray   string `json:"sourceArray"`   // Đường dẫn mảng: "diagnoses" (trong layer3)
	LabelTemplate string `json:"labelTemplate"` // "Diagnosis: {value}"
	Description   string `json:"description"`   // Mô tả: mỗi phần tử trong mảng tạo 1 cờ
}

// FlagRuleConfigMetadata metadata cho flagRuleConfig — định nghĩa metrics, thresholds, flags.
// Luồng: Metrics (chỉ số) → so sánh với Thresholds (ngưỡng) → điều kiện kết hợp → set Flag. Không có action.
type FlagRuleConfigMetadata struct {
	Operators    []string                 `json:"operators"`    // GREATER_THAN, LESS_THAN, EQUAL, IN, ... (chuẩn Meta)
	Metrics      []MetricMetadata         `json:"metrics"`      // Định nghĩa các chỉ số: nguồn, công thức, đơn vị
	Thresholds   []ThresholdMetadata     `json:"thresholds"`   // Ngưỡng cấu hình (tham số trong điều kiện)
	TrimWindow   []CommonFieldMetadata    `json:"trimWindow"`   // Trim window: trimStartHour, trimEndHour (fact inTrimWindow)
	Flags        []adsmodels.FlagDefinition `json:"flags"`        // Định nghĩa từng cờ (cùng cấu trúc FlagRuleConfig.flagDefinitions)
	DynamicFlags []DynamicFlagMetadata    `json:"dynamicFlags"` // Cờ động: code = prefix + value từ mảng
}

// ActionRuleConfigMetadata metadata cho actionRuleConfig.
type ActionRuleConfigMetadata struct {
	RuleCodes []RuleCodeMetadata `json:"ruleCodes"` // Chi tiết từng rule (kill, decrease)

	// ExceptionFlagsForKill: cờ khi có thì bỏ qua kill (FolkForm: safety_net, conv_rate_strong)
	ExceptionFlagsForKill []string `json:"exceptionFlagsForKill"`
	// ExceptionFlagsForDecrease: cờ khi có thì bỏ qua decrease
	ExceptionFlagsForDecrease []string `json:"exceptionFlagsForDecrease"`
}

// ConfigMetadata metadata đầy đủ cho frontend render config form (labels, groups, validation).
type ConfigMetadata struct {
	CommonConfig     []CommonFieldMetadata     `json:"commonConfig"`
	FlagRuleConfig   FlagRuleConfigMetadata   `json:"flagRuleConfig"`   // Chi tiết từng chỉ tiêu trong thresholds
	ActionRuleConfig ActionRuleConfigMetadata `json:"actionRuleConfig"` // Chi tiết từng rule
	AutomationConfig []AutomationFieldMetadata `json:"automationConfig"`

	// Deprecated: dùng flagRuleConfig.thresholds. Giữ để backward compat.
	Thresholds []ThresholdMetadata `json:"thresholds,omitempty"`
	// Deprecated: dùng actionRuleConfig.ruleCodes. Giữ để backward compat.
	RuleCodes []RuleCodeMetadata `json:"ruleCodes,omitempty"`
}

// GetConfigMetadata trả về metadata tĩnh cho frontend. Không lưu DB.
func GetConfigMetadata() ConfigMetadata {
	thresholds := ThresholdDefinitions()
	ruleCodes := ActionRuleDefinitions()
	return ConfigMetadata{
		CommonConfig: commonConfigMetadataList(),
		FlagRuleConfig: FlagRuleConfigMetadata{
			Operators:    AllOperators,
			Metrics:      MetricDefinitions(),
			Thresholds:   thresholds,
			Flags:        FlagDefinitions(),
			DynamicFlags: DynamicFlagDefinitions(),
		},
		ActionRuleConfig: ActionRuleConfigMetadata{
			RuleCodes:                ruleCodes,
			ExceptionFlagsForKill:    []string{"safety_net", "conv_rate_strong"},
			ExceptionFlagsForDecrease: []string{"conv_rate_strong"},
		},
		AutomationConfig: []AutomationFieldMetadata{
			{
				Key:         "autoProposeEnabled",
				Label:       "Bật auto-propose",
				Description: "Bật/tắt tự động tạo đề xuất cho ad account.",
			},
			{
				Key:         "killRulesEnabled",
				Label:       "Bật Kill Rules",
				Description: "Công tắc kill rules. FALSE → skip SL-D, SL-E, CHS Kill, KO-B (vd: Pancake down).",
			},
			{
				Key:         "budgetRulesEnabled",
				Label:       "Bật Budget Rules",
				Description: "Công tắc nhóm tăng/giảm ngân sách. FALSE → skip DECREASE/INCREASE (tắt khẩn cấp).",
			},
			{
				Key:         "actionRuleConfig",
				Label:       "Cấu hình từng hành động",
				Description: "KillRules và DecreaseRules — mỗi rule có AutoPropose và AutoApprove. Cấu hình trong actionRuleConfig.",
				Options:     AllRuleCodeStrings(),
			},
			{
				Key:         "onboardingMode",
				Label:       "Chế độ Onboarding (14 ngày)",
				Description: "FolkForm v4.1 PATCH 01: 14 ngày đầu deploy dùng threshold nới (CPA Kill 250k, CPA Pur 1.4M) tránh kill nhầm. Bật khi mới deploy.",
			},
			{
				Key:         "onboardingDeployedAt",
				Label:       "Thời điểm bật Onboarding (ms)",
				Description: "Timestamp (ms) khi bật onboarding. Nếu set, sau 14 ngày tự coi như hết onboarding. 0 = giữ cho đến khi tắt thủ công.",
			},
		},
		Thresholds: thresholds, // Backward compat
		RuleCodes:  ruleCodes,  // Backward compat
	}
}

// MetricDefinitions định nghĩa các chỉ số (metrics) — nguồn dữ liệu, công thức. Nguồn duy nhất.
func MetricDefinitions() []MetricMetadata {
	return []MetricMetadata{
		// Meta (raw.meta)
		{Key: "spend", Label: "Spend", Description: "Chi phí quảng cáo (Meta insights)", Unit: "VND", Source: "meta", Order: 1},
		{Key: "mess", Label: "Mess", Description: "Số cuộc hội thoại (messaging_conversation_started)", Unit: "số", Source: "meta", Order: 2},
		{Key: "impressions", Label: "Impressions", Description: "Số lần hiển thị quảng cáo", Unit: "số", Source: "meta", Order: 3},
		{Key: "inlineLinkClicks", Label: "Inline Link Clicks", Description: "Số click link (dùng tính msg_rate)", Unit: "số", Source: "meta", Order: 4},
		{Key: "cpm", Label: "CPM", Description: "Cost per 1000 impressions (Meta)", Unit: "VND", Source: "meta", Order: 5},
		{Key: "ctr", Label: "CTR", Description: "Click-through rate (Meta)", Unit: "%", Source: "meta", Order: 6},
		{Key: "frequency", Label: "Frequency", Description: "Số lần trung bình mỗi user thấy quảng cáo", Unit: "số", Source: "meta", Order: 7},
		{Key: "deliveryStatus", Label: "Delivery Status", Description: "Trạng thái giao hàng: ACTIVE, LIMITED, NOT_DELIVERING", Unit: "", Source: "meta", Order: 8},
		{Key: "dailyBudget", Label: "Daily Budget", Description: "Ngân sách hàng ngày (campaign level)", Unit: "VND", Source: "meta", Order: 9},
		// Pancake (raw.pancake.pos)
		{Key: "orders", Label: "Orders", Description: "Số đơn hàng từ Pancake (pos)", Unit: "số", Source: "pancake", Order: 10},
		{Key: "revenue", Label: "Revenue", Description: "Doanh thu từ đơn hàng", Unit: "VND", Source: "pancake", Order: 11},
		// Layer1 (tính từ raw) — tên thể hiện rõ chu kỳ theo FolkForm v4.1
		{Key: "cpaMess_7d", Label: "CPA Mess (7 ngày)", Description: "spend / mess — chi phí mỗi cuộc hội thoại", Unit: "VND", Source: "layer1", Order: 20},
		{Key: "cpaPurchase_7d", Label: "CPA Purchase (7 ngày)", Description: "spend / orders — chi phí mỗi đơn hàng", Unit: "VND", Source: "layer1", Order: 21},
		{Key: "convRate_7d", Label: "Conv Rate (7 ngày)", Description: "orders / mess — tỷ lệ chuyển đổi mess → đơn", Unit: "%", Source: "layer1", Order: 22},
		{Key: "convRate_2h", Label: "Conv Rate (2 giờ)", Description: "orders_2h / mess_2h — CR_now cho Momentum Tracker", Unit: "%", Source: "layer1", Order: 23},
		{Key: "convRate_1h", Label: "Conv Rate (1 giờ)", Description: "orders_1h / mess_1h — HB-3 Divergence", Unit: "%", Source: "layer1", Order: 24},
		{Key: "msgRate_7d", Label: "Msg Rate (7 ngày)", Description: "mess / inlineLinkClicks — tỷ lệ click → mess", Unit: "%", Source: "layer1", Order: 25},
		{Key: "mqs_7d", Label: "MQS (7 ngày)", Description: "Mess Quality Score: mess × conv_rate_7d", Unit: "số", Source: "layer1", Order: 26},
		{Key: "roas_7d", Label: "ROAS (7 ngày)", Description: "revenue / spend", Unit: "số", Source: "layer1", Order: 27},
		{Key: "spendPct_7d", Label: "Spend % (7 ngày)", Description: "spend / daily_budget — % ngân sách đã dùng", Unit: "%", Source: "layer1", Order: 28},
		{Key: "runtimeMinutes", Label: "Runtime (phút)", Description: "Thời gian chạy quảng cáo (phút)", Unit: "phút", Source: "layer1", Order: 29},
		// Layer3 (CHS, health)
		{Key: "chs", Label: "CHS", Description: "Campaign Health Score — điểm sức khỏe camp", Unit: "số", Source: "layer3", Order: 30},
		{Key: "healthState", Label: "Health State", Description: "Trạng thái: critical, warning, healthy", Unit: "", Source: "layer3", Order: 31},
		{Key: "portfolioCell", Label: "Portfolio Cell", Description: "Ô trong ma trận portfolio", Unit: "", Source: "layer3", Order: 32},
		// Derived (tính từ nhiều nguồn)
		{Key: "cpm_3day_avg", Label: "CPM 3 ngày TB", Description: "CPM trung bình 3 ngày gần nhất (dùng KO-C, SL-C)", Unit: "VND", Source: "derived", Order: 40},
		{Key: "inTrimWindow", Label: "Trong khung Trim", Description: "true khi giờ hiện tại trong [trimStartHour, trimEndHour) từ config", Unit: "", Source: "derived", Order: 41},
	}
}

// DynamicFlagDefinitions định nghĩa cờ động — mỗi phần tử trong mảng nguồn tạo 1 cờ. Nguồn duy nhất.
func DynamicFlagDefinitions() []DynamicFlagMetadata {
	return []DynamicFlagMetadata{
		{CodePrefix: "diagnosis_", SourceArray: "diagnoses", LabelTemplate: "Diagnosis: {value}", Description: "Mỗi chuỗi trong layer3.diagnoses[] tạo cờ diagnosis_<value>"},
	}
}

// DefaultFlagDefinitions trả về định nghĩa đầy đủ từng cờ. Evaluator CHỈ đọc từ đây hoặc từ config.
func DefaultFlagDefinitions() []adsmodels.FlagDefinition {
	return buildFlagDefinitions()
}

// GetFlagDefinitions trả về flag definitions từ config. Rỗng = dùng DefaultFlagDefinitions(). Evaluator CHỈ gọi hàm này.
func GetFlagDefinitions(cfg *adsmodels.CampaignConfigView) []adsmodels.FlagDefinition {
	if cfg != nil && len(cfg.FlagRuleConfig.FlagDefinitions) > 0 {
		return cfg.FlagRuleConfig.FlagDefinitions
	}
	return DefaultFlagDefinitions()
}

// FlagDefinitions trả về định nghĩa cờ cho API metadata. Cùng cấu trúc FlagDefinition (FlagRuleConfig.flagDefinitions).
func FlagDefinitions() []adsmodels.FlagDefinition {
	return buildFlagDefinitions()
}

// buildFlagDefinitions nguồn duy nhất — trả về []FlagDefinition với metricsUsed, logicText, conditionGroups.
func buildFlagDefinitions() []adsmodels.FlagDefinition {
	v0, v1, v2, v3, v60, v800 := 0.0, 1.0, 2.0, 3.0, 60.0, 800.0
	v0_08, v0_12, v0_45 := 0.08, 0.12, 0.45
	out := []adsmodels.FlagDefinition{
		{Code: "cpa_mess_high", Label: "CPA Mess cao", Description: "CPA_Mess(7d) > ngưỡng VÀ mess > 0.", DocReference: "Rule 01, 09",
			MetricsUsed: []string{"cpaMess_7d", "mess"}, LogicText: "cpaMess_7d > cpaMessKill AND mess > 0",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "cpaMess_7d", Operator: OpGreaterThan, ThresholdKey: KeyCpaMessKill}, {Fact: "mess", Operator: OpGreaterThan, Value: &v0}},
			}, Group: "metric", Order: 1},
		{Code: "cpa_purchase_high", Label: "CPA Purchase cao", Description: "CPA_Purchase(7d) > 1.050k VÀ orders > 0.", DocReference: "Rule 01 — SL-E",
			MetricsUsed: []string{"cpaPurchase_7d", "orders"}, LogicText: "cpaPurchase_7d > cpaPurchaseHardStop AND orders > 0",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "cpaPurchase_7d", Operator: OpGreaterThan, ThresholdKey: KeyCpaPurchaseHardStop}, {Fact: "orders", Operator: OpGreaterThan, Value: &v0}},
			}, Group: "metric", Order: 2},
		{Code: "conv_rate_low", Label: "Conv Rate thấp", Description: "Conv_Rate(7d) < 5% VÀ mess >= 15.", DocReference: "Rule 01 — SL-D, Rule 11",
			MetricsUsed: []string{"convRate_7d", "mess"}, LogicText: "convRate_7d < convRateMessTrap AND mess >= messTrapSlDMin",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "convRate_7d", Operator: OpLessThan, ThresholdKey: KeyConvRateMessTrap}, {Fact: "mess", Operator: OpGreaterThanOrEqual, ThresholdKey: KeyMessTrapSlDMin}},
			}, Group: "metric", Order: 3},
		{Code: "ctr_critical", Label: "CTR thảm họa", Description: "CTR < 0.35%.", DocReference: "Rule 01 — SL-C, Rule 06 — KO-B",
			MetricsUsed: []string{"ctr"}, LogicText: "ctr < ctrKill",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "ctr", Operator: OpLessThan, ThresholdKey: KeyCtrKill}},
			}, Group: "metric", Order: 4},
		{Code: "msg_rate_low", Label: "Msg Rate thấp", Description: "Msg_Rate(7d) < 2%.", DocReference: "Rule 01 — SL-C, Rule 06 — KO-B",
			MetricsUsed: []string{"msgRate_7d"}, LogicText: "msgRate_7d < msgRateLow",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "msgRate_7d", Operator: OpLessThan, ThresholdKey: KeyMsgRateLow}},
			}, Group: "metric", Order: 5},
		{Code: "cpm_low", Label: "CPM thấp", Description: "CPM < 60k. Dấu hiệu nguy hiểm với Folk Form.", DocReference: "Rule 11 — MESS TRAP",
			MetricsUsed: []string{"cpm"}, LogicText: "cpm < cpmMessTrapLow",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "cpm", Operator: OpLessThan, ThresholdKey: KeyCpmMessTrapLow}},
			}, Group: "metric", Order: 6},
		{Code: "cpm_high", Label: "CPM cao", Description: "CPM > ngưỡng (3day_avg × 1.5x hoặc 2.5x).", DocReference: "Rule 01 — SL-C, Rule 06 — KO-C",
			MetricsUsed: []string{"cpm"}, LogicText: "cpm > cpmHigh",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "cpm", Operator: OpGreaterThan, ThresholdKey: KeyCpmHigh}},
			}, Group: "metric", Order: 7},
		{Code: "frequency_high", Label: "Frequency cao", Description: "Freq > 2.2.", DocReference: "Rule 05 — TRIM",
			MetricsUsed: []string{"frequency"}, LogicText: "frequency > frequencyHigh",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "frequency", Operator: OpGreaterThan, ThresholdKey: KeyFrequencyHigh}},
			}, Group: "metric", Order: 8},
		// CHS — OR: healthState=critical HOẶC chs < threshold
		{Code: "chs_critical", Label: "CHS Critical", Description: "CHS > 2.0 trong 2 checkpoint (60p) VÀ Spend > 25% VÀ MQS < 1.5.", DocReference: "Rule 01 — SL-F",
			MetricsUsed: []string{"healthState", "chs"}, LogicText: "healthState = critical OR chs < chsWarningThreshold",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "healthState", Operator: OpEqual, ValueStr: "critical"}},
				{{Fact: "chs", Operator: OpLessThan, ThresholdKey: KeyChsWarningThreshold}},
			}, Group: "chs", Order: 10},
		{Code: "chs_warning", Label: "CHS Warning", Description: "CHS trong vùng warning (1.3–1.8).", DocReference: "Rule 09 — DECREASE",
			MetricsUsed: []string{"healthState", "chs"}, LogicText: "healthState = warning OR (chs >= chsWarningThreshold AND chs < 60)",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "healthState", Operator: OpEqual, ValueStr: "warning"}},
				{{Fact: "chs", Operator: OpGreaterThanOrEqual, ThresholdKey: KeyChsWarningThreshold}, {Fact: "chs", Operator: OpLessThan, Value: &v60}},
			}, Group: "chs", Order: 11},
		// Stop Loss — base = spendPct > spendPctBase AND runtimeMinutes > runtimeMinutesBase
		{Code: "sl_a", Label: "SL-A", Description: "CPA_Mess(7d) > 180k VÀ Mess < 3 VÀ MQS(7d) < 1. Exception: MQS ≥ 2 → Decrease 20%.", DocReference: "Rule 01 — SL-A",
			MetricsUsed: []string{"spendPct_7d", "runtimeMinutes", "cpaMess_7d", "mess", "mqs_7d"}, LogicText: "spendPct_7d > 20% AND runtimeMinutes > 90 AND cpaMess_7d > 180k AND mess < 3 AND mqs_7d < 1",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "spendPct_7d", Operator: OpGreaterThan, ThresholdKey: KeySpendPctBase, SpendPctFallback: true}, {Fact: "runtimeMinutes", Operator: OpGreaterThan, ThresholdKey: KeyRuntimeMinutesBase}, {Fact: "cpaMess_7d", Operator: OpGreaterThan, ThresholdKey: KeyCpaMessKill}, {Fact: "mess", Operator: OpLessThan, Value: &v3}, {Fact: "mqs_7d", Operator: OpLessThan, ThresholdKey: KeyMqsSlAMax}},
			}, Group: "stop_loss", Order: 20},
		{Code: "sl_a_decrease", Label: "SL-A Decrease", Description: "Cùng SL-A nhưng MQS(7d) ≥ 2 → Decrease 20% thay vì Kill.", DocReference: "Rule 01 — SL-A Exception",
			MetricsUsed: []string{"spendPct_7d", "runtimeMinutes", "cpaMess_7d", "mess", "mqs_7d"}, LogicText: "spendPct_7d > 20% AND runtimeMinutes > 90 AND cpaMess_7d > 180k AND mess < 3 AND mqs_7d >= 2",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "spendPct_7d", Operator: OpGreaterThan, ThresholdKey: KeySpendPctBase, SpendPctFallback: true}, {Fact: "runtimeMinutes", Operator: OpGreaterThan, ThresholdKey: KeyRuntimeMinutesBase}, {Fact: "cpaMess_7d", Operator: OpGreaterThan, ThresholdKey: KeyCpaMessKill}, {Fact: "mess", Operator: OpLessThan, Value: &v3}, {Fact: "mqs_7d", Operator: OpGreaterThanOrEqual, ThresholdKey: KeyMqsSlADecreaseMin}},
			}, Group: "stop_loss", Order: 21},
		{Code: "sl_b", Label: "SL-B", Description: "Spend > 30% (NORMAL) / 20% (BLITZ/PROTECT) VÀ Mess = 0.", DocReference: "Rule 01 — SL-B",
			MetricsUsed: []string{"spendPct_7d", "runtimeMinutes", "mess"}, LogicText: "spendPct_7d > 30% (NORMAL) hoặc 20% (BLITZ/PROTECT) AND runtimeMinutes > 90 AND mess = 0",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "spendPct_7d", Operator: OpGreaterThan, ThresholdKey: KeySpendPctSlB, ThresholdKeyByMode: "BLITZ,PROTECT:" + KeySpendPctSlBBlitz, SpendPctFallback: true}, {Fact: "runtimeMinutes", Operator: OpGreaterThan, ThresholdKey: KeyRuntimeMinutesBase}, {Fact: "mess", Operator: OpEqual, Value: &v0}},
			}, Group: "stop_loss", Order: 22},
		{Code: "sl_c", Label: "SL-C", Description: "CTR < 0.35% VÀ Spend > 15% VÀ CPM > 3day_avg × 1.5x VÀ Msg_Rate(7d) < 2%.", DocReference: "Rule 01 — SL-C",
			MetricsUsed: []string{"spendPct_7d", "runtimeMinutes", "ctr", "cpm", "msgRate_7d"}, LogicText: "spendPct_7d > 20% AND runtimeMinutes > 90 AND ctr < 0.35% AND spendPct_7d > 15% AND cpm > cpmHigh AND msgRate_7d < 2%",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "spendPct_7d", Operator: OpGreaterThan, ThresholdKey: KeySpendPctBase, SpendPctFallback: true}, {Fact: "runtimeMinutes", Operator: OpGreaterThan, ThresholdKey: KeyRuntimeMinutesBase}, {Fact: "ctr", Operator: OpLessThan, ThresholdKey: KeyCtrKill}, {Fact: "spendPct_7d", Operator: OpGreaterThan, ThresholdKey: KeySpendPctSlC, SpendPctFallback: true}, {Fact: "cpm", Operator: OpGreaterThan, ThresholdKey: KeyCpmHigh}, {Fact: "msgRate_7d", Operator: OpLessThan, ThresholdKey: KeyMsgRateLow}},
			}, Group: "stop_loss", Order: 23},
		{Code: "sl_d", Label: "SL-D", Description: "Mess ≥ 15 VÀ Conv_Rate(7d) < 5% VÀ Spend > 20%. Mess Trap pattern.", DocReference: "Rule 01 — SL-D",
			MetricsUsed: []string{"spendPct_7d", "runtimeMinutes", "mess", "convRate_7d"}, LogicText: "spendPct_7d > 20% AND runtimeMinutes > 90 AND mess >= 15 AND convRate_7d < 5% AND spendPct_7d > 20%",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "spendPct_7d", Operator: OpGreaterThan, ThresholdKey: KeySpendPctBase, SpendPctFallback: true}, {Fact: "runtimeMinutes", Operator: OpGreaterThan, ThresholdKey: KeyRuntimeMinutesBase}, {Fact: "mess", Operator: OpGreaterThanOrEqual, ThresholdKey: KeyMessTrapSlDMin}, {Fact: "convRate_7d", Operator: OpLessThan, ThresholdKey: KeyConvRateMessTrap}, {Fact: "spendPct_7d", Operator: OpGreaterThan, ThresholdKey: KeySpendPctSlD, SpendPctFallback: true}},
			}, Group: "stop_loss", Order: 24},
		{Code: "sl_e", Label: "SL-E", Description: "CPA_Purchase(7d) > 1.050k VÀ orders ≥ 3 VÀ CR(7d) < 10% VÀ MQS(7d) < 1.", DocReference: "Rule 01 — SL-E",
			MetricsUsed: []string{"spendPct_7d", "runtimeMinutes", "cpaPurchase_7d", "orders", "convRate_7d", "mqs_7d"}, LogicText: "spendPct_7d > 20% AND runtimeMinutes > 90 AND cpaPurchase_7d > 1.050k AND orders >= 3 AND convRate_7d < 10% AND mqs_7d < 1",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "spendPct_7d", Operator: OpGreaterThan, ThresholdKey: KeySpendPctBase, SpendPctFallback: true}, {Fact: "runtimeMinutes", Operator: OpGreaterThan, ThresholdKey: KeyRuntimeMinutesBase}, {Fact: "cpaPurchase_7d", Operator: OpGreaterThan, ThresholdKey: KeyCpaPurchaseHardStop}, {Fact: "orders", Operator: OpGreaterThanOrEqual, ThresholdKey: KeySlEOrdersMin}, {Fact: "convRate_7d", Operator: OpLessThan, ThresholdKey: KeySlECrMax}, {Fact: "mqs_7d", Operator: OpLessThan, ThresholdKey: KeyMqsSlEMax}},
			}, Group: "stop_loss", Order: 25},
		// Kill Off
		{Code: "ko_a", Label: "KO-A", Description: "Delivery = Limited/Not Delivering VÀ Runtime > 120p VÀ Spend(7d) < 8%.", DocReference: "Rule 06 — KO-A",
			MetricsUsed: []string{"deliveryStatus", "runtimeMinutes", "spendPct_7d"}, LogicText: "deliveryStatus IN (LIMITED, NOT_DELIVERING) AND runtimeMinutes > 120 AND spendPct_7d > 0 AND spendPct_7d < 8%",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "deliveryStatus", Operator: OpIn, ValueStr: "LIMITED,NOT_DELIVERING"}, {Fact: "runtimeMinutes", Operator: OpGreaterThan, ThresholdKey: KeyRuntimeMinutesKoA}, {Fact: "spendPct_7d", Operator: OpGreaterThan, Value: &v0}, {Fact: "spendPct_7d", Operator: OpLessThan, ThresholdKey: KeySpendPctKoAMax}},
			}, Group: "kill_off", Order: 30},
		{Code: "ko_b", Label: "KO-B", Description: "CTR > 1.8% VÀ Msg_Rate(7d) < 2% VÀ Spend(7d) > 15% VÀ MQS(7d) < 0.5. Dual-source: Pancake = 0.", DocReference: "Rule 06 — KO-B",
			MetricsUsed: []string{"ctr", "msgRate_7d", "orders", "spendPct_7d", "mqs_7d"}, LogicText: "ctr > 1.8% AND msgRate_7d < 2% AND orders = 0 AND spendPct_7d > 15% AND mqs_7d < 0.5",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "ctr", Operator: OpGreaterThan, ThresholdKey: KeyCtrTrafficRac}, {Fact: "msgRate_7d", Operator: OpLessThan, ThresholdKey: KeyMsgRateLow}, {Fact: "orders", Operator: OpEqual, Value: &v0}, {Fact: "spendPct_7d", Operator: OpGreaterThan, ThresholdKey: KeySpendPctKoB, SpendPctFallback: true}, {Fact: "mqs_7d", Operator: OpLessThan, ThresholdKey: KeyMqsKoBMax}},
			}, Group: "kill_off", Order: 31},
		{Code: "ko_c", Label: "KO-C", Description: "CPM > 3day_avg × 2.5x VÀ Impressions < 800 VÀ Spend(7d) > 10%.", DocReference: "Rule 06 — KO-C",
			MetricsUsed: []string{"cpm", "impressions", "spendPct_7d"}, LogicText: "cpm > cpmHigh × cpmKoCMultiplier AND impressions < 800 AND spendPct_7d > 10%",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "cpm", Operator: OpGreaterThan, ThresholdKey: KeyCpmHigh, ThresholdKey2: KeyCpmKoCMultiplier}, {Fact: "impressions", Operator: OpLessThan, Value: &v800}, {Fact: "spendPct_7d", Operator: OpGreaterThan, ThresholdKey: KeySpendPctKoC, SpendPctFallback: true}},
			}, Group: "kill_off", Order: 32},
		// Mess Trap
		{Code: "mess_trap_suspect", Label: "Mess Trap Suspect", Description: "Early Warning: CPA_Mess(7d) < 60k VÀ CR(7d) < 6% VÀ Spend(7d) > 15% VÀ orders = 0.", DocReference: "Rule 11 — MESS TRAP",
			MetricsUsed: []string{"cpaMess_7d", "convRate_7d", "mess", "orders", "spendPct_7d"}, LogicText: "cpaMess_7d < 60k AND convRate_7d < 6% AND mess >= 20 AND orders = 0 AND spendPct_7d > 15%",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "cpaMess_7d", Operator: OpLessThan, ThresholdKey: KeyCpaMessTrapLow}, {Fact: "convRate_7d", Operator: OpLessThan, ThresholdKey: KeyConvRateMessTrap6}, {Fact: "mess", Operator: OpGreaterThanOrEqual, ThresholdKey: KeyMessTrapSuspectMin}, {Fact: "orders", Operator: OpEqual, Value: &v0}, {Fact: "spendPct_7d", Operator: OpGreaterThan, ThresholdKey: KeySpendPctMessTrap, SpendPctFallback: true}},
			}, Group: "mess_trap", Order: 40},
		// Trim — inTrimWindow = true khi trong khung giờ (config trimStartHour, trimEndHour)
		{Code: "trim_eligible", Label: "Trim Kill", Description: "Freq > 2.2 VÀ CHS > 1.7 VÀ orders < 3. Trong khung Trim 14h–20h.", DocReference: "Rule 05 — TRIM",
			MetricsUsed: []string{"inTrimWindow", "frequency", "chs", "orders"}, LogicText: "inTrimWindow = true AND frequency > 2.2 AND chs < 60 AND orders < 3",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "inTrimWindow", Operator: OpEqual, Value: &v1}, {Fact: "frequency", Operator: OpGreaterThan, ThresholdKey: KeyFrequencyTrim}, {Fact: "chs", Operator: OpLessThan, Value: &v60}, {Fact: "orders", Operator: OpLessThan, ThresholdKey: KeyTrimOrdersMin}},
			}, Group: "trim", Order: 50},
		{Code: "trim_eligible_decrease", Label: "Trim Decrease", Description: "Freq > 2.2 VÀ CHS 1.3–1.7 VÀ orders ≥ 3. Chỉ Decrease 30%.", DocReference: "Rule 05 — TRIM Exception",
			MetricsUsed: []string{"inTrimWindow", "frequency", "chs", "orders"}, LogicText: "inTrimWindow = true AND frequency > 2.2 AND chs < 60 AND orders >= 3",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "inTrimWindow", Operator: OpEqual, Value: &v1}, {Fact: "frequency", Operator: OpGreaterThan, ThresholdKey: KeyFrequencyTrim}, {Fact: "chs", Operator: OpLessThan, Value: &v60}, {Fact: "orders", Operator: OpGreaterThanOrEqual, ThresholdKey: KeyTrimOrdersMin}},
			}, Group: "trim", Order: 51},
		// Exception
		{Code: "safety_net", Label: "Safety Net", Description: "orders ≥ 3 VÀ CPA_Purchase(7d) < 1.050k VÀ CR(7d) > 10% VÀ CHS < 1.5. Bảo vệ camp.", DocReference: "Rule 04 — SAFETY NET",
			MetricsUsed: []string{"orders", "convRate_7d", "chs"}, LogicText: "orders >= 3 AND convRate_7d >= 10% AND chs >= 60",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "orders", Operator: OpGreaterThanOrEqual, ThresholdKey: KeySafetyNetOrdersMin}, {Fact: "convRate_7d", Operator: OpGreaterThanOrEqual, ThresholdKey: KeySafetyNetCrMin}, {Fact: "chs", Operator: OpGreaterThanOrEqual, Value: &v60}},
			}, Group: "exception", Order: 60},
		{Code: "conv_rate_strong", Label: "Conv Rate Strong", Description: "Conv_Rate(7d) > 20%. Exception: KHÔNG kill dù CPA_Mess cao.", DocReference: "Rule 01 — Exception",
			MetricsUsed: []string{"convRate_7d"}, LogicText: "convRate_7d >= 20%",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "convRate_7d", Operator: OpGreaterThanOrEqual, ThresholdKey: KeyConvRateStrong}},
			}, Group: "exception", Order: 61},
		// Increase eligible — R08: CR_7day > 12%, Freq < 2.0, Spend > 45%, CHS healthy (>= 60 trong scale 0-100)
		{Code: "increase_eligible", Label: "Increase Eligible", Description: "CR(7d) > 12% VÀ Freq < 2.0 VÀ Spend > 45% VÀ CHS healthy. Camp tốt — tăng budget.", DocReference: "Rule 08 — INCREASE",
			MetricsUsed: []string{"convRate_7d", "frequency", "spendPct_7d", "chs"}, LogicText: "convRate_7d > 12% AND frequency < 2.0 AND spendPct_7d > 45% AND chs >= 60",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "convRate_7d", Operator: OpGreaterThan, Value: &v0_12}, {Fact: "frequency", Operator: OpLessThan, Value: &v2}, {Fact: "spendPct_7d", Operator: OpGreaterThan, Value: &v0_45}, {Fact: "chs", Operator: OpGreaterThanOrEqual, Value: &v60}},
			}, Group: "increase", Order: 70},
		// Portfolio — OR: portfolioCell=fix HOẶC portfolioCell=recover
		{Code: "portfolio_attention", Label: "Portfolio Attention", Description: "Camp trong ô fix hoặc recover của ma trận portfolio.", DocReference: "Portfolio Matrix",
			MetricsUsed: []string{"portfolioCell"}, LogicText: "portfolioCell = fix OR portfolioCell = recover",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "portfolioCell", Operator: OpEqual, ValueStr: "fix"}},
				{{Fact: "portfolioCell", Operator: OpEqual, ValueStr: "recover"}},
			}, Group: "portfolio", Order: 62},
		// Morning On — R02: Camp đủ điều kiện bật lại sáng (MO-A)
		{Code: "mo_eligible", Label: "Morning On Eligible", Description: "CPA_Mess < 216k VÀ CR >= 8% VÀ CHS healthy VÀ orders >= 1 VÀ mess >= 3 VÀ freq < 3.0.", DocReference: "Rule 02 — MORNING ON",
			MetricsUsed: []string{"cpaMess_7d", "convRate_7d", "chs", "orders", "mess", "frequency"}, LogicText: "cpaMess_7d < cpaMessMoMax AND convRate_7d >= 8% AND chs >= 60 AND orders >= 1 AND mess >= 3 AND frequency < 3.0",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "cpaMess_7d", Operator: OpLessThan, ThresholdKey: KeyCpaMessMoMax}, {Fact: "convRate_7d", Operator: OpGreaterThanOrEqual, Value: &v0_08}, {Fact: "chs", Operator: OpGreaterThanOrEqual, Value: &v60}, {Fact: "orders", Operator: OpGreaterThanOrEqual, Value: &v1}, {Fact: "mess", Operator: OpGreaterThanOrEqual, Value: &v3}, {Fact: "frequency", Operator: OpLessThan, Value: &v3}},
			}, Group: "morning_on", Order: 80},
		// Noon Cut — R03: Camp chết buổi trưa (tắt 12:30/14:00, bật lại 14:30)
		{Code: "noon_cut_eligible", Label: "Noon Cut Eligible", Description: "CPA_Mess > 144k VÀ Spend < 55% VÀ CHS yếu (warning/critical). Tắt camp chết trưa.", DocReference: "Rule 03 — NOON CUT",
			MetricsUsed: []string{"cpaMess_7d", "spendPct_7d", "chs", "healthState"}, LogicText: "cpaMess_7d > cpaMessNoonCutMin AND spendPct_7d < spendPctNoonCutMax AND spendPct_7d > 20% AND (healthState = warning OR healthState = critical)",
			ConditionGroups: [][]adsmodels.FlagConditionItem{
				{{Fact: "cpaMess_7d", Operator: OpGreaterThan, ThresholdKey: KeyCpaMessNoonCutMin}, {Fact: "spendPct_7d", Operator: OpLessThan, ThresholdKey: KeySpendPctNoonCutMax, SpendPctFallback: true}, {Fact: "spendPct_7d", Operator: OpGreaterThan, ThresholdKey: KeySpendPctBase, SpendPctFallback: true}, {Fact: "healthState", Operator: OpIn, ValueStr: "warning,critical"}},
			}, Group: "noon_cut", Order: 81},
	}
	// Backward compat: MetricKey = Fact
	for i := range out {
		for j := range out[i].ConditionGroups {
			for k := range out[i].ConditionGroups[j] {
				if out[i].ConditionGroups[j][k].MetricKey == "" && out[i].ConditionGroups[j][k].Fact != "" {
					out[i].ConditionGroups[j][k].MetricKey = out[i].ConditionGroups[j][k].Fact
				}
			}
		}
	}
	return out
}

// ThresholdDefinitions định nghĩa metadata ngưỡng. Nguồn duy nhất. DefaultValue dùng cho InitDefaultConfig.
func ThresholdDefinitions() []ThresholdMetadata {
	return []ThresholdMetadata{
		// Stop Loss — mô tả điều kiện metric so sánh, không đề cập action
		{Key: KeyCpaMessKill, Label: "CPA Mess ngưỡng (đ)", Description: "Điều kiện: cpa_mess > X (dùng trong sl_a, sl_a_decrease, cpa_mess_high)", Unit: "VND", Min: 50000, Max: 300000, Step: 10000, DefaultValue: 180_000, Group: "stop_loss", Order: 1},
		{Key: KeyCpaPurchaseHardStop, Label: "CPA Purchase ngưỡng (đ)", Description: "Điều kiện: cpa_purchase > X (dùng trong sl_e, cpa_purchase_high)", Unit: "VND", Min: 500000, Max: 1500000, Step: 50000, DefaultValue: 1_050_000, Group: "stop_loss", Order: 2},
		{Key: KeySpendPctSlB, Label: "Spend % SL-B (NORMAL)", Description: "Điều kiện: spend_pct > X (NORMAL mode)", Unit: "%", Min: 0.1, Max: 0.5, Step: 0.05, DefaultValue: 0.30, Group: "stop_loss", Order: 3},
		{Key: KeySpendPctSlBBlitz, Label: "Spend % SL-B (BLITZ/PROTECT)", Description: "Điều kiện: spend_pct > X khi accountMode BLITZ/PROTECT", Unit: "%", Min: 0.1, Max: 0.4, Step: 0.05, DefaultValue: 0.20, Group: "stop_loss", Order: 4},
		{Key: KeySpendPctSlC, Label: "Spend % SL-C", Description: "Điều kiện: spend_pct > X", Unit: "%", Min: 0.05, Max: 0.3, Step: 0.05, DefaultValue: 0.15, Group: "stop_loss", Order: 5},
		{Key: KeySpendPctSlD, Label: "Spend % SL-D", Description: "Điều kiện: spend_pct > X", Unit: "%", Min: 0.1, Max: 0.4, Step: 0.05, DefaultValue: 0.20, Group: "stop_loss", Order: 6},
		{Key: KeyMqsSlAMax, Label: "MQS SL-A max", Description: "Điều kiện: mqs < X (set sl_a)", Unit: "số", Min: 0.5, Max: 2, Step: 0.1, DefaultValue: 1.0, Group: "stop_loss", Order: 7},
		{Key: KeyMqsSlADecreaseMin, Label: "MQS SL-A Decrease min", Description: "Điều kiện: mqs >= X (set sl_a_decrease thay vì sl_a)", Unit: "số", Min: 1, Max: 4, Step: 0.5, DefaultValue: 2.0, Group: "stop_loss", Order: 8},
		{Key: KeySlEOrdersMin, Label: "SL-E Orders min", Description: "Điều kiện: orders >= X", Unit: "số", Min: 1, Max: 10, Step: 1, DefaultValue: 3, Group: "stop_loss", Order: 9},
		{Key: KeySlECrMax, Label: "SL-E CR max", Description: "Điều kiện: conv_rate < X", Unit: "%", Min: 0.05, Max: 0.2, Step: 0.01, DefaultValue: 0.10, Group: "stop_loss", Order: 10},
		{Key: KeyMqsSlEMax, Label: "MQS SL-E max", Description: "Điều kiện: mqs < X", Unit: "số", Min: 0.5, Max: 2, Step: 0.1, DefaultValue: 1.0, Group: "stop_loss", Order: 11},
		// Kill Off
		{Key: KeyCtrKill, Label: "CTR ngưỡng", Description: "Điều kiện: ctr < X", Unit: "%", Min: 0.002, Max: 0.01, Step: 0.0005, DefaultValue: 0.0035, Group: "kill_off", Order: 1},
		{Key: KeyMsgRateLow, Label: "Msg Rate thấp", Description: "Điều kiện: msg_rate < X", Unit: "%", Min: 0.01, Max: 0.05, Step: 0.005, DefaultValue: 0.02, Group: "kill_off", Order: 2},
		{Key: KeyCtrTrafficRac, Label: "CTR Traffic rác", Description: "Điều kiện: ctr > X (kết hợp msg_rate thấp)", Unit: "%", Min: 0.01, Max: 0.03, Step: 0.002, DefaultValue: 0.018, Group: "kill_off", Order: 3},
		{Key: KeyCpmHigh, Label: "CPM cao", Description: "Điều kiện: cpm > X", Unit: "VND", Min: 100000, Max: 300000, Step: 10000, DefaultValue: 180_000, Group: "kill_off", Order: 4},
		{Key: KeyCpmKoCMultiplier, Label: "CPM KO-C multiplier", Description: "Điều kiện: cpm > cpm_3day_avg × X", Unit: "số", Min: 1.5, Max: 4, Step: 0.5, DefaultValue: 2.5, Group: "kill_off", Order: 5},
		{Key: KeySpendPctKoAMax, Label: "Spend % KO-A max", Description: "Điều kiện: spend_pct < X", Unit: "%", Min: 0.05, Max: 0.15, Step: 0.01, DefaultValue: 0.08, Group: "kill_off", Order: 6},
		{Key: KeyRuntimeMinutesKoA, Label: "Runtime KO-A (phút)", Description: "Điều kiện: runtime_minutes > X", Unit: "phút", Min: 60, Max: 180, Step: 15, DefaultValue: 120, Group: "kill_off", Order: 7},
		{Key: KeySpendPctKoB, Label: "Spend % KO-B", Description: "Điều kiện: spend_pct > X", Unit: "%", Min: 0.1, Max: 0.3, Step: 0.05, DefaultValue: 0.15, Group: "kill_off", Order: 8},
		{Key: KeySpendPctKoC, Label: "Spend % KO-C", Description: "Điều kiện: spend_pct > X", Unit: "%", Min: 0.05, Max: 0.2, Step: 0.05, DefaultValue: 0.10, Group: "kill_off", Order: 9},
		{Key: KeyMqsKoBMax, Label: "MQS KO-B max", Description: "Điều kiện: mqs < X", Unit: "số", Min: 0.3, Max: 1, Step: 0.1, DefaultValue: 0.5, Group: "kill_off", Order: 10},
		// Mess Trap
		{Key: KeyConvRateMessTrap, Label: "Conv Rate Mess Trap", Description: "Điều kiện: conv_rate < X", Unit: "%", Min: 0.02, Max: 0.1, Step: 0.01, DefaultValue: 0.05, Group: "mess_trap", Order: 1},
		{Key: KeyConvRateMessTrap6, Label: "Conv Rate Mess Trap 6h", Description: "Điều kiện: conv_rate < X", Unit: "%", Min: 0.03, Max: 0.12, Step: 0.01, DefaultValue: 0.06, Group: "mess_trap", Order: 2},
		{Key: KeyMessTrapSlDMin, Label: "Mess min SL-D", Description: "Điều kiện: mess >= X", Unit: "số", Min: 10, Max: 30, Step: 1, DefaultValue: 15, Group: "mess_trap", Order: 3},
		{Key: KeyMessTrapSuspectMin, Label: "Mess Trap Suspect min", Description: "Điều kiện: mess >= X", Unit: "số", Min: 15, Max: 40, Step: 1, DefaultValue: 20, Group: "mess_trap", Order: 4},
		{Key: KeyCpmMessTrapLow, Label: "CPM Mess Trap thấp", Description: "Điều kiện: cpm < X", Unit: "VND", Min: 40000, Max: 100000, Step: 5000, DefaultValue: 60_000, Group: "mess_trap", Order: 5},
		{Key: KeyCpaMessTrapLow, Label: "CPA Mess Trap thấp", Description: "Điều kiện: cpa_mess < X", Unit: "VND", Min: 40000, Max: 100000, Step: 5000, DefaultValue: 60_000, Group: "mess_trap", Order: 6},
		{Key: KeySpendPctMessTrap, Label: "Spend % Mess Trap", Description: "Điều kiện: spend_pct > X", Unit: "%", Min: 0.1, Max: 0.3, Step: 0.05, DefaultValue: 0.15, Group: "mess_trap", Order: 7},
		// Trim
		{Key: KeyFrequencyHigh, Label: "Frequency cao", Description: "Điều kiện: frequency > X", Unit: "số", Min: 2.5, Max: 4, Step: 0.1, DefaultValue: 3.0, Group: "trim", Order: 1},
		{Key: KeyFrequencyTrim, Label: "Frequency Trim", Description: "Điều kiện: frequency > X", Unit: "số", Min: 1.8, Max: 3, Step: 0.1, DefaultValue: 2.2, Group: "trim", Order: 2},
		{Key: KeyTrimOrdersMin, Label: "Trim Orders min", Description: "Điều kiện: orders >= X → trim_eligible_decrease; orders < X → trim_eligible", Unit: "số", Min: 1, Max: 10, Step: 1, DefaultValue: 3, Group: "trim", Order: 3},
		// CHS
		{Key: KeyChsWarningThreshold, Label: "CHS Warning", Description: "Điều kiện: chs < X → chs_critical; chs trong [X, 60) → chs_warning", Unit: "số", Min: 30, Max: 50, Step: 5, DefaultValue: 40, Group: "chs", Order: 1},
		{Key: KeyMqsChsKillMax, Label: "MQS CHS max", Description: "Điều kiện: mqs < X (kết hợp chs_critical)", Unit: "số", Min: 1, Max: 2.5, Step: 0.1, DefaultValue: 1.5, Group: "chs", Order: 2},
		// Safety Net
		{Key: KeySafetyNetOrdersMin, Label: "Safety Net Orders min", Description: "Điều kiện: orders >= X", Unit: "số", Min: 2, Max: 6, Step: 1, DefaultValue: 3, Group: "safety_net", Order: 1},
		{Key: KeySafetyNetCrMin, Label: "Safety Net CR min", Description: "Điều kiện: conv_rate >= X", Unit: "%", Min: 0.08, Max: 0.15, Step: 0.01, DefaultValue: 0.10, Group: "safety_net", Order: 2},
		// Base condition
		{Key: KeySpendPctBase, Label: "Spend % Base", Description: "BASE: spend_pct > X (điều kiện chung cho SL rules)", Unit: "%", Min: 0.1, Max: 0.4, Step: 0.05, DefaultValue: 0.20, Group: "base", Order: 1},
		{Key: KeyRuntimeMinutesBase, Label: "Runtime Base (phút)", Description: "BASE: runtime_minutes > X (điều kiện chung cho SL rules)", Unit: "phút", Min: 60, Max: 180, Step: 15, DefaultValue: 90, Group: "base", Order: 2},
		// Exception
		{Key: KeyConvRateStrong, Label: "Conv Rate Strong", Description: "Điều kiện: conv_rate >= X → set conv_rate_strong (bảo vệ, bỏ qua kill)", Unit: "%", Min: 0.15, Max: 0.3, Step: 0.01, DefaultValue: 0.20, Group: "exception", Order: 1},
		// Morning On / Noon Cut
		{Key: KeyCpaMessMoMax, Label: "CPA Mess MO max (đ)", Description: "MO-A: CPA_Mess < X (camp tốt, được bật lại sáng)", Unit: "VND", Min: 100000, Max: 300000, Step: 10000, DefaultValue: 216_000, Group: "morning_on", Order: 1},
		{Key: KeyCpaMessNoonCutMin, Label: "CPA Mess Noon Cut min (đ)", Description: "Noon Cut: CPA_Mess > X (camp đắt, tắt trưa)", Unit: "VND", Min: 100000, Max: 200000, Step: 5000, DefaultValue: 144_000, Group: "noon_cut", Order: 1},
		{Key: KeySpendPctNoonCutMax, Label: "Spend % Noon Cut max", Description: "Noon Cut: Spend < X% (chưa tiêu hết budget)", Unit: "%", Min: 0.3, Max: 0.7, Step: 0.05, DefaultValue: 0.55, Group: "noon_cut", Order: 2},
	}
}

// DefaultThresholds trả về map ngưỡng mặc định từ ThresholdDefinitions. Nguồn duy nhất cho InitDefaultConfig.
func DefaultThresholds() map[string]float64 {
	out := make(map[string]float64)
	for _, t := range ThresholdDefinitions() {
		out[t.Key] = t.DefaultValue
	}
	return out
}

func commonConfigMetadataList() []CommonFieldMetadata {
	return []CommonFieldMetadata{
		{Key: "timezone", Label: "Múi giờ", Description: "Múi giờ cho Trim window và tính toán.", Unit: "", Placeholder: "Asia/Ho_Chi_Minh", InputType: "text", Order: 1},
	}
}

func trimWindowMetadataList() []CommonFieldMetadata {
	return []CommonFieldMetadata{
		{Key: "trimStartHour", Label: "Trim bắt đầu (giờ)", Description: "Trim chỉ chạy từ giờ này (14–20h mặc định).", Unit: "giờ", Placeholder: "14", InputType: "number", Order: 1},
		{Key: "trimEndHour", Label: "Trim kết thúc (giờ)", Description: "Trim chỉ chạy đến giờ này.", Unit: "giờ", Placeholder: "20", InputType: "number", Order: 2},
	}
}

// AllRuleCodeStrings trả về danh sách mã rule cho automation, lấy từ ActionRuleDefinitions.
func AllRuleCodeStrings() []string {
	codes := ActionRuleDefinitions()
	seen := make(map[string]bool)
	var out []string
	for _, c := range codes {
		if c.Code != "" && !seen[c.Code] {
			seen[c.Code] = true
			out = append(out, c.Code)
		}
	}
	return out
}

// ActionRuleSpec đầy đủ cho evaluation và automation: flag(s) → action + autoPropose, autoApprove.
// Gộp logic action và cài đặt automation (từ ActionRuleDefinitions).
type ActionRuleSpec struct {
	Flag         string   // Single flag (dùng khi RequireFlags rỗng)
	RequireFlags []string // Compound: TẤT CẢ phải có
	RuleCode     string   // Mã rule (sl_a, chs_warning, ...)
	Action       string   // PAUSE, DECREASE
	Reason       string   // Lý do hiển thị
	Value        float64  // % cho DECREASE (20, 30, 15)
	Freeze       bool     // Bỏ qua khi KillRulesEnabled=false
	Priority     int      // Thứ tự ưu tiên
	Label        string   // Label hiển thị (gộp từ Automation)
	AutoPropose  bool     // Tự động đề xuất khi trigger
	AutoApprove  bool     // Tự động phê duyệt
}

// DefaultKillRuleSpecs trả về kill rules mặc định (FolkForm v4.1). Gộp Label, AutoPropose, AutoApprove.
func DefaultKillRuleSpecs() []ActionRuleSpec {
	return []ActionRuleSpec{
		{Flag: "sl_a", RuleCode: "sl_a", Action: "PAUSE", Reason: "Hệ thống đề xuất [SL-A]: CPA mess cao, mess thấp, MQS thấp — Stop Loss", Freeze: false, Priority: 1, Label: "SL-A: CPA Mess + MQS", AutoPropose: true, AutoApprove: false},
		{Flag: "sl_b", RuleCode: "sl_b", Action: "PAUSE", Reason: "Hệ thống đề xuất [SL-B]: Có spend nhưng 0 mess — Blitz/Protect", Freeze: false, Priority: 2, Label: "SL-B: Đốt tiền 0 mess", AutoPropose: true, AutoApprove: false},
		{Flag: "sl_c", RuleCode: "sl_c", Action: "PAUSE", Reason: "Hệ thống đề xuất [SL-C]: CTR thảm họa, CPM tăng bất thường", Freeze: false, Priority: 3, Label: "SL-C: CTR thảm họa", AutoPropose: true, AutoApprove: false},
		{Flag: "sl_d", RuleCode: "sl_d", Action: "PAUSE", Reason: "Hệ thống đề xuất [SL-D]: Mess Trap — mess đủ mẫu nhưng CR thấp", Freeze: true, Priority: 4, Label: "SL-D: Mess Trap", AutoPropose: true, AutoApprove: false},
		{Flag: "sl_e", RuleCode: "sl_e", Action: "PAUSE", Reason: "Hệ thống đề xuất [SL-E]: CPA Purchase vượt ngưỡng, CR thấp", Freeze: true, Priority: 5, Label: "SL-E: CPA Purchase", AutoPropose: true, AutoApprove: false},
		{Flag: "chs_critical", RuleCode: "chs_critical", Action: "PAUSE", Reason: "Hệ thống đề xuất [CHS]: Camp Health Score critical 2 checkpoint liên tiếp", Freeze: true, Priority: 6, Label: "CHS Critical", AutoPropose: true, AutoApprove: false},
		{Flag: "ko_a", RuleCode: "ko_a", Action: "PAUSE", Reason: "Hệ thống đề xuất [KO-A]: Không delivery — LIMITED/NOT_DELIVERING", Freeze: false, Priority: 7, Label: "KO-A: Không delivery", AutoPropose: true, AutoApprove: false},
		{Flag: "ko_b", RuleCode: "ko_b", Action: "PAUSE", Reason: "Hệ thống đề xuất [KO-B]: Traffic rác — CTR cao, msg rate thấp, 0 đơn", Freeze: true, Priority: 8, Label: "KO-B: Traffic rác", AutoPropose: true, AutoApprove: false},
		{Flag: "ko_c", RuleCode: "ko_c", Action: "PAUSE", Reason: "Hệ thống đề xuất [KO-C]: CPM bất thường, impressions thấp", Freeze: false, Priority: 9, Label: "KO-C: CPM bất thường", AutoPropose: true, AutoApprove: false},
		{Flag: "trim_eligible", RuleCode: "trim_eligible", Action: "PAUSE", Reason: "Hệ thống đề xuất [Trim]: Frequency cao, CHS trung bình — Kill", Freeze: false, Priority: 10, Label: "Trim: Kill", AutoPropose: true, AutoApprove: false},
	}
}

// DefaultDecreaseRuleSpecs trả về decrease rules mặc định (FolkForm v4.1). Gộp Label, AutoPropose, AutoApprove.
func DefaultDecreaseRuleSpecs() []ActionRuleSpec {
	return []ActionRuleSpec{
		{Flag: "sl_a_decrease", RuleCode: "sl_a_decrease", Action: "DECREASE", Value: 20, Reason: "Hệ thống đề xuất [SL-A]: CPA mess cao nhưng MQS >= 2 — giảm budget 20% thay vì kill", Priority: 1, Label: "SL-A: Decrease", AutoPropose: true, AutoApprove: false},
		{Flag: "mess_trap_suspect", RuleCode: "mess_trap_suspect", Action: "DECREASE", Value: 30, Reason: "Hệ thống đề xuất [Mess Trap]: Nghi ngờ bẫy mess — giảm budget 30%", Priority: 2, Label: "Mess Trap Suspect", AutoPropose: true, AutoApprove: false},
		{Flag: "trim_eligible_decrease", RuleCode: "trim_eligible_decrease", Action: "DECREASE", Value: 30, Reason: "Hệ thống đề xuất [Trim]: Frequency cao, có đơn — giảm budget 30% thay vì kill", Priority: 3, Label: "Trim: Decrease", AutoPropose: true, AutoApprove: false},
		{RequireFlags: []string{"chs_warning", "cpa_mess_high"}, RuleCode: "chs_warning", Action: "DECREASE", Value: 15, Reason: "Hệ thống đề xuất [CHS Warning]: CPA mess cao, CHS warning — giảm budget 15%", Priority: 4, Label: "CHS Warning (compound)", AutoPropose: true, AutoApprove: false},
	}
}

// DefaultIncreaseRuleSpecs trả về increase rules mặc định (FolkForm v4.1 R08). Camp tốt → tăng budget.
func DefaultIncreaseRuleSpecs() []ActionRuleSpec {
	return []ActionRuleSpec{
		{Flag: "increase_eligible", RuleCode: "increase_eligible", Action: "INCREASE", Value: 30, Reason: "Hệ thống đề xuất [Increase]: Camp tốt — CR > 12%, CHS < 1.3, tăng budget 30%", Priority: 1, Label: "Increase: Camp tốt", AutoPropose: true, AutoApprove: false},
		{Flag: "safety_net", RuleCode: "increase_safety_net", Action: "INCREASE", Value: 35, Reason: "Hệ thống đề xuất [Increase]: Safety Net — camp tốt, tăng 35%", Priority: 2, Label: "Increase: Safety Net", AutoPropose: true, AutoApprove: false},
	}
}

// ActionRuleDefinitions định nghĩa metadata action rules (cờ → action). Nguồn duy nhất.
// AutoProposeDefault, AutoApproveDefault dùng cho DefaultAutomationActionRules (InitDefaultConfig).
func ActionRuleDefinitions() []RuleCodeMetadata {
	return []RuleCodeMetadata{
		// Kill rules
		{Code: "sl_a", Label: "SL-A: CPA Mess + MQS", ShortLabel: "SL-A", Category: "stop_loss", ActionType: "PAUSE", Description: "CPA mess cao, mess thấp, MQS thấp — Stop Loss", Order: 1, AutoProposeDefault: true, AutoApproveDefault: false},
		{Code: "sl_b", Label: "SL-B: Đốt tiền 0 mess", ShortLabel: "SL-B", Category: "stop_loss", ActionType: "PAUSE", Description: "Có spend nhưng 0 mess — Blitz/Protect", Order: 2, AutoProposeDefault: true, AutoApproveDefault: false},
		{Code: "sl_c", Label: "SL-C: CTR thảm họa", ShortLabel: "SL-C", Category: "stop_loss", ActionType: "PAUSE", Description: "CTR thảm họa, CPM tăng bất thường", Order: 3, AutoProposeDefault: true, AutoApproveDefault: false},
		{Code: "sl_d", Label: "SL-D: Mess Trap", ShortLabel: "SL-D", Category: "stop_loss", ActionType: "PAUSE", Description: "Mess đủ mẫu nhưng CR thấp", Order: 4, AutoProposeDefault: true, AutoApproveDefault: false},
		{Code: "sl_e", Label: "SL-E: CPA Purchase", ShortLabel: "SL-E", Category: "stop_loss", ActionType: "PAUSE", Description: "CPA Purchase vượt ngưỡng, CR thấp", Order: 5, AutoProposeDefault: true, AutoApproveDefault: false},
		{Code: "chs_critical", Label: "CHS Critical", ShortLabel: "CHS", Category: "chs", ActionType: "PAUSE", Description: "Camp Health Score critical 2 checkpoint liên tiếp", Order: 6, AutoProposeDefault: true, AutoApproveDefault: false},
		{Code: "ko_a", Label: "KO-A: Không delivery", ShortLabel: "KO-A", Category: "kill_off", ActionType: "PAUSE", Description: "Limited/NOT_DELIVERING", Order: 7, AutoProposeDefault: true, AutoApproveDefault: false},
		{Code: "ko_b", Label: "KO-B: Traffic rác", ShortLabel: "KO-B", Category: "kill_off", ActionType: "PAUSE", Description: "CTR cao, msg rate thấp, 0 đơn", Order: 8, AutoProposeDefault: true, AutoApproveDefault: false},
		{Code: "ko_c", Label: "KO-C: CPM bất thường", ShortLabel: "KO-C", Category: "kill_off", ActionType: "PAUSE", Description: "CPM bất thường, impressions thấp", Order: 9, AutoProposeDefault: true, AutoApproveDefault: false},
		{Code: "trim_eligible", Label: "Trim: Kill", ShortLabel: "Trim", Category: "trim", ActionType: "PAUSE", Description: "Frequency cao, CHS trung bình — Kill", Order: 10, AutoProposeDefault: true, AutoApproveDefault: false},
		// Decrease rules
		{Code: "sl_a_decrease", Label: "SL-A: Decrease", ShortLabel: "SL-A ↓", Category: "stop_loss", ActionType: "DECREASE", Description: "CPA mess cao nhưng MQS >= 2 — giảm 20% thay vì kill", Order: 11, AutoProposeDefault: true, AutoApproveDefault: false},
		{Code: "mess_trap_suspect", Label: "Mess Trap Suspect", ShortLabel: "Mess Trap ↓", Category: "mess_trap", ActionType: "DECREASE", Description: "Nghi ngờ bẫy mess — giảm 30%", Order: 12, AutoProposeDefault: true, AutoApproveDefault: false},
		{Code: "trim_eligible_decrease", Label: "Trim: Decrease", ShortLabel: "Trim ↓", Category: "trim", ActionType: "DECREASE", Description: "Frequency cao, có đơn — giảm 30% thay vì kill", Order: 13, AutoProposeDefault: true, AutoApproveDefault: false},
		// Compound rule: chs_warning + cpa_mess_high
		{Code: "chs_warning", Label: "CHS Warning (compound)", ShortLabel: "CHS ↓", Category: "chs", ActionType: "DECREASE", Description: "CPA mess cao + CHS warning — giảm 15%", Order: 14, AutoProposeDefault: true, AutoApproveDefault: false},
	}
}

// DefaultCommonConfig trả về CommonConfig mặc định (FolkForm v4.1). Nguồn duy nhất cho InitDefaultConfig.
func DefaultCommonConfig() adsmodels.CommonConfig {
	return adsmodels.CommonConfig{
		Timezone: "Asia/Ho_Chi_Minh",

		Cb4MessThreshold:    50,  // CB-4: mess_2h > 50 khi orders_2h=0 → trigger
		NightOffHourProtect: 21,  // PROTECT: tắt lúc 21h
		NightOffHourEfficiency: 22, // EFFICIENCY: tắt lúc 22h
		NightOffHourNormal:  22,  // NORMAL: tắt lúc 22:30
		NightOffMinuteNormal: 30,
		NightOffHourBlitz:   23,  // BLITZ: tắt lúc 23h

		ResetBudgetEnabled: false, // Logic Best_day chưa implement
		BestDayWindowDays:  3,     // Số ngày cho Best_day
	}
}

// DefaultAutomationActionRules trả về automation rules từ ActionRuleConfig (KillRules + DecreaseRules).
// Dùng cho migration BackfillAutomationActionRules (config cũ). Cấu trúc mới: rules đã gộp vào ActionRuleConfig.
func DefaultAutomationActionRules() []adsmodels.AutomationActionRule {
	def := DefaultActionRuleConfig()
	var out []adsmodels.AutomationActionRule
	for _, r := range def.KillRules {
		code := r.RuleCode
		if code == "" {
			code = r.Flag
		}
		out = append(out, adsmodels.AutomationActionRule{Code: code, Label: r.Label, AutoPropose: r.AutoPropose, AutoApprove: r.AutoApprove})
	}
	for _, r := range def.DecreaseRules {
		code := r.RuleCode
		if code == "" {
			code = r.Flag
		}
		out = append(out, adsmodels.AutomationActionRule{Code: code, Label: r.Label, AutoPropose: r.AutoPropose, AutoApprove: r.AutoApprove})
	}
	return out
}
