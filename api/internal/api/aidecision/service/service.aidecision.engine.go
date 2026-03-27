// Package aidecisionsvc — AI Decision: tầng ra quyết định liên miền.
//
// Theo docs-shared/architecture/vision/07 - decision-engine.md
package aidecisionsvc

import (
	"context"
	"fmt"
	"os"
	"strings"

	"meta_commerce/internal/api/aidecision/decisionlive"
	"meta_commerce/internal/api/aidecision/decisionlive/livecopy"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	deliverydto "meta_commerce/internal/api/delivery/dto"
	"meta_commerce/internal/approval"
	"meta_commerce/internal/utility"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	domainCix             = "cix"
	eventTypeCixPending   = "cix_action_pending_approval"
	defaultApprovalActions = "escalate_to_senior,assign_to_human_sale"
	executorApprovePath   = "/api/v1/executor/actions/approve"
	executorRejectPath    = "/api/v1/executor/actions/reject"
)

// AIDecisionService tầng ra quyết định liên miền (AI Decision).
type AIDecisionService struct{}

// NewAIDecisionService tạo service mới.
func NewAIDecisionService() *AIDecisionService {
	return &AIDecisionService{}
}

// ExecuteRequest input cho Execute — CIX payload + context.
type ExecuteRequest struct {
	SessionUid    string                 `json:"sessionUid"`
	CustomerUid   string                 `json:"customerUid"`
	CIXPayload    map[string]interface{} `json:"cixPayload,omitempty"`
	CustomerCtx   map[string]interface{} `json:"customerCtx,omitempty"`
	TraceID       string                 `json:"traceId,omitempty"`
	// W3CTraceID trace-id W3C (32 hex) — lan truyền từ queue/payload; rỗng thì Publish tự suy từ traceId.
	W3CTraceID    string                 `json:"w3cTraceId,omitempty"`
	CorrelationID string                 `json:"correlationId,omitempty"`
	BaseURL       string                 `json:"baseUrl,omitempty"`
}

// ExecuteResponse Execution Plan.
type ExecuteResponse struct {
	DecisionID       string                           `json:"decisionId"`
	TraceID          string                           `json:"traceId"`
	Actions          []deliverydto.ExecutionActionInput `json:"actions"`
	DecisionMode     string                           `json:"decisionMode,omitempty"`     // rule | llm | hybrid
	ReasoningSummary string                           `json:"reasoningSummary,omitempty"`
	Confidence       float64                          `json:"confidence,omitempty"`
}

// publishDecisionLive gắn nhãn nguồn + neo case (audit) trước khi đẩy lên timeline live.
func publishDecisionLive(ownerOrgID primitive.ObjectID, traceID string, srcKind, srcTitle string, caseDoc *aidecisionmodels.DecisionCase, ev decisionlive.DecisionLiveEvent) {
	cid, ctid := "", ""
	if caseDoc != nil {
		cid = caseDoc.DecisionCaseID
		ctid = caseDoc.TraceID
	}
	decisionlive.EnrichLiveEventFromCase(cid, ctid, &ev)
	ev.SourceKind, ev.SourceTitle = srcKind, srcTitle
	decisionlive.Publish(ownerOrgID, traceID, ev)
}

// Execute nhận context, ra quyết định, trả Execution Plan. Gọi từ API handler.
func (s *AIDecisionService) Execute(ctx context.Context, req *ExecuteRequest, ownerOrgID primitive.ObjectID) (*ExecuteResponse, error) {
	return s.ExecuteWithCase(ctx, req, ownerOrgID, nil)
}

