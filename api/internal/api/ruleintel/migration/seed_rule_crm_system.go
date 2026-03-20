// Package migration — Seed Rule Intelligence cho CRM domain (phân loại khách 2 lớp).
// Derivation Rule: raw metrics → crm_classification (valueTier, lifecycleStage, journeyStage, channel, loyaltyStage, momentumStage).
// Theo CUSTOMER_CLASSIFICATION_SYSTEM_DESIGN.
package migration

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"meta_commerce/internal/api/ruleintel/models"
	"meta_commerce/internal/api/ruleintel/service"
)

// scriptClassification — Logic Script: raw metrics → crm_classification.
// Input: ctx.layers.raw = { totalSpent, orderCount, lastOrderAt, revenueLast30d, revenueLast90d, orderCountOnline, orderCountOffline, hasConversation, conversationTags }
// Params: valueVip, valueHigh, valueMedium, valueLow, lifecycleActive, lifecycleCooling, lifecycleInactive, loyaltyCore, loyaltyRepeat, momentumRising, momentumStableLo, momentumStableHi
// Output: crm_classification (valueTier, lifecycleStage, journeyStage, channel, loyaltyStage, momentumStage)
var scriptClassification = `function evaluate(ctx) {
  var raw = ctx.layers.raw || {};
  var params = ctx.params || {};
  var report = { log: '', result: 'match' };
  var toF = function(m,k){var v=m[k];if(v==null)return 0;if(typeof v==='number')return v;return parseFloat(v)||0;};
  var toI = function(m,k){var v=m[k];if(v==null)return 0;return parseInt(v,10)||0;};
  var toI64 = function(m,k){var v=m[k];if(v==null)return 0;if(typeof v==='number')return v;return parseInt(v,10)||0;};
  var toArr = function(m,k){var v=m[k];return Array.isArray(v)?v:[];};
  var toB = function(m,k){var v=m[k];return v===true||v===1||v==='1';};
  var totalSpent = toF(raw,'totalSpent');
  var orderCount = toI(raw,'orderCount');
  var lastOrderAt = toI64(raw,'lastOrderAt');
  var rev30 = toF(raw,'revenueLast30d');
  var rev90 = toF(raw,'revenueLast90d');
  var ocOnline = toI(raw,'orderCountOnline');
  var ocOffline = toI(raw,'orderCountOffline');
  var hasConv = toB(raw,'hasConversation');
  var tags = toArr(raw,'conversationTags');
  report.log = '1. totalSpent=' + totalSpent + ', orderCount=' + orderCount + ', lastOrderAt=' + lastOrderAt + ', rev30=' + rev30 + ', rev90=' + rev90;
  var nowMs = Date.now();
  var msPerDay = 24*60*60*1000;
  var daysSince = lastOrderAt<=0 ? -1 : Math.floor((nowMs-lastOrderAt)/msPerDay);
  var thVip = params.valueVip||50000000; var thHigh = params.valueHigh||20000000; var thMed = params.valueMedium||5000000; var thLow = params.valueLow||1000000;
  var thActive = params.lifecycleActive||30; var thCool = params.lifecycleCooling||90; var thInactive = params.lifecycleInactive||180;
  var thLoyalCore = params.loyaltyCore||5; var thLoyalRepeat = params.loyaltyRepeat||2;
  var thMomRising = params.momentumRising||0.5; var thMomLo = params.momentumStableLo||0.2; var thMomHi = params.momentumStableHi||0.5;
  var valueTier = 'new';
  if(totalSpent>=thVip)valueTier='top';else if(totalSpent>=thHigh)valueTier='high';else if(totalSpent>=thMed)valueTier='medium';else if(totalSpent>=thLow)valueTier='low';
  report.log += '\n2. valueTier=' + valueTier + ' (daysSince=' + daysSince + ')';
  var lifecycleStage = '';
  if(daysSince>=0){if(daysSince<=thActive)lifecycleStage='active';else if(daysSince<=thCool)lifecycleStage='cooling';else if(daysSince<=thInactive)lifecycleStage='inactive';else lifecycleStage='dead';}
  var hasSpam = false;
  for(var i=0;i<tags.length;i++){var t=(tags[i]||'').toLowerCase();if(t==='block'||t==='spam'||t.indexOf('spam')>=0||t.indexOf('block')>=0||t.indexOf('chặn')>=0){hasSpam=true;break;}}
  var journeyStage = 'visitor';
  if(orderCount>0){if(orderCount>=2)journeyStage='repeat';else journeyStage='first';}
  else{if(hasSpam)journeyStage='blocked_spam';else if(hasConv)journeyStage='engaged';}
  var channel = '';
  if(orderCount>0){if(ocOnline>0&&ocOffline>0)channel='omnichannel';else if(ocOnline>0)channel='online';else if(ocOffline>0)channel='offline';}
  var loyaltyStage = '';
  if(orderCount>=thLoyalCore)loyaltyStage='core';else if(orderCount>=thLoyalRepeat)loyaltyStage='repeat';else if(orderCount>=1)loyaltyStage='one_time';
  var momentumStage = 'stable';
  if(daysSince>thCool)momentumStage='lost';
  else if(rev90<=0&&totalSpent>0)momentumStage='lost';
  else if(rev90>0&&rev30<=0&&daysSince<=thCool)momentumStage='declining';
  else if(rev30<=0)momentumStage='lost';
  else{var denom=rev90<1?1:rev90;var ratio=rev30/denom;if(ratio>thMomRising)momentumStage='rising';else if(ratio>=thMomLo&&ratio<=thMomHi)momentumStage='stable';else momentumStage='stable';}
  report.log += '\n3. lifecycle=' + lifecycleStage + ', journey=' + journeyStage + ', channel=' + channel + ', loyalty=' + loyaltyStage + ', momentum=' + momentumStage;
  var output = { valueTier: valueTier, lifecycleStage: lifecycleStage, journeyStage: journeyStage, channel: channel, loyaltyStage: loyaltyStage, momentumStage: momentumStage };
  return { output: output, report: report };
}`

