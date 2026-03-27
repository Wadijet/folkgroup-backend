package contextpolicy

import (
	"testing"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
)

func TestDefaultRequiredContextsForCaseType_Conversation(t *testing.T) {
	req := DefaultRequiredContextsForCaseType(aidecisionmodels.CaseTypeConversationResponse)
	if len(req) != 2 || req[0] != KeyCix || req[1] != KeyCustomer {
		t.Fatalf("conversation matrix: %+v", req)
	}
}

func TestHasAllRequiredContexts(t *testing.T) {
	c := &aidecisionmodels.DecisionCase{
		RequiredContexts: []string{KeyCix, KeyCustomer},
		ReceivedContexts: []string{KeyCix},
	}
	if HasAllRequiredContexts(c) {
		t.Fatal("expected false with one received")
	}
	c.ReceivedContexts = []string{KeyCix, KeyCustomer}
	if !HasAllRequiredContexts(c) {
		t.Fatal("expected true when both received")
	}
}

// TestHasAllRequiredContexts_OrderRisk luồng order_risk: required order phải có trong received.
func TestHasAllRequiredContexts_OrderRisk(t *testing.T) {
	c := &aidecisionmodels.DecisionCase{
		CaseType:         aidecisionmodels.CaseTypeOrderRisk,
		RequiredContexts: []string{KeyOrder},
		ReceivedContexts: []string{},
	}
	if HasAllRequiredContexts(c) {
		t.Fatal("thiếu order thì chưa đủ context")
	}
	c.ReceivedContexts = []string{KeyOrder}
	if !HasAllRequiredContexts(c) {
		t.Fatal("đã nhận order thì đủ để bước execute readiness")
	}
}
