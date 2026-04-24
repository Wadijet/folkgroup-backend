// Package migration — Seed Rule Intelligence: policy trì hoãn side-effect sau datachanged (JS + ParamSet).
// Đồng bộ nghiệp vụ với eventintake/datachanged_business.go — chỉnh Param / Logic qua CRUD Rule Intelligence, không cần deploy Go.
//
// Tắt rule (dùng lại classify Go + env): AI_DECISION_SIDE_EFFECT_RULE_DISABLED=1
// Bật ghi trace mỗi lần chạy: AI_DECISION_SIDE_EFFECT_RULE_TRACE=1
package migration

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"meta_commerce/internal/api/ruleintel/models"
	"meta_commerce/internal/api/ruleintel/service"
)

// scriptDatachangedSideEffectPolicy — phân tầng urgency + số giây gom (merge queue CRM / report / refresh cùng cửa sổ theo mức).
var scriptDatachangedSideEffectPolicy = `function evaluate(ctx) {
  var L = ctx.layers || {};
  var dc = L.datachanged || {};
  var p = ctx.params || {};
  var src = (dc.sourceCollection || '').toString();
  var op = (dc.operation || 'update').toString().toLowerCase();
  var et = (dc.eventType || '').toString();
  var force = !!dc.forceImmediate;

  function num(v, def) {
    var n = parseInt(v, 10);
    if (isNaN(n) || n < 0) { return def; }
    return n;
  }
  function inArr(x, arr) {
    if (!arr || !arr.length) { return false; }
    for (var i = 0; i < arr.length; i++) {
      if (arr[i] === x) { return true; }
    }
    return false;
  }
  function startsWith(a, b) { return a.length >= b.length && a.substring(0, b.length) === b; }
  function endsWith(a, b) { return a.length >= b.length && a.substring(a.length - b.length) === b; }

  var opSec = num(p.operationalDeferSec, 90);
  var bgSec = num(p.backgroundDeferSec, 300);

  var u = 2;
  var reason = 'default_operational';

  if (force) {
    u = 1;
    reason = 'force_immediate_payload';
  } else if (inArr(src, p.extraRealtimeCollections || [])) {
    u = 1;
    reason = 'param_extra_realtime';
  } else if (inArr(src, p.extraBackgroundCollections || [])) {
    u = 3;
    reason = 'param_extra_background';
  } else if (
    src === 'meta_campaigns' || src === 'meta_adsets' || src === 'meta_ads' || src === 'meta_ad_insights' ||
    src === 'meta_ad_insights_daily_snapshots' || src === 'meta_ad_accounts' ||
    src === 'pc_pos_products' || src === 'pc_pos_variations' || src === 'pc_pos_categories' ||
    src === 'order_src_pcpos_products' || src === 'order_src_pcpos_variations' || src === 'order_src_pcpos_categories' ||
    src === 'order_src_manual_products' || src === 'order_src_manual_variations' || src === 'order_src_manual_categories' ||
    src === 'pc_pos_shops' || src === 'pc_pos_warehouses' ||
    src === 'order_src_manual_shops' || src === 'order_src_manual_warehouses' ||
    src === 'fb_pages' || src === 'fb_posts' || src === 'webhook_logs'
  ) {
    u = 3;
    reason = 'background_catalog_ads';
  } else if (src === 'crm_notes' || src === 'crm_customers' || src === 'crm_activity_history') {
    u = 1;
    reason = 'realtime_crm_core';
  } else if (src === 'fb_messages') {
    u = (op === 'insert') ? 1 : 2;
    reason = 'fb_messages';
  } else if (src === 'fb_message_items') {
    u = (op === 'insert') ? 1 : 2;
    reason = 'fb_message_items';
  } else if (src === 'fb_conversations') {
    u = (op === 'insert') ? 1 : 2;
    reason = 'fb_conversations';
  } else if (src === 'pc_pos_orders' || src === 'order_src_pcpos_orders' || src === 'order_src_manual_orders') {
    u = (op === 'insert' || op === 'upsert') ? 1 : 2;
    reason = 'pcpos_orders';
  } else if (src === 'fb_customers' || src === 'pc_pos_customers' || src === 'pc_pos_src_customers' || src === 'order_src_manual_customers') {
    u = (op === 'insert' || op === 'upsert') ? 1 : 2;
    reason = 'mirror_customers';
  } else {
    if (startsWith(et, 'crm_note.') || startsWith(et, 'crm_customer.') || startsWith(et, 'crm_activity.')) {
      u = 1;
      reason = 'eventType_crm';
    } else if (startsWith(et, 'fb_message_item.') && endsWith(et, '.inserted')) {
      u = 1;
      reason = 'eventType_fb_message_item_inserted';
    } else if (startsWith(et, 'fb_message_item.') && endsWith(et, '.changed')) {
      u = (op === 'insert') ? 1 : 2;
      reason = 'eventType_fb_message_item_changed';
    } else if (startsWith(et, 'message.') && endsWith(et, '.inserted')) {
      u = 1;
      reason = 'eventType_message_inserted';
    } else if (startsWith(et, 'message.') && endsWith(et, '.changed')) {
      u = (op === 'insert') ? 1 : 2;
      reason = 'eventType_message_changed';
    } else if (startsWith(et, 'order.') && endsWith(et, '.inserted')) {
      u = 1;
      reason = 'eventType_order_inserted';
    } else if (startsWith(et, 'order.') && endsWith(et, '.changed')) {
      u = (op === 'insert' || op === 'upsert') ? 1 : 2;
      reason = 'eventType_order_changed';
    } else if (startsWith(et, 'meta_') || startsWith(et, 'fb_page.') || startsWith(et, 'fb_post.') ||
      startsWith(et, 'pos_product.') || startsWith(et, 'pos_shop.') || startsWith(et, 'webhook_log.')) {
      u = 3;
      reason = 'eventType_background';
    }
  }

  var sec = (u === 1) ? 0 : ((u === 3) ? bgSec : opSec);
  var rep = { log: reason + ' src=' + src + ' op=' + op + ' sec=' + sec, result: 'ok' };
  return {
    output: {
      urgency: u,
      deferReportSec: sec,
      deferCrmMergeQueueSec: sec,
      deferCrmIngestSec: sec,
      deferCrmRefreshSec: sec,
      reason: reason
    },
    report: rep
  };
}`

