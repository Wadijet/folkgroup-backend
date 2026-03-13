// Package migration — Seed data cho Rule Intelligence.
//
// Chạy seed RULE_ADS_KILL_CANDIDATE (Logic Script sl_a → PAUSE).
// Gọi từ worker hoặc script init khi cần.
// Dùng CRUD services Upsert.
package migration

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"

	"meta_commerce/internal/api/ruleintel/models"
	"meta_commerce/internal/api/ruleintel/service"
)

// SeedRuleAdsKillCandidate chèn Rule Definition, Logic Script, Param Set, Output Contract cho sl_a.
func SeedRuleAdsKillCandidate(ctx context.Context) error {
	systemOrgID := GetSystemOrgIDForSeed(ctx)
	// 1. Logic Script
	logicSvc, err := service.NewLogicScriptService()
	if err != nil {
		return err
	}
	// Logic Script SL-A theo FolkForm v4.1 Rule 01 — [SL-A] CPA Mess + MQS.
	// Điều kiện: Spend > 20%, Runtime > 90p, CPA_Mess > adaptive (180k), Mess < 3, MQS < 1.
	// Exception: Conv_Rate(Pancake) > 20% → KHÔNG kill. MQS >= 2 → sl_a_decrease (rule khác).
	logic := models.LogicScript{
		LogicID:             "LOGIC_ADS_KILL_SL_A",
		OwnerOrganizationID:  systemOrgID,
		IsSystem:             true,
		LogicVersion:  1,
		LogicType:     "script",
		Runtime:       "goja",
		EntryFunction: "evaluate",
		Status:        "active",
		Script: `function evaluate(ctx) {
  var layer1 = ctx.layers.layer1 || {};
  var raw = ctx.layers.raw || {};
  var params = ctx.params || {};
  var report = { log: '' };
  
  // 1. Lifecycle filter — Campaign NEW (< 7 ngày) không đề xuất (FolkForm v4.1 Section 2.2)
  if (layer1.lifecycle === 'NEW') {
    report.result = 'filtered';
    report.log = '1. Lifecycle: filtered — Campaign NEW (< 7 ngày)';
    return { output: null, report: report };
  }
  report.log = '1. Lifecycle: passed (' + (layer1.lifecycle || '') + ')';
  
  // 2. Exception: Conv_Rate(Pancake) > 20% → KHÔNG kill dù CPA_Mess cao (Master Rules v4.1)
  var convRate = layer1.convRate_7d || 0;
  var thConvException = params.th_convRateException || 0.2;
  if (convRate > thConvException) {
    report.result = 'no_match';
    report.log += '\n2. Exception: Conv_Rate ' + (convRate * 100).toFixed(1) + '% > ' + (thConvException * 100) + '% — không kill';
    return { output: null, report: report };
  }
  
  // 3. Điều kiện nền: Spend > 20%, Runtime > 90p, CHS < 3.0 (CHS handle bởi rule khác)
  var spendPct = layer1.spendPct_7d || 0;
  var runtimeMin = layer1.runtimeMinutes || 0;
  var thSpend = params.th_spendPctBase || 0.2;
  var thRuntime = params.th_runtimeMin || 90;
  if (spendPct <= thSpend || runtimeMin <= thRuntime) {
    report.result = 'no_match';
    report.log += '\n2. Điều kiện nền: spendPct=' + (spendPct * 100).toFixed(1) + '% (cần >' + (thSpend * 100) + '%), runtime=' + runtimeMin + 'p (cần >' + thRuntime + 'p)';
    return { output: null, report: report };
  }
  
  // 4. [SL-A] CPA_Mess > adaptive, Mess < 3, MQS < 1
  var cpaMess = layer1.cpaMess_7d || 0;
  var mqs = layer1.mqs_7d || 999;
  var thCpa = params.th_cpaMessKill || 180000;
  var thMessMax = params.th_messMax || 3;
  var thMqsMin = params.th_mqsMin || 1;
  var mess = 999;
  if (raw && raw.meta && raw.meta.mess != null) { mess = Number(raw.meta.mess); }
  
  if (mqs >= (params.th_mqsDecreaseMin || 2)) {
    report.result = 'no_match';
    report.log += '\n2. MQS >= 2 → sl_a_decrease (rule khác), không kill';
    return { output: null, report: report };
  }
  if (cpaMess <= thCpa || mess >= thMessMax || mqs >= thMqsMin) {
    report.result = 'no_match';
    report.log += '\n2. sl_a: cpaMess=' + cpaMess + ' (cần >' + thCpa + '), mess=' + mess + ' (<' + thMessMax + '), mqs=' + mqs + ' (<' + thMqsMin + ')';
    return { output: null, report: report };
  }
  
  report.result = 'match';
  report.log += '\n2. sl_a match\n3. Kết quả: PAUSE sl_a';
  var action = { action_code: 'PAUSE', ruleCode: 'sl_a', reason: 'Hệ thống đề xuất [SL-A]: CPA mess cao, mess thấp, MQS thấp — Stop Loss', value: null };
  return { output: action, report: report };
}`,
	}
	if _, err := logicSvc.Upsert(ctx, bson.M{"logic_id": logic.LogicID, "logic_version": logic.LogicVersion}, logic); err != nil {
		return err
	}

	// 2. Parameter Set
	paramSvc, err := service.NewParamSetService()
	if err != nil {
		return err
	}
	paramSet := models.ParamSet{
		ParamSetID:           "PARAM_ADS_KILL_SL_A",
		ParamVersion:         1,
		OwnerOrganizationID:  systemOrgID,
		IsSystem:             true,
		Parameters: map[string]interface{}{
			"th_spendPctBase":    0.20,
			"th_runtimeMin":      90,
			"th_cpaMessKill":     180000,
			"th_messMax":         3,
			"th_mqsMin":          1,
			"th_mqsDecreaseMin":  2,
			"th_convRateException": 0.20,
		},
		Domain:  "ads",
		Segment: "default",
	}
	if _, err := paramSvc.Upsert(ctx, bson.M{"param_set_id": paramSet.ParamSetID, "param_version": paramSet.ParamVersion}, paramSet); err != nil {
		return err
	}

	// 3. Output Contract
	outputSvc, err := service.NewOutputContractService()
	if err != nil {
		return err
	}
	outputContract := models.OutputContract{
		OutputID:             "OUT_ACTION_CANDIDATE",
		OutputVersion:        1,
		OutputType:           "action",
		OwnerOrganizationID:  systemOrgID,
		IsSystem:             true,
		SchemaDefinition: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action_code": map[string]interface{}{"type": "string", "enum": []string{"PAUSE", "DECREASE", "INCREASE", "RESUME", "ARCHIVE"}},
				"reason":      map[string]interface{}{"type": "string"},
			},
		},
		RequiredFields: []string{"action_code", "reason"},
	}
	if _, err := outputSvc.Upsert(ctx, bson.M{"output_id": outputContract.OutputID, "output_version": outputContract.OutputVersion}, outputContract); err != nil {
		return err
	}

	// 4. Rule Definition
	ruleSvc, err := service.NewRuleDefinitionService()
	if err != nil {
		return err
	}
	rule := models.RuleDefinition{
		RuleID:               "RULE_ADS_KILL_SL_A",
		RuleVersion:          1,
		RuleCode:             "sl_a",
		Domain:               "ads",
		FromLayer:            "flag",
		ToLayer:              "action",
		OwnerOrganizationID:  systemOrgID,
		IsSystem:             true,
		InputRef: models.InputRef{
			SchemaRef:      "schema_ads_layer1",
			RequiredFields: []string{"spendPct_7d", "runtimeMinutes", "cpaMess_7d", "convRate_7d", "mqs_7d", "lifecycle"},
		},
		LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_KILL_SL_A", LogicVersion: 1},
		ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_KILL_SL_A", ParamVersion: 1},
		OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1},
		Priority:  10,
		Status:    "active",
		Metadata:  map[string]string{"label": "SL-A", "description": "Stop Loss A — CH cao, runtime đủ"},
	}
	_, err = ruleSvc.Upsert(ctx, bson.M{"rule_id": rule.RuleID, "domain": rule.Domain}, rule)
	return err
}
