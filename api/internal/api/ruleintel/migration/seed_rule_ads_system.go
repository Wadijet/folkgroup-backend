// Package migration — Seed toàn bộ Rule Intelligence cho Ads domain (FolkForm v4.1).
// Tất cả rules thuộc System Organization (OwnerOrganizationID + IsSystem=true).
// Dùng CRUD services Upsert.
package migration

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"meta_commerce/internal/api/ruleintel/models"
	"meta_commerce/internal/api/ruleintel/service"
)

// scriptSlA — Logic SL-A theo FolkForm v4.1 Rule 01 (full logic từ layer1).
var scriptSlA = `function evaluate(ctx) {
  var layer1 = ctx.layers.layer1 || {};
  var raw = ctx.layers.raw || {};
  var params = ctx.params || {};
  var report = { log: '' };
  if (layer1.lifecycle === 'NEW') {
    report.result = 'filtered';
    report.log = '1. Lifecycle: filtered — Campaign NEW (< 7 ngày)';
    return { output: null, report: report };
  }
  report.log = '1. Lifecycle: passed (' + (layer1.lifecycle || '') + ')';
  var convRate = layer1.convRate_7d || 0;
  var thConvException = params.th_convRateException || 0.2;
  if (convRate > thConvException) {
    report.result = 'no_match';
    report.log += '\n2. Exception: Conv_Rate ' + (convRate * 100).toFixed(1) + '% > ' + (thConvException * 100) + '% — không kill';
    return { output: null, report: report };
  }
  var spendPct = layer1.spendPct_7d || 0;
  var runtimeMin = layer1.runtimeMinutes || 0;
  var thSpend = params.th_spendPctBase || 0.2;
  var thRuntime = params.th_runtimeMin || 90;
  if (spendPct <= thSpend || runtimeMin <= thRuntime) {
    report.result = 'no_match';
    report.log += '\n2. Điều kiện nền: spendPct=' + (spendPct * 100).toFixed(1) + '% (cần >' + (thSpend * 100) + '%), runtime=' + runtimeMin + 'p (cần >' + thRuntime + 'p)';
    return { output: null, report: report };
  }
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
}`

// scriptFlagBased — Logic chung cho rules flag → action. Nhận layers.flag, params (triggerFlag/requireFlags, action, ruleCode, reason, value, freeze, exceptionFlags, killRulesEnabled).
var scriptFlagBased = `function evaluate(ctx) {
  var flags = ctx.layers.flag || {};
  var params = ctx.params || {};
  var report = { log: '' };
  var matched = false;
  if (params.triggerFlag) {
    matched = !!flags[params.triggerFlag];
    if (!matched) { report.result = 'no_match'; report.log = 'Flag ' + params.triggerFlag + ' không có'; return { output: null, report: report }; }
  } else if (params.requireFlags && params.requireFlags.length > 0) {
    for (var i = 0; i < params.requireFlags.length; i++) {
      if (!flags[params.requireFlags[i]]) { report.result = 'no_match'; report.log = 'Require flags không đủ'; return { output: null, report: report }; }
    }
    matched = true;
  } else {
    report.result = 'no_match'; return { output: null, report: report };
  }
  var ex = params.exceptionFlags || [];
  for (var j = 0; j < ex.length; j++) {
    if (flags[ex[j]]) { report.result = 'no_match'; report.log = 'Exception: ' + ex[j]; return { output: null, report: report }; }
  }
  if (params.killRulesEnabled === false && params.freeze === true) {
    report.result = 'no_match'; report.log = 'KillRulesEnabled=false, rule freeze'; return { output: null, report: report };
  }
  if (params.skipMessTrapWindowShopping && params.windowShoppingPattern && params.isBefore1400) {
    var safetyKill = (params.msgRateRatio > 0 && params.msgRateRatio < 0.01) || (params.cpmVnd > 0 && params.cpmVnd < 40000);
    if (!safetyKill) { report.result = 'no_match'; report.log = 'PATCH 04: window shopping, suspend'; return { output: null, report: report }; }
  }
  report.result = 'match';
  var val = params.value !== undefined ? params.value : null;
  var action = { action_code: params.action, ruleCode: params.ruleCode, reason: params.reason, value: val };
  return { output: action, report: report };
}`

