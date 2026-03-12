// Package rules — FLAG_RULE evaluator: đọc definitions và đánh giá metrics → flags.
// Dùng chung logic với meta/service computeAlertFlags nhưng driven bởi FlagDefinitions.
package rules

import (
	"context"
	"strconv"
	"strings"
	"time"

	adsadaptive "meta_commerce/internal/api/ads/adaptive"
	adsconfig "meta_commerce/internal/api/ads/config"
	adsmodels "meta_commerce/internal/api/ads/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// FactsContext chứa toàn bộ facts (metrics) để evaluator đánh giá.
// Tên metric thể hiện rõ chu kỳ: _7d, _2h, _1h theo FolkForm v4.1.
type FactsContext struct {
	// Từ raw.meta (7d)
	Spend, Mess, Impressions, Cpm, Ctr, Frequency float64
	DeliveryStatus                                string
	// Từ raw.pancake.pos (7d)
	Orders float64
	// Từ layer1 — metrics theo chu kỳ
	MsgRate_7d, CpaMess_7d, CpaPurchase_7d, ConvRate_7d, Roas_7d, Mqs_7d, SpendPct_7d float64
	ConvRate_2h, ConvRate_1h                                                                 float64
	RuntimeMinutes                                                                           float64
	// Từ layer3
	Chs, HealthState, PortfolioCell string
	// Derived
	InTrimWindow  bool
	CurrentMode   string
	Cpm3DayAvg    float64 // KO-C: code dùng cpmHigh*cpmKoCMultiplier, không phải cpm_3day_avg
	Diagnoses     []string
}

// BuildFactsContext tạo FactsContext từ raw, layer1, layer2, layer3.
// cfg: dùng cho timezone (CommonConfig) và trim window (FlagRuleConfig). nil = default.
func BuildFactsContext(raw, layer1, layer2, layer3 map[string]interface{}, cfg *adsmodels.CampaignConfigView) FactsContext {
	meta, _ := raw["meta"].(map[string]interface{})
	pancake, _ := raw["pancake"].(map[string]interface{})
	pos := mapOrNil(pancake, "pos")

	spend := toFloat(meta, "spend")
	mess := toInt64(meta, "mess")
	impressions := toInt64(meta, "impressions")
	orders := toInt64(pos, "orders")

	ctx := FactsContext{
		Spend:           spend,
		Mess:            float64(mess),
		Impressions:     float64(impressions),
		Orders:          float64(orders),
		Cpm:             toFloat(meta, "cpm"),
		Ctr:             toFloat(meta, "ctr"),
		Frequency:       toFloat(meta, "frequency"),
		DeliveryStatus:  getString(meta, "deliveryStatus"),
		MsgRate_7d:      toFloat(layer1, "msgRate_7d"),
		CpaMess_7d:      toFloat(layer1, "cpaMess_7d"),
		CpaPurchase_7d:  toFloat(layer1, "cpaPurchase_7d"),
		ConvRate_7d:     toFloat(layer1, "convRate_7d"),
		ConvRate_2h:     toFloat(layer1, "convRate_2h"),
		ConvRate_1h:     toFloat(layer1, "convRate_1h"),
		Roas_7d:         toFloat(layer1, "roas_7d"),
		Mqs_7d:          toFloat(layer1, "mqs_7d"),
		SpendPct_7d:     toFloat(layer1, "spendPct_7d"),
		RuntimeMinutes:  toFloat(layer1, "runtimeMinutes"),
		Chs:             formatFloat(toFloat(layer3, "chs")),
		HealthState:     getString(layer3, "healthState"),
		PortfolioCell:   getString(layer3, "portfolioCell"),
		InTrimWindow:    isTrimTimeWindow(cfg),
		CurrentMode:     getString(layer2, "currentMode"),
	}
	if diag, ok := layer3["diagnoses"].([]interface{}); ok {
		for _, d := range diag {
			if s, ok := d.(string); ok && s != "" {
				ctx.Diagnoses = append(ctx.Diagnoses, s)
			}
		}
	}
	return ctx
}

func toFloat(m map[string]interface{}, k string) float64 {
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
		f, _ := strconv.ParseFloat(x, 64)
		return f
	}
	return 0
}

