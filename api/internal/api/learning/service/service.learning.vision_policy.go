// Package learningsvc — Điều kiện ghi learning_case khớp PLATFORM_L1 supplement §7.2.
package learningsvc

import (
	"os"
	"strings"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
)

// shouldSkipLearningForDecisionClosure: true khi decision case đã đóng với closure không đủ cho learning đầy đủ.
// Tắt hành vi bằng AI_DECISION_LEARNING_SKIP_INCOMPLETE_CLOSURE=0 (luôn ghi learning như trước).
func shouldSkipLearningForDecisionClosure(closureType string) bool {
	if strings.TrimSpace(os.Getenv("AI_DECISION_LEARNING_SKIP_INCOMPLETE_CLOSURE")) == "0" {
		return false
	}
	switch strings.TrimSpace(closureType) {
	case aidecisionmodels.ClosureTimeout, aidecisionmodels.ClosureManual, aidecisionmodels.ClosureProposed,
		aidecisionmodels.ClosureIncomplete, aidecisionmodels.ClosureFailed:
		return true
	default:
		return false
	}
}
