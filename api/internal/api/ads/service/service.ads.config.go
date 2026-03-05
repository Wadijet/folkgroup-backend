// Package adssvc — Service đọc/ghi approvalConfig từ ads_approval_config (tách khỏi meta).
package adssvc

import (
	"context"
	"fmt"
	"time"

	adsmodels "meta_commerce/internal/api/ads/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GetApprovalConfig lấy approvalConfig từ ads_approval_config theo adAccountId.
func GetApprovalConfig(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) (map[string]interface{}, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsApprovalConfig)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection ads_approval_config")
	}
	var doc struct {
		ApprovalConfig map[string]interface{} `bson:"approvalConfig"`
	}
	err := coll.FindOne(ctx, bson.M{
		"adAccountId":         adAccountId,
		"ownerOrganizationId": ownerOrgID,
	}).Decode(&doc)
	if err != nil {
		return nil, fmt.Errorf("không tìm thấy cấu hình duyệt cho ad account: %w", err)
	}
	return doc.ApprovalConfig, nil
}

// UpdateApprovalConfig cập nhật approvalConfig trong ads_approval_config.
func UpdateApprovalConfig(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID, config map[string]interface{}) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsApprovalConfig)
	if !ok {
		return fmt.Errorf("không tìm thấy collection ads_approval_config")
	}
	now := time.Now().UnixMilli()
	filter := bson.M{"adAccountId": adAccountId, "ownerOrganizationId": ownerOrgID}
	update := bson.M{
		"$set": bson.M{
			"approvalConfig": config,
			"updatedAt":      now,
		},
	}
	res, err := coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		// Upsert: tạo mới nếu chưa có
		doc := &adsmodels.AdsApprovalConfig{
			AdAccountId:         adAccountId,
			OwnerOrganizationID: ownerOrgID,
			ApprovalConfig:      config,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		_, err = coll.InsertOne(ctx, doc)
		return err
	}
	return nil
}