// SeedRuleAidecisionSideEffectPolicy seed RULE_DATACHANGED_SIDE_EFFECT_POLICY.
func SeedRuleAidecisionSideEffectPolicy(ctx context.Context) error {
	systemOrgID := GetSystemOrgIDForSeed(ctx)
	if err := seedSideEffectOutput(ctx, systemOrgID); err != nil {
		return err
	}
	if err := seedSideEffectLogic(ctx, systemOrgID); err != nil {
		return err
	}
	if err := seedSideEffectParam(ctx, systemOrgID); err != nil {
		return err
	}
	if err := seedSideEffectRule(ctx, systemOrgID); err != nil {
		return err
	}
	return nil
}

func seedSideEffectOutput(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewOutputContractService()
	if err != nil {
		return err
	}
	oc := models.OutputContract{
		OutputID:            "OUT_DATACHANGED_SIDE_EFFECT_POLICY",
		OutputVersion:       1,
		OutputType:          "datachanged_side_effect_policy",
		OwnerOrganizationID: systemOrgID,
		IsSystem:            true,
		SchemaDefinition: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"urgency":             map[string]interface{}{"type": "integer"},
				"deferReportSec":      map[string]interface{}{"type": "integer"},
				"deferCrmMergeQueueSec": map[string]interface{}{"type": "integer"},
				"deferCrmIngestSec":     map[string]interface{}{"type": "integer"},
				"deferCrmRefreshSec":    map[string]interface{}{"type": "integer"},
				"reason":              map[string]interface{}{"type": "string"},
			},
		},
		RequiredFields: []string{"urgency", "deferReportSec", "deferCrmMergeQueueSec", "deferCrmIngestSec", "deferCrmRefreshSec", "reason"},
	}
	_, err = svc.Upsert(ctx, bson.M{"output_id": oc.OutputID, "output_version": oc.OutputVersion}, oc)
	return err
}

func seedSideEffectLogic(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewLogicScriptService()
	if err != nil {
		return err
	}
	s := models.LogicScript{
		LogicID:             "LOGIC_DATACHANGED_SIDE_EFFECT_POLICY",
		LogicVersion:        1,
		LogicType:           "script",
		Runtime:             "goja",
		EntryFunction:       "evaluate",
		Status:              "active",
		OwnerOrganizationID: systemOrgID,
		IsSystem:            true,
		Script:              scriptDatachangedSideEffectPolicy,
	}
	_, err = svc.Upsert(ctx, bson.M{"logic_id": s.LogicID, "logic_version": s.LogicVersion}, s)
	return err
}

func seedSideEffectParam(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewParamSetService()
	if err != nil {
		return err
	}
	ps := models.ParamSet{
		ParamSetID:          "PARAM_DATACHANGED_SIDE_EFFECT_POLICY",
		ParamVersion:        1,
		OwnerOrganizationID: systemOrgID,
		IsSystem:            true,
		Domain:              "aidecision",
		Segment:             "default",
		Parameters: map[string]interface{}{
			"operationalDeferSec":        90,
			"backgroundDeferSec":         300,
			"extraRealtimeCollections":   []interface{}{},
			"extraBackgroundCollections": []interface{}{},
		},
	}
	_, err = svc.Upsert(ctx, bson.M{"param_set_id": ps.ParamSetID, "param_version": ps.ParamVersion}, ps)
	return err
}

func seedSideEffectRule(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewRuleDefinitionService()
	if err != nil {
		return err
	}
	rule := models.RuleDefinition{
		RuleID:              "RULE_DATACHANGED_SIDE_EFFECT_POLICY",
		RuleVersion:         1,
		RuleCode:            "datachanged_side_effect_policy",
		Domain:              "aidecision",
		FromLayer:           "datachanged",
		ToLayer:             "side_effect_timing",
		OwnerOrganizationID: systemOrgID,
		IsSystem:            true,
		Priority:            1,
		InputRef: models.InputRef{
			SchemaRef:      "schema_datachanged_side_effect",
			RequiredFields: []string{"sourceCollection"},
		},
		LogicRef:  models.LogicRef{LogicID: "LOGIC_DATACHANGED_SIDE_EFFECT_POLICY", LogicVersion: 1},
		ParamRef:  models.ParamRef{ParamSetID: "PARAM_DATACHANGED_SIDE_EFFECT_POLICY", ParamVersion: 1},
		OutputRef: models.OutputRef{OutputID: "OUT_DATACHANGED_SIDE_EFFECT_POLICY", OutputVersion: 1},
		Status:    "active",
		Metadata: map[string]string{
			"label": "AI Decision — Trì hoãn side-effect sau datachanged (CRM ingest / báo cáo / refresh)",
		},
	}
	_, err = svc.Upsert(ctx, bson.M{"rule_id": rule.RuleID, "domain": rule.Domain}, rule)
	return err
}
