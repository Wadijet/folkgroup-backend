// Package adssvc — Wrapper gọi approval.Propose cho domain ads.
package adssvc

import (
	"context"

	"meta_commerce/internal/approval"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const domainAds = "ads"
const eventTypeActionPendingApproval = "ads_action_pending_approval"

// Propose thêm đề xuất ads vào queue (gọi approval package).
func Propose(ctx context.Context, input *ProposeInput, ownerOrgID primitive.ObjectID, baseURL string) (*approval.ActionPending, error) {
	payload := map[string]interface{}{
		"adAccountId":  input.AdAccountId,
		"campaignId":  input.CampaignId,
		"campaignName": input.CampaignName,
		"adSetId":     input.AdSetId,
		"adId":        input.AdId,
		"value":       input.Value,
	}
	if input.Payload != nil {
		for k, v := range input.Payload {
			payload[k] = v
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
	Payload      map[string]interface{}
}
