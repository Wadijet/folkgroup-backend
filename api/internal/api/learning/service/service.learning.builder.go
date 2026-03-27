// Package learningsvc — Builder chuyển ActionPending thành LearningCase (schema vision 11).
//
// Chỉ gọi khi entity đã đóng vòng đời (executed/rejected/failed).
package learningsvc

import (
	"context"
	"fmt"
	"strings"
	"time"

	pkgapproval "meta_commerce/pkg/approval"

	"meta_commerce/internal/api/learning/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// BuildLearningCaseFromAction chuyển ActionPending (executed/rejected/failed) thành LearningCase.
func BuildLearningCaseFromAction(ctx context.Context, ap *pkgapproval.ActionPending) (*models.LearningCase, error) {
	if ap == nil {
		return nil, fmt.Errorf("ActionPending không được nil")
	}
	switch ap.Status {
	case pkgapproval.StatusExecuted, pkgapproval.StatusRejected, pkgapproval.StatusFailed:
	default:
		return nil, fmt.Errorf("ActionPending chưa đóng vòng đời (status=%s), không tạo learning case", ap.Status)
	}

	result := models.LearningResultSuccess
	if ap.Status == pkgapproval.StatusRejected {
		result = models.LearningResultRejected
	} else if ap.Status == pkgapproval.StatusFailed {
		result = models.LearningResultFailed
	}

	closedAt := ap.ExecutedAt
	if ap.Status == pkgapproval.StatusRejected {
		closedAt = ap.RejectedAt
	}
	if closedAt == 0 {
		closedAt = time.Now().Unix()
	}
	closedAtMs := closedAt * 1000

	entityType, entityID := resolveEntityTypeAndID(ap)
	targetType, targetId := extractTargetFromPayload(ap)
	if targetType == "" {
		targetType = entityType
	}
	if targetId == "" {
		targetId = entityID
	}

	// Context snapshot từ payload
	contextSnapshot := map[string]interface{}{}
	if ap.Payload != nil {
		if cs, ok := ap.Payload["contextSnapshot"].(map[string]interface{}); ok {
			contextSnapshot = cs
		}
	}

	// Input signals (CIX: layer1-3, flags)
	inputSignals := map[string]interface{}{}
	if ap.Payload != nil {
		if cix, ok := ap.Payload["cixPayload"].(map[string]interface{}); ok {
			inputSignals["cix"] = cix
		}
	}

	// Rules applied — query rule_execution_logs khi có trace_id
	rulesApplied, paramVersion := FetchRulesAppliedFromTraceID(ctx, ap.TraceID)

	// Action executed
	actionExecuted := buildActionExecuted(ap)

	// Outcome technical
	outcome := buildOutcome(ap, closedAtMs)

	// Decision (strategy, mode)
	decision := map[string]interface{}{
		"actionType": ap.ActionType,
		"reason":     ap.Reason,
	}
	if ap.Payload != nil {
		if mode, ok := ap.Payload["mode"].(string); ok {
			decision["mode"] = mode
		}
	}

	caseId := fmt.Sprintf("lc_%s_%d", ap.ID.Hex()[:8], closedAt)
	decisionCaseClosure := lookupDecisionCaseClosureType(ctx, ap.DecisionCaseID, ap.OwnerOrganizationID)
	corrID := extractPayloadString(ap.Payload, "correlationId", "correlation_id")
	propEvt := extractPayloadString(ap.Payload, "aidecisionProposeEventId")
	parentEvt := extractPayloadString(ap.Payload, "parentEventId", "parent_event_id")
	rootEvt := extractPayloadString(ap.Payload, "rootEventId", "root_event_id")
	idemKey := extractPayloadString(ap.Payload, "idempotencyKey", "idempotency_key")
	lc := &models.LearningCase{
		CaseId:              caseId,
		DecisionID:          ap.DecisionID,
		DecisionCaseID:      strings.TrimSpace(ap.DecisionCaseID),
		CorrelationID:       corrID,
		AIDecisionProposeEventID: propEvt,
		ParentEventID:       parentEvt,
		RootEventID:         rootEvt,
		ActionLifecycle:     buildActionLifecycle(ap, idemKey),
		EntityType:          entityType,
		EntityID:            entityID,
		ContextSnapshot:     contextSnapshot,
		InputSignals:        inputSignals,
		RulesApplied:        rulesApplied,
		ParamVersion:        paramVersion,
		Decision:            decision,
		ActionExecuted:      actionExecuted,
		ExecutionTraceID:   ap.TraceID,
		Outcome:             outcome,
		OwnerOrganizationID: ap.OwnerOrganizationID,
		SourceRefType:       "action_pending",
		SourceRefID:         ap.ID.Hex(),
		Domain:              ap.Domain,
		ActionType:          ap.ActionType,
		Result:              result,
		ClosedAt:            closedAtMs,
		CaseType:            "action",
		CaseCategory:        ap.Domain,
		GoalCode:            ap.ActionType,
		TargetType:          targetType,
		TargetId:            targetId,
		DecisionCaseClosureType: decisionCaseClosure,
	}
	return lc, nil
}

// lookupDecisionCaseClosureType đọc closureType từ decision_cases_runtime (nếu case đã đóng).
func lookupDecisionCaseClosureType(ctx context.Context, decisionCaseID string, ownerOrgID primitive.ObjectID) string {
	decisionCaseID = strings.TrimSpace(decisionCaseID)
	if decisionCaseID == "" || ownerOrgID.IsZero() {
		return ""
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionCasesRuntime)
	if !ok {
		return ""
	}
	var doc struct {
		ClosureType string `bson:"closureType"`
	}
	_ = coll.FindOne(ctx, bson.M{"decisionCaseId": decisionCaseID, "ownerOrganizationId": ownerOrgID}).Decode(&doc)
	return strings.TrimSpace(doc.ClosureType)
}

func resolveEntityTypeAndID(ap *pkgapproval.ActionPending) (entityType, entityID string) {
	switch ap.Domain {
	case "cix":
		entityType = models.EntityTypeSession
		if ap.Payload != nil {
			if s, ok := ap.Payload["sessionUid"].(string); ok {
				entityID = s
			}
		}
	case "ads":
		entityType = models.EntityTypeCampaign
		if ap.Payload != nil {
			if c, ok := ap.Payload["campaignId"].(string); ok {
				entityID = c
			}
		}
	case "cio":
		entityType = models.EntityTypeTouchpointPlan
		if ap.Payload != nil {
			if p, ok := ap.Payload["touchpointPlanId"].(string); ok {
				entityID = p
			}
		}
	default:
		entityType = models.EntityTypeActionPending
		entityID = ap.ID.Hex()
	}
	if entityID == "" {
		entityID = ap.ID.Hex()
	}
	return entityType, entityID
}

func extractTargetFromPayload(ap *pkgapproval.ActionPending) (targetType, targetId string) {
	if ap.Payload == nil {
		return "", ""
	}
	if t, ok := ap.Payload["targetType"].(string); ok {
		targetType = t
	}
	if t, ok := ap.Payload["targetId"].(string); ok {
		targetId = t
	}
	if targetType == "" && ap.Domain == "ads" {
		targetType = models.EntityTypeCampaign
		if cid, ok := ap.Payload["campaignId"].(string); ok {
			targetId = cid
		}
	}
	return targetType, targetId
}

func buildActionExecuted(ap *pkgapproval.ActionPending) map[string]interface{} {
	m := map[string]interface{}{
		"actionType": ap.ActionType,
		"reason":     ap.Reason,
		"status":     ap.Status,
	}
	if ap.Payload != nil {
		m["payload"] = ap.Payload
	}
	if ap.ExecuteResponse != nil {
		m["executeResponse"] = ap.ExecuteResponse
	}
	if ap.ExecuteError != "" {
		m["executeError"] = ap.ExecuteError
	}
	return m
}

func buildOutcome(ap *pkgapproval.ActionPending, closedAtMs int64) models.LearningOutcome {
	o := models.LearningOutcome{
		Direct: true,
		RecordedAt: time.UnixMilli(closedAtMs).Format(time.RFC3339),
	}
	if ap.Status == pkgapproval.StatusExecuted {
		o.Technical.Status = "success"
		o.Technical.Delivery = "delivered"
		if ap.ExecuteResponse != nil {
			if lat, ok := toInt64(ap.ExecuteResponse["latencyMs"]); ok {
				o.Technical.LatencyMs = lat
			}
		}
	} else if ap.Status == pkgapproval.StatusRejected {
		o.Technical.Status = "rejected"
		o.Technical.Error = ap.DecisionNote
	} else if ap.Status == pkgapproval.StatusFailed {
		o.Technical.Status = "fail"
		o.Technical.Error = ap.ExecuteError
	}
	// outcome.business để trống — Evaluation Job / session_closed enrich sau
	return o
}

func toInt64(v interface{}) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case int:
		return int64(x), true
	case float64:
		return int64(x), true
	default:
		return 0, false
	}
}

