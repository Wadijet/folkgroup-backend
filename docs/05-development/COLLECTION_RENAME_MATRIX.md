# Ma trận đổi tên collection

Tài liệu này liệt kê mapping **trước -> sau** và mô tả ngắn cho từng collection theo chuẩn mới `domain_layer_entity`.

## Tự động đổi tên khi nâng server (DB cũ)

Trên môi trường vẫn còn **tên collection cũ**, có thể bật rename một lần khi khởi động API:

- Biến môi trường: `MONGO_LEGACY_COLLECTION_RENAME=1` (hoặc `true` / `yes`).
- Logic: `database.InitRenameLegacyMongoCollectionsIfEnabled` — gọi **trước** `EnsureDatabaseAndCollections`.
- Chỉ rename khi collection **cũ tồn tại** và collection **mới chưa có**. Nếu cả hai đều có: bỏ qua và ghi log cảnh báo (cần xử lý tay).

Mapping code: `api/internal/database/mongo_legacy_rename.go` (đồng bộ với bảng dưới đây).

## Nhóm Auth + Nguồn FB/POS

| Trước | Sau | Mục đích |
|---|---|---|
| `auth_users` | `auth_core_users` | Hồ sơ người dùng hệ thống |
| `auth_permissions` | `auth_core_permissions` | Danh mục quyền |
| `auth_roles` | `auth_core_roles` | Danh mục vai trò |
| `auth_role_permissions` | `auth_rel_role_permissions` | Quan hệ role-permission |
| `auth_user_roles` | `auth_rel_user_roles` | Quan hệ user-role |
| `auth_organizations` | `auth_core_organizations` | Tổ chức/tenant |
| `auth_organization_config_items` | `auth_cfg_organization_items` | Cấu hình theo tổ chức |
| `access_tokens` | `auth_core_access_tokens` | Token truy cập |
| `fb_pages` | `fb_src_pages` | Mirror trang Facebook |
| `fb_conversations` | `fb_src_conversations` | Mirror hội thoại Facebook |
| `fb_messages` | `fb_src_messages` | Mirror metadata tin nhắn |
| `fb_message_items` | `fb_src_message_items` | Mirror message item |
| `fb_posts` | `fb_src_posts` | Mirror bài viết Facebook |
| `fb_customers` | `fb_src_customers` | Mirror khách hàng từ Facebook |
| `pc_pos_customers` | `pc_pos_src_customers` | Mirror khách hàng từ Pancake POS |
| `pc_pos_shops` | `pc_pos_src_shops` | Mirror cửa hàng Pancake POS |
| `pc_pos_warehouses` | `pc_pos_src_warehouses` | Mirror kho Pancake POS |
| `pc_pos_products` | `order_src_pcpos_products` | Nguồn sản phẩm phục vụ domain đơn |
| `pc_pos_variations` | `order_src_pcpos_variations` | Nguồn biến thể phục vụ domain đơn |
| `pc_pos_categories` | `order_src_pcpos_categories` | Nguồn danh mục phục vụ domain đơn |
| `pc_pos_orders` | `order_src_pcpos_orders` | Nguồn đơn từ Pancake POS |
| `manual_pos_customers` | `order_src_manual_customers` | Nguồn khách hàng nhập tay phục vụ domain đơn |
| `manual_pos_shops` | `order_src_manual_shops` | Nguồn cửa hàng nhập tay phục vụ domain đơn |
| `manual_pos_warehouses` | `order_src_manual_warehouses` | Nguồn kho nhập tay phục vụ domain đơn |
| `manual_pos_products` | `order_src_manual_products` | Nguồn sản phẩm nhập tay phục vụ domain đơn |
| `manual_pos_variations` | `order_src_manual_variations` | Nguồn biến thể nhập tay phục vụ domain đơn |
| `manual_pos_categories` | `order_src_manual_categories` | Nguồn danh mục nhập tay phục vụ domain đơn |
| `manual_pos_orders` | `order_src_manual_orders` | Nguồn đơn nhập tay |
| `order_canonical` | `order_core_records` | Canonical đơn đa nguồn |

## Nhóm Notification / Delivery / CTA / Agent / Webhook

| Trước | Sau | Mục đích |
|---|---|---|
| `notification_senders` | `notification_cfg_senders` | Cấu hình sender |
| `notification_channels` | `notification_cfg_channels` | Cấu hình kênh gửi |
| `notification_templates` | `notification_cfg_templates` | Template thông báo |
| `notification_routing_rules` | `notification_cfg_routing_rules` | Rule định tuyến thông báo |
| `delivery_queue` | `delivery_job_queue` | Hàng đợi gửi |
| `delivery_history` | `delivery_run_history` | Lịch sử gửi |
| `cta_library` | `cta_core_library` | Thư viện CTA |
| `cta_tracking` | `cta_run_tracking` | Theo dõi hiệu quả CTA |
| `agent_registry` | `agent_core_registry` | Danh bạ agent |
| `agent_configs` | `agent_cfg_configs` | Cấu hình agent |
| `agent_commands` | `agent_job_commands` | Lệnh chờ agent xử lý |
| `agent_activity_logs` | `agent_run_activity_logs` | Log hoạt động agent |
| `webhook_logs` | `webhook_run_logs` | Nhật ký webhook |

## Nhóm Content + AI

