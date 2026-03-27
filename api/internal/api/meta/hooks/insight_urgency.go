// Package metahooks — Phân loại insight Meta cần tính Ads Intelligence ngay (gấp) vs gom batch.
package metahooks

import (
	"os"
	"strconv"
	"strings"

	"meta_commerce/internal/api/events"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
)

// IsUrgentMetaInsightDataChange trả về true khi bản ghi meta_ad_insights có dấu hiệu bất thường
// cần gọi recompute Ads Intelligence ngay (không chờ cửa sổ gom 15 phút).
//
// Các trường hợp (ưu tiên từ trên xuống):
//  1) Cờ tường minh trong metaData: insightUrgent, adsIntelUrgent, anomaly (bool hoặc "true"/"1")
//  2) Chi tiêu (spend) trong ngày ≥ ngưỡng env ADS_INTEL_INSIGHT_URGENT_SPEND_MIN (mặc định 500 USD)
//  3) CPC ≥ ngưỡng env ADS_INTEL_INSIGHT_URGENT_CPC_MIN (mặc định 8)
//  4) Hiển thị cao nhưng CTR rất thấp: impressions ≥ ADS_INTEL_INSIGHT_URGENT_IMP_MIN (mặc định 8000)
//     và CTR (chuẩn hóa) < ADS_INTEL_INSIGHT_URGENT_CTR_MAX_RATIO (mặc định 0.002 = 0.2%)
func IsUrgentMetaInsightDataChange(e events.DataChangeEvent) bool {
	if e.Document == nil || e.CollectionName != global.MongoDB_ColNames.MetaAdInsights {
		return false
	}
	m, ok := docToBSONMap(e.Document)
	if !ok || m == nil {
		return false
	}
	if metaDataUrgent(m) {
		return true
	}
	spend := parseFloatMetric(m["spend"])
	if spend >= envFloat64("ADS_INTEL_INSIGHT_URGENT_SPEND_MIN", 500) {
		return true
	}
	cpc := parseFloatMetric(m["cpc"])
	if cpc > 0 && cpc >= envFloat64("ADS_INTEL_INSIGHT_URGENT_CPC_MIN", 8) {
		return true
	}
	imp := parseFloatMetric(m["impressions"])
	ctrNorm := ctrNormalized(m["ctr"])
	if imp >= envFloat64("ADS_INTEL_INSIGHT_URGENT_IMP_MIN", 8000) && ctrNorm >= 0 &&
		ctrNorm < envFloat64("ADS_INTEL_INSIGHT_URGENT_CTR_MAX_RATIO", 0.002) {
		return true
	}
	return false
}

func docToBSONMap(doc interface{}) (map[string]interface{}, bool) {
	switch t := doc.(type) {
	case map[string]interface{}:
		return t, true
	case bson.M:
		return map[string]interface{}(t), true
	default:
		data, err := bson.Marshal(doc)
		if err != nil {
			return nil, false
		}
		var m map[string]interface{}
		if err := bson.Unmarshal(data, &m); err != nil {
			return nil, false
		}
		return m, true
	}
}

// metaDataUrgent — đồng bộ/agent có thể đánh dấu gấp trong metaData.
func metaDataUrgent(m map[string]interface{}) bool {
	raw, ok := m["metaData"]
	if !ok || raw == nil {
		return false
	}
	meta, ok := docToBSONMap(raw)
	if !ok {
		return false
	}
	for _, k := range []string{"insightUrgent", "adsIntelUrgent", "anomaly"} {
		if truthy(meta[k]) {
			return true
		}
	}
	return false
}

func truthy(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		return s == "true" || s == "1" || s == "yes"
	case float64:
		return t != 0
	case int:
		return t != 0
	case int32:
		return t != 0
	case int64:
		return t != 0
	default:
		return false
	}
}

func parseFloatMetric(v interface{}) float64 {
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return f
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	default:
		return 0
	}
}

// ctrNormalized: Meta thường trả ctr dạng phần trăm (vd 1.23) hoặc tỷ lệ; đưa về tỷ lệ 0..1.
func ctrNormalized(v interface{}) float64 {
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return -1
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return -1
		}
		if f > 1 {
			return f / 100.0
		}
		return f
	case float64:
		if t > 1 {
			return t / 100.0
		}
		return t
	default:
		return -1
	}
}

func envFloat64(key string, def float64) float64 {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return def
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return f
}
