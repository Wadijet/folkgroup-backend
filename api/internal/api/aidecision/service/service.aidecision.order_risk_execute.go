// Package aidecisionsvc — Order risk: đủ context order → enqueue execute (dùng pipeline CIX với synthetic actionSuggestions).
package aidecisionsvc

import (
	"context"
	"os"
	"strings"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// envOrderRiskDefaultActions danh sách action CIX (executor đã đăng ký) khi có flag order — phân tách bằng dấu phẩy.
// Mặc định: giao human xem xét đơn có cờ.
const envOrderRiskDefaultActions = "ORDER_RISK_DEFAULT_ACTIONS"

// envOrderFlagsAllowDualExecute nếu "true": cùng order.flags_emitted vẫn gọi TryExecuteIfReady (conversation) sau order_risk.
const envOrderFlagsAllowDualExecute = "ORDER_FLAGS_ALLOW_DUAL_EXECUTE"

// OrderFlagsAllowDualExecute đọc env — mặc định false (tránh hai execute_requested trùng nghiệp vụ).
func OrderFlagsAllowDualExecute() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(envOrderFlagsAllowDualExecute)), "true")
}

// TryExecuteOrderRiskIfReady khi case order_risk đã nhận packet order — emit aidecision.execute_requested (CIX payload tổng hợp từ order intel).
// emitted == true khi đã gọi EmitExecuteRequested thành công (consumer có thể bỏ qua TryExecuteIfReady conversation).
func (s *AIDecisionService) TryExecuteOrderRiskIfReady(ctx context.Context, orderUID, orgID string, ownerOrgID primitive.ObjectID) (emitted bool, err error) {
	if orderUID == "" {
		return false, nil
	}
	caseDoc, err := s.FindCaseByOrder(ctx, orderUID, orgID, ownerOrgID)
	if err != nil || caseDoc == nil {
		return false, nil
	}
	if caseDoc.Status == aidecisionmodels.CaseStatusDecided ||
		caseDoc.Status == aidecisionmodels.CaseStatusActionsCreated ||
		caseDoc.Status == aidecisionmodels.CaseStatusExecuting ||
		caseDoc.Status == aidecisionmodels.CaseStatusClosed {
		return false, nil
	}
	if !HasAllRequiredContexts(caseDoc) {
		return false, nil
	}
	orderPkt, ok := caseDoc.ContextPackets["order"].(map[string]interface{})
	if !ok {
		return false, nil
	}
	actions := deriveOrderRiskActionSuggestions(orderPkt)
	if len(actions) == 0 {
		return false, nil
	}

	_ = s.UpdateCaseStatus(ctx, caseDoc.DecisionCaseID, aidecisionmodels.CaseStatusReadyForDecision)

	cix := map[string]interface{}{
		"actionSuggestions": actions,
		"layer1":            orderPkt["layer1"],
		"layer2":            orderPkt["layer2"],
		"layer3":            orderPkt["layer3"],
		"orderFlags":        orderPkt["flags"],
		"orderUid":          caseDoc.EntityRefs.OrderID,
		"source":            "order_intelligence",
	}

	req := &ExecuteRequest{
		SessionUid:    caseDoc.EntityRefs.ConversationID,
		CustomerUid:   caseDoc.EntityRefs.CustomerID,
		CIXPayload:    cix,
		TraceID:       "",
		CorrelationID: "",
		BaseURL:       os.Getenv("BASE_URL"),
	}
	_, err = s.EmitExecuteRequested(ctx, req, ownerOrgID, orgID, caseDoc.DecisionCaseID)
	if err != nil {
		return false, err
	}
	return true, nil
}

// deriveOrderRiskActionSuggestions map cờ → action CIX (chỉ khi có ít nhất một flag).
func deriveOrderRiskActionSuggestions(orderPkt map[string]interface{}) []string {
	if !orderPktHasNonEmptyFlags(orderPkt) {
		return nil
	}
	return parseDefaultOrderRiskActions()
}

func orderPktHasNonEmptyFlags(orderPkt map[string]interface{}) bool {
	v, ok := orderPkt["flags"]
	if !ok || v == nil {
		return false
	}
	switch x := v.(type) {
	case []interface{}:
		return len(x) > 0
	case []string:
		return len(x) > 0
	default:
		return false
	}
}

func parseDefaultOrderRiskActions() []string {
	raw := strings.TrimSpace(os.Getenv(envOrderRiskDefaultActions))
	if raw == "" {
		raw = "assign_to_human_sale"
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		out = []string{"assign_to_human_sale"}
	}
	return out
}