func toInt64(m map[string]interface{}, k string) int64 {
	if m == nil {
		return 0
	}
	v, ok := m[k]
	if !ok {
		return 0
	}
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	}
	return 0
}

func getString(m map[string]interface{}, k string) string {
	if m == nil {
		return ""
	}
	v, ok := m[k]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func mapOrNil(m map[string]interface{}, k string) map[string]interface{} {
	if m == nil {
		return nil
	}
	v, ok := m[k]
	if !ok {
		return nil
	}
	out, _ := v.(map[string]interface{})
	return out
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func isTrimTimeWindow(cfg *adsmodels.CampaignConfigView) bool {
	common := adsconfig.GetCommon(cfg)
	loc, err := time.LoadLocation(common.Timezone)
	if err != nil {
		loc = time.FixedZone("UTC+7", 7*3600)
	}
	hour := time.Now().In(loc).Hour()
	start, end := adsconfig.GetTrimWindow(cfg)
	return hour >= start && hour < end
}

// GetFact trả về giá trị fact từ context.
func (f *FactsContext) GetFact(key string) (float64, string, bool) {
	switch key {
	case "spend":
		return f.Spend, "", true
	case "mess":
		return f.Mess, "", true
	case "impressions":
		return f.Impressions, "", true
	case "orders":
		return f.Orders, "", true
	case "cpm":
		return f.Cpm, "", true
	case "ctr":
		return f.Ctr, "", true
	case "frequency":
		return f.Frequency, "", true
	case "deliveryStatus":
		return 0, f.DeliveryStatus, true
	case "msgRate_7d":
		return f.MsgRate_7d, "", true
	case "cpaMess_7d":
		return f.CpaMess_7d, "", true
	case "cpaPurchase_7d":
		return f.CpaPurchase_7d, "", true
	case "convRate_7d", "convRate":
		return f.ConvRate_7d, "", true
	case "convRate_2h":
		return f.ConvRate_2h, "", true
	case "convRate_1h":
		return f.ConvRate_1h, "", true
	case "roas_7d":
		return f.Roas_7d, "", true
	case "mqs_7d":
		return f.Mqs_7d, "", true
	case "spendPct_7d", "spendPct":
		return f.SpendPct_7d, "", true
	case "runtimeMinutes":
		return f.RuntimeMinutes, "", true
	case "chs":
		parsed, _ := strconv.ParseFloat(f.Chs, 64)
		return parsed, f.Chs, true
	case "healthState":
		return 0, f.HealthState, true
	case "portfolioCell":
		return 0, f.PortfolioCell, true
	case "inTrimWindow":
		if f.InTrimWindow {
			return 1, "", true
		}
		return 0, "", true
	case "cpm_3day_avg":
		return f.Cpm3DayAvg, "", true
	default:
		return 0, "", false
	}
}

// EvaluateCondition đánh giá một điều kiện. Đọc từ config (FlagConditionItem). Tham số: th = GetThreshold.
func EvaluateCondition(c adsmodels.FlagConditionItem, ctx *FactsContext, th func(string) float64) bool {
	factKey := c.Fact
	if factKey == "" {
		factKey = c.MetricKey
	}
	valNum, valStr, ok := ctx.GetFact(factKey)
	if !ok {
		return false
	}

	// Lấy giá trị so sánh
	var compareNum float64
	var compareStr string
	if c.ThresholdKey != "" {
		tv := th(c.ThresholdKey)
		if c.ThresholdKeyByMode != "" {
			// Format: "BLITZ,PROTECT:spendPctSlBBlitz"
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
		} else if c.CompareToMetric != "" {
			cm, _, _ := ctx.GetFact(c.CompareToMetric)
			compareNum = cm * tv
		} else {
			compareNum = tv
		}
	} else if c.Value != nil {
		compareNum = *c.Value
	} else if c.ValueStr != "" {
		compareStr = c.ValueStr
	}

	// So sánh theo operator
	switch c.Operator {
	case adsconfig.OpGreaterThan:
		// SpendPctFallback: khi spendPct_7d=0 dùng spend>0 thay vì spendPct>threshold
		if c.SpendPctFallback && (factKey == "spendPct" || factKey == "spendPct_7d") && valNum == 0 {
			return ctx.Spend > 0
		}
		return valNum > compareNum
	case adsconfig.OpLessThan:
		return valNum < compareNum
	case adsconfig.OpGreaterThanOrEqual:
		return valNum >= compareNum
	case adsconfig.OpLessThanOrEqual:
		return valNum <= compareNum
	case adsconfig.OpEqual:
		if compareStr != "" {
			return valStr == compareStr
		}
		return valNum == compareNum
	case adsconfig.OpNotEqual:
		if compareStr != "" {
			return valStr != compareStr
		}
		return valNum != compareNum
	case adsconfig.OpIn:
		parts := strings.Split(compareStr, ",")
		for _, p := range parts {
			if strings.TrimSpace(p) == valStr {
				return true
			}
		}
		return false
	case adsconfig.OpNotIn:
		parts := strings.Split(compareStr, ",")
		for _, p := range parts {
			if strings.TrimSpace(p) == valStr {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// isFlagEnabled kiểm tra cờ có được bật không. Đọc từ FlagDefinition.Enabled. Dynamic flags (diagnosis_xxx) = mặc định bật.
func isFlagEnabled(code string, defs []adsmodels.FlagDefinition) bool {
	for _, fd := range defs {
		if fd.Code == code {
			if fd.Enabled == nil {
				return true // nil = mặc định bật
			}
			return *fd.Enabled
		}
	}
	return true // Không có trong definitions = dynamic flag, mặc định bật
}

// EvalCampaignContext ngữ cảnh campaign cho adaptive threshold (FolkForm v4.1 Section 2.2).
type EvalCampaignContext struct {
	CampaignId  string
	AdAccountId string
	OwnerOrgID  primitive.ObjectID
}

// EvaluateFlags đánh giá definitions và trả về danh sách cờ trigger. CHỈ đọc từ config qua GetFlagDefinitions.
// Khi campCtx != nil và có campaignId: dùng Per-Camp Adaptive Threshold (GetAdaptiveThreshold).
// goCtx: context cho DB (từ caller, ví dụ c.Context() hoặc context.Background()).
func EvaluateFlags(goCtx context.Context, ctx *FactsContext, cfg *adsmodels.CampaignConfigView, campCtx *EvalCampaignContext) []string {
	// Mess Trap Event Override: trong Event window dùng ngưỡng cao hơn (7%/8%)
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	var th func(k string) float64
	if campCtx != nil && campCtx.CampaignId != "" && campCtx.AdAccountId != "" {
		// Per-Camp Adaptive Threshold (FolkForm v4.1 Section 2.2)
		th = func(k string) float64 {
			v, ok := adsadaptive.GetAdaptiveThreshold(goCtx, k, campCtx.CampaignId, campCtx.AdAccountId, campCtx.OwnerOrgID, cfg, now)
			if ok {
				return v
			}
			return adsconfig.GetThresholdWithEventOverride(k, cfg, now)
		}
	} else {
		th = func(k string) float64 { return adsconfig.GetThresholdWithEventOverride(k, cfg, now) }
	}
	defs := adsconfig.GetFlagDefinitions(cfg)
	var flags []string
	seen := make(map[string]bool)

	for _, fd := range defs {
		if !isFlagEnabled(fd.Code, defs) {
			continue
		}
		for _, group := range fd.ConditionGroups {
			allMatch := true
			for _, c := range group {
				if !EvaluateCondition(c, ctx, th) {
					allMatch = false
					break
				}
			}
			if allMatch {
				if !seen[fd.Code] {
					seen[fd.Code] = true
					flags = append(flags, fd.Code)
				}
				break
			}
		}
	}

	// Dynamic flags: diagnosis_xxx
	for _, d := range ctx.Diagnoses {
		code := "diagnosis_" + d
		if !isFlagEnabled(code, defs) {
			continue
		}
		if !seen[code] {
			seen[code] = true
			flags = append(flags, code)
		}
	}

	return flags
}
