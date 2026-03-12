// Chẩn đoán chi tiết tại sao RunAutoPropose không tạo action.
// Chạy: cd api && go run ./cmd/diagnose_auto_propose
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"meta_commerce/config"
	adsconfig "meta_commerce/internal/api/ads/config"
	adssvc "meta_commerce/internal/api/ads/service"
	adsrules "meta_commerce/internal/api/ads/rules"
	adsmodels "meta_commerce/internal/api/ads/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	cfg := config.NewConfig()
	if cfg == nil {
		log.Fatal("Không thể đọc cấu hình")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDB_ConnectionURI))
	if err != nil {
		log.Fatalf("Kết nối MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	// Init registry
	global.MongoDB_ColNames.AdsMetaConfig = "ads_meta_config"
	global.MongoDB_ColNames.MetaCampaigns = "meta_campaigns"
	global.MongoDB_ColNames.ActionPendingApproval = "action_pending_approval"
	db := client.Database(cfg.MongoDB_DBName_Auth)
	for _, name := range []string{"ads_meta_config", "meta_campaigns", "action_pending_approval"} {
		_, _ = global.RegistryCollections.Register(name, db.Collection(name))
	}

	campaigns, err := adssvc.GetCampaignsForAutoPropose(ctx, 30)
	if err != nil {
		log.Fatalf("GetCampaignsForAutoPropose: %v", err)
	}

	fmt.Printf("\n=== KẾT QUẢ GetCampaignsForAutoPropose: %d campaigns ===\n\n", len(campaigns))

	if len(campaigns) == 0 {
		fmt.Println("Không có campaign nào. Kiểm tra ads_meta_config (autoProposeEnabled) và meta_campaigns (ACTIVE, alertFlags).")
		return
	}

	for i, c := range campaigns {
		fmt.Printf("--- Campaign %d: %s (%s) ---\n", i+1, c.CampaignName, c.CampaignId)
		fmt.Printf("  adAccountId: %s\n", c.AdAccountId)

		flags := adssvc.ParseAlertFlags(c.AlertFlags)
		if len(flags) == 0 {
			fmt.Printf("  ❌ alertFlags rỗng hoặc không parse được: %T\n\n", c.AlertFlags)
			continue
		}
		flagStrs := make([]string, len(flags))
		for j, f := range flags {
			if s, ok := f.(string); ok {
				flagStrs[j] = s
			}
		}
		fmt.Printf("  alertFlags: [%s]\n", strings.Join(flagStrs, ", "))

		metaCfg, _ := adssvc.GetCampaignConfig(ctx, c.AdAccountId, c.OwnerOrganizationID)
		killEnabled := adssvc.GetKillRulesEnabled(ctx, c.AdAccountId, c.OwnerOrganizationID)
		opts := &adsrules.EvalOptions{KillRulesEnabled: killEnabled}

		result := adsrules.EvaluateAlertFlagsWithConfig(flags, opts, metaCfg)
		source := "Kill"
		if result == nil {
			result = adsrules.EvaluateForDecreaseWithConfig(flags, metaCfg)
			source = "Decrease"
		}
		if result == nil {
			result = adsrules.EvaluateForIncrease(flags, metaCfg)
			source = "Increase"
		}

		if result == nil {
			fmt.Printf("  ❌ Không có rule nào trigger\n")
			if metaCfg != nil && !metaCfg.AutomationConfig.EffectiveBudgetRulesEnabled() {
				fmt.Printf("  ⚠️ BudgetRulesEnabled = false\n")
			}
			fmt.Println()
			continue
		}

		fmt.Printf("  ✅ Rule: %s (%s) → %s\n", result.RuleCode, source, result.ActionType)

		shouldPropose := true
		if metaCfg != nil {
			shouldPropose = shouldAutoPropose(result.RuleCode, metaCfg)
		}
		if !shouldPropose {
			fmt.Printf("  ❌ ShouldAutoPropose = false\n")
			fmt.Println()
			continue
		}

		has, err := adssvc.HasPendingProposalForCampaign(ctx, c.CampaignId, c.OwnerOrganizationID)
		if err != nil {
			fmt.Printf("  ❌ HasPending error: %v\n", err)
			fmt.Println()
			continue
		}
		if has {
			fmt.Printf("  ❌ Đã có pending (tránh duplicate)\n")
			fmt.Println()
			continue
		}

		fmt.Printf("  ✅ SẴN SÀNG TẠO ACTION\n")
		fmt.Println()
	}
}

func shouldAutoPropose(ruleCode string, metaCfg *adsmodels.CampaignConfigView) bool {
	rules := getActionRules(metaCfg)
	for _, r := range rules {
		if r.code == ruleCode {
			return r.autoPropose
		}
	}
	return true
}

func getActionRules(metaCfg *adsmodels.CampaignConfigView) []struct{ code string; autoPropose, autoApprove bool } {
	if metaCfg == nil {
		return nil
	}
	var rules []struct{ code string; autoPropose, autoApprove bool }
	arc := &metaCfg.ActionRuleConfig
	for _, r := range arc.KillRules {
		code := r.RuleCode
		if code == "" {
			code = r.Flag
		}
		rules = append(rules, struct{ code string; autoPropose, autoApprove bool }{code, r.AutoPropose, r.AutoApprove})
	}
	for _, r := range arc.DecreaseRules {
		code := r.RuleCode
		if code == "" {
			code = r.Flag
		}
		rules = append(rules, struct{ code string; autoPropose, autoApprove bool }{code, r.AutoPropose, r.AutoApprove})
	}
	if len(rules) > 0 {
		return rules
	}
	def := adsconfig.DefaultActionRuleConfig()
	for _, r := range def.KillRules {
		code := r.RuleCode
		if code == "" {
			code = r.Flag
		}
		rules = append(rules, struct{ code string; autoPropose, autoApprove bool }{code, r.AutoPropose, r.AutoApprove})
	}
	for _, r := range def.DecreaseRules {
		code := r.RuleCode
		if code == "" {
			code = r.Flag
		}
		rules = append(rules, struct{ code string; autoPropose, autoApprove bool }{code, r.AutoPropose, r.AutoApprove})
	}
	return rules
}
