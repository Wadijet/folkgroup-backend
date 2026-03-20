// Package deliveryhdl — Unit test Phase 3 Delivery Gate (validate actionId khi source=APPROVAL_GATE).
package deliveryhdl

import (
	"strings"
	"testing"

	deliverydto "meta_commerce/internal/api/delivery/dto"
)

func TestValidateDeliveryExecuteActionsForGate_ValidAction_Passes(t *testing.T) {
	actions := []deliverydto.ExecutionActionInput{
		{
			ActionID:   "act_123",
			ActionType: deliverydto.ActionTypeSendMessage,
			Source:     deliverydto.SourceApprovalGate,
			Target:     deliverydto.ExecutionActionTarget{CustomerID: "cust_1", Channel: "messenger"},
		},
	}
	code, msg := validateDeliveryExecuteActionsForGate(actions)
	if code != 0 {
		t.Errorf("action hợp lệ phải pass, got code=%d msg=%s", code, msg)
	}
}

func TestValidateDeliveryExecuteActionsForGate_EmptyActionId_Rejects(t *testing.T) {
	actions := []deliverydto.ExecutionActionInput{
		{
			ActionID:   "",
			ActionType: deliverydto.ActionTypeSendMessage,
			Source:     deliverydto.SourceApprovalGate,
			Target:     deliverydto.ExecutionActionTarget{CustomerID: "cust_1"},
		},
	}
	code, msg := validateDeliveryExecuteActionsForGate(actions)
	if code != 403 {
		t.Errorf("actionId rỗng phải trả 403, got code=%d", code)
	}
	if msg == "" || !strings.Contains(msg, "actionId") {
		t.Errorf("message phải mention actionId, got %s", msg)
	}
}

func TestValidateDeliveryExecuteActionsForGate_WrongSource_Rejects(t *testing.T) {
	actions := []deliverydto.ExecutionActionInput{
		{
			ActionID:   "act_123",
			ActionType: deliverydto.ActionTypeSendMessage,
			Source:     "AI_DECISION_ENGINE",
			Target:     deliverydto.ExecutionActionTarget{CustomerID: "cust_1"},
		},
	}
	code, msg := validateDeliveryExecuteActionsForGate(actions)
	if code != 403 {
		t.Errorf("source khác APPROVAL_GATE phải trả 403, got code=%d", code)
	}
	if msg == "" || !strings.Contains(msg, "APPROVAL_GATE") {
		t.Errorf("message phải mention APPROVAL_GATE, got %s", msg)
	}
}

func TestValidateDeliveryExecuteActionsForGate_MultipleActions_AllValidated(t *testing.T) {
	actions := []deliverydto.ExecutionActionInput{
		{ActionID: "act_1", Source: deliverydto.SourceApprovalGate, Target: deliverydto.ExecutionActionTarget{}},
		{ActionID: "", Source: deliverydto.SourceApprovalGate, Target: deliverydto.ExecutionActionTarget{}},
	}
	code, _ := validateDeliveryExecuteActionsForGate(actions)
	if code != 403 {
		t.Errorf("action thứ 2 có actionId rỗng phải trả 403, got code=%d", code)
	}
}
