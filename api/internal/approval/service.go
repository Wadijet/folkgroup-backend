// Package approval — Bridge: delegate sang pkg/approval engine.
package approval

import (
	"context"

	pkgapproval "meta_commerce/pkg/approval"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ProposeInput input cho Propose (giữ tương thích với callers).
type ProposeInput struct {
	ActionType       string
	Reason           string
	Payload          map[string]interface{}
	EventTypePending string
	ApprovePath      string
	RejectPath       string
}

// Propose thêm đề xuất vào queue. Delegate sang engine.
func Propose(ctx context.Context, domain string, input ProposeInput, ownerOrgID primitive.ObjectID, baseURL string) (*pkgapproval.ActionPending, error) {
	Init()
	return GetEngine().Propose(ctx, domain, pkgapproval.ProposeInput{
		ActionType:       input.ActionType,
		Reason:           input.Reason,
		Payload:          input.Payload,
		EventTypePending: input.EventTypePending,
		ApprovePath:      input.ApprovePath,
		RejectPath:       input.RejectPath,
	}, ownerOrgID, baseURL)
}

// Approve duyệt đề xuất. Delegate sang engine.
func Approve(ctx context.Context, actionId string, ownerOrgID primitive.ObjectID) (*pkgapproval.ActionPending, error) {
	Init()
	return GetEngine().Approve(ctx, actionId, ownerOrgID)
}

// Reject từ chối đề xuất. Delegate sang engine.
func Reject(ctx context.Context, actionId string, ownerOrgID primitive.ObjectID, decisionNote, rejectedBy string) (*pkgapproval.ActionPending, error) {
	Init()
	return GetEngine().Reject(ctx, actionId, ownerOrgID, decisionNote, rejectedBy)
}

// ListPending danh sách đề xuất chờ duyệt. Delegate sang engine.
func ListPending(ctx context.Context, ownerOrgID primitive.ObjectID, domain string, limit int) ([]pkgapproval.ActionPending, error) {
	Init()
	return GetEngine().ListPending(ctx, ownerOrgID, domain, limit)
}

// ActionPending re-export từ pkg/approval để callers không cần import pkg.
type ActionPending = pkgapproval.ActionPending
