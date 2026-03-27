// Package aidecisionsvc — Context Policy Matrix qua Rule Engine (PARAM_CONTEXT_POLICY_MATRIX + RULE_CONTEXT_POLICY_RESOLVE) — version/audit/learning.
package aidecisionsvc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	"meta_commerce/internal/api/aidecision/contextpolicy"
	ruleintelmodels "meta_commerce/internal/api/ruleintel/models"
	ruleintelsvc "meta_commerce/internal/api/ruleintel/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	ruleIDContextPolicyResolve = "RULE_CONTEXT_POLICY_RESOLVE"
)

var (
	contextPolicyRuleEngineOnce sync.Once
	contextPolicyRuleEngine     *ruleintelsvc.RuleEngineService
	contextPolicyRuleEngineErr  error

	// Cache kết quả merge (rule + override org) — key gồm org, caseType, fingerprint param/logic, revision override.
	contextPolicyResolveCache sync.Map // string -> ctxPolicyCacheEntry
)

type ctxPolicyCacheEntry struct {
	required  []string
	expiresMs int64
}

func contextPolicyCacheTTLMillis() int64 {
	sec := int64(300)
	if v := os.Getenv("AI_DECISION_CONTEXT_POLICY_CACHE_TTL_SEC"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
			sec = n
		}
	}
	return sec * 1000
}

func contextPolicyCacheEnabled() bool {
	return os.Getenv("AI_DECISION_CONTEXT_POLICY_CACHE") != "0"
}

// invalidateContextPolicyCache xóa toàn bộ cache (gọi sau CRUD override org).
func invalidateContextPolicyCache() {
	contextPolicyResolveCache.Range(func(k, _ interface{}) bool {
		contextPolicyResolveCache.Delete(k)
		return true
	})
}

// fingerprintContextPolicyRule đọc ParamRef/LogicRef từ rule_definitions (nhẹ hơn chạy script) để làm khóa cache.
func fingerprintContextPolicyRule(ctx context.Context) (string, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.RuleDefinitions)
	if !ok {
		return "", errors.New("không tìm thấy collection rule_definitions")
	}
	var rd ruleintelmodels.RuleDefinition
	err := coll.FindOne(ctx, bson.M{"rule_id": ruleIDContextPolicyResolve, "domain": domainAidecisionRule, "status": "active"}).Decode(&rd)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s|%d|%d", rd.ParamRef.ParamSetID, rd.ParamRef.ParamVersion, rd.LogicRef.LogicVersion), nil
}

func getContextPolicyRuleEngine() (*ruleintelsvc.RuleEngineService, error) {
	contextPolicyRuleEngineOnce.Do(func() {
		contextPolicyRuleEngine, contextPolicyRuleEngineErr = ruleintelsvc.NewRuleEngineService()
	})
	return contextPolicyRuleEngine, contextPolicyRuleEngineErr
}

