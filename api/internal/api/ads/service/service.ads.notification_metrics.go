// Package adssvc — Helper format metrics (raw, layer, flags) cho nội dung notification ads action.
// Cung cấp căn cứ tạo đề xuất: raw, layer1, layer3, alertFlags.
// Mỗi chỉ số nêu cụ thể giá trị, ngưỡng và phép so sánh (>, <, >=, <=) theo FolkForm v4.1.
package adssvc

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	adsconfig "meta_commerce/internal/api/ads/config"
	adsmodels "meta_commerce/internal/api/ads/models"
	adsrules "meta_commerce/internal/api/ads/rules"
)

// FormatMetricsForNotification tạo các chuỗi summary từ currentMetrics để hiển thị trong notification.
// Dùng làm căn cứ (evidence) cho đề xuất: raw, layer1, layer3, flags.
// Trả về map với keys: rawSummary, layer1Summary, layer3Summary, flagsSummary.
// Nếu không có dữ liệu, giá trị tương ứng là chuỗi rỗng.
func FormatMetricsForNotification(currentMetrics map[string]interface{}) map[string]string {
	return FormatMetricsForNotificationWithConfig(context.Background(), currentMetrics, nil)
}

// FormatMetricsForNotificationWithConfig tạo summary với config — bổ sung flagsDetail chi tiết từng chỉ số vs ngưỡng.
// cfg: campaign config từ ads_meta_config (adAccountId, ownerOrgID). nil = dùng default thresholds, không có flagsDetail.
func FormatMetricsForNotificationWithConfig(ctx context.Context, currentMetrics map[string]interface{}, cfg *adsmodels.CampaignConfigView) map[string]string {
	out := map[string]string{
		"rawSummary":    "",
		"layer1Summary": "",
		"layer3Summary": "",
		"flagsSummary":  "",
		"flagsDetail":   "",
	}
	if currentMetrics == nil {
		return out
	}

	raw, _ := currentMetrics["raw"].(map[string]interface{})
	layer1, _ := currentMetrics["layer1"].(map[string]interface{})
	layer2, _ := currentMetrics["layer2"].(map[string]interface{})
	layer3, _ := currentMetrics["layer3"].(map[string]interface{})
	alertFlags := currentMetrics["alertFlags"]

	out["rawSummary"] = formatRawSummary(raw)
	out["layer1Summary"] = formatLayer1Summary(layer1)
	out["layer3Summary"] = formatLayer3Summary(layer3)
	out["flagsSummary"] = formatFlagsSummary(alertFlags)
	if cfg != nil {
		out["flagsDetail"] = formatFlagsDetail(raw, layer1, layer2, layer3, alertFlags, cfg)
	}
	return out
}

// formatRawSummary format raw metrics 7d — nhóm theo nguồn (Meta, Pancake), trình bày logic.
func formatRawSummary(raw map[string]interface{}) string {
	if raw == nil {
		return ""
	}
	r7d := getRaw7dForNotification(raw)
	if r7d == nil {
		return ""
	}
	meta, _ := r7d["meta"].(map[string]interface{})
	pancake, _ := r7d["pancake"].(map[string]interface{})
	pos := mapOrNilForNotification(pancake, "pos")

	spend := toFloatForNotification(meta, "spend")
	mess := toInt64ForNotification(meta, "mess")
	orders := toInt64ForNotification(pos, "orders")
	revenue := toFloatForNotification(pos, "revenue")

	var groups []string
	if spend > 0 || mess > 0 {
		metaParts := []string{}
		if spend > 0 {
			metaParts = append(metaParts, fmt.Sprintf("Spend %.0f", spend))
		}
		if mess > 0 {
			metaParts = append(metaParts, fmt.Sprintf("Mess %d", mess))
		}
		groups = append(groups, "Meta: "+strings.Join(metaParts, ", "))
	}
	if orders > 0 || revenue > 0 {
		panParts := []string{}
		if orders > 0 {
			panParts = append(panParts, fmt.Sprintf("Orders %d", orders))
		}
		if revenue > 0 {
			panParts = append(panParts, fmt.Sprintf("Revenue %.0f", revenue))
		}
		groups = append(groups, "Pancake: "+strings.Join(panParts, ", "))
	}
	if len(groups) == 0 {
		return ""
	}
	return strings.Join(groups, " | ")
}

