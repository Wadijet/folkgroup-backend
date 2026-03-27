package contextpolicy

import aidecisionmodels "meta_commerce/internal/api/aidecision/models"

// HasAllRequiredContexts true khi mọi mục trong requiredContexts đã có trong receivedContexts.
func HasAllRequiredContexts(c *aidecisionmodels.DecisionCase) bool {
	if c == nil || len(c.RequiredContexts) == 0 {
		return true
	}
	received := make(map[string]bool)
	for _, r := range c.ReceivedContexts {
		received[r] = true
	}
	for _, req := range c.RequiredContexts {
		if !received[req] {
			return false
		}
	}
	return true
}
