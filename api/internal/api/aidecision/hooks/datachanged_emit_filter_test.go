package hooks

import (
	"testing"

	"meta_commerce/internal/global"
)

func TestShouldEmitDatachangedToDecisionQueue_mapOverride(t *testing.T) {
	global.MongoDB_ColNames.MetaAdInsights = "meta_ad_insights"
	global.MongoDB_ColNames.MetaCampaigns = "meta_campaigns"
	global.MongoDB_ColNames.FbConvesations = "fb_conversations"
	saved := DatachangedEmitPerCollection
	t.Cleanup(func() {
		DatachangedEmitPerCollection = saved
	})
	DatachangedEmitPerCollection = map[string]bool{
		"meta_campaigns":   true,
		"fb_conversations": false,
	}
	if !ShouldEmitDatachangedToDecisionQueue("meta_campaigns") {
		t.Fatal("map phải bật meta_campaigns")
	}
	if ShouldEmitDatachangedToDecisionQueue("fb_conversations") {
		t.Fatal("map phải tắt fb_conversations")
	}
}

func TestShouldEmitDatachangedToDecisionQueue_metaInsightOnlyDefault(t *testing.T) {
	global.MongoDB_ColNames.MetaAdInsights = "meta_ad_insights"
	global.MongoDB_ColNames.MetaCampaigns = "meta_campaigns"
	saved := DatachangedEmitPerCollection
	t.Cleanup(func() {
		DatachangedEmitPerCollection = saved
	})
	DatachangedEmitPerCollection = nil
	if ShouldEmitDatachangedToDecisionQueue("meta_campaigns") {
		t.Fatal("mặc định Meta (không trong map) chỉ insight emit")
	}
	if !ShouldEmitDatachangedToDecisionQueue("meta_ad_insights") {
		t.Fatal("meta_ad_insights phải emit")
	}
}

func TestShouldEmitDatachangedToDecisionQueue_nonMetaDefaultTrue(t *testing.T) {
	global.MongoDB_ColNames.MetaAdInsights = "meta_ad_insights"
	saved := DatachangedEmitPerCollection
	t.Cleanup(func() {
		DatachangedEmitPerCollection = saved
	})
	DatachangedEmitPerCollection = nil
	if !ShouldEmitDatachangedToDecisionQueue("pc_pos_orders") {
		t.Fatal("non-Meta không trong map phải emit")
	}
}