// formatLayer1Summary format layer1 — nhóm: Chỉ số hiệu quả (CPA, CR, RoAS) | Ngân sách & thời gian.
func formatLayer1Summary(layer1 map[string]interface{}) string {
	if layer1 == nil {
		return ""
	}
	cpaMess := toFloatForNotification(layer1, "cpaMess_7d")
	cpaPurchase := toFloatForNotification(layer1, "cpaPurchase_7d")
	convRate7d := toFloatForNotification(layer1, "convRate_7d")
	convRate2h := toFloatForNotification(layer1, "convRate_2h")
	roas := toFloatForNotification(layer1, "roas_7d")
	spendPct := toFloatForNotification(layer1, "spendPct_7d")
	mqs := toFloatForNotification(layer1, "mqs_7d")
	runtime := toFloatForNotification(layer1, "runtimeMinutes")

	var groups []string
	// Nhóm 1: Chỉ số hiệu quả
	effParts := []string{}
	if cpaMess > 0 {
		effParts = append(effParts, fmt.Sprintf("CPA_Mess %.0fk", cpaMess/1000))
	}
	if cpaPurchase > 0 {
		effParts = append(effParts, fmt.Sprintf("CPA_Purchase %.0fk", cpaPurchase/1000))
	}
	if convRate7d > 0 {
		effParts = append(effParts, fmt.Sprintf("CR_7d %.1f%%", convRate7d*100))
	}
	if convRate2h > 0 {
		effParts = append(effParts, fmt.Sprintf("CR_2h %.1f%%", convRate2h*100))
	}
	if roas > 0 {
		effParts = append(effParts, fmt.Sprintf("RoAS %.2f", roas))
	}
	if mqs > 0 {
		effParts = append(effParts, fmt.Sprintf("MQS %.2f", mqs))
	}
	if len(effParts) > 0 {
		groups = append(groups, strings.Join(effParts, ", "))
	}
	// Nhóm 2: Ngân sách & thời gian
	budgetParts := []string{}
	if spendPct > 0 {
		budgetParts = append(budgetParts, fmt.Sprintf("Spend%% %.0f%%", spendPct*100))
	}
	if runtime > 0 {
		budgetParts = append(budgetParts, fmt.Sprintf("Runtime %.0fp", runtime))
	}
	if len(budgetParts) > 0 {
		groups = append(groups, strings.Join(budgetParts, ", "))
	}
	if len(groups) == 0 {
		return ""
	}
	return strings.Join(groups, " | ")
}

// formatLayer3Summary format layer3: CHS, healthState, portfolioCell.
func formatLayer3Summary(layer3 map[string]interface{}) string {
	if layer3 == nil {
		return ""
	}
	chs := getStringForNotification(layer3, "chs")
	health := getStringForNotification(layer3, "healthState")
	cell := getStringForNotification(layer3, "portfolioCell")

	parts := []string{}
	if chs != "" {
		parts = append(parts, fmt.Sprintf("CHS: %s", chs))
	}
	if health != "" {
		parts = append(parts, fmt.Sprintf("Health: %s", health))
	}
	if cell != "" {
		parts = append(parts, fmt.Sprintf("Cell: %s", cell))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " | ")
}

// formatFlagsSummary format alertFlags thành chuỗi.
func formatFlagsSummary(alertFlags interface{}) string {
	flags := parseFlagsForNotification(alertFlags)
	if len(flags) == 0 {
		return ""
	}
	return strings.Join(flags, ", ")
}

