// Package approval — Unit test cho Phase 2 Intake (ensureIdempotencyKey, validatePayload).
package approval

import (
	"strings"
	"testing"
)

func TestEnsureIdempotencyKey_NilPayload_NoPanic(t *testing.T) {
	ensureIdempotencyKey(nil, "ads", "KILL", 1234567890)
}

func TestEnsureIdempotencyKey_EmptyPayload_GeneratesKey(t *testing.T) {
	payload := map[string]interface{}{}
	ensureIdempotencyKey(payload, "ads", "KILL", 1700000000000)
	if v, ok := payload["idempotencyKey"].(string); !ok || v == "" {
		t.Errorf("ensureIdempotencyKey phải gán idempotencyKey, got %v", payload["idempotencyKey"])
	}
	if got := payload["idempotencyKey"].(string); !strings.HasPrefix(got, "ads:KILL:") {
		t.Errorf("idempotencyKey phải có format domain:actionType:proposedAt, got %s", got)
	}
}

func TestEnsureIdempotencyKey_WithDecisionCaseId_UsesDecisionCaseId(t *testing.T) {
	payload := map[string]interface{}{
		"decisionCaseId": "dc_abc123",
		"adAccountId":    "act_xxx",
	}
	ensureIdempotencyKey(payload, "ads", "KILL", 1700000000000)
	got := payload["idempotencyKey"].(string)
	if !strings.HasPrefix(got, "dc_abc123:KILL:") {
		t.Errorf("idempotencyKey phải có format decisionCaseId:actionType:proposedAt, got %s", got)
	}
}

func TestEnsureIdempotencyKey_WithExistingKey_KeepsExisting(t *testing.T) {
	payload := map[string]interface{}{
		"idempotencyKey": "my-custom-key-123",
	}
	ensureIdempotencyKey(payload, "ads", "KILL", 1700000000000)
	if got := payload["idempotencyKey"].(string); got != "my-custom-key-123" {
		t.Errorf("idempotencyKey có sẵn không được ghi đè, got %s", got)
	}
}

func TestValidatePayload_AdsKill_RequiresAdAccountId(t *testing.T) {
	err := validatePayload("ads", "KILL", map[string]interface{}{})
	if err == nil {
		t.Error("validatePayload thiếu adAccountId phải trả lỗi")
	}
	if err != nil && !strings.Contains(err.Error(), "adAccountId") {
		t.Errorf("lỗi phải mention adAccountId, got %v", err)
	}

	err = validatePayload("ads", "KILL", map[string]interface{}{"adAccountId": "act_123"})
	if err != nil {
		t.Errorf("validatePayload đủ adAccountId không được lỗi: %v", err)
	}
}

func TestValidatePayload_AdsIncrease_RequiresAdAccountIdAndCampaignId(t *testing.T) {
	err := validatePayload("ads", "INCREASE", map[string]interface{}{"adAccountId": "act_123"})
	if err == nil {
		t.Error("validatePayload thiếu campaignId phải trả lỗi")
	}

	err = validatePayload("ads", "INCREASE", map[string]interface{}{
		"adAccountId": "act_123",
		"campaignId":  "camp_456",
	})
	if err != nil {
		t.Errorf("validatePayload đủ field không được lỗi: %v", err)
	}
}

func TestValidatePayload_UnknownDomain_NoError(t *testing.T) {
	err := validatePayload("unknown", "SOME_ACTION", map[string]interface{}{})
	if err != nil {
		t.Errorf("domain không có schema không được lỗi: %v", err)
	}
}

