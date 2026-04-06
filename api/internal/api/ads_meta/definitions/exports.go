// Package definitions — Nguồn định nghĩa duy nhất.
package definitions

import (
	"meta_commerce/internal/api/ads_meta/config"
)

// Flags trả về định nghĩa tất cả cờ (metrics + thresholds → flag). Nguồn duy nhất.
func Flags() []config.FlagDefinition {
	return config.FlagDefinitions()
}

// Metrics trả về định nghĩa tất cả chỉ số.
func Metrics() []config.MetricMetadata {
	return config.MetricDefinitions()
}

// Thresholds trả về metadata tất cả ngưỡng.
func Thresholds() []config.ThresholdMetadata {
	return config.ThresholdDefinitions()
}

// DynamicFlags trả về định nghĩa cờ động.
func DynamicFlags() []config.DynamicFlagMetadata {
	return config.DynamicFlagDefinitions()
}

// ActionRuleCodes trả về metadata action rules (cờ → action).
func ActionRuleCodes() []config.RuleCodeMetadata {
	return config.ActionRuleDefinitions()
}

// AllRuleCodeStrings trả về danh sách mã rule cho automation.
func AllRuleCodeStrings() []string {
	return config.AllRuleCodeStrings()
}