// SeedRuleAdsSystem seed toàn bộ rules Ads (Kill, Decrease, Increase) — OwnerOrganizationID + IsSystem.
func SeedRuleAdsSystem(ctx context.Context) error {
	systemOrgID := GetSystemOrgIDForSeed(ctx)
	if err := seedOutputContract(ctx, systemOrgID); err != nil {
		return err
	}
	if err := seedLogicScripts(ctx, systemOrgID); err != nil {
		return err
	}
	if err := seedParamSets(ctx, systemOrgID); err != nil {
		return err
	}
	if err := seedRuleDefinitions(ctx, systemOrgID); err != nil {
		return err
	}
	return nil
}

func seedOutputContract(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewOutputContractService()
	if err != nil {
		return err
	}
	oc := models.OutputContract{
		OutputID:            "OUT_ACTION_CANDIDATE",
		OutputVersion:       1,
		OutputType:          "action",
		OwnerOrganizationID: systemOrgID,
		IsSystem:            true,
		SchemaDefinition: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action_code": map[string]interface{}{"type": "string", "enum": []string{"PAUSE", "DECREASE", "INCREASE", "RESUME", "ARCHIVE"}},
				"reason":      map[string]interface{}{"type": "string"},
			},
		},
		RequiredFields: []string{"action_code", "reason"},
	}
	_, err = svc.Upsert(ctx, bson.M{"output_id": oc.OutputID, "output_version": oc.OutputVersion}, oc)
	return err
}

func seedLogicScripts(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewLogicScriptService()
	if err != nil {
		return err
	}
	scripts := []models.LogicScript{
		// sl_a — full logic (đã có trong seed_rule_ads_kill.go, gọi từ đây)
		{LogicID: "LOGIC_ADS_KILL_SL_A", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptSlA},
		// Flag-based kill rules
		{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1, LogicType: "script", Runtime: "goja", EntryFunction: "evaluate", Status: "active", OwnerOrganizationID: systemOrgID, IsSystem: true, Script: scriptFlagBased},
	}
	for _, s := range scripts {
		if _, err := svc.Upsert(ctx, bson.M{"logic_id": s.LogicID, "logic_version": s.LogicVersion}, s); err != nil {
			return err
		}
	}
	return nil
}

