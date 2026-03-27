// Package migration — Seed Rule Intelligence: quyết định dispatch consumer AI Decision (noop vs dispatch).
// Dữ liệu cấu hình noop theo org vẫn lưu decision_routing_rules; logic so khớp event_type nằm trong script (mở rộng sau không cần deploy).
//
// Chạy seed thủ công (từ thư mục api): go run ./cmd/server --seed-aidecision-dispatch
// Hoặc: ..\scripts\seed_aidecision_dispatch.ps1 — cần .env MongoDB như khi chạy server.
package migration

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"meta_commerce/internal/api/ruleintel/models"
	"meta_commerce/internal/api/ruleintel/service"
)

// scriptDecisionConsumerDispatch — noop nếu eventType nằm trong layers.routing.noopEventTypes.
var scriptDecisionConsumerDispatch = `function evaluate(ctx) {
  var r = ctx.layers.routing || {};
  var et = (r.eventType || '').toString();
  var list = r.noopEventTypes || [];
  var noop = false;
  for (var i = 0; i < list.length; i++) {
    if ((list[i] || '').toString() === et) { noop = true; break; }
  }
  var report = { log: 'eventType=' + et + ' noop=' + noop + ' noopCount=' + list.length, result: noop ? 'noop' : 'dispatch' };
  return { output: { dispatch: noop ? 'noop' : 'dispatch' }, report: report };
}`

// SeedRuleAidecisionDispatch seed rule + logic + param + output cho domain aidecision.
func SeedRuleAidecisionDispatch(ctx context.Context) error {
	systemOrgID := GetSystemOrgIDForSeed(ctx)
	if err := seedAidecisionDispatchOutput(ctx, systemOrgID); err != nil {
		return err
	}
	if err := seedAidecisionDispatchLogic(ctx, systemOrgID); err != nil {
		return err
	}
	if err := seedAidecisionDispatchParam(ctx, systemOrgID); err != nil {
		return err
	}
	if err := seedAidecisionDispatchRule(ctx, systemOrgID); err != nil {
		return err
	}
	return nil
}

func seedAidecisionDispatchOutput(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewOutputContractService()
	if err != nil {
		return err
	}
	oc := models.OutputContract{
		OutputID:            "OUT_DECISION_CONSUMER_DISPATCH",
		OutputVersion:       1,
		OutputType:          "decision_dispatch",
		OwnerOrganizationID: systemOrgID,
		IsSystem:            true,
		SchemaDefinition: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"dispatch": map[string]interface{}{"type": "string", "enum": []string{"noop", "dispatch"}},
			},
		},
		RequiredFields: []string{"dispatch"},
	}
	_, err = svc.Upsert(ctx, bson.M{"output_id": oc.OutputID, "output_version": oc.OutputVersion}, oc)
	return err
}

func seedAidecisionDispatchLogic(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewLogicScriptService()
	if err != nil {
		return err
	}
	s := models.LogicScript{
		LogicID:             "LOGIC_DECISION_CONSUMER_DISPATCH",
		LogicVersion:        1,
		LogicType:           "script",
		Runtime:             "goja",
		EntryFunction:       "evaluate",
		Status:              "active",
		OwnerOrganizationID: systemOrgID,
		IsSystem:            true,
		Script:              scriptDecisionConsumerDispatch,
	}
	_, err = svc.Upsert(ctx, bson.M{"logic_id": s.LogicID, "logic_version": s.LogicVersion}, s)
	return err
}

func seedAidecisionDispatchParam(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewParamSetService()
	if err != nil {
		return err
	}
	ps := models.ParamSet{
		ParamSetID:          "PARAM_DECISION_CONSUMER_DISPATCH",
		ParamVersion:        1,
		OwnerOrganizationID: systemOrgID,
		IsSystem:            true,
		Domain:              "aidecision",
		Segment:             "default",
		Parameters:          map[string]interface{}{},
	}
	_, err = svc.Upsert(ctx, bson.M{"param_set_id": ps.ParamSetID, "param_version": ps.ParamVersion}, ps)
	return err
}

func seedAidecisionDispatchRule(ctx context.Context, systemOrgID primitive.ObjectID) error {
	svc, err := service.NewRuleDefinitionService()
	if err != nil {
		return err
	}
	rule := models.RuleDefinition{
		RuleID:              "RULE_DECISION_CONSUMER_DISPATCH",
		RuleVersion:         1,
		RuleCode:            "decision_consumer_dispatch",
		Domain:              "aidecision",
		FromLayer:           "routing",
		ToLayer:             "dispatch_decision",
		OwnerOrganizationID: systemOrgID,
		IsSystem:            true,
		Priority:            1,
		InputRef: models.InputRef{
			SchemaRef:      "schema_decision_routing",
			RequiredFields: []string{"eventType", "noopEventTypes"},
		},
		LogicRef:  models.LogicRef{LogicID: "LOGIC_DECISION_CONSUMER_DISPATCH", LogicVersion: 1},
		ParamRef:  models.ParamRef{ParamSetID: "PARAM_DECISION_CONSUMER_DISPATCH", ParamVersion: 1},
		OutputRef: models.OutputRef{OutputID: "OUT_DECISION_CONSUMER_DISPATCH", OutputVersion: 1},
		Status:    "active",
		Metadata:  map[string]string{"label": "AI Decision — routing consumer (noop|dispatch)"},
	}
	_, err = svc.Upsert(ctx, bson.M{"rule_id": rule.RuleID, "domain": rule.Domain}, rule)
	return err
}
