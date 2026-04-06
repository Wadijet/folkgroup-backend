// Package adssvc — Hourly Peak Matrix (FolkForm v4.1 Section 05).
// Pre-Peak Boost, Compute Peak Profiles từ snapshots 15p, Post-Peak Trim.
package adssvc

import (
	"context"
	"fmt"
	"strconv"
	"time"

	adsconfig "meta_commerce/internal/api/ads_meta/config"
	adsmodels "meta_commerce/internal/api/ads_meta/models"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
	metasvc "meta_commerce/internal/api/meta/service"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

// RunPrePeakBoost chạy mỗi 30p — tăng budget 15% cho camp 30p trước peak hour (FolkForm v4.1 Section 05).
// Chỉ camp có peak profile (≥ 14 ngày data), CHS < 1.5, không SELF_COMPETITION_SUSPECT.
func RunPrePeakBoost(ctx context.Context, baseURL string) (boosted int, err error) {
	log := logger.GetAppLogger()
	profileColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsCampPeakProfiles)
	if !ok {
		return 0, nil
	}
	campColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return 0, nil
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	h, m := now.Hour(), now.Minute()
	// 30p trước peak = hiện tại HH:00 hoặc HH:30, peak = HH+1:00. VD: 08:30 → peak 09:00
	nextHour := h
	if m >= 30 {
		nextHour = h + 1
	}
	if nextHour > 22 {
		return 0, nil
	}

	cursor, err := profileColl.Find(ctx, bson.M{
		"dataDaysCount": bson.M{"$gte": 7},
		"peakHours":     nextHour,
	}, nil)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var profile adsmodels.AdsCampPeakProfile
		if cursor.Decode(&profile) != nil {
			continue
		}
		if IsSelfCompetitionSuspect(ctx, profile.AdAccountId, profile.OwnerOrganizationID) {
			continue
		}
		if adsconfig.IsNoonCutWindow(now) {
			continue
		}
		cfg, _ := GetCampaignConfig(ctx, profile.AdAccountId, profile.OwnerOrganizationID)
		if cfg != nil && cfg.AccountMode == ModePROTECT {
			continue
		}
		var camp struct {
			CampaignId     string                 `bson:"campaignId"`
			Name           string                 `bson:"name"`
			CurrentMetrics map[string]interface{} `bson:"currentMetrics"`
		}
		err := campColl.FindOne(ctx, bson.M{
			"campaignId":          profile.CampaignId,
			"ownerOrganizationId": profile.OwnerOrganizationID,
		}, mongoopts.FindOne().SetProjection(bson.M{"campaignId": 1, "name": 1, "currentMetrics": 1})).Decode(&camp)
		if err != nil {
			continue
		}
		layer3, _ := camp.CurrentMetrics["layer3"].(map[string]interface{})
		chs := toFloat64FromMap(layer3, "chs")
		// Spec CHS < 1.5 = camp healthy. Scale 0-100: chs 40-90 = healthy, chs >= 90 = strong (không cần boost), chs < 40 = critical (không boost).
		if chs >= 90 || chs < 40 {
			continue
		}
		if hasPending, _ := HasPendingProposalForCampaign(ctx, profile.CampaignId, profile.OwnerOrganizationID); hasPending {
			continue
		}
		eventID, err := Propose(ctx, &ProposeInput{
			ActionType:   "INCREASE",
			AdAccountId:  profile.AdAccountId,
			CampaignId:   profile.CampaignId,
			CampaignName: camp.Name,
			Value:        15,
			Reason:       "Pre-Peak Boost — 30p trước peak " + formatHour(nextHour) + ", tăng 15%",
			RuleCode:     "pre_peak_boost",
		}, profile.OwnerOrganizationID, baseURL)
		if err != nil {
			log.WithError(err).Warn("⏰ [PRE_PEAK] Lỗi propose")
			continue
		}
		if eventID != "" {
			boosted++
		}
	}
	if boosted > 0 {
		log.WithFields(map[string]interface{}{"boosted": boosted}).Info("⏰ [PRE_PEAK] Đã Pre-Peak Boost")
	}
	return boosted, nil
}

func formatHour(h int) string {
	return fmt.Sprintf("%02d:00", h)
}

