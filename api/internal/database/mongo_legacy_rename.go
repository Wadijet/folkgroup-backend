package database

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// legacyCollectionRenames — tên collection cũ (server/db trước refactor) → tên mới (khớp initColNames).
// Chỉ dùng khi bật MONGO_LEGACY_COLLECTION_RENAME; xem RenameLegacyMongoCollections.
var legacyCollectionRenames = []struct {
	Old string
	New string
}{
	{"access_tokens", "auth_core_access_tokens"},
	{"action_pending_approval", "approval_job_pending_actions"},
	{"ads_activity_history", "ads_run_activity_history"},
	{"ads_approval_config", "ads_cfg_approval"},
	{"ads_campaign_hourly", "ads_rm_campaign_hourly"},
	{"ads_camp_peak_profiles", "ads_rm_campaign_peak_profiles"},
	{"ads_camp_thresholds", "ads_cfg_campaign_thresholds"},
	{"ads_counterfactual_outcomes", "ads_run_counterfactual_outcomes"},
	{"ads_intel_compute", "ads_job_intel"},
	{"ads_kill_snapshots", "ads_rm_kill_snapshots"},
	{"ads_meta_config", "ads_cfg_meta"},
	{"ads_meta_intel_runs", "ads_run_intel"},
	{"ads_metric_definitions", "ads_cfg_metric_definitions"},
	{"ads_throttle_state", "ads_state_throttle"},
	{"agent_activity_logs", "agent_run_activity_logs"},
	{"agent_commands", "agent_job_commands"},
	{"agent_configs", "agent_cfg_configs"},
	{"agent_registry", "agent_core_registry"},
	{"ai_candidates", "ai_core_candidates"},
	{"ai_generation_batches", "ai_job_generation_batches"},
	{"ai_prompt_templates", "ai_cfg_prompt_templates"},
	{"ai_provider_profiles", "ai_cfg_provider_profiles"},
	{"ai_runs", "ai_run_generations"},
	{"ai_step_runs", "ai_run_steps"},
	{"ai_steps", "ai_core_steps"},
	{"ai_workflow_commands", "ai_job_workflow_commands"},
	{"ai_workflow_runs", "ai_run_workflows"},
	{"ai_workflows", "ai_core_workflows"},
	{"approval_mode_config", "approval_cfg_mode"},
	{"auth_organization_config_items", "auth_cfg_organization_items"},
	{"auth_organization_shares", "auth_rel_organization_shares"},
	{"auth_organizations", "auth_core_organizations"},
	{"auth_permissions", "auth_core_permissions"},
	{"auth_role_permissions", "auth_rel_role_permissions"},
	{"auth_roles", "auth_core_roles"},
	{"auth_user_roles", "auth_rel_user_roles"},
	{"auth_users", "auth_core_users"},
	{"content_nodes", "content_core_nodes"},
	{"content_publications", "content_core_publications"},
	{"content_videos", "content_core_videos"},
	{"cta_library", "cta_core_library"},
	{"cta_tracking", "cta_run_tracking"},
	{"customer_activity_history", "customer_run_activity_history"},
	{"customer_bulk_jobs", "customer_job_bulk"},
	{"customer_customers", "customer_core_records"},
	{"customer_intel_compute", "customer_job_intel"},
	{"customer_intel_runs", "customer_run_intel"},
	{"customer_notes", "customer_core_notes"},
	{"customer_pending_merge", "customer_job_pending_merge"},
	{"cix_analysis_results", "cix_run_analysis_results"},
	{"cix_intel_compute", "cix_job_intel"},
	{"decision_cases_runtime", "decision_state_cases_runtime"},
	{"decision_context_policy_overrides", "decision_cfg_context_policy_overrides"},
	{"decision_debounce_state", "decision_state_debounce"},
	{"decision_events_queue", "decision_job_events"},
	{"decision_org_live_events", "decision_run_org_live_events"},
	{"decision_recompute_debounce_queue", "decision_state_recompute_debounce"},
	{"decision_routing_rules", "decision_cfg_routing_rules"},
	{"decision_trailing_debounce", "decision_state_trailing_debounce"},
	{"delivery_history", "delivery_run_history"},
	{"delivery_queue", "delivery_job_queue"},
	{"fb_conversations", "fb_src_conversations"},
	{"fb_customers", "fb_src_customers"},
	{"fb_message_items", "fb_src_message_items"},
	{"fb_messages", "fb_src_messages"},
	{"fb_pages", "fb_src_pages"},
	{"fb_posts", "fb_src_posts"},
	{"learning_cases", "learning_core_cases"},
	{"meta_ad_accounts", "meta_src_ad_accounts"},
	{"meta_ad_insights_daily_snapshots", "meta_rm_ad_insights_daily_snapshots"},
	{"meta_ad_insights", "meta_src_ad_insights"},
	{"meta_ads", "meta_src_ads"},
	{"meta_adsets", "meta_src_adsets"},
	{"meta_campaigns", "meta_src_campaigns"},
	{"notification_channels", "notification_cfg_channels"},
	{"notification_routing_rules", "notification_cfg_routing_rules"},
	{"notification_senders", "notification_cfg_senders"},
	{"notification_templates", "notification_cfg_templates"},
	{"order_canonical", "order_core_records"},
	{"order_intel_compute", "order_job_intel"},
	{"order_intel_runs", "order_run_intel"},
	{"order_intel_snapshots", "order_rm_intel"},
	{"pc_pos_categories", "order_src_pcpos_categories"},
	{"pc_pos_customers", "pc_pos_src_customers"},
	{"pc_pos_orders", "order_src_pcpos_orders"},
	{"pc_pos_products", "order_src_pcpos_products"},
	{"pc_pos_shops", "pc_pos_src_shops"},
	{"pc_pos_variations", "order_src_pcpos_variations"},
	{"pc_pos_warehouses", "pc_pos_src_warehouses"},
	{"report_definitions", "report_cfg_definitions"},
	{"report_dirty_periods", "report_state_dirty_periods"},
	{"report_snapshots", "report_rm_snapshots"},
	{"rule_definitions", "rule_cfg_definitions"},
	{"rule_execution_logs", "rule_run_execution_logs"},
	{"rule_logic_definitions", "rule_cfg_logic_definitions"},
	{"rule_output_definitions", "rule_cfg_output_definitions"},
	{"rule_param_sets", "rule_cfg_param_sets"},
	{"rule_suggestions", "learning_rm_rule_suggestions"},
	{"webhook_logs", "webhook_run_logs"},
}