| Trước | Sau | Mục đích |
|---|---|---|
| `content_nodes` | `content_core_nodes` | Cây nội dung |
| `content_videos` | `content_core_videos` | Nội dung video |
| `content_publications` | `content_core_publications` | Bản phát hành nội dung |
| `ai_workflows` | `ai_core_workflows` | Định nghĩa workflow AI |
| `ai_steps` | `ai_core_steps` | Định nghĩa step workflow AI |
| `ai_prompt_templates` | `ai_cfg_prompt_templates` | Mẫu prompt |
| `ai_provider_profiles` | `ai_cfg_provider_profiles` | Cấu hình provider AI |
| `ai_workflow_runs` | `ai_run_workflows` | Lịch sử chạy workflow |
| `ai_step_runs` | `ai_run_steps` | Lịch sử chạy step |
| `ai_generation_batches` | `ai_job_generation_batches` | Job batch sinh nội dung |
| `ai_candidates` | `ai_core_candidates` | Ứng viên output AI |
| `ai_runs` | `ai_run_generations` | Lịch sử generation |
| `ai_workflow_commands` | `ai_job_workflow_commands` | Lệnh workflow |

## Nhóm Report + Customer

| Trước | Sau | Mục đích |
|---|---|---|
| `report_definitions` | `report_cfg_definitions` | Định nghĩa báo cáo |
| `report_snapshots` | `report_rm_snapshots` | Snapshot báo cáo |
| `report_dirty_periods` | `report_state_dirty_periods` | Kỳ cần tính lại |
| `customer_customers` | `customer_core_records` | Canonical khách hàng |
| `customer_activity_history` | `customer_run_activity_history` | Lịch sử hoạt động khách |
| `customer_notes` | `customer_core_notes` | Ghi chú khách hàng |
| `customer_pending_merge` | `customer_job_pending_merge` | Queue merge khách |
| `customer_bulk_jobs` | `customer_job_bulk` | Job xử lý hàng loạt |
| `customer_intel_compute` | `customer_job_intel` | Queue tính customer intel |
| `customer_intel_runs` | `customer_run_intel` | Lịch sử chạy customer intel |

## Nhóm Meta / Ads / Approval / Learning / Rule

| Trước | Sau | Mục đích |
|---|---|---|
| `meta_ad_accounts` | `meta_src_ad_accounts` | Nguồn ad account Meta |
| `meta_campaigns` | `meta_src_campaigns` | Nguồn campaign Meta |
| `meta_adsets` | `meta_src_adsets` | Nguồn adset Meta |
| `meta_ads` | `meta_src_ads` | Nguồn ads Meta |
| `meta_ad_insights` | `meta_src_ad_insights` | Nguồn insight Meta |
| `meta_ad_insights_daily_snapshots` | `meta_rm_ad_insights_daily_snapshots` | Snapshot insight theo ngày |
| `action_pending_approval` | `approval_job_pending_actions` | Queue action chờ duyệt |
| `approval_mode_config` | `approval_cfg_mode` | Cấu hình mode duyệt |
| `ads_approval_config` | `ads_cfg_approval` | Cấu hình duyệt ads |
| `ads_activity_history` | `ads_run_activity_history` | Lịch sử hoạt động ads |
| `ads_meta_config` | `ads_cfg_meta` | Cấu hình nghiệp vụ ads |
| `ads_metric_definitions` | `ads_cfg_metric_definitions` | Định nghĩa metric ads |
| `ads_camp_thresholds` | `ads_cfg_campaign_thresholds` | Ngưỡng theo campaign |
| `ads_kill_snapshots` | `ads_rm_kill_snapshots` | Snapshot khi kill ads |
| `ads_counterfactual_outcomes` | `ads_run_counterfactual_outcomes` | Kết quả counterfactual |
| `ads_campaign_hourly` | `ads_rm_campaign_hourly` | Read model theo giờ |
| `ads_camp_peak_profiles` | `ads_rm_campaign_peak_profiles` | Hồ sơ khung giờ tốt |
| `ads_throttle_state` | `ads_state_throttle` | State throttle ads |
| `decision_recompute_debounce_queue` | `decision_state_recompute_debounce` | State debounce recompute |
| `ads_intel_compute` | `ads_job_intel` | Queue tính ads intel |
| `ads_meta_intel_runs` | `ads_run_intel` | Lịch sử chạy ads intel |
| `learning_cases` | `learning_core_cases` | Case học từ hành động |
| `rule_suggestions` | `learning_rm_rule_suggestions` | Gợi ý rule từ learning |
| `rule_definitions` | `rule_cfg_definitions` | Định nghĩa rule |
| `rule_logic_definitions` | `rule_cfg_logic_definitions` | Logic script của rule |
| `rule_param_sets` | `rule_cfg_param_sets` | Bộ tham số rule |
| `rule_output_definitions` | `rule_cfg_output_definitions` | Contract output rule |
| `rule_execution_logs` | `rule_run_execution_logs` | Log thực thi rule |

## Nhóm CIX / Order Intel / Decision

| Trước | Sau | Mục đích |
|---|---|---|
| `cix_analysis_results` | `cix_run_analysis_results` | Kết quả phân tích CIX |
| `cix_intel_compute` | `cix_job_intel` | Queue tính CIX intel |
| `order_intel_snapshots` | `order_rm_intel` | Read model order intel |
| `order_intel_compute` | `order_job_intel` | Queue tính order intel |
| `order_intel_runs` | `order_run_intel` | Lịch sử chạy order intel |
| `decision_events_queue` | `decision_job_events` | Queue event cho AI Decision |
| `decision_cases_runtime` | `decision_state_cases_runtime` | State runtime case |
| `decision_debounce_state` | `decision_state_debounce` | State debounce message/event |
| `decision_trailing_debounce` | `decision_state_trailing_debounce` | State trailing debounce |
| `decision_routing_rules` | `decision_cfg_routing_rules` | Rule routing decision |
| `decision_context_policy_overrides` | `decision_cfg_context_policy_overrides` | Override policy context |
| `decision_org_live_events` | `decision_run_org_live_events` | Log org live event |
