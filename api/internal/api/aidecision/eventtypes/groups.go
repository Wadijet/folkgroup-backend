package eventtypes

// ConversationLifecycleEventTypes — datachanged hội thoại / tin nhắn → debounce orchestrate.
var ConversationLifecycleEventTypes = []string{
	ConversationChanged,
	MessageChanged,
}

// OrderLifecycleEventTypes — datachanged đơn → orchestrate order.
var OrderLifecycleEventTypes = []string{
	OrderChanged,
}

// MessageFastPathEventTypes — xử lý nguồn tin (không debounce cùng nhóm trên).
var MessageFastPathEventTypes = []string{
	ConversationMessageInserted,
	MessageBatchReady,
}

// MetaCampaignPipelineHookEventTypes — campaign_intel_recomputed hoặc meta_campaign.* → ProcessMetaCampaignDataChanged.
var MetaCampaignPipelineHookEventTypes = []string{
	CampaignIntelRecomputed,
	MetaCampaignChanged,
}
