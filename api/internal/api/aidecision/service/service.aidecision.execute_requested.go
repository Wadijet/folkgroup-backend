// Package aidecisionsvc — Event aidecision.execute_requested: mọi Execute đi qua queue, worker gọi ExecuteWithCase.
package aidecisionsvc

import (
	"context"
	"encoding/json"
	"strings"

	"meta_commerce/internal/api/aidecision/decisionlive"
	"meta_commerce/internal/api/aidecision/decisionlive/livecopy"
	"meta_commerce/internal/api/aidecision/eventtypes"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	"meta_commerce/internal/traceutil"
	"meta_commerce/internal/utility"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// executePayloadWire payload JSON trong decision_events_queue cho EventTypeExecuteRequested.
type executePayloadWire struct {
	SessionUid     string                 `json:"sessionUid"`
	CustomerUid    string                 `json:"customerUid"`
	CIXPayload     map[string]interface{} `json:"cixPayload,omitempty"`
	CustomerCtx    map[string]interface{} `json:"customerCtx,omitempty"`
	TraceID        string                 `json:"traceId,omitempty"`
	W3CTraceID     string                 `json:"w3cTraceId,omitempty"`
	CorrelationID  string                 `json:"correlationId,omitempty"`
	BaseURL        string                 `json:"baseUrl,omitempty"`
	DecisionCaseID string                 `json:"decisionCaseId,omitempty"`
}

// EmitExecuteRequested ghi event thực thi quyết định — không gọi Execute trực tiếp.
func (s *AIDecisionService) EmitExecuteRequested(ctx context.Context, req *ExecuteRequest, ownerOrgID primitive.ObjectID, orgID string, decisionCaseID string) (*EmitEventResult, error) {
	if req == nil {
		req = &ExecuteRequest{}
	}
	if req.TraceID == "" {
		req.TraceID = utility.GenerateUID(utility.UIDPrefixTrace)
	}
	if strings.TrimSpace(req.W3CTraceID) == "" {
		req.W3CTraceID = traceutil.W3CTraceIDFromKey(strings.TrimSpace(req.TraceID))
	}
	w := executePayloadWire{
		SessionUid:     req.SessionUid,
		CustomerUid:    req.CustomerUid,
		CIXPayload:     req.CIXPayload,
		CustomerCtx:    req.CustomerCtx,
		TraceID:        req.TraceID,
		W3CTraceID:     req.W3CTraceID,
		CorrelationID:  req.CorrelationID,
		BaseURL:        req.BaseURL,
		DecisionCaseID: decisionCaseID,
	}
	payload, err := wireToMap(&w)
	if err != nil {
		return nil, err
	}
	entityID := req.SessionUid
	if entityID == "" {
		entityID = "execute"
	}
	res, err := s.EmitEvent(ctx, &EmitEventInput{
		EventType:     EventTypeExecuteRequested,
		EventSource:   eventtypes.EventSourceAIDecision,
		PipelineStage: eventtypes.PipelineStageAIDCoordination,
		EntityType:    "decision_execution",
		EntityID:      entityID,
		OrgID:         orgID,
		OwnerOrgID:    ownerOrgID,
		Priority:      "high",
		Lane:          aidecisionmodels.EventLaneFast,
		TraceID:       req.TraceID,
		CorrelationID: req.CorrelationID,
		Payload:       payload,
	})
	if err != nil {
		return nil, err
	}
	sk, st := decisionlive.InferSourceForFeed(req.CIXPayload, req.SessionUid, req.CustomerUid)
	// Chỉ phần tình huống — khung catalog §5.3 do livecopy.BuildExecuteQueuedEvent gắn theo phase queued.
	queuedSummary := ""
	switch sk {
	case decisionlive.SourceOrder:
		if st != "" {
			queuedSummary = "Đơn · " + st
		} else {
			queuedSummary = "Đơn hàng cần quyết định"
		}
	case decisionlive.SourceConversation:
		if st != "" {
			queuedSummary = "Hội thoại / tin · " + st
		} else {
			queuedSummary = "Hội thoại hoặc tin nhắn mới"
		}
	default:
		if st != "" {
			queuedSummary = "Yêu cầu · " + st
		}
	}
	evQueued := livecopy.BuildExecuteQueuedEvent(sk, st, queuedSummary, decisionCaseID, res.EventID, req.W3CTraceID, req.CorrelationID)
	decisionlive.Publish(ownerOrgID, req.TraceID, evQueued)
	return res, nil
}

func wireToMap(w *executePayloadWire) (map[string]interface{}, error) {
	b, err := json.Marshal(w)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// ProcessExecuteRequestedEvent consumer: parse payload → ExecuteWithCase (case tùy chọn).
func (s *AIDecisionService) ProcessExecuteRequestedEvent(ctx context.Context, evt *aidecisionmodels.DecisionEvent) error {
	if evt == nil || evt.Payload == nil {
		return nil
	}
	req, caseID, err := parseExecutePayloadMap(evt.Payload)
	if err != nil {
		return err
	}
	ownerOrgID := evt.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		return nil
	}
	var caseDoc *aidecisionmodels.DecisionCase
	if caseID != "" {
		caseDoc, _ = s.FindCaseByDecisionCaseID(ctx, caseID, ownerOrgID)
	}
	if req.TraceID != "" {
		sk, st := decisionlive.InferSourceForFeed(req.CIXPayload, req.SessionUid, req.CustomerUid)
		sum := ""
		if st != "" {
			sum = "Ngữ cảnh nguồn (" + sk + "): " + st
		}
		w3cLive := strings.TrimSpace(evt.W3CTraceID)
		if w3cLive == "" {
			w3cLive = strings.TrimSpace(req.W3CTraceID)
		}
		evCons := livecopy.BuildExecuteConsumingEvent(sk, st, sum, caseID, req.CorrelationID, w3cLive, evt)
		decisionlive.Publish(ownerOrgID, req.TraceID, evCons)
	}
	_, err = s.ExecuteWithCase(ctx, req, ownerOrgID, caseDoc)
	return err
}

func parseExecutePayloadMap(payload map[string]interface{}) (*ExecuteRequest, string, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	var w executePayloadWire
	if err := json.Unmarshal(b, &w); err != nil {
		return nil, "", err
	}
	req := &ExecuteRequest{
		SessionUid:    w.SessionUid,
		CustomerUid:   w.CustomerUid,
		CIXPayload:    w.CIXPayload,
		CustomerCtx:   w.CustomerCtx,
		TraceID:       w.TraceID,
		W3CTraceID:    w.W3CTraceID,
		CorrelationID: w.CorrelationID,
		BaseURL:       w.BaseURL,
	}
	if strings.TrimSpace(req.W3CTraceID) == "" && strings.TrimSpace(req.TraceID) != "" {
		req.W3CTraceID = traceutil.W3CTraceIDFromKey(strings.TrimSpace(req.TraceID))
	}
	return req, w.DecisionCaseID, nil
}