// isLegacyCollectionRenameEnabled — bật rename khi khởi động (DB cũ còn tên collection trước refactor).
// Env: MONGO_LEGACY_COLLECTION_RENAME=1|true|yes (không phân biệt hoa thường).
func isLegacyCollectionRenameEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("MONGO_LEGACY_COLLECTION_RENAME")))
	return v == "1" || v == "true" || v == "yes"
}

// RenameLegacyMongoCollections đổi tên collection cũ → mới bằng lệnh rename của MongoDB (cùng database).
// Gọi một lần khi nâng server mới lên DB cũ: bật MONGO_LEGACY_COLLECTION_RENAME=1.
//
// Quy tắc an toàn:
// - Chỉ rename khi collection cũ tồn tại và collection đích chưa tồn tại.
// - Nếu cả hai đều tồn tại: ghi cảnh báo và bỏ qua (cần xử lý thủ công).
func RenameLegacyMongoCollections(ctx context.Context, client *mongo.Client, dbName string) error {
	if !isLegacyCollectionRenameEnabled() {
		return nil
	}
	if client == nil || strings.TrimSpace(dbName) == "" {
		return nil
	}
	db := client.Database(dbName)
	collList, err := db.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("legacy rename: không liệt kê được collection: %w", err)
	}
	exists := make(map[string]bool, len(collList))
	for _, n := range collList {
		exists[n] = true
	}

	var renamed int
	for _, pair := range legacyCollectionRenames {
		oldName, newName := pair.Old, pair.New
		if oldName == newName {
			continue
		}
		if !exists[oldName] {
			continue
		}
		if exists[newName] {
			logger.GetAppLogger().WithFields(map[string]interface{}{
				"old": oldName, "new": newName,
			}).Warn("[LEGACY_RENAME] Bỏ qua: collection đích đã tồn tại — cần gộp/xóa thủ công")
			continue
		}
		// renameCollection — lệnh chạy trên DB admin (driver Go bản dự án không có Collection.Rename).
		fromNS := dbName + "." + oldName
		toNS := dbName + "." + newName
		cmd := bson.D{
			{Key: "renameCollection", Value: fromNS},
			{Key: "to", Value: toNS},
		}
		if err := client.Database("admin").RunCommand(ctx, cmd).Err(); err != nil {
			return fmt.Errorf("legacy rename %s -> %s: %w", oldName, newName, err)
		}
		logger.GetAppLogger().WithFields(map[string]interface{}{
			"old": oldName, "new": newName,
		}).Info("[LEGACY_RENAME] Đã đổi tên collection")
		exists[newName] = true
		delete(exists, oldName)
		renamed++
	}
	if renamed > 0 {
		logger.GetAppLogger().WithField("renamedCount", renamed).Info("[LEGACY_RENAME] Hoàn tất đổi tên collection cũ")
	}
	return nil
}

// InitRenameLegacyMongoCollectionsIfEnabled gói RenameLegacyMongoCollections với timeout ngắn (startup).
func InitRenameLegacyMongoCollectionsIfEnabled(client *mongo.Client, dbName string) error {
	if !isLegacyCollectionRenameEnabled() {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	return RenameLegacyMongoCollections(ctx, client, dbName)
}
