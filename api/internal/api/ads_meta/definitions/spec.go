// Package definitions — Nguồn định nghĩa duy nhất cho FLAG và ACTION (FolkForm v4.1).
//
// # Luồng tổng quan
//
//	Metrics (chỉ số) → so sánh với Thresholds (ngưỡng) → điều kiện kết hợp → FLAG
//	FLAG → ACTION (PAUSE, DECREASE, INCREASE)
//
// # Phân tách rõ — tất cả driven bởi definitions
//
//   - FLAG: Điều kiện metrics + thresholds → set cờ. KHÔNG có action.
//     Định nghĩa: FlagDefinitions() (conditionGroups).
//     Thực thi: meta/service ComputeAlertFlags (Rule Engine RULE_ADS_FLAG_*).
//
//   - ACTION: Cờ → hành động. Định nghĩa: DefaultKillRuleSpecs, DefaultDecreaseRuleSpecs, ActionRuleDefinitions.
//     Thực thi: Rule Engine (ruleintel) — RULE_ADS_KILL_*, RULE_ADS_DECREASE_*, RULE_ADS_INCREASE_*.
//     Khi config rỗng: getKillRules/getDecreaseRules dùng DefaultActionRuleConfig (từ specs).
//
// # Init config
//
//   - InitDefaultConfig, DefaultFlagRuleConfig, DefaultAutomationConfig: tất cả từ definitions.
//   - DefaultThresholds từ ThresholdDefinitions (DefaultValue).
//   - DefaultCommonConfig, DefaultAutomationActionRules từ metadata.
//   - ShouldAutoPropose/ShouldAutoApprove: ActionRules rỗng → DefaultAutomationActionRules.
//
// # Cấu trúc
//
//   - Metrics: Định nghĩa chỉ số (cpaMess, cpm, mess, ...)
//   - Thresholds: Ngưỡng cấu hình (DefaultValue cho init)
//   - Flags: Điều kiện set cờ (conditionGroups: AND trong group, OR giữa groups)
//   - Operators: GREATER_THAN, LESS_THAN, EQUAL, IN, ... (chuẩn Meta Ad Rules)
//   - DynamicFlags: Cờ động (diagnosis_xxx từ mảng)
//   - ActionRules: Cờ → action (killRules, decreaseRules từ ActionRuleSpecs)
package definitions
