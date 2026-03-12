// Package adssvc — Service đọc/ghi approvalConfig. Đọc/ghi từ ads_meta_config (automationConfig).
// Deprecated: Dùng GetAdsMetaConfig / UpdateAdsMetaConfig thay thế. API /ads/config/approval giữ để tương thích.
package adssvc

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GetApprovalConfig lấy approvalConfig từ ads_meta_config (account.automationConfig). Trả về map tương thích API cũ.
func GetApprovalConfig(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) (map[string]interface{}, error) {
	cfg, err := GetAdsMetaConfig(ctx, adAccountId, ownerOrgID)
	if err != nil {
		return nil, fmt.Errorf("không tìm thấy cấu hình duyệt cho ad account: %w", err)
	}
	return map[string]interface{}{
		"autoProposeEnabled":  cfg.Account.AutomationConfig.AutoProposeEnabled,
		"killRulesEnabled":    cfg.Account.AutomationConfig.EffectiveKillRulesEnabled(),
		"budgetRulesEnabled":  cfg.Account.AutomationConfig.EffectiveBudgetRulesEnabled(),
	}, nil
}

// UpdateApprovalConfig cập nhật approvalConfig vào ads_meta_config (account.automationConfig).
func UpdateApprovalConfig(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID, config map[string]interface{}) error {
	existing, err := GetAdsMetaConfig(ctx, adAccountId, ownerOrgID)
	if err != nil {
		return err
	}
	if v, ok := config["autoProposeEnabled"]; ok {
		if b, ok := v.(bool); ok {
			existing.Account.AutomationConfig.AutoProposeEnabled = b
		}
	}
	// killRulesEnabled: công tắc mới. freezeKillRules (legacy): true = tắt kill → killRulesEnabled=false
	if v, ok := config["killRulesEnabled"]; ok {
		if b, ok := v.(bool); ok {
			existing.Account.AutomationConfig.KillRulesEnabled = &b
		}
	} else if v, ok := config["freezeKillRules"]; ok {
		if b, ok := v.(bool); ok {
			enabled := !b
			existing.Account.AutomationConfig.KillRulesEnabled = &enabled
		}
	}
	if v, ok := config["budgetRulesEnabled"]; ok {
		if b, ok := v.(bool); ok {
			existing.Account.AutomationConfig.BudgetRulesEnabled = &b
		}
	}
	return UpdateAdsMetaConfig(ctx, adAccountId, ownerOrgID, existing)
}
