package learningsvc

import (
	"os"
	"testing"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
)

func TestShouldSkipLearningForDecisionClosure(t *testing.T) {
	t.Setenv("AI_DECISION_LEARNING_SKIP_INCOMPLETE_CLOSURE", "")
	if !shouldSkipLearningForDecisionClosure(aidecisionmodels.ClosureTimeout) {
		t.Fatal("timeout phải skip")
	}
	if !shouldSkipLearningForDecisionClosure(aidecisionmodels.ClosureManual) {
		t.Fatal("manual phải skip")
	}
	if !shouldSkipLearningForDecisionClosure(aidecisionmodels.ClosureProposed) {
		t.Fatal("proposed phải skip")
	}
	if shouldSkipLearningForDecisionClosure(aidecisionmodels.ClosureComplete) {
		t.Fatal("complete không skip")
	}
	if !shouldSkipLearningForDecisionClosure(aidecisionmodels.ClosureIncomplete) {
		t.Fatal("incomplete phải skip")
	}
	if !shouldSkipLearningForDecisionClosure(aidecisionmodels.ClosureFailed) {
		t.Fatal("failed phải skip")
	}
	if shouldSkipLearningForDecisionClosure(aidecisionmodels.ClosureNoAction) {
		t.Fatal("no_action không skip learning (có thể dùng cho học rule)")
	}
	if shouldSkipLearningForDecisionClosure("") {
		t.Fatal("rỗng (case mở / chưa gắn) không skip")
	}

	t.Setenv("AI_DECISION_LEARNING_SKIP_INCOMPLETE_CLOSURE", "0")
	if shouldSkipLearningForDecisionClosure(aidecisionmodels.ClosureTimeout) {
		t.Fatal("env=0 không skip")
	}
	_ = os.Unsetenv("AI_DECISION_LEARNING_SKIP_INCOMPLETE_CLOSURE")
}
