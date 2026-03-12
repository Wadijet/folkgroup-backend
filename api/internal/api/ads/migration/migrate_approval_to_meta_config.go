// Package migration — Copy autoProposeEnabled, killRulesEnabled từ ads_approval_config sang ads_meta_config.
// Chạy một lần khi chuyển từ ads_approval_config sang ads_meta_config.
package migration

import (
	"context"
	"strings"
	"time"

	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MigrateApprovalConfigToAdsMetaConfig copy autoProposeEnabled, killRulesEnabled từ ads_approval_config sang ads_meta_config.
// Chỉ update ads_meta_config đã có; không tạo mới. Bỏ qua nếu ads_approval_config không tồn tại.
func MigrateApprovalConfigToAdsMetaConfig(ctx context.Context) (updated int, err error) {
	approvalColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsApprovalConfig)
	if !ok {
		return 0, nil
	}
	metaColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsMetaConfig)
	if !ok {
		return 0, nil
	}
	cursor, err := approvalColl.Find(ctx, bson.M{})
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
		if err := cursor.Decode(&doc); err != nil || doc.AdAccountId == "" {
			continue
		}
		autoPropose := true
		killRulesEnabled := true // freezeKillRules=false → enabled
		if doc.ApprovalConfig != nil {
			if v, ok := doc.ApprovalConfig["autoProposeEnabled"]; ok {
				if b, ok := v.(bool); ok {
					autoPropose = b
				} else if s, ok := v.(string); ok {
					autoPropose = strings.EqualFold(s, "true")
				}
			}
			if v, ok := doc.ApprovalConfig["freezeKillRules"]; ok {
				var freezeKill bool
				if b, ok := v.(bool); ok {
					freezeKill = b
				} else if s, ok := v.(string); ok {
					freezeKill = strings.EqualFold(s, "true")
				}
				killRulesEnabled = !freezeKill
			}
		}
		res, err := metaColl.UpdateOne(ctx,
			bson.M{
				"adAccountId":         doc.AdAccountId,
				"ownerOrganizationId": doc.OwnerOrganizationID,
			},
			bson.M{
				"$set": bson.M{
					"account.automationConfig.autoProposeEnabled": autoPropose,
					"account.automationConfig.killRulesEnabled":   killRulesEnabled,
					"updatedAt":                                    now,
				},
			},
		)
		if err != nil {
			continue
		}
		if res.ModifiedCount > 0 {
			updated++
		}
	}
	return updated, nil
}
