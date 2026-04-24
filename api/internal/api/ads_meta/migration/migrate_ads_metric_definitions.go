// Package migration — Seed ads_metric_definitions theo FolkForm v4.1.
// Định nghĩa đầy đủ metrics: 7d (Kill/MQS), 2h (Momentum Tracker), 1h (CB/HB), 30p (Msg_Rate).
package migration

import (
	"context"
	"time"

	adsmodels "meta_commerce/internal/api/ads_meta/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// WindowMs chuyển window string sang milliseconds.
func windowMs(window string) int64 {
	switch window {
	case adsmodels.Window7d:
		return 7 * 24 * 60 * 60 * 1000
	case adsmodels.Window2h:
		return 2 * 60 * 60 * 1000
	case adsmodels.Window1h:
		return 60 * 60 * 1000
	case adsmodels.Window30p:
		return 30 * 60 * 1000
	case adsmodels.Window2d:
		return 2 * 24 * 60 * 60 * 1000
	case adsmodels.Window3d:
		return 3 * 24 * 60 * 60 * 1000
	case adsmodels.Window14d:
		return 14 * 24 * 60 * 60 * 1000
	default:
		return 7 * 24 * 60 * 60 * 1000
	}
}

// SeedAdsMetricDefinitions seed định nghĩa metrics theo FolkForm v4.1.
// Upsert từng document theo key. Gọi khi init hoặc migration.
func SeedAdsMetricDefinitions(ctx context.Context) (int, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsMetricDefinitions)
	if !ok {
		return 0, nil
	}

	now := time.Now().Unix()
	seeds := []adsmodels.AdsMetricDefinition{
		// === RAW 7d — currentMetrics, Kill rules, MQS ===
		{Key: "spend_7d", Label: "Spend (7 ngày)", Description: "Chi phí quảng cáo Meta insights", Unit: "VND", Source: adsmodels.SourceMeta, Window: adsmodels.Window7d, WindowMs: windowMs(adsmodels.Window7d), Type: adsmodels.MetricTypeRaw, SourceCollection: global.MongoDB_ColNames.MetaAdInsights, TimeField: "dateStart", OutputPath: "meta.spend", UseCase: "MQS, Kill rules, currentMetrics", DocReference: "FolkForm 01", Order: 1, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Key: "mess_7d", Label: "Mess (7 ngày)", Description: "Số cuộc hội thoại messaging_conversation_started", Unit: "số", Source: adsmodels.SourceMeta, Window: adsmodels.Window7d, WindowMs: windowMs(adsmodels.Window7d), Type: adsmodels.MetricTypeRaw, SourceCollection: global.MongoDB_ColNames.MetaAdInsights, TimeField: "dateStart", OutputPath: "meta.mess", UseCase: "MQS, Conv_Rate_7day, Mess Trap", DocReference: "FolkForm 01", Order: 2, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Key: "orders_7d", Label: "Orders (7 ngày)", Description: "Số đơn Pancake trong 7 ngày", Unit: "số", Source: adsmodels.SourcePancakePos, Window: adsmodels.Window7d, WindowMs: windowMs(adsmodels.Window7d), Type: adsmodels.MetricTypeRaw, SourceCollection: global.MongoDB_ColNames.OrderCanonical, TimeField: "insertedAt", AggregationField: "posData.ad_id", OutputPath: "pancake.pos.orders", UseCase: "Conv_Rate_7day, MQS, Mess Trap", DocReference: "FolkForm 01", Order: 10, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Key: "revenue_7d", Label: "Revenue (7 ngày)", Description: "Doanh thu từ đơn hàng Pancake", Unit: "VND", Source: adsmodels.SourcePancakePos, Window: adsmodels.Window7d, WindowMs: windowMs(adsmodels.Window7d), Type: adsmodels.MetricTypeRaw, SourceCollection: global.MongoDB_ColNames.OrderCanonical, TimeField: "insertedAt", AggregationField: "posData.ad_id", OutputPath: "pancake.pos.revenue", UseCase: "ROAS, CPA Purchase", DocReference: "FolkForm 01", Order: 11, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Key: "conversationCount_7d", Label: "Conversations (7 ngày)", Description: "Số hội thoại có ad_ids", Unit: "số", Source: adsmodels.SourcePancakeConversation, Window: adsmodels.Window7d, WindowMs: windowMs(adsmodels.Window7d), Type: adsmodels.MetricTypeRaw, SourceCollection: global.MongoDB_ColNames.FbConvesations, TimeField: "panCakeUpdatedAt", AggregationField: "panCakeData.ad_ids", OutputPath: "pancake.conversation.conversationCount", UseCase: "Attribution", DocReference: "FolkForm", Order: 12, IsActive: true, CreatedAt: now, UpdatedAt: now},
		// Meta raw 7d (impressions, clicks, ...)
		{Key: "impressions_7d", Label: "Impressions (7 ngày)", Description: "Số lần hiển thị quảng cáo", Unit: "số", Source: adsmodels.SourceMeta, Window: adsmodels.Window7d, WindowMs: windowMs(adsmodels.Window7d), Type: adsmodels.MetricTypeRaw, SourceCollection: global.MongoDB_ColNames.MetaAdInsights, TimeField: "dateStart", OutputPath: "meta.impressions", UseCase: "CTR, CPM", Order: 3, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Key: "inlineLinkClicks_7d", Label: "Inline Link Clicks (7 ngày)", Description: "Số click link (msg_rate)", Unit: "số", Source: adsmodels.SourceMeta, Window: adsmodels.Window7d, WindowMs: windowMs(adsmodels.Window7d), Type: adsmodels.MetricTypeRaw, SourceCollection: global.MongoDB_ColNames.MetaAdInsights, TimeField: "dateStart", OutputPath: "meta.inlineLinkClicks", UseCase: "Msg_Rate", Order: 4, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Key: "cpm_7d", Label: "CPM (7 ngày)", Description: "Cost per 1000 impressions", Unit: "VND", Source: adsmodels.SourceMeta, Window: adsmodels.Window7d, WindowMs: windowMs(adsmodels.Window7d), Type: adsmodels.MetricTypeRaw, SourceCollection: global.MongoDB_ColNames.MetaAdInsights, TimeField: "dateStart", OutputPath: "meta.cpm", UseCase: "CHS, Mess Trap", Order: 5, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Key: "ctr_7d", Label: "CTR (7 ngày)", Description: "Click-through rate", Unit: "%", Source: adsmodels.SourceMeta, Window: adsmodels.Window7d, WindowMs: windowMs(adsmodels.Window7d), Type: adsmodels.MetricTypeRaw, SourceCollection: global.MongoDB_ColNames.MetaAdInsights, TimeField: "dateStart", OutputPath: "meta.ctr", UseCase: "Kill rules", Order: 6, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Key: "frequency_7d", Label: "Frequency (7 ngày)", Description: "Số lần TB mỗi user thấy quảng cáo", Unit: "số", Source: adsmodels.SourceMeta, Window: adsmodels.Window7d, WindowMs: windowMs(adsmodels.Window7d), Type: adsmodels.MetricTypeRaw, SourceCollection: global.MongoDB_ColNames.MetaAdInsights, TimeField: "dateStart", OutputPath: "meta.frequency", UseCase: "Trim, CHS", Order: 7, IsActive: true, CreatedAt: now, UpdatedAt: now},

		// === RAW 2h — Momentum Tracker, CB-4 ===
		// FolkForm 04: Conv_Rate_now = Pancake_orders_2h / FB_Mess_2h. meta_ad_insights chỉ daily → mess từ fb_conversations (DB).
		{Key: "orders_2h", Label: "Orders (2 giờ)", Description: "Số đơn Pancake trong 2h gần nhất", Unit: "số", Source: adsmodels.SourcePancakePos, Window: adsmodels.Window2h, WindowMs: windowMs(adsmodels.Window2h), Type: adsmodels.MetricTypeRaw, SourceCollection: global.MongoDB_ColNames.OrderCanonical, TimeField: "insertedAt", AggregationField: "posData.ad_id", OutputPath: "orders", UseCase: "Momentum Tracker, ACCELERATING, CB-4", DocReference: "FolkForm 04, 07", Order: 20, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Key: "mess_2h", Label: "Mess (2 giờ)", Description: "Số hội thoại fb_conversations trong 2h gần nhất", Unit: "số", Source: adsmodels.SourcePancakeConversation, Window: adsmodels.Window2h, WindowMs: windowMs(adsmodels.Window2h), Type: adsmodels.MetricTypeRaw, SourceCollection: global.MongoDB_ColNames.FbConvesations, TimeField: "panCakeUpdatedAt", AggregationField: "panCakeData.ad_ids", OutputPath: "mess", UseCase: "Conv_Rate_now, CB-4", DocReference: "FolkForm 04, 07", Order: 21, IsActive: true, CreatedAt: now, UpdatedAt: now},

		// === RAW 1h — CB-1, HB-3 ===
		{Key: "orders_1h", Label: "Orders (1 giờ)", Description: "Số đơn Pancake trong 1h", Unit: "số", Source: adsmodels.SourcePancakePos, Window: adsmodels.Window1h, WindowMs: windowMs(adsmodels.Window1h), Type: adsmodels.MetricTypeRaw, SourceCollection: global.MongoDB_ColNames.OrderCanonical, TimeField: "insertedAt", AggregationField: "posData.ad_id", OutputPath: "orders", UseCase: "HB-3 Divergence", DocReference: "FolkForm 07, HB-3", Order: 30, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Key: "mess_1h", Label: "Mess (1 giờ)", Description: "Số hội thoại fb_conversations trong 1h", Unit: "số", Source: adsmodels.SourcePancakeConversation, Window: adsmodels.Window1h, WindowMs: windowMs(adsmodels.Window1h), Type: adsmodels.MetricTypeRaw, SourceCollection: global.MongoDB_ColNames.FbConvesations, TimeField: "panCakeUpdatedAt", AggregationField: "panCakeData.ad_ids", OutputPath: "mess", UseCase: "HB-3 Divergence", DocReference: "FolkForm HB-3", Order: 31, IsActive: true, CreatedAt: now, UpdatedAt: now},

		// === RAW 30p — Msg_Rate, MQS ===
		// meta_ad_insights chỉ có daily → mess_30p lấy từ fb_conversations (như mess_2h, mess_1h).
		{Key: "mess_30p", Label: "Mess (30 phút)", Description: "Số hội thoại fb_conversations trong 30p gần nhất", Unit: "số", Source: adsmodels.SourcePancakeConversation, Window: adsmodels.Window30p, WindowMs: windowMs(adsmodels.Window30p), Type: adsmodels.MetricTypeRaw, SourceCollection: global.MongoDB_ColNames.FbConvesations, TimeField: "panCakeUpdatedAt", AggregationField: "panCakeData.ad_ids", OutputPath: "mess", UseCase: "Msg_Rate, MQS, early warning", DocReference: "FolkForm 01, 04", Order: 40, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Key: "inlineLinkClicks_30p", Label: "Inline Link Clicks (30p)", Description: "Số click trong 30p", Unit: "số", Source: adsmodels.SourceMeta, Window: adsmodels.Window30p, WindowMs: windowMs(adsmodels.Window30p), Type: adsmodels.MetricTypeRaw, SourceCollection: global.MongoDB_ColNames.MetaAdInsights, TimeField: "dateStart", OutputPath: "meta.inlineLinkClicks", UseCase: "Msg_Rate = mess/clicks", DocReference: "FolkForm 01", Order: 41, IsActive: true, CreatedAt: now, UpdatedAt: now},

		// === DERIVED 7d ===
		{Key: "convRate_7d", Label: "Conv Rate (7 ngày)", Description: "orders_7d / mess_7d — tỷ lệ chuyển đổi mess → đơn", Unit: "%", Source: adsmodels.SourceDerived, Window: adsmodels.Window7d, WindowMs: windowMs(adsmodels.Window7d), Type: adsmodels.MetricTypeDerived, FormulaRef: "orders/mess", DependsOn: []string{"orders_7d", "mess_7d"}, UseCase: "MQS, Mess Trap, Kill rules", DocReference: "FolkForm 01", Order: 50, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Key: "cpaMess_7d", Label: "CPA Mess (7 ngày)", Description: "spend / mess — chi phí mỗi cuộc hội thoại", Unit: "VND", Source: adsmodels.SourceDerived, Window: adsmodels.Window7d, WindowMs: windowMs(adsmodels.Window7d), Type: adsmodels.MetricTypeDerived, FormulaRef: "spend/mess", DependsOn: []string{"spend_7d", "mess_7d"}, UseCase: "Kill rules", DocReference: "FolkForm 01", Order: 51, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Key: "cpaPurchase_7d", Label: "CPA Purchase (7 ngày)", Description: "spend / orders — chi phí mỗi đơn", Unit: "VND", Source: adsmodels.SourceDerived, Window: adsmodels.Window7d, WindowMs: windowMs(adsmodels.Window7d), Type: adsmodels.MetricTypeDerived, FormulaRef: "spend/orders", DependsOn: []string{"spend_7d", "orders_7d"}, UseCase: "Kill rules", DocReference: "FolkForm 01", Order: 52, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Key: "msgRate_7d", Label: "Msg Rate (7 ngày)", Description: "mess / inlineLinkClicks — tỷ lệ click → mess", Unit: "%", Source: adsmodels.SourceDerived, Window: adsmodels.Window7d, WindowMs: windowMs(adsmodels.Window7d), Type: adsmodels.MetricTypeDerived, FormulaRef: "mess/clicks", DependsOn: []string{"mess_7d", "inlineLinkClicks_7d"}, UseCase: "Kill rules", DocReference: "FolkForm 01", Order: 53, IsActive: true, CreatedAt: now, UpdatedAt: now},

		// === DERIVED 2h ===
		{Key: "convRate_2h", Label: "Conv Rate (2 giờ)", Description: "orders_2h / mess_2h — CR_now cho Momentum Tracker", Unit: "%", Source: adsmodels.SourceDerived, Window: adsmodels.Window2h, WindowMs: windowMs(adsmodels.Window2h), Type: adsmodels.MetricTypeDerived, FormulaRef: "orders/mess", DependsOn: []string{"orders_2h", "mess_2h"}, UseCase: "Momentum Tracker, ACCELERATING/SLOWING/DROPPING", DocReference: "FolkForm 04", Order: 60, IsActive: true, CreatedAt: now, UpdatedAt: now},

		// === DERIVED 30p ===
		{Key: "msgRate_30p", Label: "Msg Rate (30 phút)", Description: "mess_30p / inlineLinkClicks_30p — early warning", Unit: "%", Source: adsmodels.SourceDerived, Window: adsmodels.Window30p, WindowMs: windowMs(adsmodels.Window30p), Type: adsmodels.MetricTypeDerived, FormulaRef: "mess/clicks", DependsOn: []string{"mess_30p", "inlineLinkClicks_30p"}, UseCase: "Early warning 90-120p trước CR drop", DocReference: "FolkForm 01, 04", Order: 70, IsActive: true, CreatedAt: now, UpdatedAt: now},
	}

	opts := options.Replace().SetUpsert(true)
	count := 0
	for _, s := range seeds {
		filter := bson.M{"key": s.Key}
		if _, err := coll.ReplaceOne(ctx, filter, s, opts); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}
