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

// Execute thực thi thủ công đề xuất đã duyệt (status=queued). Dùng cho test — user trigger thay vì chờ worker.
func Execute(ctx context.Context, actionId string, ownerOrgID primitive.ObjectID) (*pkgapproval.ActionPending, error) {
	Init()
	return GetEngine().ExecuteOne(ctx, actionId, ownerOrgID)
}

// ListPending danh sách đề xuất chờ duyệt. Delegate sang engine.
func ListPending(ctx context.Context, ownerOrgID primitive.ObjectID, domain string, limit int) ([]pkgapproval.ActionPending, error) {
	Init()
	return GetEngine().ListPending(ctx, ownerOrgID, domain, limit)
}

// FindById xem chi tiết một đề xuất theo id — phục vụ frontend.
func FindById(ctx context.Context, actionId string, ownerOrgID primitive.ObjectID) (*pkgapproval.ActionPending, error) {
	Init()
	return GetEngine().FindById(ctx, actionId, ownerOrgID)
}

// Find danh sách với filter (domain, status, limit, sort) — phục vụ frontend xem.
func Find(ctx context.Context, ownerOrgID primitive.ObjectID, filter pkgapproval.FindFilter) ([]pkgapproval.ActionPending, error) {
	Init()
	return GetEngine().Find(ctx, ownerOrgID, filter)
}

// FindWithPagination danh sách có phân trang — phục vụ frontend table.
func FindWithPagination(ctx context.Context, ownerOrgID primitive.ObjectID, filter pkgapproval.FindWithPaginationFilter) ([]pkgapproval.ActionPending, int64, error) {
	Init()
	return GetEngine().FindWithPagination(ctx, ownerOrgID, filter)
}

// Count đếm theo filter — phục vụ dashboard badges.
func Count(ctx context.Context, ownerOrgID primitive.ObjectID, domain, status string, fromProposedAt, toProposedAt int64) (int64, error) {
	Init()
	return GetEngine().Count(ctx, ownerOrgID, domain, status, fromProposedAt, toProposedAt)
}

// Cancel hủy đề xuất pending — chỉ cho phép khi status=pending.
func Cancel(ctx context.Context, actionId string, ownerOrgID primitive.ObjectID) (*pkgapproval.ActionPending, error) {
	Init()
	return GetEngine().Cancel(ctx, actionId, ownerOrgID)
}

// FindQueued danh sách item status=queued để worker xử lý (domain ads).
func FindQueued(ctx context.Context, domain string, limit int) ([]pkgapproval.ActionPending, error) {
	Init()
	return GetEngine().FindQueued(ctx, domain, limit)
}

// Update cập nhật document (worker dùng sau khi execute/retry).
func Update(ctx context.Context, doc *pkgapproval.ActionPending) error {
	Init()
	return GetEngine().Update(ctx, doc)
}

// NotifyExecuted gửi thông báo executed (worker gọi sau khi thực thi thành công).
func NotifyExecuted(ctx context.Context, doc *pkgapproval.ActionPending) {
	Init()
	GetEngine().NotifyExecuted(ctx, doc)
}

// NotifyFailed gửi thông báo khi thực thi thất bại sau hết retry (worker gọi).
func NotifyFailed(ctx context.Context, doc *pkgapproval.ActionPending) {
	Init()
	GetEngine().NotifyFailed(ctx, doc)
}

// ActionPending re-export từ pkg/approval để callers không cần import pkg.
type ActionPending = pkgapproval.ActionPending

// FindFilter re-export từ pkg/approval.
type FindFilter = pkgapproval.FindFilter

// FindWithPaginationFilter re-export từ pkg/approval.
type FindWithPaginationFilter = pkgapproval.FindWithPaginationFilter