// SeedRuleCrmSystem seed toàn bộ rules CRM (classification) — OwnerOrganizationID + IsSystem.
func SeedRuleCrmSystem(ctx context.Context) error {
	systemOrgID := GetSystemOrgIDForSeed(ctx)
	if err := seedCrmOutputContract(ctx, systemOrgID); err != nil {
		return err
	}
	if err := seedCrmLogicScripts(ctx, systemOrgID); err != nil {
		return err
	}
	if err := seedCrmParamSets(ctx, systemOrgID); err != nil {
		return err
	}
	if err := seedCrmRuleDefinitions(ctx, systemOrgID); err != nil {
		return err
	}
	return nil
}

func seedCrmOutputContract(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewOutputContractService()
	if err != nil {
		return err
	}
	oc := models.OutputContract{
		OutputID:            "OUT_CRM_CLASSIFICATION",
		OutputVersion:       1,
		OutputType:          "crm_classification",
		OwnerOrganizationID: systemOrgID,
		IsSystem:            true,
		SchemaDefinition: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"valueTier":      map[string]interface{}{"type": "string"},
				"lifecycleStage": map[string]interface{}{"type": "string"},
				"journeyStage":   map[string]interface{}{"type": "string"},
				"channel":        map[string]interface{}{"type": "string"},
				"loyaltyStage":   map[string]interface{}{"type": "string"},
				"momentumStage":  map[string]interface{}{"type": "string"},
			},
		},
		RequiredFields: []string{"valueTier", "lifecycleStage", "journeyStage", "channel", "loyaltyStage", "momentumStage"},
	}
	_, err = svc.Upsert(ctx, bson.M{"output_id": oc.OutputID, "output_version": oc.OutputVersion}, oc)
	return err
}

func seedCrmLogicScripts(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewLogicScriptService()
	if err != nil {
		return err
	}
	s := models.LogicScript{
		LogicID:             "LOGIC_CRM_CLASSIFICATION",
		LogicVersion:        1,
		LogicType:           "script",
		Runtime:             "goja",
		EntryFunction:       "evaluate",
		Status:              "active",
		OwnerOrganizationID: systemOrgID,
		IsSystem:            true,
		Script:              scriptClassification,
	}
	_, err = svc.Upsert(ctx, bson.M{"logic_id": s.LogicID, "logic_version": s.LogicVersion}, s)
	return err
}

func seedCrmParamSets(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewParamSetService()
	if err != nil {
		return err
	}
	ps := models.ParamSet{
		ParamSetID:            "PARAM_CRM_CLASSIFICATION",
		ParamVersion:          1,
		OwnerOrganizationID:   systemOrgID,
		IsSystem:              true,
		Domain:                "crm",
		Segment:               "default",
		Parameters: map[string]interface{}{
			"valueVip":          50000000,
			"valueHigh":         20000000,
			"valueMedium":       5000000,
			"valueLow":          1000000,
			"lifecycleActive":   30,
			"lifecycleCooling":  90,
			"lifecycleInactive": 180,
			"loyaltyCore":       5,
			"loyaltyRepeat":     2,
			"momentumRising":    0.5,
			"momentumStableLo":  0.2,
			"momentumStableHi":  0.5,
		},
	}
	_, err = svc.Upsert(ctx, bson.M{"param_set_id": ps.ParamSetID, "param_version": ps.ParamVersion}, ps)
	return err
}

func seedCrmRuleDefinitions(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewRuleDefinitionService()
	if err != nil {
		return err
	}
	rule := models.RuleDefinition{
		RuleID:               "RULE_CRM_CLASSIFICATION",
		RuleVersion:          1,
		RuleCode:             "crm_classification",
		Domain:               "crm",
		FromLayer:            "raw",
		ToLayer:              "crm_classification",
		OwnerOrganizationID:  systemOrgID,
		IsSystem:             true,
		Priority:             1,
		InputRef:             models.InputRef{SchemaRef: "schema_crm_raw", RequiredFields: []string{"totalSpent", "orderCount", "lastOrderAt", "revenueLast30d", "revenueLast90d", "orderCountOnline", "orderCountOffline", "hasConversation", "conversationTags"}},
		LogicRef:             models.LogicRef{LogicID: "LOGIC_CRM_CLASSIFICATION", LogicVersion: 1},
		ParamRef:             models.ParamRef{ParamSetID: "PARAM_CRM_CLASSIFICATION", ParamVersion: 1},
		OutputRef:            models.OutputRef{OutputID: "OUT_CRM_CLASSIFICATION", OutputVersion: 1},
		Status:               "active",
		Metadata:             map[string]string{"label": "CRM Classification"},
	}
	_, err = svc.Upsert(ctx, bson.M{"rule_id": rule.RuleID, "domain": rule.Domain}, rule)
	return err
}
