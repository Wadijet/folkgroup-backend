// Package aidecisionsvc — ReceiveCixPayload: entry point từ CIX sang AI Decision.
package aidecisionsvc

import (
	"context"
	"os"
	"strings"
	"time"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	cixmodels "meta_commerce/internal/api/cix/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ReceiveCixPayload nhận kết quả CIX, cập nhật decision case, gọi TryExecuteIfReady.
// Fallback: nếu không có case (luồng cũ CIX gọi trực tiếp), Execute ngay với CIX payload.
func (s *AIDecisionService) ReceiveCixPayload(ctx context.Context, result *cixmodels.CixAnalysisResult, ownerOrgID primitive.ObjectID) error {
	if result == nil || len(result.ActionSuggestions) == 0 {
		return nil
	}

	cixPayload := toCixPayloadMap(result)

	// Cập nhật decision_cases_runtime: contextPackets.cix, receivedContexts (sessionUid = conversationId trong CIX)
	_ = s.UpdateCaseWithCixContext(ctx, result.SessionUid, result.CustomerUid, ownerOrgID.Hex(), ownerOrgID, cixPayload)

	err := s.TryExecuteIfReady(ctx, result.SessionUid, result.CustomerUid, ownerOrgID.Hex(), ownerOrgID)
	if err != nil {
		return err
	}
	// Fallback: gọi CIX trực tiếp (POST /cix/analyze) không qua case — Execute ngay khi không tìm thấy case runtime.
	caseDoc, findErr := s.FindCaseByConversation(ctx, result.SessionUid, result.CustomerUid, ownerOrgID.Hex(), ownerOrgID)
	if findErr != nil || caseDoc == nil {
		req := &ExecuteRequest{
			SessionUid:    result.SessionUid,
			CustomerUid:   result.CustomerUid,
			TraceID:       result.TraceID,
			CorrelationID: result.CorrelationID,
			CIXPayload:    cixPayload,
			BaseURL:       os.Getenv("BASE_URL"),
		}
		_, _ = s.ExecuteWithCase(ctx, req, ownerOrgID, nil)
	}
	return nil
}

// TryExecuteIfReady kiểm tra case đủ context → cập nhật status → Execute.
// Gọi từ processCixAnalysisCompleted và processCustomerContextReady.
func (s *AIDecisionService) TryExecuteIfReady(ctx context.Context, conversationID, customerID, orgID string, ownerOrgID primitive.ObjectID) error {
	caseDoc, err := s.FindCaseByConversation(ctx, conversationID, customerID, orgID, ownerOrgID)
	if err != nil || caseDoc == nil {
		return nil
	}

	// Đã Execute rồi — tránh chạy lại
	if caseDoc.Status == aidecisionmodels.CaseStatusDecided ||
		caseDoc.Status == aidecisionmodels.CaseStatusActionsCreated ||
		caseDoc.Status == aidecisionmodels.CaseStatusExecuting ||
		caseDoc.Status == aidecisionmodels.CaseStatusClosed {
		return nil
	}

	// AI_DECISION_REQUIRE_BOTH_CONTEXT=true: chỉ Execute khi có cả cix và customer
	requireBoth := strings.TrimSpace(strings.ToLower(os.Getenv("AI_DECISION_REQUIRE_BOTH_CONTEXT"))) == "true"
	if requireBoth && !HasAllRequiredContexts(caseDoc) {
		return nil
	}

	// Cần có ít nhất CIX
	cixPayload, _ := caseDoc.ContextPackets["cix"].(map[string]interface{})
	if cixPayload == nil {
		return nil
	}

	// Cập nhật status → ready_for_decision
	_ = s.updateCaseStatus(ctx, caseDoc.DecisionCaseID, aidecisionmodels.CaseStatusReadyForDecision)

	// Lấy traceId từ cixPayload (toCixPayloadMap đã thêm traceId) — Learning Engine cần để query rule_execution_logs
	traceID := ""
	if t, ok := cixPayload["traceId"].(string); ok {
		traceID = t
	}

	req := &ExecuteRequest{
		SessionUid:    conversationID,
		CustomerUid:   customerID,
		TraceID:       traceID,
		CorrelationID: "",
		CIXPayload:    cixPayload,
		CustomerCtx:   nil,
		BaseURL:       os.Getenv("BASE_URL"),
	}
	if custPayload, ok := caseDoc.ContextPackets["customer"].(map[string]interface{}); ok {
		req.CustomerCtx = custPayload
	}

	_, execErr := s.ExecuteWithCase(ctx, req, ownerOrgID, caseDoc)
	if execErr != nil {
		return execErr
	}

	return nil
}

// updateCaseStatus cập nhật status của case.
func (s *AIDecisionService) updateCaseStatus(ctx context.Context, decisionCaseID, status string) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionCasesRuntime)
	if !ok {
		return nil
	}
	now := time.Now().UnixMilli()
	_, _ = coll.UpdateOne(ctx, bson.M{"decisionCaseId": decisionCaseID}, bson.M{
		"$set": bson.M{"status": status, "updatedAt": now},
	})
	return nil
}

func toCixPayloadMap(r *cixmodels.CixAnalysisResult) map[string]interface{} {
	m := map[string]interface{}{
		"traceId":           r.TraceID,
		"actionSuggestions": r.ActionSuggestions,
		"layer1": map[string]interface{}{"stage": r.Layer1.Stage},
		"layer2": map[string]interface{}{
			"intentStage":  r.Layer2.IntentStage,
			"urgencyLevel": r.Layer2.UrgencyLevel,
			"riskLevelRaw": r.Layer2.RiskLevelRaw,
			"riskLevelAdj": r.Layer2.RiskLevelAdj,
		},
		"layer3": map[string]interface{}{
			"buyingIntent":   r.Layer3.BuyingIntent,
			"objectionLevel": r.Layer3.ObjectionLevel,
			"sentiment":      r.Layer3.Sentiment,
		},
	}
	if len(r.Flags) > 0 {
		flags := make([]map[string]interface{}, 0, len(r.Flags))
		for _, f := range r.Flags {
			flags = append(flags, map[string]interface{}{
				"name":           f.Name,
				"severity":       f.Severity,
				"triggeredByRule": f.TriggeredByRule,
			})
		}
		m["flags"] = flags
	}
	return m
}
