// Package aidecisionsvc — decision_routing_rules + Rule Engine (RULE_DECISION_CONSUMER_DISPATCH) quyết định noop vs dispatch.
package aidecisionsvc

import (
	"context"
	"errors"
	"strings"
	"sync"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	ruleintelmodels "meta_commerce/internal/api/ruleintel/models"
	ruleintelsvc "meta_commerce/internal/api/ruleintel/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	ruleIDDecisionConsumerDispatch = "RULE_DECISION_CONSUMER_DISPATCH"
	domainAidecisionRule           = "aidecision"
)

var (
	decisionRoutingRuleEngineOnce sync.Once
	decisionRoutingRuleEngine     *ruleintelsvc.RuleEngineService
	decisionRoutingRuleEngineErr  error
)

func getDecisionRoutingRuleEngine() (*ruleintelsvc.RuleEngineService, error) {
	decisionRoutingRuleEngineOnce.Do(func() {
		decisionRoutingRuleEngine, decisionRoutingRuleEngineErr = ruleintelsvc.NewRuleEngineService()
	})
	return decisionRoutingRuleEngine, decisionRoutingRuleEngineErr
}

// ShouldSkipDispatchForRoutingRule true khi Rule Engine trả dispatch=noop (cấu hình noop trong decision_routing_rules).
// Không có rule noop nào cho org → không gọi engine (fail nhẹ). Lỗi engine/DB → fail-open (vẫn dispatch).
func (s *AIDecisionService) ShouldSkipDispatchForRoutingRule(ctx context.Context, ownerOrgID primitive.ObjectID, eventType string) bool {
	if ownerOrgID.IsZero() || eventType == "" {
		return false
	}
	noopTypes, err := s.listNoopEventTypesForOrg(ctx, ownerOrgID)
	if err != nil {
		logrus.WithError(err).Warn("decision_routing_rules: đọc danh sách noop thất bại, fail-open")
		return false
	}
	if len(noopTypes) == 0 {
		return false
	}
	re, err := getDecisionRoutingRuleEngine()
	if err != nil {
		logrus.WithError(err).Warn("Rule Engine (routing consumer): khởi tạo thất bại, fail-open")
		return false
	}
	layers := map[string]interface{}{
		"routing": map[string]interface{}{
			"eventType":           eventType,
			"ownerOrganizationId": ownerOrgID.Hex(),
			"noopEventTypes":      noopTypes,
		},
	}
	runRes, err := re.Run(ctx, &ruleintelsvc.RunInput{
		RuleID:    ruleIDDecisionConsumerDispatch,
		Domain:    domainAidecisionRule,
		EntityRef: ruleintelmodels.EntityRef{Domain: domainAidecisionRule, ObjectType: "decision_event", ObjectID: eventType, OwnerOrganizationID: ownerOrgID.Hex()},
		Layers:    layers,
		SkipTrace: true,
	})
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			logrus.Warn("Rule RULE_DECISION_CONSUMER_DISPATCH chưa seed — chạy seed Rule Intelligence (Aidecision dispatch), fail-open")
			return false
		}
		logrus.WithError(err).WithFields(logrus.Fields{
			"ownerOrganizationId": ownerOrgID.Hex(),
			"eventType":           eventType,
		}).Warn("Rule Engine (routing consumer): Run thất bại, fail-open")
		return false
	}
	if runRes == nil || runRes.Result == nil {
		return false
	}
	m, ok := runRes.Result.(map[string]interface{})
	if !ok {
		return false
	}
	d, _ := m["dispatch"].(string)
	return strings.TrimSpace(strings.ToLower(d)) == aidecisionmodels.RoutingBehaviorNoop
}

// listNoopEventTypesForOrg các eventType có behavior=noop và enabled cho org.
func (s *AIDecisionService) listNoopEventTypesForOrg(ctx context.Context, ownerOrgID primitive.ObjectID) ([]string, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionRoutingRules)
	if !ok {
		return nil, errors.New("collection decision_routing_rules chưa đăng ký")
	}
	cur, err := coll.Find(ctx, bson.M{
		"ownerOrganizationId": ownerOrgID,
		"enabled":             true,
		"behavior":            aidecisionmodels.RoutingBehaviorNoop,
	})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	seen := make(map[string]struct{})
	var out []string
	for cur.Next(ctx) {
		var doc aidecisionmodels.DecisionRoutingRule
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		et := strings.TrimSpace(doc.EventType)
		if et == "" {
			continue
		}
		if _, dup := seen[et]; dup {
			continue
		}
		seen[et] = struct{}{}
		out = append(out, et)
	}
	return out, cur.Err()
}
