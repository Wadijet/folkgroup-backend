// Package contextpolicy — Context Policy Matrix (PLATFORM_L1_EVENT_DECISION_SUPPLEMENT §3.4).
//
// Khóa context: cix | customer | order | ads — khớp contextPackets.* và receivedContexts.
// Nguồn chạy thật: Rule Engine RULE_CONTEXT_POLICY_RESOLVE + PARAM_CONTEXT_POLICY_MATRIX (version trong DB).
// Bản map Go (Matrix) chỉ là fallback khi rule chưa seed / lỗi Run.
package contextpolicy

import aidecisionmodels "meta_commerce/internal/api/aidecision/models"

// Khóa context chuẩn (bảng matrix).
const (
	KeyCix       = "cix"
	KeyCustomer  = "customer"
	KeyOrder     = "order"
	KeyAds       = "ads"
)

// Row một dòng matrix: Required bắt buộc trước execute; Optional không chặn readiness.
type Row struct {
	Required []string
	Optional []string
}

// Matrix theo caseType — map từ bảng Vision (reply, winback, ads_opt, escalate) sang case runtime.
var Matrix = map[string]Row{
	// reply → conversation_response: conversation(CIX) + customer; order tùy chọn
	aidecisionmodels.CaseTypeConversationResponse: {
		Required: []string{KeyCix, KeyCustomer},
		Optional: []string{KeyOrder},
	},
	// order_risk: cờ/thông tin đơn
	aidecisionmodels.CaseTypeOrderRisk: {
		Required: []string{KeyOrder},
		Optional: []string{KeyCix, KeyCustomer},
	},
	// ads_opt: ads bắt buộc; order/customer tùy (supplement: ads+order required — order có thể bổ sung sau khi pipeline gắn order)
	aidecisionmodels.CaseTypeAdsOptimization: {
		Required: []string{KeyAds},
		Optional: []string{KeyOrder, KeyCustomer},
	},
	// winback-style: customer + order
	aidecisionmodels.CaseTypeCustomerState: {
		Required: []string{KeyCustomer, KeyOrder},
		Optional: []string{KeyCix},
	},
	aidecisionmodels.CaseTypeExecutionRecovery: {
		Required: []string{},
		Optional: []string{KeyCix, KeyCustomer, KeyOrder},
	},
}

// DefaultRequiredContextsForCaseType fallback khi Rule Engine (RULE_CONTEXT_POLICY_RESOLVE) không chạy được — đồng bộ với seed PARAM_CONTEXT_POLICY_MATRIX.
func DefaultRequiredContextsForCaseType(caseType string) []string {
	r, ok := Matrix[caseType]
	if !ok || len(r.Required) == 0 {
		return nil
	}
	out := make([]string, len(r.Required))
	copy(out, r.Required)
	return out
}

// DefaultOptionalContextsForCaseType fallback / debug.
func DefaultOptionalContextsForCaseType(caseType string) []string {
	r, ok := Matrix[caseType]
	if !ok {
		return nil
	}
	out := make([]string, len(r.Optional))
	copy(out, r.Optional)
	return out
}
