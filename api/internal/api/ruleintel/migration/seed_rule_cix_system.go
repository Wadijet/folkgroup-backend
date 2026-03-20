// Package migration — Seed Rule Intelligence cho CIX domain (Contextual Conversation Intelligence).
// RULE_CIX_LAYER1_STAGE, LAYER2_STATE, LAYER2_ADJUST, LAYER3_SIGNALS, FLAGS, ACTIONS.
package migration

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"meta_commerce/internal/api/ruleintel/models"
	"meta_commerce/internal/api/ruleintel/service"
)

// scriptCixLayer1Stage — Logic: raw_conversation, customer_context → stage.
var scriptCixLayer1Stage = `function evaluate(ctx) {
  var raw = ctx.layers.cix_raw || {};
  var customer = ctx.layers.cix_customer_context || {};
  var report = { input: { raw, customer }, log: '' };
  var turnCount = (raw.turns || []).length;
  if (turnCount === 0) return { output: { stage: 'new' }, report: report };
  if (turnCount < 3) return { output: { stage: 'engaged' }, report: report };
  return { output: { stage: 'consulting' }, report: report };
}`

// scriptCixLayer2State — Logic: L1, raw, customer_context → intentStage, urgencyLevel, riskLevelRaw.
var scriptCixLayer2State = `function evaluate(ctx) {
  var L1 = ctx.layers.cix_layer1 || {};
  var raw = ctx.layers.cix_raw || {};
  var report = { input: { L1, raw }, log: '' };
  var stage = L1.stage || 'new';
  var risk = 'safe';
  if (stage === 'stalled') risk = 'warning';
  return {
    output: {
      intentStage: 'medium',
      urgencyLevel: 'normal',
      riskLevelRaw: risk
    },
    report: report
  };
}`

// scriptCixLayer2Adjust — Logic: L2.riskLevelRaw, customer_context → riskLevelAdj, adjustmentReason (VIP).
var scriptCixLayer2Adjust = `function evaluate(ctx) {
  var L2 = ctx.layers.cix_layer2 || {};
  var customer = ctx.layers.cix_customer_context || {};
  var report = { input: { L2, customer }, log: '' };
  var raw = L2.riskLevelRaw || 'safe';
  var valueTier = customer.valueTier || '';
  if (raw === 'warning' && (valueTier === 'top' || valueTier === 'high')) {
    report.log = 'VIP + warning → danger';
    return {
      output: {
        riskLevelAdj: 'danger',
        adjustmentReason: 'vip_customer_complaint',
        ruleId: 'ADJUST_RISK_VIP_v1'
      },
      report: report
    };
  }
  return {
    output: {
      riskLevelAdj: raw,
      adjustmentReason: '',
      ruleId: ''
    },
    report: report
  };
}`

// scriptCixLayer3Signals — Logic: raw, L1, L2 → buyingIntent, objectionLevel, sentiment.
var scriptCixLayer3Signals = `function evaluate(ctx) {
  var raw = ctx.layers.cix_raw || {};
  var L1 = ctx.layers.cix_layer1 || {};
  var L2 = ctx.layers.cix_layer2 || {};
  var report = { input: { raw, L1, L2 }, log: '' };
  return {
    output: {
      buyingIntent: 'inquiring',
      objectionLevel: 'none',
      sentiment: 'neutral'
    },
    report: report
  };
}`

// scriptCixFlags — Logic: L2.adj, L3, customer_context → flags[].
var scriptCixFlags = `function evaluate(ctx) {
  var L2 = ctx.layers.cix_layer2_adj || ctx.layers.cix_layer2 || {};
  var L3 = ctx.layers.cix_layer3 || {};
  var customer = ctx.layers.cix_customer_context || {};
  var report = { input: { L2, L3, customer }, log: '' };
  var flags = [];
  var risk = L2.riskLevelAdj || L2.riskLevelRaw || 'safe';
  var valueTier = customer.valueTier || '';
  if (risk === 'danger' && (valueTier === 'top' || valueTier === 'high')) {
    flags.push({ name: 'vip_at_risk', severity: 'critical', triggeredByRule: 'FLAG_VIP_RISK_v1' });
  }
  return { output: { flags: flags }, report: report };
}`

// scriptCixActions — Logic: flags, L2, L3 → actionSuggestions[].
var scriptCixActions = `function evaluate(ctx) {
  var flags = ctx.layers.cix_flags || {};
  var flagsArr = flags.flags || [];
  var L2 = ctx.layers.cix_layer2_adj || ctx.layers.cix_layer2 || {};
  var report = { input: { flags: flagsArr, L2 }, log: '' };
  var actions = [];
  if (flagsArr.some(function(f){ return f.name === 'vip_at_risk'; })) {
    actions.push('escalate_to_senior');
  }
  if (actions.length === 0) actions.push('none');
  return { output: { actionSuggestions: actions }, report: report };
}`

