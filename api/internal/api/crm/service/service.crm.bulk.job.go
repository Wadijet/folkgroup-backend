// Package crmvc - Service cho crm_bulk_jobs: queue sync, backfill, recalculate cho worker.
package crmvc

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"

	crmmodels "meta_commerce/internal/api/crm/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
)

// CrmBulkJobService service CRUD cho crm_bulk_jobs (dùng cho API đọc queue).
type CrmBulkJobService struct {
	*basesvc.BaseServiceMongoImpl[crmmodels.CrmBulkJob]
}

// NewCrmBulkJobService tạo service CRUD cho crm_bulk_jobs.
func NewCrmBulkJobService() (*CrmBulkJobService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CrmBulkJobs)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.CrmBulkJobs, common.ErrNotFound)
	}
	return &CrmBulkJobService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[crmmodels.CrmBulkJob](coll),
	}, nil
}

// Enqueue thêm job vào queue crm_bulk_jobs.
// Trả về jobID và error.
func (s *CrmBulkJobService) Enqueue(ctx context.Context, jobType string, ownerOrgID primitive.ObjectID, params bson.M) (primitive.ObjectID, error) {
	if params == nil {
		params = bson.M{}
	}
	now := time.Now().Unix()
	doc := &crmmodels.CrmBulkJob{
		JobType:             jobType,
		OwnerOrganizationID: ownerOrgID,
		Params:              params,
		CreatedAt:           now,
	}
	inserted, err := s.InsertOne(ctx, *doc)
	if err != nil {
		return primitive.NilObjectID, err
	}
	return inserted.ID, nil
}

// GetUnprocessed lấy tối đa limit job chưa xử lý, sort theo createdAt asc.
func (s *CrmBulkJobService) GetUnprocessed(ctx context.Context, limit int) ([]crmmodels.CrmBulkJob, error) {
	if limit <= 0 {
		limit = 5
	}
	filter := bson.M{"processedAt": nil}
	opts := mongoopts.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}).SetLimit(int64(limit))
	list, err := s.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []crmmodels.CrmBulkJob{}
	}
	return list, nil
}

// SetProcessed đánh dấu job đã xử lý (thành công hoặc lỗi).
func (s *CrmBulkJobService) SetProcessed(ctx context.Context, id primitive.ObjectID, processErr string, result bson.M) error {
	now := time.Now().Unix()
	update := bson.M{"processedAt": now, "processError": processErr}
	if result != nil {
		update["result"] = result
	}
	_, err := s.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update}, nil)
	return err
}
