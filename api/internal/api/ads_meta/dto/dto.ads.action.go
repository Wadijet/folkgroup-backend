package dto

// ActionType constants — các loại lệnh Meta API hỗ trợ.
const (
	ActionKILL             = "KILL"             // Tạm dừng (PAUSED)
	ActionPAUSE             = "PAUSE"            // Tạm dừng (PAUSED)
	ActionRESUME            = "RESUME"           // Bật lại (ACTIVE)
	ActionARCHIVE           = "ARCHIVE"          // Lưu trữ (ARCHIVED)
	ActionDELETE            = "DELETE"           // Xóa (DELETED)
	ActionSET_BUDGET        = "SET_BUDGET"        // Đặt daily_budget (cent)
	ActionSET_LIFETIME_BUDGET = "SET_LIFETIME_BUDGET" // Đặt lifetime_budget (cent)
	ActionINCREASE          = "INCREASE"         // Tăng budget theo % (value = %)
	ActionDECREASE          = "DECREASE"          // Giảm budget theo % (value = %)
	ActionSET_NAME          = "SET_NAME"          // Đổi tên (value = tên mới)
)

// ProposeInput body cho API tạo lệnh (POST /ads/commands, POST /ads/actions/propose).
// User trực tiếp tạo lệnh chờ duyệt — cần ít nhất một trong campaignId, adSetId, adId.
type ProposeInput struct {
	ActionType   string                 `json:"actionType" validate:"required"`   // KILL, PAUSE, RESUME, ARCHIVE, DELETE, SET_BUDGET, SET_LIFETIME_BUDGET, INCREASE, DECREASE, SET_NAME
	AdAccountId  string                 `json:"adAccountId" validate:"required"`
	CampaignId   string                 `json:"campaignId"`
	CampaignName string                 `json:"campaignName"`
	AdSetId      string                 `json:"adSetId"`
	AdId         string                 `json:"adId"`
	Value        interface{}            `json:"value"`   // Budget (cent), % (INCREASE/DECREASE), tên mới (SET_NAME)
	Reason       string                 `json:"reason" validate:"required"` // Lý do đề xuất — bắt buộc
	RuleCode     string                 `json:"ruleCode"`                   // Mã rule / idempotency — đồng bộ với ads service
	TraceID      string                 `json:"traceId"`                    // Link rule_execution_logs (tuỳ chọn)
	Payload      map[string]interface{} `json:"payload"` // Bổ sung (vd: name cho SET_NAME)
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
