// Package adssvc — Commands: /resume_ads, /pancake_ok (FolkForm v4.1).
package adssvc

import (
	"context"

	"meta_commerce/internal/approval"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ResumeAds bật lại tất cả campaign sau Circuit Breaker. Xóa circuitBreakerTriggered, RESUME từng campaign PAUSED.
func ResumeAds(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) (resumed int, err error) {
	log := logger.GetAppLogger()
	accColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdAccounts)
	if !ok {
		return 0, nil
	}
	campColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return 0, nil
	}
	// Xóa circuit breaker state
	_, err = accColl.UpdateOne(ctx,
		bson.M{
			"adAccountId":         bson.M{"$regex": "^" + adAccountId + "$", "$options": "i"},
			"ownerOrganizationId": ownerOrgID,
		},
		bson.M{"$unset": bson.M{
			"circuitBreakerTriggered": "",
			"circuitBreakerAt":        "",
			"circuitBreakerSnapshot":  "",
		}},
	)
	if err != nil {
		return 0, err
	}
	// Lấy campaigns PAUSED
	cursor, err := campColl.Find(ctx, bson.M{
		"adAccountId":         adAccountId,
		"ownerOrganizationId": ownerOrgID,
		"$or":                 []bson.M{{"effectiveStatus": "PAUSED"}, {"status": "PAUSED"}},
	}, nil)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	resumed = 0
	for cursor.Next(ctx) {
		var doc struct {
			CampaignId string `bson:"campaignId"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		pending, err := Propose(ctx, &ProposeInput{
			ActionType:   "RESUME",
			AdAccountId:  adAccountId,
			CampaignId:   doc.CampaignId,
			Reason:       "Resume Ads — /resume_ads sau Circuit Breaker",
			RuleCode:     "resume_ads",
		}, ownerOrgID, "")
		if err != nil {
			log.WithError(err).WithFields(map[string]interface{}{"campaignId": doc.CampaignId}).Warn("[RESUME_ADS] Lỗi propose")
			continue
		}
		if pending != nil {
			_, _ = approval.Approve(ctx, pending.ID.Hex(), ownerOrgID)
			resumed++
		}
	}
	log.WithFields(map[string]interface{}{
		"adAccountId": adAccountId,
		"resumed":     resumed,
	}).Info("🔄 [RESUME_ADS] Đã bật lại campaign")
	return resumed, nil
}

// PancakeOk gỡ pancakeDownOverride — xác nhận Pancake đã hoạt động trở lại. /pancake_ok
func PancakeOk(ctx context.Context) (cleared int, err error) {
	configColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsMetaConfig)
	if !ok {
		return 0, nil
	}
	res, err := configColl.UpdateMany(ctx, bson.M{"account.automationConfig.pancakeDownOverride": true}, bson.M{
		"$unset": bson.M{
			"account.automationConfig.pancakeDownOverride": "",
			"account.automationConfig.pancakeDownAt":       "",
		},
	})
	if err != nil {
		return 0, err
	}
	return int(res.ModifiedCount), nil
}