// SeedRuleCixSystem seed toàn bộ rules CIX.
func SeedRuleCixSystem(ctx context.Context) error {
	systemOrgID := GetSystemOrgIDForSeed(ctx)
	if systemOrgID.IsZero() {
		return nil
	}
	if err := seedCixOutputContracts(ctx, systemOrgID); err != nil {
		return err
	}
	if err := seedCixLogicScripts(ctx, systemOrgID); err != nil {
		return err
	}
	if err := seedCixParamSets(ctx, systemOrgID); err != nil {
		return err
	}
	if err := seedCixRuleDefinitions(ctx, systemOrgID); err != nil {
		return err
	}
	return nil
}

func seedCixOutputContracts(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewOutputContractService()
	if err != nil {
		return err
	}
	contracts := []models.OutputContract{
		{OutputID: "OUT_CIX_LAYER1", OutputVersion: 1, OutputType: "cix_layer1", OwnerOrganizationID: systemOrgID, IsSystem: true,
			SchemaDefinition: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"stage": map[string]interface{}{"type": "string"}}, "required": []string{"stage"}},
			RequiredFields: []string{"stage"}},
		{OutputID: "OUT_CIX_LAYER2", OutputVersion: 1, OutputType: "cix_layer2", OwnerOrganizationID: systemOrgID, IsSystem: true,
			SchemaDefinition: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"intentStage": map[string]interface{}{"type": "string"}, "urgencyLevel": map[string]interface{}{"type": "string"}, "riskLevelRaw": map[string]interface{}{"type": "string"}}, "required": []string{"intentStage", "urgencyLevel", "riskLevelRaw"}},
			RequiredFields: []string{"intentStage", "urgencyLevel", "riskLevelRaw"}},
		{OutputID: "OUT_CIX_LAYER2_ADJ", OutputVersion: 1, OutputType: "cix_layer2_adj", OwnerOrganizationID: systemOrgID, IsSystem: true,
			SchemaDefinition: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"riskLevelAdj": map[string]interface{}{"type": "string"}, "adjustmentReason": map[string]interface{}{"type": "string"}, "ruleId": map[string]interface{}{"type": "string"}}, "required": []string{"riskLevelAdj"}},
			RequiredFields: []string{"riskLevelAdj"}},
		{OutputID: "OUT_CIX_LAYER3", OutputVersion: 1, OutputType: "cix_layer3", OwnerOrganizationID: systemOrgID, IsSystem: true,
			SchemaDefinition: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"buyingIntent": map[string]interface{}{"type": "string"}, "objectionLevel": map[string]interface{}{"type": "string"}, "sentiment": map[string]interface{}{"type": "string"}}, "required": []string{"buyingIntent", "objectionLevel", "sentiment"}},
			RequiredFields: []string{"buyingIntent", "objectionLevel", "sentiment"}},
		{OutputID: "OUT_CIX_FLAGS", OutputVersion: 1, OutputType: "cix_flags", OwnerOrganizationID: systemOrgID, IsSystem: true,
			SchemaDefinition: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"flags": map[string]interface{}{"type": "array"}}, "required": []string{"flags"}},
			RequiredFields: []string{"flags"}},
		{OutputID: "OUT_CIX_ACTIONS", OutputVersion: 1, OutputType: "cix_actions", OwnerOrganizationID: systemOrgID, IsSystem: true,
			SchemaDefinition: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"actionSuggestions": map[string]interface{}{"type": "array"}}, "required": []string{"actionSuggestions"}},
			RequiredFields: []string{"actionSuggestions"}},
	}
	for _, oc := range contracts {
		_, err = svc.Upsert(ctx, bson.M{"output_id": oc.OutputID, "output_version": oc.OutputVersion}, oc)
		if err != nil {
			return err
		}
	}
	return nil
}

