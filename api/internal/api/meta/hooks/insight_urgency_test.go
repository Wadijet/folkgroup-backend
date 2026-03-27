package metahooks

import (
	"testing"

	"meta_commerce/internal/api/events"
	"meta_commerce/internal/global"
)

func TestIsUrgentMetaInsightDataChange(t *testing.T) {
	global.MongoDB_ColNames.MetaAdInsights = "meta_ad_insights"
	base := events.DataChangeEvent{
		CollectionName: "meta_ad_insights",
		Operation:      events.OpUpsert,
		Document: map[string]interface{}{
			"objectType":  "ad",
			"objectId":    "x",
			"adAccountId": "act_1",
		},
	}
	if IsUrgentMetaInsightDataChange(base) {
		t.Fatal("không có chỉ số bất thường thì không gấp")
	}
	urgentMeta := events.DataChangeEvent{
		CollectionName: "meta_ad_insights",
		Operation:      events.OpUpsert,
		Document: map[string]interface{}{
			"objectType":  "ad",
			"objectId":    "x",
			"adAccountId": "act_1",
			"metaData": map[string]interface{}{
				"insightUrgent": true,
			},
		},
	}
	if !IsUrgentMetaInsightDataChange(urgentMeta) {
		t.Fatal("metaData.insightUrgent=true phải gấp")
	}
	highSpend := events.DataChangeEvent{
		CollectionName: "meta_ad_insights",
		Operation:      events.OpUpsert,
		Document: map[string]interface{}{
			"objectType":  "ad",
			"objectId":    "x",
			"adAccountId": "act_1",
			"spend":       "600",
		},
	}
	if !IsUrgentMetaInsightDataChange(highSpend) {
		t.Fatal("spend vượt ngưỡng mặc định phải gấp")
	}
}