// ExecuteWithCase giống Execute nhưng nhận case để dùng idempotency_key.
// decisionCaseID:actionType:version (version = case.CreatedAt).
func (s *AIDecisionService) ExecuteWithCase(ctx context.Context, req *ExecuteRequest, ownerOrgID primitive.ObjectID, caseDoc *aidecisionmodels.DecisionCase) (*ExecuteResponse, error) {
	decisionID := utility.GenerateUID(utility.UIDPrefixDecision)
	traceID := req.TraceID
	if traceID == "" {
		traceID = utility.GenerateUID(utility.UIDPrefixTrace)
	}
	req.TraceID = traceID
	srcKind, srcTitle := decisionlive.InferSourceForFeed(req.CIXPayload, req.SessionUid, req.CustomerUid)

	if req.CIXPayload == nil {
		publishDecisionLive(ownerOrgID, traceID, srcKind, srcTitle, caseDoc, livecopy.BuildEngineSkippedNoCix(req.CorrelationID))
		return &ExecuteResponse{
			DecisionID: decisionID,
			TraceID:    traceID,
			Actions:    []deliverydto.ExecutionActionInput{},
		}, nil
	}

	actionSuggestions := parseActionSuggestions(req.CIXPayload)
	publishDecisionLive(ownerOrgID, traceID, srcKind, srcTitle, caseDoc, livecopy.BuildEngineParseEvent(req.CorrelationID, len(actionSuggestions)))
	decisionMode := "rule"
	var confidence float64
	reasoningSummary := "Đánh giá gợi ý từ tình huống; tách hành động tự động và cần người duyệt."

	llmActions, llmMeta := resolveActionsWithLLM(ctx, ownerOrgID, req, actionSuggestions, caseDoc)
	actionSuggestions = llmActions
	if llmMeta != nil {
		decisionMode = llmMeta.Mode
		confidence = llmMeta.Confidence
		reasoningSummary = llmMeta.ReasoningSummary
	}

	if len(actionSuggestions) == 0 {
		publishDecisionLive(ownerOrgID, traceID, srcKind, srcTitle, caseDoc, livecopy.BuildEngineEmptyActions(req.CorrelationID, decisionMode, confidence, reasoningSummary))
		return &ExecuteResponse{
			DecisionID:       decisionID,
			TraceID:          traceID,
			Actions:          []deliverydto.ExecutionActionInput{},
			DecisionMode:     decisionMode,
			ReasoningSummary: reasoningSummary,
			Confidence:       confidence,
		}, nil
	}

	publishDecisionLive(ownerOrgID, traceID, srcKind, srcTitle, caseDoc, livecopy.BuildEngineDecisionEvent(req.CorrelationID, decisionMode, confidence, reasoningSummary, actionSuggestions))

	if caseDoc != nil {
		pkt := map[string]interface{}{
			"decision_id":       decisionID,
			"decision_mode":     decisionMode,
			"confidence":        confidence,
			"reasoning_summary": reasoningSummary,
			"trace_id":          traceID,
			"selected_actions":  actionSuggestions,
			"correlation_id":    req.CorrelationID,
		}
		_ = s.SetDecisionPacketOnCase(ctx, caseDoc.DecisionCaseID, pkt)
	}

	needApproval, autoActions := s.applyPolicy(actionSuggestions)
	publishDecisionLive(ownerOrgID, traceID, srcKind, srcTitle, caseDoc, livecopy.BuildEnginePolicyEvent(req.CorrelationID, len(needApproval), len(autoActions)))
	baseURL := req.BaseURL
	if baseURL == "" {
		baseURL = os.Getenv("BASE_URL")
	}

	var actionIDs []string
	for _, a := range needApproval {
		if doc, err := s.proposeCixAction(ctx, a, req, ownerOrgID, baseURL, caseDoc, decisionID, traceID); err == nil && doc != nil {
			actionIDs = append(actionIDs, doc.ID.Hex())
		}
	}

	for _, a := range autoActions {
		if doc, err := s.proposeAndApproveAutoCixAction(ctx, a, req, ownerOrgID, decisionID, traceID, caseDoc); err == nil && doc != nil {
			actionIDs = append(actionIDs, doc.ID.Hex())
		}
	}

	if len(actionIDs) > 0 {
		detail := map[string]interface{}{"actionIds": actionIDs, "count": len(actionIDs)}
		publishDecisionLive(ownerOrgID, traceID, srcKind, srcTitle, caseDoc, livecopy.BuildEngineProposeSuccess(req.CorrelationID, actionIDs, detail))
	} else {
		publishDecisionLive(ownerOrgID, traceID, srcKind, srcTitle, caseDoc, livecopy.BuildEngineProposeNone(req.CorrelationID))
	}

	publishDecisionLive(ownerOrgID, traceID, srcKind, srcTitle, caseDoc, livecopy.BuildEngineDoneEvent(req.CorrelationID, srcTitle))

	if caseDoc != nil {
		if len(actionIDs) > 0 {
			_ = s.AppendActionIDsToCase(ctx, caseDoc.DecisionCaseID, actionIDs)
		}
		// Đóng case ngay — đã tạo xong proposals. Executor quản lý actions, case không chờ outcome.
		_ = s.CloseCase(ctx, caseDoc.DecisionCaseID, aidecisionmodels.ClosureProposed)
	}

	return &ExecuteResponse{
		DecisionID:       decisionID,
		TraceID:          traceID,
		Actions:          []deliverydto.ExecutionActionInput{},
		DecisionMode:     decisionMode,
		ReasoningSummary: reasoningSummary,
		Confidence:       confidence,
	}, nil
}

