// Package adssvc — Anti Self-Competition (FolkForm v4.1 Section 06).
// Giới hạn số camp tăng cùng lúc theo mode. CPM Spike Detection → SELF_COMPETITION_SUSPECT.
package adssvc

import (
	"context"
	"time"

	adsconfig "meta_commerce/internal/api/ads/config"
	metasvc "meta_commerce/internal/api/meta/service"
	"meta_commerce/internal/approval"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

const (
	colAdsSelfCompetitionState = "ads_self_competition_state"
)

// GetIncreaseLimit trả về số camp tối đa được tăng cùng lúc theo mode (FolkForm v4.1 Section 06).
// PROTECT: 1 | EFFICIENCY: 1 | NORMAL: 2 | BLITZ: 3
func GetIncreaseLimit(mode string) int {
	switch mode {
	case ModePROTECT:
		return 1
	case ModeEFFICIENCY:
		return 1
	case ModeNORMAL:
		return 2
	case ModeBLITZ:
		return 3
	default:
		return 2 // NORMAL mặc định
	}
}

// IsSelfCompetitionSuspect kiểm tra account có đang trong thời gian pause Increase (60p) không.
func IsSelfCompetitionSuspect(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) bool {
	coll, ok := global.RegistryCollections.Get(colAdsSelfCompetitionState)
	if !ok {
		return false
	}
	var doc struct {
		SuspectUntil int64 `bson:"suspectUntil"`
	}
	err := coll.FindOne(ctx, bson.M{
		"adAccountId":         adAccountId,
		"ownerOrganizationId": ownerOrgID,
	}, mongoopts.FindOne().SetProjection(bson.M{"suspectUntil": 1})).Decode(&doc)
	if err != nil {
		return false
	}
	return time.Now().UnixMilli() < doc.SuspectUntil
}

// SetSelfCompetitionSuspect set flag SELF_COMPETITION_SUSPECT — pause Increase 60p.
func SetSelfCompetitionSuspect(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID, cpmNow, cpmChangePct float64) {
	coll, ok := global.RegistryCollections.Get(colAdsSelfCompetitionState)
	if !ok {
		return
	}
	now := time.Now().UnixMilli()
	suspectUntil := now + 60*60*1000 // 60 phút
	_, err := coll.UpdateOne(ctx,
		bson.M{"adAccountId": adAccountId, "ownerOrganizationId": ownerOrgID},
		bson.M{"$set": bson.M{
			"suspectUntil":  suspectUntil,
			"triggeredAt":   now,
			"cpmAtTrigger":  cpmNow,
			"cpmChangePct":  cpmChangePct,
			"updatedAt":     now,
		}},
		mongoopts.Update().SetUpsert(true),
	)
	if err != nil {
		logger.GetAppLogger().WithError(err).Warn("⚔️ [SELF_COMP] Lỗi set suspect")
		return
	}
	logger.GetAppLogger().WithFields(map[string]interface{}{
		"adAccountId": adAccountId,
		"cpmNow":      cpmNow,
		"cpmChangePct": cpmChangePct,
	}).Info("⚔️ [SELF_COMP] CPM spike — Pause Increase 60p")
}

// ClearSelfCompetitionSuspect gỡ flag khi CPM trở về bình thường.
func ClearSelfCompetitionSuspect(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) {
	coll, ok := global.RegistryCollections.Get(colAdsSelfCompetitionState)
	if !ok {
		return
	}
	_, _ = coll.DeleteOne(ctx, bson.M{
		"adAccountId":         adAccountId,
		"ownerOrganizationId": ownerOrgID,
	})
}

// RunCPMSpikeDetectionAndActions chạy mỗi 30p (FolkForm v4.1 Section 06).
// 1. CPM Spike Detection: CPM_avg tăng >30% so với 1h trước VÀ ≥3 camp cùng tăng → SetSelfCompetitionSuspect
// 2. Action khi flag: giảm 10% budget camp CPA_Purchase cao nhất
// 3. Gỡ flag: CPM < 3day_avg × 1.15x → ClearSelfCompetitionSuspect
func RunCPMSpikeDetectionAndActions(ctx context.Context, baseURL string) error {
	log := logger.GetAppLogger()
	accColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdAccounts)
	if !ok {
		return nil
	}
	cursor, err := accColl.Find(ctx, bson.M{}, mongoopts.Find().SetProjection(bson.M{"adAccountId": 1, "ownerOrganizationId": 1}))
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var acc struct {
			AdAccountId         string             `bson:"adAccountId"`
			OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
		}
		if cursor.Decode(&acc) != nil {
			continue
		}
		if isEvent, _, _ := adsconfig.IsEventWindow(time.Now()); isEvent {
			continue
		}
		spend1h, imp1h, ok1 := metasvc.GetSpendImpressions1h(ctx, acc.AdAccountId, acc.OwnerOrganizationID)
		spend1hAgo, imp1hAgo, ok1Ago := metasvc.GetSpendImpressions1hAgo(ctx, acc.AdAccountId, acc.OwnerOrganizationID)
		cpm3d, ok3d := metasvc.GetCPM3dayAvgFromInsights(ctx, acc.AdAccountId, acc.OwnerOrganizationID)

		if IsSelfCompetitionSuspect(ctx, acc.AdAccountId, acc.OwnerOrganizationID) {
			if ok3d && imp1h > 0 {
				cpmNow := spend1h / (imp1h / 1000)
				if cpmNow < cpm3d*1.15 {
					ClearSelfCompetitionSuspect(ctx, acc.AdAccountId, acc.OwnerOrganizationID)
					log.WithFields(map[string]interface{}{"adAccountId": acc.AdAccountId}).Info("⚔️ [SELF_COMP] CPM bình thường — gỡ flag")
				} else {
					reduceWorstCampWhenSuspect(ctx, acc.AdAccountId, acc.OwnerOrganizationID, baseURL)
				}
			}
			continue
		}
		if !ok1 || !ok1Ago || imp1h < 1000 || imp1hAgo < 1000 {
			continue
		}
		cpmNow := spend1h / (imp1h / 1000)
		cpmAgo := spend1hAgo / (imp1hAgo / 1000)
		if cpmAgo <= 0 {
			continue
		}
		changePct := (cpmNow - cpmAgo) / cpmAgo * 100
		if changePct <= 30 {
			continue
		}
		campCount := countCampsWithCPMIncrease(ctx, acc.AdAccountId, acc.OwnerOrganizationID, 30)
		if campCount < 3 {
			continue
		}
		SetSelfCompetitionSuspect(ctx, acc.AdAccountId, acc.OwnerOrganizationID, cpmNow, changePct)
		reduceWorstCampWhenSuspect(ctx, acc.AdAccountId, acc.OwnerOrganizationID, baseURL)
	}
	return nil
}

