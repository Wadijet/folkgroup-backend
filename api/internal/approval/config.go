// Package approval — GetApprovalMode: đọc config theo domain/scope; fallback ads_meta_config, CIX_APPROVAL_ACTIONS.
package approval

import (
	"context"
	"os"
	"strings"

	pkgapproval "meta_commerce/pkg/approval"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

// GetApprovalMode trả về mode duyệt cho (domain, scopeKey, actionType).
// Fallback: domain=ads → ads_meta_config.ActionRuleConfig; domain=cix → env CIX_APPROVAL_ACTIONS.
func GetApprovalMode(ctx context.Context, ownerOrgID primitive.ObjectID, domain, scopeKey, actionType, ruleCode string) (mode string, err error) {
	// 1. Ưu tiên approval_mode_config
	cfg, err := findApprovalModeConfig(ctx, ownerOrgID, domain, scopeKey)
	if err == nil && cfg != nil {
		if cfg.ActionOverrides != nil {
			if m, ok := cfg.ActionOverrides[actionType]; ok && m != "" {
				return m, nil
			}
		}
		return cfg.Mode, nil
	}

	// 2. Fallback: domain=ads → ads_meta_config
	if domain == "ads" && scopeKey != "" {
		if auto := getAdsAutoApproveFromMetaConfig(ctx, ownerOrgID, scopeKey, ruleCode); auto {
			return pkgapproval.ApprovalModeAutoByRule, nil
		}
		return pkgapproval.ApprovalModeManualRequired, nil
	}

	// 3. Fallback: domain=cix → env CIX_APPROVAL_ACTIONS (default: escalate_to_senior,assign_to_human_sale)
	if domain == "cix" {
		approvalList := strings.TrimSpace(os.Getenv("CIX_APPROVAL_ACTIONS"))
		if approvalList == "" {
			approvalList = "escalate_to_senior,assign_to_human_sale"
		}
		needApproval := strings.Split(approvalList, ",")
		for _, a := range needApproval {
			if strings.TrimSpace(strings.ToLower(a)) == strings.TrimSpace(strings.ToLower(actionType)) {
				return pkgapproval.ApprovalModeManualRequired, nil
			}
		}
		return pkgapproval.ApprovalModeFullyAuto, nil
	}

	return pkgapproval.ApprovalModeManualRequired, nil
}

// findApprovalModeConfig tìm config theo (ownerOrgID, domain, scopeKey).
// Ưu tiên scopeKey cụ thể; nếu không có thì thử scopeKey="" (default).
func findApprovalModeConfig(ctx context.Context, ownerOrgID primitive.ObjectID, domain, scopeKey string) (*pkgapproval.ApprovalModeConfig, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ApprovalModeConfig)
	if !ok {
		return nil, nil
	}
	// Thử scopeKey cụ thể trước
	if scopeKey != "" {
		var cfg pkgapproval.ApprovalModeConfig
		err := coll.FindOne(ctx, bson.M{"ownerOrganizationId": ownerOrgID, "domain": domain, "scopeKey": scopeKey}, mongoopts.FindOne()).Decode(&cfg)
		if err == nil {
			return &cfg, nil
		}
		if err != mongo.ErrNoDocuments {
			return nil, err
		}
	}
	// Fallback: scopeKey="" (default)
	var cfg pkgapproval.ApprovalModeConfig
	err := coll.FindOne(ctx, bson.M{"ownerOrganizationId": ownerOrgID, "domain": domain, "scopeKey": ""}, mongoopts.FindOne()).Decode(&cfg)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &cfg, nil
}

// adsActionRuleItem struct tối thiểu để decode từ ads_meta_config (tránh import cycle).
type adsActionRuleItem struct {
	RuleCode    string `bson:"ruleCode"`
	Flag        string `bson:"flag"`
	AutoApprove bool   `bson:"autoApprove"`
}

// adsMetaConfigMinimal struct tối thiểu để decode ads_meta_config.
type adsMetaConfigMinimal struct {
	Campaign struct {
		ActionRuleConfig struct {
			KillRules     []adsActionRuleItem `bson:"killRules"`
			DecreaseRules []adsActionRuleItem `bson:"decreaseRules"`
			IncreaseRules []adsActionRuleItem `bson:"increaseRules"`
		} `bson:"actionRuleConfig"`
	} `bson:"campaign"`
}

// getAdsAutoApproveFromMetaConfig đọc ads_meta_config, kiểm tra ruleCode có autoApprove không.
func getAdsAutoApproveFromMetaConfig(ctx context.Context, ownerOrgID primitive.ObjectID, adAccountId, ruleCode string) bool {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsMetaConfig)
	if !ok {
		return false
	}
	var doc adsMetaConfigMinimal
	err := coll.FindOne(ctx, bson.M{"adAccountId": adAccountId, "ownerOrganizationId": ownerOrgID}).Decode(&doc)
	if err != nil {
		return false
	}
	arc := &doc.Campaign.ActionRuleConfig
	for _, r := range arc.KillRules {
		code := r.RuleCode
		if code == "" {
			code = r.Flag
		}
		if code == ruleCode {
			return r.AutoApprove
		}
	}
	for _, r := range arc.DecreaseRules {
		code := r.RuleCode
		if code == "" {
			code = r.Flag
		}
		if code == ruleCode {
			return r.AutoApprove
		}
	}
	for _, r := range arc.IncreaseRules {
		code := r.RuleCode
		if code == "" {
			code = r.Flag
		}
		if code == ruleCode {
			return r.AutoApprove
		}
	}
	return false
}