// formatFlagsDetail format chi tiết từng flag — mỗi flag một block, mỗi điều kiện một dòng thụt lề.
// Trình bày: Flag → điều kiện 1 ✓ | điều kiện 2 ✓ | ... (logic, dễ đọc).
func formatFlagsDetail(raw, layer1, layer2, layer3 map[string]interface{}, alertFlags interface{}, cfg *adsmodels.CampaignConfigView) string {
	flags := parseFlagsForNotification(alertFlags)
	if len(flags) == 0 {
		return ""
	}
	ctx := adsrules.BuildFactsContext(getRaw7dForNotificationMap(raw), layer1, layer2, layer3, cfg)
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	th := func(k string) float64 { return adsconfig.GetThresholdWithEventOverride(k, cfg, now) }
	defs := adsconfig.GetFlagDefinitions(cfg)
	flagDefByCode := make(map[string]*adsmodels.FlagDefinition)
	for i := range defs {
		flagDefByCode[defs[i].Code] = &defs[i]
	}

	var blocks []string
	indent := "  "
	for i, code := range flags {
		fd, ok := flagDefByCode[code]
		if !ok {
			if strings.HasPrefix(code, "diagnosis_") {
				blocks = append(blocks, fmt.Sprintf("• %s (cờ động)", code))
			} else {
				blocks = append(blocks, fmt.Sprintf("• %s (không có định nghĩa)", code))
			}
			continue
		}
		// Tìm group điều kiện đã match
		var matchedGroup []adsmodels.FlagConditionItem
		for _, group := range fd.ConditionGroups {
			allMatch := true
			for _, c := range group {
				if !adsrules.EvaluateCondition(c, &ctx, th) {
					allMatch = false
					break
				}
			}
			if allMatch {
				matchedGroup = group
				break
			}
		}
		if len(matchedGroup) == 0 {
			matchedGroup = fd.ConditionGroups[0]
		}
		label := fd.Label
		if label == "" {
			label = code
		}
		// Block: tiêu đề flag + các điều kiện thụt lề
		condLines := make([]string, 0, len(matchedGroup))
		for _, c := range matchedGroup {
			s := formatConditionDetail(c, &ctx, th)
			if s != "" {
				condLines = append(condLines, indent+"• "+s)
			}
		}
		if len(condLines) > 0 {
			prefix := "• "
			if i > 0 {
				blocks = append(blocks, "")
			}
			blocks = append(blocks, prefix+label+":")
			blocks = append(blocks, condLines...)
		} else {
			blocks = append(blocks, "• "+label+": "+fd.LogicText)
		}
	}
	return strings.Join(blocks, "\n")
}

// formatConditionDetail format một điều kiện: "metric = value op ngưỡng" — gọn, dễ đọc.
func formatConditionDetail(c adsmodels.FlagConditionItem, ctx *adsrules.FactsContext, th func(string) float64) string {
	factKey := c.Fact
	if factKey == "" {
		factKey = c.MetricKey
	}
	valNum, valStr, ok := ctx.GetFact(factKey)
	if !ok {
		return ""
	}
	var compareNum float64
	var compareStr string
	if c.ThresholdKey != "" {
		tv := th(c.ThresholdKey)
		if c.ThresholdKeyByMode != "" {
			if parts := strings.SplitN(c.ThresholdKeyByMode, ":", 2); len(parts) == 2 {
				modes := strings.Split(parts[0], ",")
				for _, m := range modes {
					if strings.TrimSpace(m) == ctx.CurrentMode {
						tv = th(strings.TrimSpace(parts[1]))
						break
					}
				}
			}
		}
		if c.ThresholdKey2 != "" {
			compareNum = tv * th(c.ThresholdKey2)
		} else {
			compareNum = tv
		}
	} else if c.Value != nil {
		compareNum = *c.Value
	} else if c.ValueStr != "" {
		compareStr = c.ValueStr
	}
	opStr := operatorToStr(c.Operator)
	label := factLabel(factKey)
	valDisp := formatFactValue(factKey, valNum, valStr)
	compareDisp := formatCompareValue(factKey, compareNum, compareStr)
	// Format gọn: "CPA_Mess 195k > 180k" hoặc "CR_7d 4.5% < 6%"
	return fmt.Sprintf("%s %s %s %s", label, valDisp, opStr, compareDisp)
}

func operatorToStr(op string) string {
	switch op {
	case adsconfig.OpGreaterThan:
		return ">"
	case adsconfig.OpLessThan:
		return "<"
	case adsconfig.OpGreaterThanOrEqual:
		return ">="
	case adsconfig.OpLessThanOrEqual:
		return "<="
	case adsconfig.OpEqual:
		return "="
	case adsconfig.OpNotEqual:
		return "≠"
	case adsconfig.OpIn:
		return "∈"
	case adsconfig.OpNotIn:
		return "∉"
	}
	return op
}

func factLabel(key string) string {
	labels := map[string]string{
		"spend": "Spend", "mess": "Mess", "orders": "Orders", "impressions": "Impressions",
		"cpm": "CPM", "ctr": "CTR", "frequency": "Freq", "deliveryStatus": "Delivery",
		"cpaMess_7d": "CPA_Mess", "cpaPurchase_7d": "CPA_Purchase",
		"convRate_7d": "CR_7d", "convRate_2h": "CR_2h", "convRate_1h": "CR_1h",
		"msgRate_7d": "Msg_Rate", "mqs_7d": "MQS", "roas_7d": "RoAS",
		"spendPct_7d": "Spend%", "spendPct": "Spend%",
		"runtimeMinutes": "Runtime", "chs": "CHS", "healthState": "Health",
		"portfolioCell": "Cell", "inTrimWindow": "TrimWindow",
	}
	if l, ok := labels[key]; ok {
		return l
	}
	return key
}