func countCampsWithCPMIncrease(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID, minPct float64) int {
	campColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return 0
	}
	filter := bson.M{
		"adAccountId":         adAccountId,
		"ownerOrganizationId": ownerOrgID,
		"$and": bson.A{
			bson.M{"$or": []bson.M{{"effectiveStatus": "ACTIVE"}, {"status": "ACTIVE"}}},
			adsconfig.ScopeFilterPurchaseMessaging(),
		},
	}
	cursor, err := campColl.Find(ctx, filter, mongoopts.Find().SetProjection(bson.M{"campaignId": 1, "adAccountId": 1}))
	if err != nil {
		return 0
	}
	defer cursor.Close(ctx)
	var count int
	for cursor.Next(ctx) {
		var c struct {
			CampaignId  string `bson:"campaignId"`
			AdAccountId string `bson:"adAccountId"`
		}
		if cursor.Decode(&c) != nil {
			continue
		}
		s1, i1, ok1 := metasvc.GetSpendImpressions1hForCampaign(ctx, c.CampaignId, c.AdAccountId, ownerOrgID)
		s2, i2, ok2 := metasvc.GetSpendImpressions1hAgoForCampaign(ctx, c.CampaignId, c.AdAccountId, ownerOrgID)
		if !ok1 || !ok2 || i1 < 100 || i2 < 100 {
			continue
		}
		cpm1 := s1 / (float64(i1) / 1000)
		cpm2 := s2 / (float64(i2) / 1000)
		if cpm2 <= 0 {
			continue
		}
		if (cpm1-cpm2)/cpm2*100 >= minPct {
			count++
		}
	}
	return count
}

func reduceWorstCampWhenSuspect(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID, baseURL string) {
	campColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return
	}
	filter := bson.M{
		"adAccountId":         adAccountId,
		"ownerOrganizationId": ownerOrgID,
		"currentMetrics.layer1": bson.M{"$exists": true},
		"$and": bson.A{
			bson.M{"$or": []bson.M{{"effectiveStatus": "ACTIVE"}, {"status": "ACTIVE"}}},
			adsconfig.ScopeFilterPurchaseMessaging(),
		},
	}
	cursor, err := campColl.Find(ctx, filter, nil)
	if err != nil {
		return
	}
	defer cursor.Close(ctx)
	var worst struct {
		CampaignId  string
		Name        string
		CpaPurchase float64
	}
	worst.CpaPurchase = -1
	for cursor.Next(ctx) {
		var c struct {
			CampaignId     string                 `bson:"campaignId"`
			Name           string                 `bson:"name"`
			CurrentMetrics map[string]interface{} `bson:"currentMetrics"`
		}
		if cursor.Decode(&c) != nil {
			continue
		}
		layer1, _ := c.CurrentMetrics["layer1"].(map[string]interface{})
		cpaPur := toFloat64SelfComp(layer1, "cpaPurchase_7d")
		if cpaPur > worst.CpaPurchase {
			worst.CampaignId = c.CampaignId
			worst.Name = c.Name
			worst.CpaPurchase = cpaPur
		}
	}
	if worst.CampaignId == "" {
		return
	}
	if hasPending, _ := HasPendingProposalForCampaign(ctx, worst.CampaignId, ownerOrgID); hasPending {
		return
	}
	pending, err := Propose(ctx, &ProposeInput{
		ActionType:   "DECREASE",
		AdAccountId:  adAccountId,
		CampaignId:   worst.CampaignId,
		CampaignName: worst.Name,
		Value:        10,
		Reason:       "Anti Self-Competition — CPM spike, giảm 10% camp CPA Purchase cao nhất",
		RuleCode:     "self_comp_reduce",
	}, ownerOrgID, baseURL)
	if err != nil || pending == nil {
		return
	}
	approval.Approve(ctx, pending.ID.Hex(), ownerOrgID)
}

func toFloat64SelfComp(m map[string]interface{}, k string) float64 {
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