// RequiredContextsForCaseTypeFromRule gọi Rule Engine (script + ParamSet version); lỗi/seed thiếu → fallback contextpolicy.DefaultRequiredContextsForCaseType.
// Có cache in-memory theo (org, caseType, param/logic fingerprint, revision override) — tắt bằng AI_DECISION_CONTEXT_POLICY_CACHE=0.
// Ghi trace rule_execution_logs khi thực sự Run (cache miss).
func (s *AIDecisionService) RequiredContextsForCaseTypeFromRule(ctx context.Context, ownerOrgID primitive.ObjectID, caseType string) []string {
	if caseType == "" {
		return nil
	}

	var ovRev int64
	ovDoc, errOv := loadContextPolicyOverrideDoc(ctx, ownerOrgID, caseType)
	if errOv != nil {
		logrus.WithError(errOv).Debug("Đọc context policy override (bỏ qua nếu không có collection)")
	}
	if ovDoc != nil {
		ovRev = ovDoc.UpdatedAt
	}

	if contextPolicyCacheEnabled() && contextPolicyCacheTTLMillis() > 0 {
		fp, errFp := fingerprintContextPolicyRule(ctx)
		if errFp == nil && fp != "" {
			key := fmt.Sprintf("%s|%s|%s|%d", ownerOrgID.Hex(), caseType, fp, ovRev)
			if v, ok := contextPolicyResolveCache.Load(key); ok {
				ent := v.(ctxPolicyCacheEntry)
				if time.Now().UnixMilli() < ent.expiresMs {
					return append([]string(nil), ent.required...)
				}
				contextPolicyResolveCache.Delete(key)
			}
		}
	}

	re, err := getContextPolicyRuleEngine()
	if err != nil {
		logrus.WithError(err).Warn("Rule Engine (context policy): khởi tạo thất bại, dùng matrix mặc định trong code")
		return mergeOrgOverrideRequired(caseType, ovDoc, contextpolicy.DefaultRequiredContextsForCaseType(caseType))
	}
	runRes, err := re.Run(ctx, &ruleintelsvc.RunInput{
		RuleID: ruleIDContextPolicyResolve,
		Domain: domainAidecisionRule,
		EntityRef: ruleintelmodels.EntityRef{
			Domain:              domainAidecisionRule,
			ObjectType:          "context_policy",
			ObjectID:            caseType,
			OwnerOrganizationID: ownerOrgID.Hex(),
		},
		Layers: map[string]interface{}{
			"policy": map[string]interface{}{"caseType": caseType},
		},
	})
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			logrus.Warn("RULE_CONTEXT_POLICY_RESOLVE chưa seed — dùng matrix mặc định trong code")
		} else {
			logrus.WithError(err).Warn("RULE_CONTEXT_POLICY_RESOLVE Run thất bại — dùng matrix mặc định")
		}
		out := mergeOrgOverrideRequired(caseType, ovDoc, contextpolicy.DefaultRequiredContextsForCaseType(caseType))
		storeContextPolicyCache(ctx, ownerOrgID, caseType, ovRev, out)
		return out
	}
	if runRes == nil || runRes.Result == nil {
		out := mergeOrgOverrideRequired(caseType, ovDoc, contextpolicy.DefaultRequiredContextsForCaseType(caseType))
		storeContextPolicyCache(ctx, ownerOrgID, caseType, ovRev, out)
		return out
	}
	req := extractStringSliceFromRuleOutput(runRes.Result, "required")
	if m := asMap(runRes.Result); m != nil {
		if _, ok := m["caseType"]; ok {
			var out []string
			if ovDoc != nil && ovDoc.Enabled && len(ovDoc.RequiredContexts) > 0 {
				out = append([]string(nil), ovDoc.RequiredContexts...)
			} else {
				out = append([]string(nil), req...)
			}
			storeContextPolicyCache(ctx, ownerOrgID, caseType, ovRev, out)
			return out
		}
	}
	out := mergeOrgOverrideRequired(caseType, ovDoc, contextpolicy.DefaultRequiredContextsForCaseType(caseType))
	storeContextPolicyCache(ctx, ownerOrgID, caseType, ovRev, out)
	return out
}

func mergeOrgOverrideRequired(caseType string, ov *aidecisionmodels.DecisionContextPolicyOverride, fromRule []string) []string {
	if ov != nil && ov.Enabled && len(ov.RequiredContexts) > 0 {
		return append([]string(nil), ov.RequiredContexts...)
	}
	if len(fromRule) > 0 {
		return append([]string(nil), fromRule...)
	}
	return contextpolicy.DefaultRequiredContextsForCaseType(caseType)
}

func storeContextPolicyCache(ctx context.Context, ownerOrgID primitive.ObjectID, caseType string, ovRev int64, required []string) {
	if !contextPolicyCacheEnabled() || contextPolicyCacheTTLMillis() <= 0 {
		return
	}
	fp, err := fingerprintContextPolicyRule(ctx)
	if err != nil || fp == "" {
		return
	}
	key := fmt.Sprintf("%s|%s|%s|%d", ownerOrgID.Hex(), caseType, fp, ovRev)
	ent := ctxPolicyCacheEntry{
		required:  append([]string(nil), required...),
		expiresMs: time.Now().UnixMilli() + contextPolicyCacheTTLMillis(),
	}
	contextPolicyResolveCache.Store(key, ent)
}

func asMap(v interface{}) map[string]interface{} {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	return m
}

func extractStringSliceFromRuleOutput(v interface{}, key string) []string {
	m := asMap(v)
	if m == nil {
		return nil
	}
	raw, ok := m[key]
	if !ok || raw == nil {
		return nil
	}
	switch x := raw.(type) {
	case []string:
		return append([]string(nil), x...)
	case []interface{}:
		out := make([]string, 0, len(x))
		for _, e := range x {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
