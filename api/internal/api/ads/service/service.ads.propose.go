// Package adssvc — Wrapper gọi approval.Propose cho domain ads.
package adssvc

import (
	"context"
	"fmt"

	"meta_commerce/internal/approval"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

const domainAds = "ads"
const eventTypeActionPendingApproval = "ads_action_pending_approval"

// supportedActions — danh sách action Meta API hỗ trợ.
var supportedActions = map[string]bool{
	"KILL": true, "PAUSE": true, "RESUME": true, "ARCHIVE": true, "DELETE": true,
	"SET_BUDGET": true, "SET_LIFETIME_BUDGET": true, "INCREASE": true, "DECREASE": true, "SET_NAME": true,
}

// budgetActions — các action cần adSetId hoặc campaignId (Ad không có budget).
var budgetActions = map[string]bool{
	"SET_BUDGET": true, "SET_LIFETIME_BUDGET": true, "INCREASE": true, "DECREASE": true,
}

// GetMetricsPayloadForPropose lấy currentMetrics từ campaign và format thành payload (rawSummary, layer1Summary, layer3Summary, flagsSummary, flagsDetail).
// Dùng khi propose thủ công có campaignId — bổ sung căn cứ tạo đề xuất vào notification.
func GetMetricsPayloadForPropose(ctx context.Context, campaignId string, adAccountId string, ownerOrgID primitive.ObjectID) map[string]interface{} {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return nil
	}
	var doc struct {
		CurrentMetrics map[string]interface{} `bson:"currentMetrics"`
		AdAccountId    string                `bson:"adAccountId"`
	}
	err := coll.FindOne(ctx, bson.M{
		"campaignId":          campaignId,
		"ownerOrganizationId": ownerOrgID,
	}, mongoopts.FindOne().SetProjection(bson.M{"currentMetrics": 1, "adAccountId": 1})).Decode(&doc)
	if err != nil {
		return nil
	}
	accId := adAccountId
	if accId == "" {
		accId = doc.AdAccountId
	}
	cfg, _ := GetCampaignConfig(ctx, accId, ownerOrgID)
	summaries := FormatMetricsForNotificationWithConfig(ctx, doc.CurrentMetrics, cfg)
	payload := make(map[string]interface{})
	for k, v := range summaries {
		payload[k] = v
	}
	return payload
}

// Propose thêm đề xuất ads vào queue (gọi approval package).
// Hỗ trợ campaign, adset, ad — cần ít nhất một trong campaignId, adSetId, adId.
// Budget actions (SET_BUDGET, SET_LIFETIME_BUDGET, INCREASE, DECREASE) bắt buộc có adSetId hoặc campaignId.
func Propose(ctx context.Context, input *ProposeInput, ownerOrgID primitive.ObjectID, baseURL string) (*approval.ActionPending, error) {
	if input.Reason == "" {
		return nil, fmt.Errorf("lý do (reason) không được để trống")
	}
	if !supportedActions[input.ActionType] {
		return nil, fmt.Errorf("actionType không hỗ trợ: %s", input.ActionType)
	}
	if input.CampaignId == "" && input.AdSetId == "" && input.AdId == "" {
		return nil, fmt.Errorf("cần ít nhất một trong campaignId, adSetId, adId")
	}
	// Budget actions chỉ áp dụng cho campaign/adset — Meta API không hỗ trợ budget ở level Ad.
	if budgetActions[input.ActionType] && input.AdSetId == "" && input.CampaignId == "" {
		return nil, fmt.Errorf("action %s cần adSetId hoặc campaignId (Ad không có budget)", input.ActionType)
	}
	payload := map[string]interface{}{
		"adAccountId":  input.AdAccountId,
		"campaignId":   input.CampaignId,
		"campaignName": input.CampaignName,
		"adSetId":      input.AdSetId,
		"adId":         input.AdId,
		"value":        input.Value,
		"reason":       input.Reason,
		"ruleCode":     input.RuleCode, // Mã rule (sl_a, mess_trap_suspect, ...) — dùng cho auto-approve
	}
	if input.Payload != nil {
		for k, v := range input.Payload {
			payload[k] = v
		}
	}
	// Bổ sung metrics (raw, layer, flags) khi có campaignId và chưa có — làm căn cứ tạo đề xuất trong notification
	if input.CampaignId != "" && payload["flagsSummary"] == nil {
		if metrics := GetMetricsPayloadForPropose(ctx, input.CampaignId, input.AdAccountId, ownerOrgID); metrics != nil {
			for k, v := range metrics {
				payload[k] = v
			}
		}
	}
	// Đảm bảo template luôn có đủ biến — nếu thiếu thì dùng chuỗi rỗng để tránh hiển thị {{placeholder}}
	for _, key := range []string{"flagsSummary", "rawSummary", "layer1Summary", "layer3Summary", "flagsDetail"} {
		if payload[key] == nil {
			payload[key] = ""
		}
	}
	return approval.Propose(ctx, domainAds, approval.ProposeInput{
		ActionType:       input.ActionType,
		Reason:           input.Reason,
		Payload:          payload,
		EventTypePending: eventTypeActionPendingApproval,
		ApprovePath:      "/api/v1/approval/actions/approve",
		RejectPath:       "/api/v1/approval/actions/reject",
	}, ownerOrgID, baseURL)
}

// ProposeInput input cho Propose (ads-specific).
type ProposeInput struct {
	ActionType   string
	AdAccountId  string
	CampaignId   string
	CampaignName string
	AdSetId      string
	AdId         string
	Value        interface{}
	Reason       string
	RuleCode     string            // Mã rule (sl_a, mess_trap_suspect, ...) — dùng cho automation config
	Payload      map[string]interface{}
}
