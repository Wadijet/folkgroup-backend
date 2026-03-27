// Package eventintake — Gọi Rule Intelligence (Logic Script JS) để quyết định cửa sổ trì hoãn side-effect datachanged.
// Fallback: ClassifyDatachangedBusinessUrgency + DeferWindowFor khi rule tắt / lỗi / chưa seed.
package eventintake

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	ruleintelmodels "meta_commerce/internal/api/ruleintel/models"
	ruleintelsvc "meta_commerce/internal/api/ruleintel/service"
)

const (
	// RuleDatachangedSideEffectPolicyID rule_id trong rule_definitions (domain aidecision).
	RuleDatachangedSideEffectPolicyID = "RULE_DATACHANGED_SIDE_EFFECT_POLICY"
	// RuleDomainAidecisionSideEffect domain cố định cho rule side-effect.
	RuleDomainAidecisionSideEffect = "aidecision"
)

var (
	sideEffectRuleEngineMu   sync.Mutex
	sideEffectRuleEngineInst *ruleintelsvc.RuleEngineService
)

func getRuleEngineForSideEffect() (*ruleintelsvc.RuleEngineService, error) {
	sideEffectRuleEngineMu.Lock()
	defer sideEffectRuleEngineMu.Unlock()
	if sideEffectRuleEngineInst != nil {
		return sideEffectRuleEngineInst, nil
	}
	s, err := ruleintelsvc.NewRuleEngineService()
	if err != nil {
		return nil, err
	}
	sideEffectRuleEngineInst = s
	return s, nil
}

// ResolveDatachangedDeferWindowsViaRule chạy RULE_DATACHANGED_SIDE_EFFECT_POLICY; trả ok=true nếu parse output thành công.
// SkipTrace mặc định (tránh đầy rule_execution_logs); bật AI_DECISION_SIDE_EFFECT_RULE_TRACE=1 để audit.
func ResolveDatachangedDeferWindowsViaRule(ctx context.Context, evt *aidecisionmodels.DecisionEvent, src, op string) (ingest, report, refresh time.Duration, ok bool) {
	if strings.TrimSpace(os.Getenv("AI_DECISION_SIDE_EFFECT_RULE_DISABLED")) == "1" {
		return 0, 0, 0, false
	}
	if evt == nil {
		return 0, 0, 0, false
	}
	eng, err := getRuleEngineForSideEffect()
	if err != nil {
		return 0, 0, 0, false
	}

	forceImm := false
	idHex := strings.TrimSpace(evt.EntityID)
	if evt.Payload != nil {
		forceImm = payloadBoolTrue(evt.Payload, "immediateSideEffects") ||
			payloadBoolTrue(evt.Payload, "forceImmediateSideEffects") ||
			payloadBoolTrue(evt.Payload, "urgentSideEffects")
		if u, ok := evt.Payload["normalizedRecordUid"].(string); ok && strings.TrimSpace(u) != "" {
			idHex = strings.TrimSpace(u)
		}
	}

	orgHex := evt.OrgID
	if orgHex == "" && !evt.OwnerOrganizationID.IsZero() {
		orgHex = evt.OwnerOrganizationID.Hex()
	}

	layers := map[string]interface{}{
		"datachanged": map[string]interface{}{
			"eventType":          evt.EventType,
			"sourceCollection":   src,
			"operation":          op,
			"ownerOrgIdHex":      orgHex,
			"forceImmediate":     forceImm,
			"normalizedRecordUid": strings.TrimSpace(idHex),
		},
	}

	skipTrace := strings.TrimSpace(os.Getenv("AI_DECISION_SIDE_EFFECT_RULE_TRACE")) != "1"

	res, err := eng.Run(ctx, &ruleintelsvc.RunInput{
		RuleID:    RuleDatachangedSideEffectPolicyID,
		Domain:    RuleDomainAidecisionSideEffect,
		EntityRef: ruleintelmodels.EntityRef{Domain: RuleDomainAidecisionSideEffect, ObjectType: "datachanged", ObjectID: strings.TrimSpace(idHex), OwnerOrganizationID: orgHex},
		Layers:    layers,
		SkipTrace: skipTrace,
	})
	if err != nil || res == nil {
		return 0, 0, 0, false
	}

	m, okMap := res.Result.(map[string]interface{})
	if !okMap {
		return 0, 0, 0, false
	}

	dr := nonNegativeSecFromRule(m, "deferReportSec")
	di := nonNegativeSecFromRule(m, "deferCrmIngestSec")
	df := nonNegativeSecFromRule(m, "deferCrmRefreshSec")
	if dr < 0 || di < 0 || df < 0 {
		return 0, 0, 0, false
	}

	return time.Duration(di) * time.Second, time.Duration(dr) * time.Second, time.Duration(df) * time.Second, true
}

func nonNegativeSecFromRule(m map[string]interface{}, key string) int {
	v, ok := m[key]
	if !ok || v == nil {
		return -1
	}
	var n int
	switch t := v.(type) {
	case int:
		n = t
	case int32:
		n = int(t)
	case int64:
		n = int(t)
	case float64:
		n = int(t)
	default:
		return -1
	}
	if n < 0 {
		return -1
	}
	return n
}
