package aidecisionsvc

import (
	"os"
	"testing"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
)

func TestCaseHasEntityKeyForResolve(t *testing.T) {
	in := &ResolveOrCreateInput{
		CaseType: aidecisionmodels.CaseTypeConversationResponse,
		EntityRefs: aidecisionmodels.DecisionCaseEntityRefs{
			ConversationID: "c1",
		},
	}
	if !caseHasEntityKeyForResolve(in) {
		t.Fatal("có conversationId")
	}
	in.EntityRefs.ConversationID = ""
	if caseHasEntityKeyForResolve(in) {
		t.Fatal("không có khóa entity")
	}
}

func TestReopenWindowSecFromEnv(t *testing.T) {
	t.Setenv("AI_DECISION_REOPEN_WINDOW_SEC", "")
	if reopenWindowSecFromEnv() != 300 {
		t.Fatalf("mặc định 300, got %d", reopenWindowSecFromEnv())
	}
	t.Setenv("AI_DECISION_REOPEN_WINDOW_SEC", "0")
	if reopenWindowSecFromEnv() != 0 {
		t.Fatalf("0 = tắt reopen")
	}
	_ = os.Unsetenv("AI_DECISION_REOPEN_WINDOW_SEC")
}
