package adsautop

import (
	"context"
	"time"

	adsconfig "meta_commerce/internal/api/ads/config"
	adssvc "meta_commerce/internal/api/ads/service"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RunVolumePush chạy 16:00 (BLITZ) hoặc 18:00 (NORMAL). EFFICIENCY không có.
// Gọi RunAutoPropose (pipeline AI Decision — emit ads.propose_requested).
func RunVolumePush(ctx context.Context, baseURL string) {
	log := logger.GetAppLogger()
	accColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdAccounts)
	if !ok {
		return
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	h := now.Hour()

	cursor, err := accColl.Find(ctx, bson.M{}, nil)
	if err != nil {
		return
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var acc struct {
			AdAccountId         string             `bson:"adAccountId"`
			OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
		}
		if err := cursor.Decode(&acc); err != nil {
			continue
		}
		cfg, _ := adsconfig.GetConfigForCampaign(ctx, acc.AdAccountId, acc.OwnerOrganizationID)
		accountMode := adssvc.ModeNORMAL
		if cfg != nil && cfg.AccountMode != "" {
			accountMode = cfg.AccountMode
		}
		if accountMode == adssvc.ModeEFFICIENCY {
			continue
		}
		targetHour := 18
		if accountMode == adssvc.ModeBLITZ {
			targetHour = 16
		}
		if h != targetHour {
			continue
		}

		_, err := RunAutoPropose(ctx, baseURL)
		if err != nil {
			log.WithError(err).WithFields(map[string]interface{}{"adAccountId": acc.AdAccountId}).Warn("[VOLUME_PUSH] Lỗi")
		}
	}
	log.Info("📈 [VOLUME_PUSH] Đã chạy Volume Push")
}