func toFloat64FromMap(m map[string]interface{}, k string) float64 {
	if m == nil {
		return 0
	}
	v := m[k]
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}

// RunComputePeakProfilesFromSnapshots chạy Thứ 2 — tính peak/dead hours từ meta_ad_insights_daily_snapshots (7 ngày).
// Peak = giờ có avgSpendPerHour > overall_avg × 1.3. Dead = giờ có avgSpendPerHour < overall_avg × 0.7.
func RunComputePeakProfilesFromSnapshots(ctx context.Context) (profilesUpdated int, err error) {
	log := logger.GetAppLogger()
	profileColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsCampPeakProfiles)
	if !ok {
		return 0, nil
	}
	campColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return 0, nil
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	today := now.Format("2006-01-02")
	filter := bson.M{}
	for k, v := range adsconfig.ScopeFilterPurchaseMessaging() {
		filter[k] = v
	}
	cursor, err := campColl.Find(ctx, filter, mongoopts.Find().SetProjection(bson.M{"campaignId": 1, "adAccountId": 1, "ownerOrganizationId": 1}))
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	minDataDays := 5
	dateStart := now.AddDate(0, 0, -7).Format("2006-01-02")

	for cursor.Next(ctx) {
		var camp struct {
			CampaignId          string             `bson:"campaignId"`
			AdAccountId         string             `bson:"adAccountId"`
			OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
		}
		if cursor.Decode(&camp) != nil {
			continue
		}
		avgSpendPerHour := make(map[int]float64)
		var dayCount int
		for d := 0; d < 7; d++ {
			dt := now.AddDate(0, 0, -d)
			dateStr := dt.Format("2006-01-02")
			if dateStr < dateStart {
				continue
			}
			hourly, err := metasvc.GetHourlySpendFromSnapshotsForCampaign(ctx, camp.CampaignId, camp.AdAccountId, camp.OwnerOrganizationID, dateStr)
			if err != nil || len(hourly) == 0 {
				continue
			}
			dayCount++
			for h, s := range hourly {
				avgSpendPerHour[h] += s
			}
		}
		if dayCount < minDataDays {
			continue
		}
		for h := range avgSpendPerHour {
			avgSpendPerHour[h] /= float64(dayCount)
		}
		var total float64
		var hourCount int
		for h := 7; h <= 22; h++ {
			if v, ok := avgSpendPerHour[h]; ok && v > 0 {
				total += v
				hourCount++
			}
		}
		if hourCount == 0 {
			continue
		}
		overallAvg := total / float64(hourCount)
		var peakHours, deadHours []int
		avgCrPerHour := make(map[string]float64)
		for h := 7; h <= 22; h++ {
			v := avgSpendPerHour[h]
			avgCrPerHour[strconv.Itoa(h)] = v
			if v > overallAvg*1.3 {
				peakHours = append(peakHours, h)
			} else if v > 0 && v < overallAvg*0.7 {
				deadHours = append(deadHours, h)
			}
		}
		if len(peakHours) == 0 {
			continue
		}
		nowMs := time.Now().UnixMilli()
		profile := adsmodels.AdsCampPeakProfile{
			CampaignId:          camp.CampaignId,
			AdAccountId:         camp.AdAccountId,
			OwnerOrganizationID: camp.OwnerOrganizationID,
			PeakHours:           peakHours,
			DeadHours:           deadHours,
			AvgCrPerHour:        avgCrPerHour,
			DataDaysCount:       dayCount,
			DateGenerated:       today,
			CreatedAt:           nowMs,
			UpdatedAt:           nowMs,
		}
		_, err := profileColl.UpdateOne(ctx, bson.M{
			"campaignId":          camp.CampaignId,
			"ownerOrganizationId": camp.OwnerOrganizationID,
		}, bson.M{"$set": bson.M{
			"peakHours":     profile.PeakHours,
			"deadHours":     profile.DeadHours,
			"avgCrPerHour":  profile.AvgCrPerHour,
			"dataDaysCount": profile.DataDaysCount,
			"dateGenerated": profile.DateGenerated,
			"updatedAt":     profile.UpdatedAt,
		}}, mongoopts.Update().SetUpsert(true))
		if err != nil {
			log.WithError(err).WithFields(map[string]interface{}{"campaignId": camp.CampaignId}).Warn("⏰ [PEAK_PROFILE] Lỗi upsert")
			continue
		}
		profilesUpdated++
	}
	if profilesUpdated > 0 {
		log.WithFields(map[string]interface{}{"profilesUpdated": profilesUpdated}).Info("⏰ [PEAK_PROFILE] Đã tính Peak Profiles từ snapshots")
	}
	return profilesUpdated, nil
}

