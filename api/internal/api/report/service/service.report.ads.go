// Package reportsvc - ComputeAdsDailyReport: aggregate meta_ad_insights + meta_campaigns → report_snapshots (ads_daily).
// Theo ADS_INTELLIGENCE_DESIGN.md Phần 9A: phát sinh trong kỳ từ Meta (spend, clicks, impressions...), activeCampaigns.
package reportsvc

import (
	"context"
	"fmt"
	"time"

	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ComputeAdsDailyReport tính snapshot ads_daily: aggregate meta_ad_insights (dateStart=periodKey) + meta_campaigns.
// adAccountId: dimensions theo account. Khi rỗng (manual trigger), compute cho tất cả ad accounts của org.
func (s *ReportService) ComputeAdsDailyReport(ctx context.Context, periodKey string, ownerOrganizationID primitive.ObjectID, adAccountId string) error {
	if adAccountId != "" {
		return s.computeAdsDailyForAccount(ctx, periodKey, ownerOrganizationID, adAccountId)
	}
	// Manual trigger: lấy tất cả ad accounts của org
	accColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdAccounts)
	if !ok {
		return fmt.Errorf("không tìm thấy collection meta_ad_accounts")
	}
	cursor, err := accColl.Find(ctx, bson.M{"ownerOrganizationId": ownerOrganizationID}, nil)
	if err != nil {
		return fmt.Errorf("lấy ad accounts: %w", err)
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var doc struct {
			AdAccountId string `bson:"adAccountId"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		if doc.AdAccountId == "" {
			continue
		}
		if err := s.computeAdsDailyForAccount(ctx, periodKey, ownerOrganizationID, doc.AdAccountId); err != nil {
			return err
		}
	}
	return nil
}

func (s *ReportService) computeAdsDailyForAccount(ctx context.Context, periodKey string, ownerOrganizationID primitive.ObjectID, adAccountId string) error {
	insightsColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdInsights)
	if !ok {
		return fmt.Errorf("không tìm thấy collection %s", global.MongoDB_ColNames.MetaAdInsights)
	}
	campaignsColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return fmt.Errorf("không tìm thấy collection %s", global.MongoDB_ColNames.MetaCampaigns)
	}

	metrics := make(map[string]interface{})

	// 1. Aggregate meta_ad_insights: dateStart = periodKey, ownerOrganizationId, adAccountId.
	filterInsights := bson.M{
		"dateStart":            periodKey,
		"ownerOrganizationId": ownerOrganizationID,
		"adAccountId":         adAccountId,
	}
	extractInlineClicks := bson.M{"$convert": bson.M{"input": bson.M{"$ifNull": bson.A{"$metaData.inline_link_clicks", "0"}}, "to": "long", "onError": 0, "onNull": 0}}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: filterInsights}},
		{{Key: "$addFields", Value: bson.M{"_extractedInlineClicks": extractInlineClicks}}},
		{{Key: "$group", Value: bson.M{
			"_id":               nil,
			"spend":             bson.M{"$sum": bson.M{"$convert": bson.M{"input": "$spend", "to": "double", "onError": 0, "onNull": 0}}},
			"impressions":       bson.M{"$sum": bson.M{"$convert": bson.M{"input": "$impressions", "to": "long", "onError": 0, "onNull": 0}}},
			"clicks":            bson.M{"$sum": bson.M{"$convert": bson.M{"input": "$clicks", "to": "long", "onError": 0, "onNull": 0}}},
			"reach":             bson.M{"$sum": bson.M{"$convert": bson.M{"input": "$reach", "to": "long", "onError": 0, "onNull": 0}}},
			"inlineLinkClicks":  bson.M{"$sum": "$_extractedInlineClicks"},
		}}},
	}
	cursor, err := insightsColl.Aggregate(ctx, pipeline, options.Aggregate())
	if err != nil {
		return fmt.Errorf("aggregate meta_ad_insights: %w", err)
	}
	defer cursor.Close(ctx)

	if cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			return fmt.Errorf("decode insights: %w", err)
		}
		if v, ok := raw["spend"].(float64); ok {
			metrics["spend"] = v
		} else {
			metrics["spend"] = 0.0
		}
		if v, ok := raw["impressions"].(int64); ok {
			metrics["impressions"] = v
		} else if v, ok := raw["impressions"].(int32); ok {
			metrics["impressions"] = int64(v)
		} else {
			metrics["impressions"] = int64(0)
		}
		if v, ok := raw["clicks"].(int64); ok {
			metrics["clicks"] = v
		} else if v, ok := raw["clicks"].(int32); ok {
			metrics["clicks"] = int64(v)
		} else {
			metrics["clicks"] = int64(0)
		}
		if v, ok := raw["reach"].(int64); ok {
			metrics["reach"] = v
		} else if v, ok := raw["reach"].(int32); ok {
			metrics["reach"] = int64(v)
		} else {
			metrics["reach"] = int64(0)
		}
		if v, ok := raw["inlineLinkClicks"].(int64); ok {
			metrics["inlineLinkClicks"] = v
		} else if v, ok := raw["inlineLinkClicks"].(int32); ok {
			metrics["inlineLinkClicks"] = int64(v)
		} else {
			metrics["inlineLinkClicks"] = int64(0)
		}
	}
	if len(metrics) == 0 {
		metrics["spend"] = 0.0
		metrics["impressions"] = int64(0)
		metrics["clicks"] = int64(0)
		metrics["reach"] = int64(0)
		metrics["inlineLinkClicks"] = int64(0)
	}
	if _, ok := metrics["inlineLinkClicks"]; !ok {
		metrics["inlineLinkClicks"] = int64(0)
	}

	// 2. Parse inline_link_clicks từ metaData nếu có (Phase 1: sum thủ công từ cursor)
	// Meta lưu spend/impressions/clicks/reach dạng string — $convert xử lý. inline_link_clicks trong metaData.
	// Tạm bỏ qua inline_link_clicks nếu không có field extract — bổ sung sau.

	// 3. activeCampaigns: count meta_campaigns có effectiveStatus = ACTIVE, ownerOrganizationId, adAccountId
	filterCampaigns := bson.M{
		"ownerOrganizationId": ownerOrganizationID,
		"adAccountId":         adAccountId,
		"effectiveStatus":     "ACTIVE",
	}
	activeCount, err := campaignsColl.CountDocuments(ctx, filterCampaigns)
	if err != nil {
		return fmt.Errorf("count active campaigns: %w", err)
	}
	metrics["activeCampaigns"] = activeCount

	// 4. campaignsCreatedInPeriod: count meta_campaigns có createdAt trong kỳ
	loc, err := time.LoadLocation(ReportTimezone)
	if err != nil {
		return fmt.Errorf("load timezone: %w", err)
	}
	t, err := time.ParseInLocation("2006-01-02", periodKey, loc)
	if err != nil {
		return fmt.Errorf("parse periodKey: %w", err)
	}
	startMs := t.Unix() * 1000
	endMs := t.AddDate(0, 0, 1).Unix()*1000 - 1
	filterCreated := bson.M{
		"ownerOrganizationId": ownerOrganizationID,
		"adAccountId":         adAccountId,
		"createdAt":           bson.M{"$gte": startMs, "$lte": endMs},
	}
	createdCount, err := campaignsColl.CountDocuments(ctx, filterCreated)
	if err != nil {
		return fmt.Errorf("count campaigns created: %w", err)
	}
	metrics["campaignsCreatedInPeriod"] = createdCount

	// 5. Derived: cpc, cpm, ctr (nếu có đủ data)
	if spend, ok := metrics["spend"].(float64); ok && spend > 0 {
		if imp, ok := metrics["impressions"].(int64); ok && imp > 0 {
			metrics["cpm"] = spend / float64(imp) * 1000
		}
		if clk, ok := metrics["clicks"].(int64); ok && clk > 0 {
			metrics["cpc"] = spend / float64(clk)
		}
		if imp, ok := metrics["impressions"].(int64); ok && imp > 0 {
			if clk, ok := metrics["clicks"].(int64); ok {
				metrics["ctr"] = float64(clk) / float64(imp) * 100
			}
		}
	}

	dimensions := map[string]interface{}{"adAccountId": adAccountId}
	return s.upsertSnapshotWithDimensions(ctx, "ads_daily", periodKey, "day", ownerOrganizationID, dimensions, metrics)
}

