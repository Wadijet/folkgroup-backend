// Package bridge — Implementation Storage cho pkg/approval (MongoDB).
package bridge

import (
	"context"
	"fmt"

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
	_, err := coll.UpdateOne(ctx, bson.M{"_id": doc.ID}, bson.M{
		"$set": bson.M{
			"status":          doc.Status,
			"approvedAt":      doc.ApprovedAt,
			"rejectedAt":     doc.RejectedAt,
			"rejectedBy":      doc.RejectedBy,
			"decisionNote":    doc.DecisionNote,
			"executedAt":      doc.ExecutedAt,
			"executeResponse": doc.ExecuteResponse,
			"executeError":    doc.ExecuteError,
			"updatedAt":      doc.UpdatedAt,
		},
	})
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
