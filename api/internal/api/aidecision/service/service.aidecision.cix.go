// Package aidecisionsvc — ReceiveCixPayload: entry point từ CIX sang AI Decision.
package aidecisionsvc

import (
	"context"
	"os"
	"strings"
	"time"

	"meta_commerce/internal/api/aidecision/decisionlive"
	"meta_commerce/internal/api/aidecision/decisionlive/livecopy"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	cixmodels "meta_commerce/internal/api/cix/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ReceiveCixPayload nhận kết quả CIX, cập nhật decision case, enqueue aidecision.execute_requested khi đủ điều kiện.
// Không gọi ExecuteWithCase đồng bộ — worker consume EventTypeExecuteRequested.
func (s *AIDecisionService) ReceiveCixPayload(ctx context.Context, result *cixmodels.CixAnalysisResult, ownerOrgID primitive.ObjectID) error {
	if result == nil || len(result.ActionSuggestions) == 0 {
		return nil
	}

	cixPayload := toCixPayloadMap(result)

	// Cập nhật decision_cases_runtime: contextPackets.cix, receivedContexts (sessionUid = conversationId trong CIX)
	_ = s.UpdateCaseWithCixContext(ctx, result.SessionUid, result.CustomerUid, ownerOrgID.Hex(), ownerOrgID, cixPayload)

	caseDoc, _ := s.FindCaseByConversation(ctx, result.SessionUid, result.CustomerUid, ownerOrgID.Hex(), ownerOrgID)
	traceForPublish := strings.TrimSpace(result.TraceID)
	if caseDoc != nil && traceForPublish == "" {
		traceForPublish = strings.TrimSpace(caseDoc.TraceID)
	}
	if traceForPublish != "" {
		publishCixIntegratedStep(ownerOrgID, traceForPublish, caseDoc, result)
	}

	// Chỉ luồng case-centric: phải có decision_cases_runtime + đủ Context Policy Matrix mới execute (TryExecuteIfReady).
	return s.TryExecuteIfReady(ctx, result.SessionUid, result.CustomerUid, ownerOrgID.Hex(), ownerOrgID)
}

// publishCixIntegratedStep timeline/audit: đã ghi CIX vào case (gạch đầu dòng + section chi tiết).
func publishCixIntegratedStep(ownerOrgID primitive.ObjectID, traceID string, caseDoc *aidecisionmodels.DecisionCase, result *cixmodels.CixAnalysisResult) {
	if ownerOrgID.IsZero() || traceID == "" || result == nil {
		return
	}
	ev := livecopy.BuildCixIntegratedEvent(traceID, caseDoc, result)
	decisionlive.Publish(ownerOrgID, traceID, ev)
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

	// Context Policy Matrix: đủ requiredContexts trên case (§3.4).
	if !HasAllRequiredContexts(caseDoc) {
		return nil
	}

	// Cần có ít nhất CIX
	cixPayload, _ := caseDoc.ContextPackets["cix"].(map[string]interface{})
	if cixPayload == nil {
		return nil
	}

	// Cập nhật status → ready_for_decision
	_ = s.UpdateCaseStatus(ctx, caseDoc.DecisionCaseID, aidecisionmodels.CaseStatusReadyForDecision)

	// Lấy traceId từ cixPayload (toCixPayloadMap đã thêm traceId) — Learning Engine cần để query rule_execution_logs
	traceID := ""
	if t, ok := cixPayload["traceId"].(string); ok {
		traceID = strings.TrimSpace(t)
	}
	if traceID == "" {
		traceID = strings.TrimSpace(caseDoc.TraceID)
	}
	correlationID := strings.TrimSpace(caseDoc.CorrelationID)

	publishExecuteReadyStep(ownerOrgID, traceID, correlationID, caseDoc)

	req := &ExecuteRequest{
		SessionUid:    conversationID,
		CustomerUid:   customerID,
		TraceID:       traceID,
		CorrelationID: correlationID,
		CIXPayload:    cixPayload,
		CustomerCtx:   nil,
		BaseURL:       os.Getenv("BASE_URL"),
	}
	if custPayload, ok := caseDoc.ContextPackets["customer"].(map[string]interface{}); ok {
		req.CustomerCtx = custPayload
	}

	_, execErr := s.EmitExecuteRequested(ctx, req, ownerOrgID, orgID, caseDoc.DecisionCaseID)
	return execErr
}

// UpdateCaseStatus cập nhật status runtime trên decision_cases_runtime (conversation, order, Ads, …).
func (s *AIDecisionService) UpdateCaseStatus(ctx context.Context, decisionCaseID, status string) error {
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

// publishExecuteReadyStep audit/UI: đủ điều kiện — sắp gửi execute_requested.
func publishExecuteReadyStep(ownerOrgID primitive.ObjectID, traceID, correlationID string, caseDoc *aidecisionmodels.DecisionCase) {
	if ownerOrgID.IsZero() || traceID == "" || caseDoc == nil {
		return
	}
	ev := livecopy.BuildExecuteReadyEvent(traceID, correlationID, caseDoc)
	decisionlive.Publish(ownerOrgID, traceID, ev)
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
	if len(r.PipelineRuleTraceIDs) > 0 {
		m["pipelineRuleTraceIds"] = r.PipelineRuleTraceIDs
	}
	return m
}