// extractPayloadString lấy chuỗi đầu tiên khác rỗng theo thứ tự khóa (hỗ trợ snake_case / camelCase).
func extractPayloadString(m map[string]interface{}, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		switch s := v.(type) {
		case string:
			if t := strings.TrimSpace(s); t != "" {
				return t
			}
		}
	}
	return ""
}

// buildActionLifecycle snapshot timeline action_pending khi đóng (trace E2E: propose → duyệt/từ chối → thực thi).
func buildActionLifecycle(ap *pkgapproval.ActionPending, idempotencyKey string) models.LearningActionLifecycle {
	if ap == nil {
		return models.LearningActionLifecycle{}
	}
	lc := models.LearningActionLifecycle{
		ProposedAt:      ap.ProposedAt,
		ApprovedAt:      ap.ApprovedAt,
		RejectedAt:      ap.RejectedAt,
		ExecutedAt:      ap.ExecutedAt,
		FinalStatus:     ap.Status,
		IdempotencyKey:  strings.TrimSpace(idempotencyKey),
		ActionCreatedAt: ap.CreatedAt,
		ActionUpdatedAt: ap.UpdatedAt,
	}
	return lc
}

// BuildLearningCaseFromCIOChoice — CIO sẽ sửa và nối luồng sau. Giữ stub để không break.
func BuildLearningCaseFromCIOChoice(in *CIOChoiceInput) (*models.LearningCase, error) {
	if in == nil {
		return nil, fmt.Errorf("CIOChoiceInput không được nil")
	}
	if in.PlanID == "" || in.UnifiedID == "" || in.GoalCode == "" {
		return nil, fmt.Errorf("CIOChoiceInput thiếu planId, unifiedId hoặc goalCode")
	}
	sourceClosedAt := in.SourceClosedAt
	if sourceClosedAt == 0 {
		sourceClosedAt = time.Now().UnixMilli()
	}
	planIdShort := in.PlanID
	if len(planIdShort) > 8 {
		planIdShort = planIdShort[:8]
	}
	return &models.LearningCase{
		EntityType:          models.EntityTypeTouchpointPlan,
		EntityID:            in.PlanID,
		OwnerOrganizationID: in.OwnerOrganizationID,
		SourceRefType:       "cio_touchpoint_plan",
		SourceRefID:         in.PlanID,
		Domain:              "cio",
		ActionType:          in.GoalCode,
		Result:              models.LearningResultSuccess,
		ClosedAt:            sourceClosedAt,
		CaseType:            "cio_choice",
		CaseCategory:        "cio",
		GoalCode:            in.GoalCode,
		TargetType:          "customer",
		TargetId:            in.UnifiedID,
	}, nil
}

// CIOChoiceInput input cho BuildLearningCaseFromCIOChoice.
type CIOChoiceInput struct {
	PlanID              string
	UnifiedID           string
	GoalCode            string
	ChannelChosen       string
	Reason              string
	SourceClosedAt      int64
	OwnerOrganizationID primitive.ObjectID
	ExperimentID        string
	Variant             string
}
