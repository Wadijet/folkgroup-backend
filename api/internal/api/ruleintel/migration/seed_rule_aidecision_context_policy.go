// Package migration — Seed Rule Intelligence: Context Policy Matrix (§3.4) — script + ParamSet để version/audit/learning qua rule_execution_logs.
package migration

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	"meta_commerce/internal/api/ruleintel/models"
	"meta_commerce/internal/api/ruleintel/service"
)

// scriptContextPolicyMatrix — đọc params.policyMatrix[caseType]; output required/optional để ghi lên DecisionCase.
var scriptContextPolicyMatrix = `function evaluate(ctx) {
  var policy = ctx.layers.policy || {};
  var caseType = (policy.caseType || '').toString();
  var params = ctx.params || {};
  var matrix = params.policyMatrix || {};
  var row = matrix[caseType];
  if (!row) {
    var reportEmpty = { log: 'không có dòng matrix cho caseType=' + caseType, result: 'empty_row' };
    return { output: { required: [], optional: [], caseType: caseType }, report: reportEmpty };
  }
  var req = row.required || [];
  var opt = row.optional || [];
  var report = { log: 'caseType=' + caseType + ' required=' + req.length + ' optional=' + opt.length, result: 'ok' };
  return { output: { required: req, optional: opt, caseType: caseType }, report: report };
}`

// SeedRuleAidecisionContextPolicy seed RULE_CONTEXT_POLICY_RESOLVE + logic + param (policyMatrix) + output.
func SeedRuleAidecisionContextPolicy(ctx context.Context) error {
	systemOrgID := GetSystemOrgIDForSeed(ctx)
	if err := seedContextPolicyOutput(ctx, systemOrgID); err != nil {
		return err
	}
	if err := seedContextPolicyLogic(ctx, systemOrgID); err != nil {
		return err
	}
	if err := seedContextPolicyParam(ctx, systemOrgID); err != nil {
		return err
	}
	if err := seedContextPolicyRule(ctx, systemOrgID); err != nil {
		return err
	}
	return nil
}

func seedContextPolicyOutput(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewOutputContractService()
	if err != nil {
		return err
	}
	oc := models.OutputContract{
		OutputID:            "OUT_CONTEXT_POLICY",
		OutputVersion:       1,
		OutputType:          "context_policy",
		OwnerOrganizationID: systemOrgID,
		IsSystem:            true,
		SchemaDefinition: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"caseType": map[string]interface{}{"type": "string"},
				"required": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"optional": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
			},
		},
		RequiredFields: []string{"caseType", "required", "optional"},
	}
	_, err = svc.Upsert(ctx, bson.M{"output_id": oc.OutputID, "output_version": oc.OutputVersion}, oc)
	return err
}

func seedContextPolicyLogic(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewLogicScriptService()
	if err != nil {
		return err
	}
	s := models.LogicScript{
		LogicID:             "LOGIC_CONTEXT_POLICY_MATRIX",
		LogicVersion:        1,
		LogicType:           "script",
		Runtime:             "goja",
		EntryFunction:       "evaluate",
		Status:              "active",
		OwnerOrganizationID: systemOrgID,
		IsSystem:            true,
		Script:              scriptContextPolicyMatrix,
	}
	_, err = svc.Upsert(ctx, bson.M{"logic_id": s.LogicID, "logic_version": s.LogicVersion}, s)
	return err
}

func defaultPolicyMatrixMap() map[string]interface{} {
	return map[string]interface{}{
		aidecisionmodels.CaseTypeConversationResponse: map[string]interface{}{
			"required": []string{"cix", "customer"},
			"optional": []string{"order"},
		},
		aidecisionmodels.CaseTypeOrderRisk: map[string]interface{}{
			"required": []string{"order"},
			"optional": []string{"cix", "customer"},
		},
		aidecisionmodels.CaseTypeAdsOptimization: map[string]interface{}{
			"required": []string{"ads"},
			"optional": []string{"order", "customer"},
		},
		aidecisionmodels.CaseTypeCustomerState: map[string]interface{}{
			"required": []string{"customer", "order"},
			"optional": []string{"cix"},
		},
		aidecisionmodels.CaseTypeExecutionRecovery: map[string]interface{}{
			"required": []string{},
			"optional": []string{"cix", "customer", "order"},
		},
	}
}

func seedContextPolicyParam(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewParamSetService()
	if err != nil {
		return err
	}
	ps := models.ParamSet{
		ParamSetID:          "PARAM_CONTEXT_POLICY_MATRIX",
		ParamVersion:        1,
		OwnerOrganizationID: systemOrgID,
		IsSystem:            true,
		Domain:              "aidecision",
		Segment:             "default",
		Parameters: map[string]interface{}{
			"policyMatrix": defaultPolicyMatrixMap(),
		},
	}
	_, err = svc.Upsert(ctx, bson.M{"param_set_id": ps.ParamSetID, "param_version": ps.ParamVersion}, ps)
	return err
}

func seedContextPolicyRule(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewRuleDefinitionService()
	if err != nil {
		return err
	}
	rule := models.RuleDefinition{
		RuleID:              "RULE_CONTEXT_POLICY_RESOLVE",
		RuleVersion:         1,
		RuleCode:            "context_policy_resolve",
		Domain:              "aidecision",
		FromLayer:           "policy",
		ToLayer:             "context_requirements",
		OwnerOrganizationID: systemOrgID,
		IsSystem:            true,
		Priority:            1,
		InputRef: models.InputRef{
			SchemaRef:      "schema_context_policy",
			RequiredFields: []string{"caseType"},
		},
		LogicRef:  models.LogicRef{LogicID: "LOGIC_CONTEXT_POLICY_MATRIX", LogicVersion: 1},
		ParamRef:  models.ParamRef{ParamSetID: "PARAM_CONTEXT_POLICY_MATRIX", ParamVersion: 1},
		OutputRef: models.OutputRef{OutputID: "OUT_CONTEXT_POLICY", OutputVersion: 1},
		Status:    "active",
		Metadata: map[string]string{
			"label": "AI Decision — Context Policy Matrix (required/optional theo caseType)",
		},
	}
	_, err = svc.Upsert(ctx, bson.M{"rule_id": rule.RuleID, "domain": rule.Domain}, rule)
	return err
}
