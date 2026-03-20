// Package approval — Validation schema theo domain+actionType (Vision 08 Sub-layer 2).
package approval

import (
	"context"
	"fmt"
	"strings"
)

// requiredFieldsByDomain map domain -> actionType -> required field paths (trong payload).
var requiredFieldsByDomain = map[string]map[string][]string{
	"ads": {
		"KILL": {"adAccountId"}, "PAUSE": {"adAccountId"}, "RESUME": {"adAccountId"},
		"ARCHIVE": {"adAccountId"}, "DELETE": {"adAccountId"},
		"SET_BUDGET": {"adAccountId"}, "SET_LIFETIME_BUDGET": {"adAccountId"},
		"INCREASE": {"adAccountId", "campaignId"}, "DECREASE": {"adAccountId"},
		"SET_NAME": {"adAccountId"},
	},
	"cix": {
		"trigger_fast_response":     {"customerUid", "sessionUid"},
		"escalate_to_senior":         {"customerUid", "sessionUid"},
		"assign_to_human_sale":        {"customerUid", "sessionUid"},
		"prioritize_followup":        {"customerUid"},
	},
}

// validatePayload kiểm tra payload có đủ required fields theo domain+actionType.
func validatePayload(domain, actionType string, payload map[string]interface{}) error {
	byDomain, ok := requiredFieldsByDomain[domain]
	if !ok {
		return nil
	}
	required, ok := byDomain[actionType]
	if !ok {
		return nil
	}
	var missing []string
	for _, field := range required {
		v, ok := payload[field]
		if !ok || v == nil {
			missing = append(missing, field)
			continue
		}
		if s, ok := v.(string); ok && s == "" {
			missing = append(missing, field)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("payload thiếu field bắt buộc: %s", strings.Join(missing, ", "))
	}
	return nil
}

// PayloadValidator interface cho validation tùy chỉnh (app có thể inject).
type PayloadValidator interface {
	Validate(ctx context.Context, domain, actionType string, payload map[string]interface{}) error
}

// ensureIdempotencyKey tự gán idempotencyKey vào payload nếu thiếu (Phase 2 Intake).
// Format: decisionCaseId:actionType:proposedAt (nếu có decisionCaseId) hoặc domain:actionType:proposedAt.
func ensureIdempotencyKey(payload map[string]interface{}, domain, actionType string, proposedAt int64) {
	if payload == nil {
		return
	}
	if s, ok := payload["idempotencyKey"].(string); ok && s != "" {
		return
	}
	dcID := extractStr(payload, "decisionCaseId")
	if dcID != "" {
		payload["idempotencyKey"] = fmt.Sprintf("%s:%s:%d", dcID, actionType, proposedAt)
	} else {
		payload["idempotencyKey"] = fmt.Sprintf("%s:%s:%d", domain, actionType, proposedAt)
	}
}

// extractStr lấy string từ map (dùng cho Explainability Snapshot).
func extractStr(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key].(string)
	if !ok {
		return ""
	}
	return v
}
