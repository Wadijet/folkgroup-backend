// Package migration — Migration approvalConfig từ meta_ad_accounts sang ads_approval_config.
package migration

import (
	"context"
	"time"

	adsmodels "meta_commerce/internal/api/ads/models"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// MigrateApprovalConfigFromMetaAdAccounts chuyển approvalConfig từ meta_ad_accounts sang ads_approval_config.
// Chạy một lần khi upgrade. Bỏ qua nếu ads_approval_config đã có bản ghi cho adAccountId.
func MigrateApprovalConfigFromMetaAdAccounts(ctx context.Context) (migrated int, err error) {
	metaColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdAccounts)
	if !ok {
		return 0, nil
	}
	adsColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsApprovalConfig)
	if !ok {
		return 0, nil
	}
	cursor, err := metaColl.Find(ctx, bson.M{"approvalConfig": bson.M{"$exists": true, "$ne": nil}})
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)
	now := time.Now().UnixMilli()
	for cursor.Next(ctx) {
		var doc struct {
			AdAccountId         string                 `bson:"adAccountId"`
			OwnerOrganizationID primitive.ObjectID     `bson:"ownerOrganizationId"`
			ApprovalConfig      map[string]interface{} `bson:"approvalConfig"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		if doc.AdAccountId == "" || doc.ApprovalConfig == nil || len(doc.ApprovalConfig) == 0 {
			continue
		}
		oid := doc.OwnerOrganizationID
		// Kiểm tra đã có trong ads_approval_config chưa
		var existing adsmodels.AdsApprovalConfig
		err = adsColl.FindOne(ctx, bson.M{
			"adAccountId":         doc.AdAccountId,
			"ownerOrganizationId": oid,
		}).Decode(&existing)
		if err == nil {
			continue // Đã có, bỏ qua
		}
		if err != mongo.ErrNoDocuments {
			continue
		}
		// Insert vào ads_approval_config
		newDoc := &adsmodels.AdsApprovalConfig{
			AdAccountId:         doc.AdAccountId,
			OwnerOrganizationID: oid,
			ApprovalConfig:      doc.ApprovalConfig,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		_, err = adsColl.InsertOne(ctx, newDoc)
		if err != nil {
			logger.GetAppLogger().WithError(err).WithField("adAccountId", doc.AdAccountId).Warn("[MIGRATION] Không insert được ads_approval_config")
			continue
		}
		migrated++
	}
	return migrated, nil
}