func seedParamSets(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewParamSetService()
	if err != nil {
		return err
	}
	sets := []models.ParamSet{
		{ParamSetID: "PARAM_ADS_KILL_SL_A", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"th_spendPctBase": 0.20, "th_runtimeMin": 90, "th_cpaMessKill": 180000, "th_messMax": 3, "th_mqsMin": 1, "th_mqsDecreaseMin": 2, "th_convRateException": 0.20,
		}},
		{ParamSetID: "PARAM_ADS_KILL_SL_B", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "sl_b", "action": "PAUSE", "ruleCode": "sl_b", "reason": "Hệ thống đề xuất [SL-B]: Có spend nhưng 0 mess — Blitz/Protect", "freeze": false,
		}},
		{ParamSetID: "PARAM_ADS_KILL_SL_C", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "sl_c", "action": "PAUSE", "ruleCode": "sl_c", "reason": "Hệ thống đề xuất [SL-C]: CTR thảm họa, CPM tăng bất thường", "freeze": false,
		}},
		{ParamSetID: "PARAM_ADS_KILL_SL_D", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "sl_d", "action": "PAUSE", "ruleCode": "sl_d", "reason": "Hệ thống đề xuất [SL-D]: Mess Trap — mess đủ mẫu nhưng CR thấp", "freeze": true,
		}},
		{ParamSetID: "PARAM_ADS_KILL_SL_E", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "sl_e", "action": "PAUSE", "ruleCode": "sl_e", "reason": "Hệ thống đề xuất [SL-E]: CPA Purchase vượt ngưỡng, CR thấp", "freeze": true,
		}},
		{ParamSetID: "PARAM_ADS_KILL_CHS", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "chs_critical", "action": "PAUSE", "ruleCode": "chs_critical", "reason": "Hệ thống đề xuất [CHS]: Camp Health Score critical 2 checkpoint liên tiếp", "freeze": true,
		}},
		{ParamSetID: "PARAM_ADS_KILL_KO_A", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "ko_a", "action": "PAUSE", "ruleCode": "ko_a", "reason": "Hệ thống đề xuất [KO-A]: Không delivery — LIMITED/NOT_DELIVERING", "freeze": false,
		}},
		{ParamSetID: "PARAM_ADS_KILL_KO_B", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "ko_b", "action": "PAUSE", "ruleCode": "ko_b", "reason": "Hệ thống đề xuất [KO-B]: Traffic rác — CTR cao, msg rate thấp, 0 đơn", "freeze": true,
		}},
		{ParamSetID: "PARAM_ADS_KILL_KO_C", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "ko_c", "action": "PAUSE", "ruleCode": "ko_c", "reason": "Hệ thống đề xuất [KO-C]: CPM bất thường, impressions thấp", "freeze": false,
		}},
		{ParamSetID: "PARAM_ADS_KILL_TRIM", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "trim_eligible", "action": "PAUSE", "ruleCode": "trim_eligible", "reason": "Hệ thống đề xuất [Trim]: Frequency cao, CHS trung bình — Kill", "freeze": false,
		}},
		// Decrease
		{ParamSetID: "PARAM_ADS_DECREASE_SL_A", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "sl_a_decrease", "action": "DECREASE", "ruleCode": "sl_a_decrease", "reason": "Hệ thống đề xuất [SL-A]: CPA mess cao nhưng MQS >= 2 — giảm budget 20% thay vì kill", "value": 20,
		}},
		{ParamSetID: "PARAM_ADS_DECREASE_MESS_TRAP", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "mess_trap_suspect", "action": "DECREASE", "ruleCode": "mess_trap_suspect", "reason": "Hệ thống đề xuất [Mess Trap]: Nghi ngờ bẫy mess — giảm budget 30%", "value": 30,
		}},
		{ParamSetID: "PARAM_ADS_DECREASE_TRIM", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "trim_eligible_decrease", "action": "DECREASE", "ruleCode": "trim_eligible_decrease", "reason": "Hệ thống đề xuất [Trim]: Frequency cao, có đơn — giảm budget 30% thay vì kill", "value": 30,
		}},
		{ParamSetID: "PARAM_ADS_DECREASE_CHS", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"requireFlags": []interface{}{"chs_warning", "cpa_mess_high"}, "action": "DECREASE", "ruleCode": "chs_warning", "reason": "Hệ thống đề xuất [CHS Warning]: CPA mess cao, CHS warning — giảm budget 15%", "value": 15,
		}},
		// Increase
		{ParamSetID: "PARAM_ADS_INCREASE_ELIGIBLE", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "increase_eligible", "action": "INCREASE", "ruleCode": "increase_eligible", "reason": "Hệ thống đề xuất [Increase]: Camp tốt — CR > 12%, CHS < 1.3, tăng budget 30%", "value": 30,
		}},
		{ParamSetID: "PARAM_ADS_INCREASE_SAFETY", ParamVersion: 1, OwnerOrganizationID: systemOrgID, IsSystem: true, Domain: "ads", Segment: "default", Parameters: map[string]interface{}{
			"triggerFlag": "safety_net", "action": "INCREASE", "ruleCode": "increase_safety_net", "reason": "Hệ thống đề xuất [Increase]: Safety Net — camp tốt, tăng 35%", "value": 35,
		}},
	}
	for _, ps := range sets {
		if _, err := svc.Upsert(ctx, bson.M{"param_set_id": ps.ParamSetID, "param_version": ps.ParamVersion}, ps); err != nil {
			return err
		}
	}
	return nil
}

