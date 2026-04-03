package eventopstier

import (
	"testing"

	"meta_commerce/internal/api/aidecision/eventtypes"
)

func TestClassifyEventType(t *testing.T) {
	tests := []struct {
		et       string
		wantTier string
	}{
		{eventtypes.AIDecisionExecuteRequested, TierDecision},
		{eventtypes.ExecutorProposeRequested, TierDecision},
		{eventtypes.OrderInserted, TierPipeline},
		{"fb_customer.updated", TierPipeline},
		{"crm_customer.inserted", TierPipeline},
		{"crm_note.updated", TierPipeline},
		{"meta_ad.updated", TierOperational},
		{"pos_product.inserted", TierOperational},
		{eventtypes.CrmIntelligenceComputeRequested, TierOperational},
		{eventtypes.CrmIntelligenceRecomputeRequested, TierOperational},
		{"", TierUnknown},
		{"custom.vendor.event", TierUnknown},
	}
	for _, tc := range tests {
		tier, _ := ClassifyEventType(tc.et)
		if tier != tc.wantTier {
			t.Errorf("ClassifyEventType(%q) tier=%q want %q", tc.et, tier, tc.wantTier)
		}
	}
}