func parseActionSuggestions(payload map[string]interface{}) []string {
	v, ok := payload["actionSuggestions"]
	if !ok || v == nil {
		return nil
	}
	switch val := v.(type) {
	case []string:
		return val
	case []interface{}:
		var out []string
		for _, e := range val {
			if s, ok := e.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func (s *AIDecisionService) applyPolicy(actions []string) (needApproval, auto []string) {
	approvalList := os.Getenv("CIX_APPROVAL_ACTIONS")
	if approvalList == "" {
		approvalList = defaultApprovalActions
	}
	approvalSet := make(map[string]bool)
	for _, a := range strings.Split(approvalList, ",") {
		a = strings.TrimSpace(strings.ToLower(a))
		if a != "" {
			approvalSet[a] = true
		}
	}
	for _, a := range actions {
		key := strings.TrimSpace(strings.ToLower(a))
		if approvalSet[key] {
			needApproval = append(needApproval, a)
		} else {
			auto = append(auto, a)
		}
	}
	return needApproval, auto
}

func (s *AIDecisionService) proposeCixAction(ctx context.Context, actionType string, req *ExecuteRequest, ownerOrgID primitive.ObjectID, baseURL string, caseDoc *aidecisionmodels.DecisionCase, decisionID, traceID string) (*approval.ActionPending, error) {
	reason := buildReasonFromCixPayload(req.CIXPayload)
	payload := map[string]interface{}{
		"sessionUid":    req.SessionUid,
		"customerUid":   req.CustomerUid,
		"actionType":    actionType,
		"traceId":       traceID,
		"decisionId":    decisionID,
		"correlationId": req.CorrelationID,
	}
	if req.CIXPayload != nil {
		payload["cixPayload"] = req.CIXPayload
	}
	if req.CustomerCtx != nil {
		payload["customerCtx"] = req.CustomerCtx
	}
	// contextSnapshot cho Learning Engine — CIX + Customer
	payload["contextSnapshot"] = buildContextSnapshot(req)
	if caseDoc != nil {
		payload["idempotencyKey"] = buildIdempotencyKey(caseDoc, actionType)
		payload["decisionCaseId"] = caseDoc.DecisionCaseID
	}
	doc, err := approval.Propose(ctx, domainCix, approval.ProposeInput{
		ActionType:       actionType,
		Reason:           reason,
		Payload:          payload,
		EventTypePending: eventTypeCixPending,
		ApprovePath:      executorApprovePath,
		RejectPath:       executorRejectPath,
	}, ownerOrgID, baseURL)
	return doc, err
}

func (s *AIDecisionService) proposeAndApproveAutoCixAction(ctx context.Context, actionType string, req *ExecuteRequest, ownerOrgID primitive.ObjectID, decisionID, traceID string, caseDoc *aidecisionmodels.DecisionCase) (*approval.ActionPending, error) {
	reason := buildReasonFromCixPayload(req.CIXPayload)
	payload := map[string]interface{}{
		"sessionUid":    req.SessionUid,
		"customerUid":   req.CustomerUid,
		"channel":       "messenger",
		"content":       "",
		"traceId":       traceID,
		"decisionId":    decisionID,
		"correlationId": req.CorrelationID,
	}
	if req.CIXPayload != nil {
		payload["cixPayload"] = req.CIXPayload
	}
	if req.CustomerCtx != nil {
		payload["customerCtx"] = req.CustomerCtx
	}
	payload["contextSnapshot"] = buildContextSnapshot(req)
	if caseDoc != nil {
		payload["idempotencyKey"] = buildIdempotencyKey(caseDoc, actionType)
		payload["decisionCaseId"] = caseDoc.DecisionCaseID
	}
	doc, err := approval.ProposeAndApproveAuto(ctx, domainCix, approval.ProposeInput{
		ActionType: actionType,
		Reason:     reason,
		Payload:    payload,
	}, ownerOrgID)
	if err != nil {
		return nil, err
	}
	// Learning case: OnActionClosed → learningsvc.
	return doc, nil
}

// buildIdempotencyKey format {decision_case_id}:{action_type}:{version} — tránh duplicate action khi retry.
func buildIdempotencyKey(c *aidecisionmodels.DecisionCase, actionType string) string {
	if c == nil {
		return ""
	}
	version := c.CreatedAt
	if version == 0 {
		version = c.UpdatedAt
	}
	return fmt.Sprintf("%s:%s:%d", c.DecisionCaseID, actionType, version)
}

// buildContextSnapshot tạo snapshot context cho Learning Engine — CIX + Customer.
func buildContextSnapshot(req *ExecuteRequest) map[string]interface{} {
	m := map[string]interface{}{
		"sessionId":   req.SessionUid,
		"customerId":  req.CustomerUid,
	}
	if req.CIXPayload != nil {
		m["cix"] = req.CIXPayload
		if of, ok := req.CIXPayload["orderFlags"]; ok {
			m["orderFlags"] = of
		}
		if ou, ok := req.CIXPayload["orderUid"].(string); ok && ou != "" {
			m["orderUid"] = ou
		}
	}
	if req.CustomerCtx != nil {
		m["customer"] = req.CustomerCtx
	}
	return m
}

func buildReasonFromCixPayload(payload map[string]interface{}) string {
	if payload == nil {
		return "CIX đề xuất"
	}
	parts := []string{}
	if l1, ok := payload["layer1"].(map[string]interface{}); ok {
		if s, ok := l1["stage"].(string); ok && s != "" {
			parts = append(parts, "Stage: "+s)
		}
	}
	if l3, ok := payload["layer3"].(map[string]interface{}); ok {
		if s, ok := l3["sentiment"].(string); ok && s != "" {
			parts = append(parts, "Sentiment: "+s)
		}
	}
	if len(parts) > 0 {
		return "CIX: " + strings.Join(parts, ", ")
	}
	return "CIX đề xuất"
}
