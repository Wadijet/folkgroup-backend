// Package bridge — Implementation Storage cho pkg/approval (MongoDB).
package bridge

import (
	"context"
	"fmt"
	"time"

	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"

	pkgapproval "meta_commerce/pkg/approval"
)

// MongoStorage implement pkg/approval.Storage.
type MongoStorage struct{}

// NewMongoStorage tạo storage dùng collection action_pending_approval.
func NewMongoStorage() *MongoStorage {
	return &MongoStorage{}
}

// Insert thêm document, gán doc.ID sau khi insert.
func (s *MongoStorage) Insert(ctx context.Context, doc *pkgapproval.ActionPending) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ActionPendingApproval)
	if !ok {
		return fmt.Errorf("không tìm thấy collection action_pending_approval")
	}
	res, err := coll.InsertOne(ctx, doc)
	if err != nil {
		return err
	}
	doc.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

// Update cập nhật document theo _id.
func (s *MongoStorage) Update(ctx context.Context, doc *pkgapproval.ActionPending) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ActionPendingApproval)
	if !ok {
		return fmt.Errorf("không tìm thấy collection action_pending_approval")
	}
	set := bson.M{
		"status":          doc.Status,
		"approvedAt":      doc.ApprovedAt,
		"rejectedAt":      doc.RejectedAt,
		"rejectedBy":      doc.RejectedBy,
		"decisionNote":    doc.DecisionNote,
		"executedAt":      doc.ExecutedAt,
		"executeResponse": doc.ExecuteResponse,
		"executeError":    doc.ExecuteError,
		"retryCount":      doc.RetryCount,
		"maxRetries":      doc.MaxRetries,
		"updatedAt":       doc.UpdatedAt,
	}
	if doc.NextRetryAt != nil {
		set["nextRetryAt"] = *doc.NextRetryAt
	} else {
		set["nextRetryAt"] = nil
	}
	_, err := coll.UpdateOne(ctx, bson.M{"_id": doc.ID}, bson.M{"$set": set})
	return err
}

// FindById tìm theo _id và ownerOrganizationId.
func (s *MongoStorage) FindById(ctx context.Context, id primitive.ObjectID, ownerOrgID primitive.ObjectID) (*pkgapproval.ActionPending, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ActionPendingApproval)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection action_pending_approval")
	}
	var doc pkgapproval.ActionPending
	err := coll.FindOne(ctx, bson.M{"_id": id, "ownerOrganizationId": ownerOrgID}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("không tìm thấy đề xuất")
		}
		return nil, err
	}
	return &doc, nil
}

// FindPending danh sách pending theo ownerOrgID, domain (rỗng = tất cả), limit.
func (s *MongoStorage) FindPending(ctx context.Context, ownerOrgID primitive.ObjectID, domain string, limit int) ([]pkgapproval.ActionPending, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ActionPendingApproval)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection action_pending_approval")
	}
	if limit <= 0 {
		limit = 50
	}
	filter := bson.M{"ownerOrganizationId": ownerOrgID, "status": pkgapproval.StatusPending}
	if domain != "" {
		filter["domain"] = domain
	}
	opts := mongoopts.Find().SetSort(bson.D{{Key: "proposedAt", Value: -1}}).SetLimit(int64(limit))
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var out []pkgapproval.ActionPending
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []pkgapproval.ActionPending{}
	}
	return out, nil
}

// FindQueued danh sách item status=queued để worker xử lý.
// Filter: status=queued, domain, và (nextRetryAt null hoặc nextRetryAt <= now).
func (s *MongoStorage) FindQueued(ctx context.Context, domain string, limit int) ([]pkgapproval.ActionPending, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ActionPendingApproval)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection action_pending_approval")
	}
	if limit <= 0 {
		limit = 20
	}
	nowSec := time.Now().Unix()
	filter := bson.M{
		"domain": domain,
		"status": pkgapproval.StatusQueued,
		"$or": []bson.M{
			{"nextRetryAt": nil},
			{"nextRetryAt": bson.M{"$lte": nowSec}},
		},
	}
	opts := mongoopts.Find().SetSort(bson.D{{Key: "approvedAt", Value: 1}}).SetLimit(int64(limit))
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var out []pkgapproval.ActionPending
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []pkgapproval.ActionPending{}
	}
	return out, nil
}