func factFormula(key string) string {
	formulas := map[string]string{
		"cpaMess_7d": "spend/mess", "cpaPurchase_7d": "spend/orders",
		"convRate_7d": "orders/mess", "msgRate_7d": "mess/clicks",
		"mqs_7d": "mess×CR", "roas_7d": "revenue/spend",
		"spendPct_7d": "spend/budget",
	}
	if f, ok := formulas[key]; ok {
		return f
	}
	return ""
}

func formatFactValue(key string, num float64, str string) string {
	if str != "" {
		return str
	}
	// %: convRate, spendPct, ctr, msgRate (0-1 hoặc 0-100)
	if strings.Contains(key, "convRate") || strings.Contains(key, "spendPct") || key == "ctr" || key == "msgRate_7d" {
		return fmt.Sprintf("%.1f%%", num*100)
	}
	// VND: spend, cpm, cpaMess, cpaPurchase
	if key == "spend" || key == "cpm" || key == "cpaMess_7d" || key == "cpaPurchase_7d" {
		if num >= 1000 {
			return fmt.Sprintf("%.0fk", num/1000)
		}
		return fmt.Sprintf("%.0f", num)
	}
	// Phút
	if key == "runtimeMinutes" {
		return fmt.Sprintf("%.0fp", num)
	}
	// Số nguyên
	if key == "mess" || key == "orders" || key == "impressions" {
		return fmt.Sprintf("%.0f", num)
	}
	// Mặc định
	return fmt.Sprintf("%v", num)
}

func formatCompareValue(key string, num float64, str string) string {
	if str != "" {
		return str
	}
	if strings.Contains(key, "convRate") || strings.Contains(key, "spendPct") || key == "ctr" || key == "msgRate_7d" {
		return fmt.Sprintf("%.1f%%", num*100)
	}
	if key == "spend" || key == "cpm" || key == "cpaMess_7d" || key == "cpaPurchase_7d" {
		if num >= 1000 {
			return fmt.Sprintf("%.0fk", num/1000)
		}
		return fmt.Sprintf("%.0f", num)
	}
	if key == "runtimeMinutes" {
		return fmt.Sprintf("%.0fp", num)
	}
	return fmt.Sprintf("%v", num)
}

func getRaw7dForNotificationMap(raw map[string]interface{}) map[string]interface{} {
	if raw == nil {
		return nil
	}
	if r, ok := raw["7d"].(map[string]interface{}); ok && r != nil {
		return r
	}
	return raw
}

func getRaw7dForNotification(raw map[string]interface{}) map[string]interface{} {
	if r, ok := raw["7d"].(map[string]interface{}); ok && r != nil {
		return r
	}
	return raw
}

func mapOrNilForNotification(m map[string]interface{}, k string) map[string]interface{} {
	if m == nil {
		return nil
	}
	v, ok := m[k]
	if !ok {
		return nil
	}
	switch x := v.(type) {
	case map[string]interface{}:
		return x
	case map[interface{}]interface{}:
		out := make(map[string]interface{})
		for kk, vv := range x {
			if s, ok := kk.(string); ok {
				out[s] = vv
			}
		}
		return out
	}
	return nil
}

func toFloatForNotification(m map[string]interface{}, k string) float64 {
	if m == nil {
		return 0
	}
	v, ok := m[k]
	if !ok {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		var f float64
		fmt.Sscanf(x, "%f", &f)
		return f
	}
	return 0
}

func toInt64ForNotification(m map[string]interface{}, k string) int64 {
	if m == nil {
		return 0
	}
	v, ok := m[k]
	if !ok {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int:
		return int64(x)
	case int64:
		return x
	}
	return 0
}

func getStringForNotification(m map[string]interface{}, k string) string {
	if m == nil {
		return ""
	}
	v, ok := m[k]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func parseFlagsForNotification(v interface{}) []string {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case []string:
		return val
	case []interface{}:
		var out []string
		for _, e := range val {
			if s, ok := e.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return nil
	}
	var out []string
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i).Interface()
		if s, ok := elem.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}
