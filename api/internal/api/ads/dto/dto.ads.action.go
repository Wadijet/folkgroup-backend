package dto

// ProposeInput body cho API propose (thêm đề xuất vào queue).
// Gọi từ n8n hoặc AI khi có đề xuất cần duyệt.
type ProposeInput struct {
	ActionType   string                 `json:"actionType" validate:"required"`   // KILL, INCREASE, DECREASE, PAUSE, RESUME, SET_BUDGET
	AdAccountId  string                 `json:"adAccountId" validate:"required"`
	CampaignId   string                 `json:"campaignId"`
	CampaignName string                 `json:"campaignName"`
	AdSetId      string                 `json:"adSetId"`
	AdId         string                 `json:"adId"`
	Value        interface{}            `json:"value"`   // Budget, %, v.v.
	Reason       string                 `json:"reason"`  // Lý do đề xuất
	Payload      map[string]interface{} `json:"payload"` // Bổ sung cho template
}

// ApproveInput body cho API approve.
type ApproveInput struct {
	ActionId string `json:"actionId" validate:"required"`
}

// RejectInput body cho API reject. decisionNote bắt buộc khi reject.
type RejectInput struct {
	ActionId     string `json:"actionId" validate:"required"`
	DecisionNote string `json:"decisionNote" validate:"required"`
}
