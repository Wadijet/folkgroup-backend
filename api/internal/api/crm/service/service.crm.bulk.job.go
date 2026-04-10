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
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CustomerBulkJobs)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.CustomerBulkJobs, common.ErrNotFound)
	}
	return &CrmBulkJobService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[crmmodels.CrmBulkJob](coll),
	}, nil
}

// Enqueue thêm job vào queue crm_bulk_jobs.
// Trả về jobID và error.
// isPriority: true = job ưu tiên, bắt buộc chạy ngay không bị throttle.
func (s *CrmBulkJobService) Enqueue(ctx context.Context, jobType string, ownerOrgID primitive.ObjectID, params bson.M, isPriority bool) (primitive.ObjectID, error) {
	if params == nil {
		params = bson.M{}
	}
	now := time.Now().Unix()
	doc := &crmmodels.CrmBulkJob{
		JobType:             jobType,
		OwnerOrganizationID: ownerOrgID,
		Params:              params,
		IsPriority:          isPriority,
		CreatedAt:           now,
	}
	inserted, err := s.InsertOne(ctx, *doc)
	if err != nil {
		return primitive.NilObjectID, err
	}
	return inserted.ID, nil
}

// GetUnprocessed lấy tối đa limit job chưa xử lý, sort theo isPriority desc (ưu tiên trước), createdAt asc.
func (s *CrmBulkJobService) GetUnprocessed(ctx context.Context, limit int) ([]crmmodels.CrmBulkJob, error) {
	if limit <= 0 {
		limit = 5
	}
	filter := bson.M{"processedAt": nil}
	opts := mongoopts.Find().
		SetSort(bson.D{{Key: "isPriority", Value: -1}, {Key: "createdAt", Value: 1}}).
		SetLimit(int64(limit))
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

// UpdateProgress cập nhật tiến độ job (để resume khi restart). Không ghi processedAt.
func (s *CrmBulkJobService) UpdateProgress(ctx context.Context, id primitive.ObjectID, progress bson.M) error {
	if progress == nil {
		return nil
	}
	_, err := s.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"progress": progress}}, nil)
	return err
}

// Retry đưa job về trạng thái chờ xử lý (processedAt=null, processError="") và tùy chọn đặt isPriority.
// Dùng cho retry job đã lỗi hoặc muốn chạy lại ngay với ưu tiên.
func (s *CrmBulkJobService) Retry(ctx context.Context, id primitive.ObjectID, isPriority *bool) error {
	update := bson.M{
		"$unset": bson.M{"processedAt": "", "processError": "", "result": ""},
	}
	if isPriority != nil {
		update["$set"] = bson.M{"isPriority": *isPriority}
	}
	_, err := s.UpdateOne(ctx, bson.M{"_id": id}, update, nil)
	return err
}

// EnqueueRecalculateAllBatches tạo N job recalculate_batch thay vì 1 job recalculate_all.
// batchSize: số khách mỗi batch (mặc định 200). Trả về danh sách jobIds và error.
func (s *CrmBulkJobService) EnqueueRecalculateAllBatches(ctx context.Context, ownerOrgID primitive.ObjectID, batchSize int, isPriority bool) ([]primitive.ObjectID, error) {
	if batchSize <= 0 {
		batchSize = 200
	}
	customerSvc, err := NewCrmCustomerService()
	if err != nil {
		return nil, err
	}
	total, err := customerSvc.CountDocuments(ctx, bson.M{"ownerOrganizationId": ownerOrgID})
	if err != nil {
		return nil, err
	}
	if total == 0 {
		return []primitive.ObjectID{}, nil
	}
	n := (total + int64(batchSize) - 1) / int64(batchSize)
	jobIds := make([]primitive.ObjectID, 0, n)
	now := time.Now().Unix()
	for i := int64(0); i < n; i++ {
		offset := int(i * int64(batchSize))
		params := bson.M{"offset": offset, "limit": batchSize}
		doc := &crmmodels.CrmBulkJob{
			JobType:             crmmodels.CrmBulkJobRecalculateBatch,
			OwnerOrganizationID: ownerOrgID,
			Params:              params,
			IsPriority:          isPriority,
			CreatedAt:           now,
		}
		inserted, err := s.InsertOne(ctx, *doc)
		if err != nil {
			return jobIds, err
		}
		jobIds = append(jobIds, inserted.ID)
	}
	return jobIds, nil
}
