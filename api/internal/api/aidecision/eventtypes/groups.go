package eventtypes

// ConversationLifecycleEventTypes — datachanged hội thoại / tin nhắn → debounce orchestrate.
var ConversationLifecycleEventTypes = []string{
	ConversationInserted,
	ConversationUpdated,
	MessageInserted,
	MessageUpdated,
}

// OrderLifecycleEventTypes — datachanged đơn → orchestrate order.
var OrderLifecycleEventTypes = []string{
	OrderInserted,
	OrderUpdated,
}

// MessageFastPathEventTypes — xử lý nguồn tin (không debounce cùng nhóm trên).
var MessageFastPathEventTypes = []string{
	ConversationMessageInserted,
	MessageBatchReady,
}

// MetaCampaignPipelineHookEventTypes — campaign_intel_recomputed hoặc legacy meta_campaign.* → ProcessMetaCampaignDataChanged.
var MetaCampaignPipelineHookEventTypes = []string{
	CampaignIntelRecomputed,
	MetaCampaignInserted,
	MetaCampaignUpdated,
}