func seedRuleDefinitions(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewRuleDefinitionService()
	if err != nil {
		return err
	}
	rules := []models.RuleDefinition{
		{RuleID: "RULE_ADS_KILL_SL_A", RuleVersion: 1, RuleCode: "sl_a", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 1,
			InputRef: models.InputRef{SchemaRef: "schema_ads_layer1", RequiredFields: []string{"spendPct_7d", "runtimeMinutes", "cpaMess_7d", "convRate_7d", "mqs_7d", "lifecycle"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_KILL_SL_A", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_KILL_SL_A", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "SL-A"}},
		{RuleID: "RULE_ADS_KILL_SL_B", RuleVersion: 1, RuleCode: "sl_b", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 2,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"sl_b"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_KILL_SL_B", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "SL-B"}},
		{RuleID: "RULE_ADS_KILL_SL_C", RuleVersion: 1, RuleCode: "sl_c", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 3,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"sl_c"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_KILL_SL_C", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "SL-C"}},
		{RuleID: "RULE_ADS_KILL_SL_D", RuleVersion: 1, RuleCode: "sl_d", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 4,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"sl_d"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_KILL_SL_D", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "SL-D"}},
		{RuleID: "RULE_ADS_KILL_SL_E", RuleVersion: 1, RuleCode: "sl_e", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 5,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"sl_e"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_KILL_SL_E", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "SL-E"}},
		{RuleID: "RULE_ADS_KILL_CHS", RuleVersion: 1, RuleCode: "chs_critical", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 6,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"chs_critical"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_KILL_CHS", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "CHS Critical"}},
		{RuleID: "RULE_ADS_KILL_KO_A", RuleVersion: 1, RuleCode: "ko_a", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 7,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"ko_a"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_KILL_KO_A", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "KO-A"}},
		{RuleID: "RULE_ADS_KILL_KO_B", RuleVersion: 1, RuleCode: "ko_b", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 8,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"ko_b"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_KILL_KO_B", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "KO-B"}},
		{RuleID: "RULE_ADS_KILL_KO_C", RuleVersion: 1, RuleCode: "ko_c", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 9,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"ko_c"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_KILL_KO_C", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "KO-C"}},
		{RuleID: "RULE_ADS_KILL_TRIM", RuleVersion: 1, RuleCode: "trim_eligible", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 10,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"trim_eligible"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_KILL_TRIM", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Trim"}},
		// Decrease
		{RuleID: "RULE_ADS_DECREASE_SL_A", RuleVersion: 1, RuleCode: "sl_a_decrease", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 11,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"sl_a_decrease"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_DECREASE_SL_A", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "SL-A Decrease"}},
		{RuleID: "RULE_ADS_DECREASE_MESS_TRAP", RuleVersion: 1, RuleCode: "mess_trap_suspect", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 12,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"mess_trap_suspect"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_DECREASE_MESS_TRAP", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Mess Trap Suspect"}},
		{RuleID: "RULE_ADS_DECREASE_TRIM", RuleVersion: 1, RuleCode: "trim_eligible_decrease", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 13,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"trim_eligible_decrease"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_DECREASE_TRIM", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Trim Decrease"}},
		{RuleID: "RULE_ADS_DECREASE_CHS", RuleVersion: 1, RuleCode: "chs_warning", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 14,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"chs_warning", "cpa_mess_high"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_DECREASE_CHS", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "CHS Warning"}},
		// Increase
		{RuleID: "RULE_ADS_INCREASE_ELIGIBLE", RuleVersion: 1, RuleCode: "increase_eligible", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 15,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"increase_eligible"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_INCREASE_ELIGIBLE", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Increase"}},
		{RuleID: "RULE_ADS_INCREASE_SAFETY", RuleVersion: 1, RuleCode: "increase_safety_net", Domain: "ads", FromLayer: "flag", ToLayer: "action", OwnerOrganizationID: systemOrgID, IsSystem: true, Priority: 16,
			InputRef: models.InputRef{SchemaRef: "schema_ads_flag", RequiredFields: []string{"safety_net"}},
			LogicRef: models.LogicRef{LogicID: "LOGIC_ADS_ACTION_FLAG_BASED", LogicVersion: 1}, ParamRef: models.ParamRef{ParamSetID: "PARAM_ADS_INCREASE_SAFETY", ParamVersion: 1},
			OutputRef: models.OutputRef{OutputID: "OUT_ACTION_CANDIDATE", OutputVersion: 1}, Status: "active", Metadata: map[string]string{"label": "Increase Safety Net"}},
	}
	for _, r := range rules {
		if _, err := svc.Upsert(ctx, bson.M{"rule_id": r.RuleID, "domain": r.Domain}, r); err != nil {
			return err
		}
	}
	return nil
}
