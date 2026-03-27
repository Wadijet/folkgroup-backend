package eventopstier

import "testing"

func TestClassifyEventType(t *testing.T) {
	tests := []struct {
		et       string
		wantTier string
	}{
		{"aidecision.execute_requested", TierDecision},
		{"executor.propose_requested", TierDecision},
		{"order.inserted", TierPipeline},
		{"fb_customer.updated", TierPipeline},
		{"crm_customer.inserted", TierPipeline},
		{"crm_note.updated", TierPipeline},
		{"meta_ad.updated", TierOperational},
		{"pos_product.inserted", TierOperational},
		{"crm.intelligence.compute_requested", TierOperational},
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
