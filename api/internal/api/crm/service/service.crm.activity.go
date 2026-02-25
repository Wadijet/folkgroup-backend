// Package crmvc - Service lịch sử hoạt động CRM (crm_activity_history).
package crmvc

import (
	"context"
	"fmt"
	"time"

	crmmodels "meta_commerce/internal/api/crm/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CrmActivityService xử lý lịch sử hoạt động khách.
type CrmActivityService struct {
	*basesvc.BaseServiceMongoImpl[crmmodels.CrmActivityHistory]
}

// NewCrmActivityService tạo CrmActivityService mới.
func NewCrmActivityService() (*CrmActivityService, error) {
	coll, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.CrmActivityHistory)
	if !exist {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.CrmActivityHistory, common.ErrNotFound)
	}
	return &CrmActivityService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[crmmodels.CrmActivityHistory](coll),
	}, nil
}

// LogActivity ghi hoạt động mới.
func (s *CrmActivityService) LogActivity(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID, activityType, source string, sourceRef, metadata map[string]interface{}) error {
	now := time.Now().UnixMilli()
	doc := crmmodels.CrmActivityHistory{
		UnifiedId:           unifiedId,
		OwnerOrganizationID: ownerOrgID,
		ActivityType:       activityType,
		ActivityAt:         now,
		Source:             source,
		SourceRef:          sourceRef,
		Metadata:           metadata,
		CreatedAt:          now,
	}
	_, err := s.InsertOne(ctx, doc)
	return err
}

// LogActivityIfNotExists ghi hoạt động nếu chưa tồn tại (idempotent cho backfill).
// Kiểm tra (unifiedId, ownerOrgID, activityType, sourceRef) trước khi insert.
func (s *CrmActivityService) LogActivityIfNotExists(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID, activityType, source string, sourceRef, metadata map[string]interface{}) (inserted bool, err error) {
	filter := bson.M{
		"unifiedId":            unifiedId,
		"ownerOrganizationId": ownerOrgID,
		"activityType":        activityType,
	}
	if len(sourceRef) > 0 {
		for k, v := range sourceRef {
			filter["sourceRef."+k] = v
		}
	}
	exists, err := s.DocumentExists(ctx, filter)
	if err != nil || exists {
		return false, err
	}
	return true, s.LogActivity(ctx, unifiedId, ownerOrgID, activityType, source, sourceRef, metadata)
}

// FindByUnifiedId trả về danh sách hoạt động của khách (mới nhất trước).
func (s *CrmActivityService) FindByUnifiedId(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID, limit int) ([]crmmodels.CrmActivityHistory, error) {
	if limit <= 0 {
		limit = 50
	}
	filter := bson.M{
		"unifiedId":            unifiedId,
		"ownerOrganizationId": ownerOrgID,
	}
	opts := mongoopts.Find().SetLimit(int64(limit)).SetSort(bson.D{{Key: "activityAt", Value: -1}})
	return s.Find(ctx, filter, opts)
}
