// Package aidecisionsvc — Emit event propose request (Vision 08 event-driven).
// Domain Ads emit event thay vì gọi Propose trực tiếp; consumer gọi ProposeForAds.
package aidecisionsvc

import (
	"context"
	"encoding/json"
	"os"

	"meta_commerce/internal/approval"
	"meta_commerce/internal/utility"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Event types cho propose request (Vision 08 event-driven).
const (
	EventTypeAdsProposeRequested = "ads.propose_requested"
)

// EmitAdsProposeRequest emit event ads.propose_requested — Ads gọi thay vì ProposeForAds.
// Consumer sẽ xử lý và gọi ProposeForAds.
func EmitAdsProposeRequest(ctx context.Context, proposeInput approval.ProposeInput, ownerOrgID primitive.ObjectID, baseURL string) (eventID string, err error) {
	if baseURL == "" {
		baseURL = os.Getenv("BASE_URL")
	}
	if baseURL == "" {
		baseURL = "https://localhost"
	}
	payload := buildProposeEventPayload(proposeInput, ownerOrgID, baseURL)
	entityID := ""
	if proposeInput.Payload != nil {
		if cid, ok := proposeInput.Payload["campaignId"].(string); ok {
			entityID = cid
		}
	}
	svc := NewAIDecisionService()
	res, err := svc.EmitEvent(ctx, &EmitEventInput{
		EventType:   EventTypeAdsProposeRequested,
		EventSource: "ads",
		EntityType:  "campaign",
		EntityID:   entityID,
		OrgID:      ownerOrgID.Hex(),
		OwnerOrgID: ownerOrgID,
		Priority:   "high",
		Lane:       aidecisionmodels.EventLaneFast,
		Payload:    payload,
	})
	if err != nil {
		return "", err
	}
	return res.EventID, nil
}

func buildProposeEventPayload(proposeInput approval.ProposeInput, ownerOrgID primitive.ObjectID, baseURL string) map[string]interface{} {
	payload := map[string]interface{}{
		"ownerOrgIdHex": ownerOrgID.Hex(),
		"baseURL":       baseURL,
		"actionType":    proposeInput.ActionType,
		"reason":        proposeInput.Reason,
		"eventTypePending": proposeInput.EventTypePending,
		"approvePath":   proposeInput.ApprovePath,
		"rejectPath":    proposeInput.RejectPath,
	}
	if proposeInput.Payload != nil {
		payload["payload"] = proposeInput.Payload
	} else {
		payload["payload"] = map[string]interface{}{}
	}
	return payload
}

// EnrichProposeInputWithTrace inject decisionId, contextSnapshot vào payload (Vision 08: chỉ AI Decision gán trace).
func EnrichProposeInputWithTrace(domain string, input *approval.ProposeInput) {
	if input.Payload == nil {
		input.Payload = make(map[string]interface{})
	}
	decisionID := utility.GenerateUID(utility.UIDPrefixDecision)
	input.Payload["decisionId"] = decisionID
	switch domain {
	case "ads":
		input.Payload["contextSnapshot"] = buildAdsContextSnapshotForPropose(input.Payload)
	default:
		input.Payload["contextSnapshot"] = map[string]interface{}{}
	}
}

// buildAdsContextSnapshotForPropose tạo context snapshot cho Learning Engine — campaign, metrics, flags.
func buildAdsContextSnapshotForPropose(payload map[string]interface{}) map[string]interface{} {
	m := map[string]interface{}{
		"campaignId":    payload["campaignId"],
		"adSetId":      payload["adSetId"],
		"adId":         payload["adId"],
		"adAccountId":  payload["adAccountId"],
		"ruleCode":     payload["ruleCode"],
		"flagsSummary": payload["flagsSummary"],
		"rawSummary":   payload["rawSummary"],
		"layer1Summary": payload["layer1Summary"],
		"layer3Summary": payload["layer3Summary"],
		"flagsDetail":   payload["flagsDetail"],
	}
	return m
}

// ParseProposeInputFromEventPayload parse approval.ProposeInput từ event payload.
func ParseProposeInputFromEventPayload(payload map[string]interface{}) (approval.ProposeInput, error) {
	var input approval.ProposeInput
	input.ActionType, _ = payload["actionType"].(string)
	input.Reason, _ = payload["reason"].(string)
	input.EventTypePending, _ = payload["eventTypePending"].(string)
	input.ApprovePath, _ = payload["approvePath"].(string)
	input.RejectPath, _ = payload["rejectPath"].(string)
	if input.ApprovePath == "" {
		input.ApprovePath = "/api/v1/executor/actions/approve"
	}
	if input.RejectPath == "" {
		input.RejectPath = "/api/v1/executor/actions/reject"
	}
	if p, ok := payload["payload"].(map[string]interface{}); ok {
		input.Payload = p
	} else if p, ok := payload["payload"]; ok && p != nil {
		// BSON có thể trả map khác type — roundtrip qua JSON
		b, _ := json.Marshal(p)
		var m map[string]interface{}
		_ = json.Unmarshal(b, &m)
		if m != nil {
			input.Payload = m
		} else {
			input.Payload = make(map[string]interface{})
		}
	} else {
		input.Payload = make(map[string]interface{})
	}
	return input, nil
}
