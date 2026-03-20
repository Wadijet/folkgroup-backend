// Package cix — Executor cho domain cix. Đăng ký với approval package.
//
// Package tách riêng để tránh import cycle (cix/service -> decision -> approval).
// Khi user approve action CIX (escalate_to_senior, assign_to_human_sale), executor thực thi.
package cix

import (
	"context"

	deliverydto "meta_commerce/internal/api/delivery/dto"
	deliverysvc "meta_commerce/internal/api/delivery/service"
	"meta_commerce/internal/approval"
	pkgapproval "meta_commerce/pkg/approval"
	"meta_commerce/internal/utility"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const domainCix = "cix"

func init() {
	approval.RegisterExecutor(domainCix, pkgapproval.ExecutorFunc(executeCixAction))
	approval.RegisterEventTypes(domainCix, map[string]string{
		"executed":  "cix_action_executed",
		"rejected":  "cix_action_rejected",
		"failed":    "cix_action_failed",
		"cancelled": "cix_action_cancelled",
	})
}

func executeCixAction(ctx context.Context, doc *pkgapproval.ActionPending) (map[string]interface{}, error) {
	actionType := doc.ActionType
	payload := doc.Payload
	if payload == nil {
		payload = map[string]interface{}{}
	}

	customerUid := getStr(payload, "customerUid")
	sessionUid := getStr(payload, "sessionUid")
	channel := getStr(payload, "channel")
	if channel == "" {
		channel = "messenger"
	}
	content := getStr(payload, "content")
	traceId := getStr(payload, "traceId")
	idempotencyKey := getStr(payload, "idempotencyKey")

	var actions []deliverydto.ExecutionActionInput
	switch actionType {
	case "trigger_fast_response":
		actions = append(actions, deliverydto.ExecutionActionInput{
			ActionID:       utility.GenerateUID(utility.UIDPrefixAction),
			ActionType:     deliverydto.ActionTypeSendMessage,
			Target:         deliverydto.ExecutionActionTarget{CustomerID: customerUid, Channel: channel},
			Payload:        map[string]interface{}{"recipient": customerUid, "content": content},
			Source:         "AI_DECISION_ENGINE",
			TraceID:        traceId,
			IdempotencyKey: idempotencyKey,
		})
	case "escalate_to_senior", "assign_to_human_sale":
		actions = append(actions, deliverydto.ExecutionActionInput{
			ActionID:       utility.GenerateUID(utility.UIDPrefixAction),
			ActionType:     deliverydto.ActionTypeAssignToAgent,
			Target:         deliverydto.ExecutionActionTarget{CustomerID: customerUid},
			Payload:        map[string]interface{}{"sessionUid": sessionUid, "actionType": actionType},
			Source:         "AI_DECISION_ENGINE",
			TraceID:        traceId,
			IdempotencyKey: idempotencyKey,
		})
	case "prioritize_followup":
		actions = append(actions, deliverydto.ExecutionActionInput{
			ActionID:       utility.GenerateUID(utility.UIDPrefixAction),
			ActionType:     deliverydto.ActionTypeTagCustomer,
			Target:         deliverydto.ExecutionActionTarget{CustomerID: customerUid},
			Payload:        map[string]interface{}{"tag": "prioritize_followup"},
			Source:         "AI_DECISION_ENGINE",
			TraceID:        traceId,
			IdempotencyKey: idempotencyKey,
		})
	default:
		return map[string]interface{}{"skipped": true, "reason": "actionType chưa map: " + actionType}, nil
	}

	if len(actions) == 0 {
		return map[string]interface{}{"executed": 0}, nil
	}

	ownerOrgID := doc.OwnerOrganizationID
	if ownerOrgID == primitive.NilObjectID {
		return map[string]interface{}{"executed": 0, "error": "ownerOrganizationId thiếu"}, nil
	}

	deliverySvc, err := deliverysvc.NewDeliveryExecuteService()
	if err != nil {
		return nil, err
	}
	queued, err := deliverySvc.ExecuteActions(ctx, actions, ownerOrgID)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"executed": len(actions), "queued": queued}, nil
}

func getStr(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key].(string)
	if !ok {
		return ""
	}
	return v
}
