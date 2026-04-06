# Module Ads — Sử dụng cơ chế duyệt + Queue + Retry

Module ads **dùng** package `approval` (cơ chế duyệt tách riêng). Sau khi approve, lệnh được đưa vào hàng đợi để worker xử lý với cơ chế retry.

## Phạm vi

- **Propose:** Wrapper gọi `approval.Propose(domain="ads", ...)` với payload ads-specific
- **Config:** `ads_meta_config` — 1 document per (adAccountId, ownerOrganizationId). Gồm account, campaign, adSet, ad. account: accountMode, CommonConfig, AutomationConfig. campaign: FlagRuleConfig, ActionRuleConfig.
- **Executor:** Đăng ký `approval.RegisterExecutor("ads", ...)` — thực thi qua Meta API
- **Deferred execution:** Domain ads dùng queue — sau approve → status=queued, worker xử lý với retry

## Luồng xử lý

1. **Propose** → status=pending
2. **Approve** → status=queued (không execute ngay)
3. **AdsExecutionWorker** poll queue mỗi 30s, thực thi qua Meta API
4. **Thành công** → status=executed, gửi notification
5. **Thất bại** → retry với exponential backoff (tối đa 5 lần), sau đó status=failed

## Phạm vi hỗ trợ

- **Tất cả level:** campaign, adset, ad — executor xử lý theo ưu tiên ad > adset > campaign
- **Actions (đủ loại Meta API hỗ trợ):**

| Action | Mô tả | value |
|--------|-------|-------|
| KILL, PAUSE | Tạm dừng (status=PAUSED) | — |
| RESUME | Bật lại (status=ACTIVE) | — |
| ARCHIVE | Lưu trữ (status=ARCHIVED) | — |
| DELETE | Xóa (status=DELETED) | — |
| SET_BUDGET | Đặt daily_budget (cent) | số cent |
| SET_LIFETIME_BUDGET | Đặt lifetime_budget (cent) | số cent |
| INCREASE | Tăng budget theo % | % (vd: 15 = +15%) |
| DECREASE | Giảm budget theo % | % (vd: 10 = -10%) |
| SET_NAME | Đổi tên | tên mới |

- **Lý do (reason):** Bắt buộc khi tạo lệnh — lưu trong payload để hiển thị khi duyệt

## API

| Route | Mô tả | Permission |
|-------|-------|------------|
| POST /ads/commands | **Tạo lệnh chờ duyệt** — user trực tiếp tạo | MetaAdAccount.Read |
| POST /ads/actions/propose | Thêm đề xuất (alias) | MetaAdAccount.Update |
| POST /ads/actions/approve | Duyệt → đưa vào queue | MetaAdAccount.Update |
| POST /ads/actions/reject | Từ chối | MetaAdAccount.Update |
| GET /ads/actions/pending | Danh sách pending domain=ads | MetaAdAccount.Read |
| POST /ads/commands/resume-ads | Bật lại campaign sau Circuit Breaker | MetaAdAccount.Update |
| POST /ads/commands/pancake-ok | Gỡ Pancake Down override | MetaAdAccount.Update |
| GET/PUT /ads/config/approval | approvalConfig (legacy, trong meta_ad_accounts) | MetaAdAccount.Update |
| GET/PUT /ads/config/meta | Cấu hình Meta Ads (account, campaign, adSet, ad). 1 document per ad account | MetaAdAccount.Update |
| GET /ads/config/metric-definitions | Danh sách metric definitions (7d, 2h, 1h, 30p) | MetaAdAccount.Read |

## Cấu hình ads_meta_config (1 document per ad account)

- **account:** accountMode (BLITZ/NORMAL/EFFICIENCY/PROTECT), CommonConfig (timezone, CB4, Night Off, Reset Budget), AutomationConfig (AutoProposeEnabled, KillRulesEnabled, BudgetRulesEnabled, PancakeDownOverride)
- **campaign:** FlagRuleConfig (thresholds, trim window, flagDefinitions), ActionRuleConfig (KillRules, DecreaseRules, IncreaseRules — mỗi rule có AutoPropose, AutoApprove)
- **adSet, ad:** dự phòng cho tương lai

## Scheduler (FolkForm v4.1)

| Job | Giờ | Mô tả |
|-----|-----|-------|
| Reset Budget | 05:30 | Best_day logic (TODO: chưa implement) |
| Morning On | 06:00 | Bật lại camp tốt (mo_eligible) |
| Noon Cut | 12:30, 14:00 | Tắt camp chết trưa (noon_cut_eligible) |
| Noon Cut Resume | 14:30 | Bật lại camp đã tắt bởi Noon Cut |
| Night Off | 21h–23h | Tắt camp theo mode (PROTECT 21h, EFFICIENCY 22h, NORMAL 22:30, BLITZ 23h) |
| Volume Push | 16h (BLITZ), 18h (NORMAL) | RunAutoPropose — tăng budget camp tốt |

## Metric definitions

Collection `ads_metric_definitions` — định nghĩa metrics theo window (7d, 2h, 1h, 30p). Dùng cho Momentum Tracker, CB-4, evaluation pipeline.

## Cơ chế duyệt

Logic queue, approve, reject nằm ở **`internal/approval`** — package độc lập, dùng chung cho ads, content, ...