func seedCixLogicScripts(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewLogicScriptService()
	if err != nil {
		return err
	}
	scripts := []struct {
		LogicID string
		Script  string
	}{
		{"LOGIC_CIX_LAYER1_STAGE", scriptCixLayer1Stage},
		{"LOGIC_CIX_LAYER2_STATE", scriptCixLayer2State},
		{"LOGIC_CIX_LAYER2_ADJUST", scriptCixLayer2Adjust},
		{"LOGIC_CIX_LAYER3_SIGNALS", scriptCixLayer3Signals},
		{"LOGIC_CIX_FLAGS", scriptCixFlags},
		{"LOGIC_CIX_ACTIONS", scriptCixActions},
	}
	for _, s := range scripts {
		_, err = svc.Upsert(ctx, bson.M{"logic_id": s.LogicID, "logic_version": 1}, models.LogicScript{
			LogicID:             s.LogicID,
			LogicVersion:        1,
			LogicType:           "script",
			Runtime:             "goja",
			EntryFunction:       "evaluate",
			Status:              "active",
			OwnerOrganizationID: systemOrgID,
			IsSystem:            true,
			Script:              s.Script,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func seedCixParamSets(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewParamSetService()
	if err != nil {
		return err
	}
	_, err = svc.Upsert(ctx, bson.M{"param_set_id": "PARAM_CIX_DEFAULT", "param_version": 1}, models.ParamSet{
		ParamSetID:           "PARAM_CIX_DEFAULT",
		ParamVersion:         1,
		OwnerOrganizationID:  systemOrgID,
		IsSystem:             true,
		Domain:               "cix",
		Segment:              "default",
		Parameters:           map[string]interface{}{},
	})
	return err
}

func seedCixRuleDefinitions(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewRuleDefinitionService()
	if err != nil {
		return err
	}
	rules := []models.RuleDefinition{
		{RuleID: "RULE_CIX_LAYER1_STAGE", RuleVersion: 1, RuleCode: "cix_layer1_stage", Domain: "cix", FromLayer: "cix_raw", ToLayer: "cix_layer1", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 1,
			InputRef: models.InputRef{SchemaRef: "schema_cix_raw", RequiredFields: []string{"turns"}}, LogicRef: models.LogicRef{LogicID: "LOGIC_CIX_LAYER1_STAGE", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_CIX_DEFAULT", ParamVersion: 1}, OutputRef: models.OutputRef{OutputID: "OUT_CIX_LAYER1", OutputVersion: 1}, Status: "active"},
		{RuleID: "RULE_CIX_LAYER2_STATE", RuleVersion: 1, RuleCode: "cix_layer2_state", Domain: "cix", FromLayer: "cix_layer1", ToLayer: "cix_layer2", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 2,
			InputRef: models.InputRef{SchemaRef: "schema_cix_layer1", RequiredFields: []string{"stage"}}, LogicRef: models.LogicRef{LogicID: "LOGIC_CIX_LAYER2_STATE", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_CIX_DEFAULT", ParamVersion: 1}, OutputRef: models.OutputRef{OutputID: "OUT_CIX_LAYER2", OutputVersion: 1}, Status: "active"},
		{RuleID: "RULE_CIX_LAYER2_ADJUST", RuleVersion: 1, RuleCode: "cix_layer2_adjust", Domain: "cix", FromLayer: "cix_layer2", ToLayer: "cix_layer2_adj", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 3,
			InputRef: models.InputRef{SchemaRef: "schema_cix_layer2", RequiredFields: []string{"riskLevelRaw"}}, LogicRef: models.LogicRef{LogicID: "LOGIC_CIX_LAYER2_ADJUST", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_CIX_DEFAULT", ParamVersion: 1}, OutputRef: models.OutputRef{OutputID: "OUT_CIX_LAYER2_ADJ", OutputVersion: 1}, Status: "active"},
		{RuleID: "RULE_CIX_LAYER3_SIGNALS", RuleVersion: 1, RuleCode: "cix_layer3_signals", Domain: "cix", FromLayer: "cix_layer2", ToLayer: "cix_layer3", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 4,
			InputRef: models.InputRef{SchemaRef: "schema_cix_raw", RequiredFields: []string{}}, LogicRef: models.LogicRef{LogicID: "LOGIC_CIX_LAYER3_SIGNALS", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_CIX_DEFAULT", ParamVersion: 1}, OutputRef: models.OutputRef{OutputID: "OUT_CIX_LAYER3", OutputVersion: 1}, Status: "active"},
		{RuleID: "RULE_CIX_FLAGS", RuleVersion: 1, RuleCode: "cix_flags", Domain: "cix", FromLayer: "cix_layer2_adj", ToLayer: "cix_flags", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 5,
			InputRef: models.InputRef{SchemaRef: "schema_cix_layer2_adj", RequiredFields: []string{}}, LogicRef: models.LogicRef{LogicID: "LOGIC_CIX_FLAGS", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_CIX_DEFAULT", ParamVersion: 1}, OutputRef: models.OutputRef{OutputID: "OUT_CIX_FLAGS", OutputVersion: 1}, Status: "active"},
		{RuleID: "RULE_CIX_ACTIONS", RuleVersion: 1, RuleCode: "cix_actions", Domain: "cix", FromLayer: "cix_flags", ToLayer: "cix_actions", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 6,
			InputRef: models.InputRef{SchemaRef: "schema_cix_flags", RequiredFields: []string{}}, LogicRef: models.LogicRef{LogicID: "LOGIC_CIX_ACTIONS", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_CIX_DEFAULT", ParamVersion: 1}, OutputRef: models.OutputRef{OutputID: "OUT_CIX_ACTIONS", OutputVersion: 1}, Status: "active"},
	}
	for _, r := range rules {
		_, err = svc.Upsert(ctx, bson.M{"rule_id": r.RuleID, "domain": r.Domain}, r)
		if err != nil {
			return err
		}
	}
	return nil
}