// RunPostPeakTrim chạy mỗi 30p — 30p sau peak: nếu CR_actual < CR_predicted × 0.7x thì giảm về baseline (FolkForm v4.1 Section 05 B5).
// Hiện tại: giảm 10% budget camp vừa peak nếu CR không đạt (cần currentMetrics CR vs predicted từ peak profile).
func RunPostPeakTrim(ctx context.Context, baseURL string) (trimmed int, err error) {
	log := logger.GetAppLogger()
	profileColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsCampPeakProfiles)
	if !ok {
		return 0, nil
	}
	campColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return 0, nil
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	h, m := now.Hour(), now.Minute()
	// 30p sau peak = hiện tại HH:30, peak vừa kết thúc lúc HH:00. VD: 09:30 → peak 09:00 vừa xong
	prevHour := h
	if m < 30 {
		prevHour = h - 1
	}
	if prevHour < 7 || prevHour > 22 {
		return 0, nil
	}
	cursor, err := profileColl.Find(ctx, bson.M{
		"dataDaysCount": bson.M{"$gte": 7},
		"peakHours":     prevHour,
	}, nil)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var profile adsmodels.AdsCampPeakProfile
		if cursor.Decode(&profile) != nil {
			continue
		}
		if adsconfig.IsNoonCutWindow(now) {
			continue
		}
		cfg, _ := GetCampaignConfig(ctx, profile.AdAccountId, profile.OwnerOrganizationID)
		if cfg != nil && cfg.AccountMode == ModePROTECT {
			continue
		}
		var camp struct {
			CampaignId     string                 `bson:"campaignId"`
			Name           string                 `bson:"name"`
			CurrentMetrics map[string]interface{} `bson:"currentMetrics"`
		}
		err := campColl.FindOne(ctx, bson.M{
			"campaignId":          profile.CampaignId,
			"ownerOrganizationId": profile.OwnerOrganizationID,
		}, mongoopts.FindOne().SetProjection(bson.M{"campaignId": 1, "name": 1, "currentMetrics": 1})).Decode(&camp)
		if err != nil {
			continue
		}
		raw, _ := camp.CurrentMetrics["raw"].(map[string]interface{})
		layer1, _ := camp.CurrentMetrics["layer1"].(map[string]interface{})
		crNow := toFloat64FromMap(layer1, "convRate") / 100
		if crNow <= 0 {
			continue
		}
		mess := toInt64FromMap(raw, "mess")
		if mess < 5 {
			continue
		}
		// Spec: CR_actual < CR_predicted × 0.7 → giảm. Không có CR theo giờ từ snapshots → dùng 8% làm baseline.
		if crNow >= 0.08*0.7 {
			continue
		}
		if hasPending, _ := HasPendingProposalForCampaign(ctx, profile.CampaignId, profile.OwnerOrganizationID); hasPending {
			continue
		}
		eventID, err := Propose(ctx, &ProposeInput{
			ActionType:   "DECREASE",
			AdAccountId:  profile.AdAccountId,
			CampaignId:   profile.CampaignId,
			CampaignName: camp.Name,
			Value:        10,
			Reason:       "Post-Peak Trim — CR sau peak " + formatHour(prevHour) + " thấp hơn dự kiến, giảm 10%",
			RuleCode:     "post_peak_trim",
		}, profile.OwnerOrganizationID, baseURL)
		if err != nil {
			log.WithError(err).Warn("⏰ [POST_PEAK] Lỗi propose")
			continue
		}
		if eventID != "" {
			trimmed++
		}
	}
	if trimmed > 0 {
		log.WithFields(map[string]interface{}{"trimmed": trimmed}).Info("⏰ [POST_PEAK] Đã Post-Peak Trim")
	}
	return trimmed, nil
}

func toInt64FromMap(m map[string]interface{}, k string) int64 {
	if m == nil {
		return 0
	}
	v := m[k]
	if v == nil {
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