// Find danh sách với filter (domain, status, limit, sort) — phục vụ frontend xem.
func (s *MongoStorage) Find(ctx context.Context, ownerOrgID primitive.ObjectID, filter pkgapproval.FindFilter) ([]pkgapproval.ActionPending, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ActionPendingApproval)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection action_pending_approval")
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.SortField == "" {
		filter.SortField = "proposedAt"
	}
	if filter.SortOrder == 0 {
		filter.SortOrder = -1
	}
	f := bson.M{"ownerOrganizationId": ownerOrgID}
	if filter.Domain != "" {
		f["domain"] = filter.Domain
	}
	if filter.Status != "" {
		f["status"] = filter.Status
	}
	if filter.FromProposedAt > 0 || filter.ToProposedAt > 0 {
		proposedAt := bson.M{}
		if filter.FromProposedAt > 0 {
			proposedAt["$gte"] = filter.FromProposedAt
		}
		if filter.ToProposedAt > 0 {
			proposedAt["$lte"] = filter.ToProposedAt
		}
		f["proposedAt"] = proposedAt
	}
	order := 1
	if filter.SortOrder < 0 {
		order = -1
	}
	opts := mongoopts.Find().SetSort(bson.D{{Key: filter.SortField, Value: order}}).SetLimit(int64(filter.Limit))
	cursor, err := coll.Find(ctx, f, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var out []pkgapproval.ActionPending
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []pkgapproval.ActionPending{}
	}
	return out, nil
}

// FindWithPagination danh sách có phân trang — trả items, total.
func (s *MongoStorage) FindWithPagination(ctx context.Context, ownerOrgID primitive.ObjectID, filter pkgapproval.FindWithPaginationFilter) ([]pkgapproval.ActionPending, int64, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ActionPendingApproval)
	if !ok {
		return nil, 0, fmt.Errorf("không tìm thấy collection action_pending_approval")
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.SortField == "" {
		filter.SortField = "proposedAt"
	}
	if filter.SortOrder == 0 {
		filter.SortOrder = -1
	}
	f := bson.M{"ownerOrganizationId": ownerOrgID}
	if filter.Domain != "" {
		f["domain"] = filter.Domain
	}
	if filter.Status != "" {
		f["status"] = filter.Status
	}
	if filter.FromProposedAt > 0 || filter.ToProposedAt > 0 {
		proposedAt := bson.M{}
		if filter.FromProposedAt > 0 {
			proposedAt["$gte"] = filter.FromProposedAt
		}
		if filter.ToProposedAt > 0 {
			proposedAt["$lte"] = filter.ToProposedAt
		}
		f["proposedAt"] = proposedAt
	}
	total, err := coll.CountDocuments(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	order := 1
	if filter.SortOrder < 0 {
		order = -1
	}
	skip := (filter.Page - 1) * int64(filter.Limit)
	opts := mongoopts.Find().SetSort(bson.D{{Key: filter.SortField, Value: order}}).SetSkip(skip).SetLimit(int64(filter.Limit))
	cursor, err := coll.Find(ctx, f, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)
	var out []pkgapproval.ActionPending
	if err := cursor.All(ctx, &out); err != nil {
		return nil, 0, err
	}
	if out == nil {
		out = []pkgapproval.ActionPending{}
	}
	return out, total, nil
}

// Count đếm theo filter.
func (s *MongoStorage) Count(ctx context.Context, ownerOrgID primitive.ObjectID, domain, status string, fromProposedAt, toProposedAt int64) (int64, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ActionPendingApproval)
	if !ok {
		return 0, fmt.Errorf("không tìm thấy collection action_pending_approval")
	}
	f := bson.M{"ownerOrganizationId": ownerOrgID}
	if domain != "" {
		f["domain"] = domain
	}
	if status != "" {
		f["status"] = status
	}
	if fromProposedAt > 0 || toProposedAt > 0 {
		proposedAt := bson.M{}
		if fromProposedAt > 0 {
			proposedAt["$gte"] = fromProposedAt
		}
		if toProposedAt > 0 {
			proposedAt["$lte"] = toProposedAt
		}
		f["proposedAt"] = proposedAt
	}
	return coll.CountDocuments(ctx, f)
}
