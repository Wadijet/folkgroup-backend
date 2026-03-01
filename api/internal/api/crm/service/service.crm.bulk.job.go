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

// EnqueueCrmBulkJob thêm job vào queue crm_bulk_jobs.
// Trả về jobID và error.
func EnqueueCrmBulkJob(ctx context.Context, jobType string, ownerOrgID primitive.ObjectID, params bson.M) (primitive.ObjectID, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CrmBulkJobs)
	if !ok {
		return primitive.NilObjectID, fmt.Errorf("không tìm thấy collection %s", global.MongoDB_ColNames.CrmBulkJobs)
	}
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
	result, err := coll.InsertOne(ctx, doc)
	if err != nil {
		return primitive.NilObjectID, err
	}
	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		return oid, nil
	}
	return primitive.NilObjectID, fmt.Errorf("không lấy được InsertedID")
}

// GetUnprocessedCrmBulkJobs lấy tối đa limit job chưa xử lý, sort theo createdAt asc.
func GetUnprocessedCrmBulkJobs(ctx context.Context, limit int) ([]crmmodels.CrmBulkJob, error) {
	if limit <= 0 {
		limit = 5
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CrmBulkJobs)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s", global.MongoDB_ColNames.CrmBulkJobs)
	}
	filter := bson.M{"processedAt": nil}
	opts := mongoopts.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}).SetLimit(int64(limit))
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var list []crmmodels.CrmBulkJob
	if err := cursor.All(ctx, &list); err != nil {
		return nil, err
	}
	if list == nil {
		list = []crmmodels.CrmBulkJob{}
	}
	return list, nil
}

// SetCrmBulkJobProcessed đánh dấu job đã xử lý (thành công hoặc lỗi).
func SetCrmBulkJobProcessed(ctx context.Context, id primitive.ObjectID, processErr string, result bson.M) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CrmBulkJobs)
	if !ok {
		return fmt.Errorf("không tìm thấy collection %s", global.MongoDB_ColNames.CrmBulkJobs)
	}
	now := time.Now().Unix()
	setDoc := bson.M{"processedAt": now, "processError": processErr}
	if result != nil {
		setDoc["result"] = result
	}
	_, err := coll.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": setDoc})
	return err
}
