// Package migration — Map ads_meta_config.ActionRuleConfig (autoApprove) sang approval_mode_config.
// Chạy một lần khi chuyển sang Vision 08 — config duyệt thống nhất.
// Lưu ý: approval_mode_config dùng actionOverrides[actionType], không per-ruleCode.
// Nếu bất kỳ rule nào trong nhóm (Kill/Decrease/Increase) có autoApprove → actionType đó auto.
package migration

import (
	"context"
	"time"

	pkgapproval "meta_commerce/pkg/approval"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

// adsActionRuleItem struct tối thiểu để decode từ ads_meta_config.
type adsActionRuleItem struct {
	RuleCode    string `bson:"ruleCode"`
	Flag        string `bson:"flag"`
	AutoApprove bool   `bson:"autoApprove"`
}

// adsMetaConfigForApproval struct tối thiểu để decode ads_meta_config.
type adsMetaConfigForApproval struct {
	AdAccountId         string              `bson:"adAccountId"`
	OwnerOrganizationID primitive.ObjectID  `bson:"ownerOrganizationId"`
	Campaign            struct {
		ActionRuleConfig struct {
			KillRules     []adsActionRuleItem `bson:"killRules"`
			DecreaseRules []adsActionRuleItem `bson:"decreaseRules"`
			IncreaseRules []adsActionRuleItem `bson:"increaseRules"`
		} `bson:"actionRuleConfig"`
	} `bson:"campaign"`
}

// MigrateAdsMetaConfigToApprovalMode tạo approval_mode_config từ ads_meta_config.
// Chỉ tạo khi chưa có approval_mode_config cho (ownerOrgID, domain=ads, scopeKey=adAccountId).
// actionOverrides: PAUSE/DECREASE/INCREASE → auto_by_rule khi có ít nhất 1 rule autoApprove.
func MigrateAdsMetaConfigToApprovalMode(ctx context.Context) (created int, err error) {
	metaColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsMetaConfig)
	if !ok {
		return 0, nil
	}
	approvalColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ApprovalModeConfig)
	if !ok {
		return 0, nil
	}

	cursor, err := metaColl.Find(ctx, bson.M{})
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	now := time.Now().UnixMilli()
	for cursor.Next(ctx) {
		var doc adsMetaConfigForApproval
		if err := cursor.Decode(&doc); err != nil || doc.AdAccountId == "" {
			continue
		}

		// Chỉ tạo khi chưa có approval_mode_config cho scope này
		var existing pkgapproval.ApprovalModeConfig
		err := approvalColl.FindOne(ctx, bson.M{
			"ownerOrganizationId": doc.OwnerOrganizationID,
			"domain":              "ads",
			"scopeKey":            doc.AdAccountId,
		}, mongoopts.FindOne()).Decode(&existing)
		if err == nil {
			continue // Đã có config, bỏ qua
		}

		// Xây actionOverrides từ ActionRuleConfig
		overrides := make(map[string]string)
		arc := &doc.Campaign.ActionRuleConfig

		if hasAnyAutoApprove(arc.KillRules) {
			overrides["PAUSE"] = pkgapproval.ApprovalModeAutoByRule
		}
		if hasAnyAutoApprove(arc.DecreaseRules) {
			overrides["DECREASE"] = pkgapproval.ApprovalModeAutoByRule
		}
		if hasAnyAutoApprove(arc.IncreaseRules) {
			overrides["INCREASE"] = pkgapproval.ApprovalModeAutoByRule
		}

		// Nếu không có rule nào autoApprove → mode=manual, không cần tạo doc (fallback ads_meta_config xử lý)
		if len(overrides) == 0 {
			continue
		}

		cfg := pkgapproval.ApprovalModeConfig{
			OwnerOrganizationID: doc.OwnerOrganizationID,
			Domain:              "ads",
			ScopeKey:            doc.AdAccountId,
			Mode:                pkgapproval.ApprovalModeManualRequired,
			ActionOverrides:     overrides,
		}
		// Thêm metadata cho migration
		insertDoc := bson.M{
			"ownerOrganizationId": cfg.OwnerOrganizationID,
			"domain":              cfg.Domain,
			"scopeKey":            cfg.ScopeKey,
			"mode":                cfg.Mode,
			"actionOverrides":     cfg.ActionOverrides,
			"createdAt":           now,
			"updatedAt":           now,
			"migratedFrom":        "ads_meta_config",
		}

		_, err = approvalColl.InsertOne(ctx, insertDoc)
		if err != nil {
			continue
		}
		created++
	}
	return created, nil
}

func hasAnyAutoApprove(rules []adsActionRuleItem) bool {
	for _, r := range rules {
		if r.AutoApprove {
			return true
		}
	}
	return false
}
