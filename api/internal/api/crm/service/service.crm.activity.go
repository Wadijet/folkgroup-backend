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
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// LogActivityInput input cho LogActivity — đầy đủ thông tin audit.
// ActivityAt: thời điểm sự kiện xảy ra (Unix ms). Nếu 0 → dùng thời điểm ghi (now).
// Khi sự kiện từ nguồn khác (order, conversation, note) nên truyền timestamp của nguồn để timeline đúng.
type LogActivityInput struct {
	UnifiedId      string
	OwnerOrgID     primitive.ObjectID
	Domain         string
	ActivityType   string
	Source         string
	SourceRef      map[string]interface{}
	Metadata       map[string]interface{}
	DisplayLabel   string
	DisplayIcon    string
	DisplaySubtext string
	ActorId        *primitive.ObjectID
	ActorName      string
	Changes        []crmmodels.ActivityChangeItem
	Reason         string
	ClientIp       string
	UserAgent      string
	Status         string
	ActivityAt     int64 // Thời điểm sự kiện (từ nguồn). 0 = dùng now.
}

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

// LogActivity ghi hoạt động. Cùng nguồn (unifiedId + activityType + source + sourceRef) → upsert; khác → insert.
// ActivityAt: nếu input.ActivityAt > 0 dùng thời điểm nguồn; ngược lại dùng now.
func (s *CrmActivityService) LogActivity(ctx context.Context, input LogActivityInput) error {
	now := time.Now().UnixMilli()
	activityAt := input.ActivityAt
	if activityAt <= 0 {
		activityAt = now
	}
	domain := input.Domain
	if domain == "" {
		domain = crmmodels.ActivityTypeToDomain[input.ActivityType]
	}
	// Cùng nguồn = có sourceRef để xác định duy nhất → upsert
	if len(input.SourceRef) > 0 {
		err := s.logActivityUpsert(ctx, input, now, activityAt, domain)
		if err != nil {
			logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
				"unifiedId": input.UnifiedId, "activityType": input.ActivityType, "source": input.Source,
			}).Warn("[CRM] LogActivity upsert lỗi")
		}
		return err
	}
	// Không có sourceRef → insert mới (ActorId nil = omitempty, không lưu 000...000)
	doc := crmmodels.CrmActivityHistory{
		UnifiedId:           input.UnifiedId,
		OwnerOrganizationID: input.OwnerOrgID,
		Domain:              domain,
		ActivityType:        input.ActivityType,
		ActivityAt:          activityAt,
		Source:              input.Source,
		SourceRef:           input.SourceRef,
		Metadata:            input.Metadata,
		DisplayLabel:        input.DisplayLabel,
		DisplayIcon:         input.DisplayIcon,
		DisplaySubtext:     input.DisplaySubtext,
		ActorId:             input.ActorId, // nil → omitempty, không lưu
		ActorName:           input.ActorName,
		Changes:             input.Changes,
		Reason:              input.Reason,
		ClientIp:            input.ClientIp,
		UserAgent:           input.UserAgent,
		Status:              input.Status,
		CreatedAt:           now,
	}
	_, err := s.InsertOne(ctx, doc)
	if err != nil {
		logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
			"unifiedId": input.UnifiedId, "activityType": input.ActivityType, "source": input.Source,
		}).Warn("[CRM] LogActivity insert lỗi")
	}
	return err
}

// logActivityUpsert upsert khi cùng nguồn. Order: 1 đơn = 1 bản ghi (không dùng activityType trong filter).
func (s *CrmActivityService) logActivityUpsert(ctx context.Context, input LogActivityInput, now, activityAt int64, domain string) error {
	filter := bson.M{
		"unifiedId":            input.UnifiedId,
		"ownerOrganizationId": input.OwnerOrgID,
		"source":              input.Source,
	}
	for k, v := range input.SourceRef {
		filter["sourceRef."+k] = v
	}
	// Order: 1 đơn = 1 bản ghi, cập nhật activityType khi trạng thái đổi (created→completed→cancelled)
	if domain != crmmodels.ActivityDomainOrder {
		filter["activityType"] = input.ActivityType
	}
	setFields := bson.M{
		"activityAt":    activityAt,
		"activityType":  input.ActivityType, // Order: cập nhật khi trạng thái đổi
		"metadata":      input.Metadata,
		"displayLabel":  input.DisplayLabel,
		"displayIcon":   input.DisplayIcon,
		"displaySubtext": input.DisplaySubtext,
		"actorName":     input.ActorName,
		"changes":       input.Changes,
		"reason":        input.Reason,
		"clientIp":      input.ClientIp,
		"userAgent":     input.UserAgent,
		"status":        input.Status,
	}
	if input.ActorId != nil && !input.ActorId.IsZero() {
		setFields["actorId"] = *input.ActorId
	}
	update := bson.M{
		"$set":        setFields,
		"$setOnInsert": buildSetOnInsert(input, domain, now),
	}
	if input.ActorId == nil || input.ActorId.IsZero() {
		update["$unset"] = bson.M{"actorId": ""} // Xóa actorId cũ (tránh lưu 000...000)
	}
	opts := mongoopts.Update().SetUpsert(true)
	_, err := s.Collection().UpdateOne(ctx, filter, update, opts)
	return err
}

func buildSetOnInsert(input LogActivityInput, domain string, now int64) bson.M {
	// Không thêm activityType, source, sourceRef vào $setOnInsert — đã có trong $set hoặc gây conflict
	m := bson.M{
		"unifiedId":            input.UnifiedId,
		"ownerOrganizationId": input.OwnerOrgID,
		"domain":               domain,
		"source":               input.Source,
		"sourceRef":            input.SourceRef,
		"createdAt":            now,
	}
	if input.ActorId != nil && !input.ActorId.IsZero() {
		m["actorId"] = *input.ActorId
	}
	return m
}

// LogActivityLegacy ghi hoạt động (signature cũ — backward compatible cho backfill).
func (s *CrmActivityService) LogActivityLegacy(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID, activityType, source string, sourceRef, metadata map[string]interface{}) error {
	return s.LogActivity(ctx, LogActivityInput{
		UnifiedId:    unifiedId,
		OwnerOrgID:   ownerOrgID,
		ActivityType: activityType,
		Source:       source,
		SourceRef:    sourceRef,
		Metadata:     metadata,
	})
}

// LogActivityIfNotExists ghi hoạt động nếu chưa tồn tại. Có sourceRef → LogActivity đã upsert, gọi trực tiếp.
func (s *CrmActivityService) LogActivityIfNotExists(ctx context.Context, input LogActivityInput) (inserted bool, err error) {
	err = s.LogActivity(ctx, input)
	return err == nil, err
}

// LogActivityIfNotExistsLegacy signature cũ cho backfill.
func (s *CrmActivityService) LogActivityIfNotExistsLegacy(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID, activityType, source string, sourceRef, metadata map[string]interface{}) (inserted bool, err error) {
	return s.LogActivityIfNotExists(ctx, LogActivityInput{
		UnifiedId:    unifiedId,
		OwnerOrgID:   ownerOrgID,
		ActivityType: activityType,
		Source:       source,
		SourceRef:    sourceRef,
		Metadata:     metadata,
	})
}

// FindByUnifiedId trả về danh sách hoạt động của khách (mới nhất trước).
// domains: lọc theo domain (rỗng = tất cả). limit: số mục (mặc định 50).
func (s *CrmActivityService) FindByUnifiedId(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID, domains []string, limit int) ([]crmmodels.CrmActivityHistory, error) {
	if limit <= 0 {
		limit = 50
	}
	filter := bson.M{
		"unifiedId":            unifiedId,
		"ownerOrganizationId": ownerOrgID,
	}
	if len(domains) > 0 {
		filter["domain"] = bson.M{"$in": domains}
	}
	opts := mongoopts.Find().SetLimit(int64(limit)).SetSort(bson.D{{Key: "activityAt", Value: -1}})
	return s.Find(ctx, filter, opts)
}

// GetLastSnapshotForCustomer lấy profileSnapshot và metricsSnapshot từ activity gần nhất có snapshot.
// Dùng để so sánh và chỉ lưu snapshot mới khi có thay đổi.
// excludeSource + excludeSourceRef: loại trừ activity khớp (vd: order đang upsert, conversation đang upsert)
// để so sánh với snapshot TRƯỚC sự kiện này — tránh metrics không thay đổi khi order_completed so với order_created.
func (s *CrmActivityService) GetLastSnapshotForCustomer(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID, excludeSource string, excludeSourceRef map[string]interface{}) (profile, metrics map[string]interface{}, ok bool) {
	filter := bson.M{
		"unifiedId": unifiedId,
		"ownerOrganizationId": ownerOrgID,
		"$or": []bson.M{
			{"metadata.profileSnapshot": bson.M{"$exists": true}},
			{"metadata.metricsSnapshot": bson.M{"$exists": true}},
		},
	}
	if excludeSource != "" && len(excludeSourceRef) > 0 {
		excludeMatch := bson.M{"source": excludeSource}
		for k, v := range excludeSourceRef {
			excludeMatch["sourceRef."+k] = v
		}
		filter["$nor"] = []bson.M{excludeMatch}
	}
	opts := mongoopts.FindOne().SetSort(bson.D{{Key: "activityAt", Value: -1}})
	var doc crmmodels.CrmActivityHistory
	if err := s.Collection().FindOne(ctx, filter, opts).Decode(&doc); err != nil {
		return nil, nil, false
	}
	if doc.Metadata == nil {
		return nil, nil, false
	}
	if p, ok := doc.Metadata["profileSnapshot"].(map[string]interface{}); ok {
		profile = p
	}
	if m, ok := doc.Metadata["metricsSnapshot"].(map[string]interface{}); ok {
		metrics = m
	}
	return profile, metrics, profile != nil || metrics != nil
}

// FindActivitiesNewerThan lấy các activity có activityAt > afterActivityAt, sắp xếp theo activityAt tăng dần (cũ trước).
// Dùng cho recalc snapshot khi insert activity cũ hơn — cần tính lại snapshot của các activity mới hơn.
func (s *CrmActivityService) FindActivitiesNewerThan(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID, afterActivityAt int64) ([]crmmodels.CrmActivityHistory, error) {
	filter := bson.M{
		"unifiedId":           unifiedId,
		"ownerOrganizationId": ownerOrgID,
		"activityAt":          bson.M{"$gt": afterActivityAt},
	}
	opts := mongoopts.Find().SetSort(bson.D{{Key: "activityAt", Value: 1}})
	return s.Find(ctx, filter, opts)
}

// GetLastSnapshotBeforeActivityAt lấy profileSnapshot và metricsSnapshot từ activity gần nhất có activityAt < beforeActivityAt.
// Dùng cho recalc: cần "snapshot trước" khi tính lại snapshot của activity mới hơn.
func (s *CrmActivityService) GetLastSnapshotBeforeActivityAt(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID, beforeActivityAt int64) (profile, metrics map[string]interface{}, ok bool) {
	filter := bson.M{
		"unifiedId":           unifiedId,
		"ownerOrganizationId": ownerOrgID,
		"activityAt":          bson.M{"$lt": beforeActivityAt},
		"$or": []bson.M{
			{"metadata.profileSnapshot": bson.M{"$exists": true}},
			{"metadata.metricsSnapshot": bson.M{"$exists": true}},
		},
	}
	opts := mongoopts.FindOne().SetSort(bson.D{{Key: "activityAt", Value: -1}})
	var doc crmmodels.CrmActivityHistory
	if err := s.Collection().FindOne(ctx, filter, opts).Decode(&doc); err != nil {
		return nil, nil, false
	}
	if doc.Metadata == nil {
		return nil, nil, false
	}
	if p, ok := doc.Metadata["profileSnapshot"].(map[string]interface{}); ok {
		profile = p
	}
	if m, ok := doc.Metadata["metricsSnapshot"].(map[string]interface{}); ok {
		metrics = m
	}
	return profile, metrics, profile != nil || metrics != nil
}

// UpdateActivityMetadata cập nhật metadata của activity theo _id (dùng cho recalc snapshot).
func (s *CrmActivityService) UpdateActivityMetadata(ctx context.Context, activityID primitive.ObjectID, metadataUpdates map[string]interface{}) error {
	if len(metadataUpdates) == 0 {
		return nil
	}
	set := bson.M{}
	for k, v := range metadataUpdates {
		set["metadata."+k] = v
	}
	_, err := s.Collection().UpdateOne(ctx, bson.M{"_id": activityID}, bson.M{"$set": set})
	return err
}

// GetLastSnapshotPerCustomerBeforeEndMs lấy metricsSnapshot cuối cùng của mỗi khách trước endMs (dùng cho report).
// Aggregation: $match ownerOrgId, activityAt<=endMs, metadata.metricsSnapshot exists; $sort activityAt desc; $group by unifiedId $first.
// Trả về map[unifiedId]metricsSnapshot. Khách không có activity với snapshot không có trong map.
func (s *CrmActivityService) GetLastSnapshotPerCustomerBeforeEndMs(ctx context.Context, ownerOrgID primitive.ObjectID, endMs int64) (map[string]map[string]interface{}, error) {
	pipe := []bson.M{
		{"$match": bson.M{
			"ownerOrganizationId": ownerOrgID,
			"activityAt":          bson.M{"$lte": endMs},
			"metadata.metricsSnapshot": bson.M{"$exists": true},
		}},
		{"$sort": bson.M{"activityAt": -1}},
		{"$group": bson.M{
			"_id": "$unifiedId",
			"metricsSnapshot": bson.M{"$first": "$metadata.metricsSnapshot"},
		}},
	}
	cursor, err := s.Collection().Aggregate(ctx, pipe)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	result := make(map[string]map[string]interface{})
	for cursor.Next(ctx) {
		var doc struct {
			ID              string                 `bson:"_id"`
			MetricsSnapshot map[string]interface{} `bson:"metricsSnapshot"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		if doc.ID != "" && doc.MetricsSnapshot != nil {
			result[doc.ID] = doc.MetricsSnapshot
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, common.ConvertMongoError(err)
	}
	return result, nil
}

// GetExistingActivityBySourceRef lấy activity đã tồn tại theo source + sourceRef (dùng khi upsert).
// Trả về metadata của doc nếu tìm thấy; nil nếu không.
func (s *CrmActivityService) GetExistingActivityBySourceRef(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID, source string, sourceRef map[string]interface{}) *crmmodels.CrmActivityHistory {
	if len(sourceRef) == 0 {
		return nil
	}
	filter := bson.M{
		"unifiedId":            unifiedId,
		"ownerOrganizationId": ownerOrgID,
		"source":               source,
	}
	for k, v := range sourceRef {
		filter["sourceRef."+k] = v
	}
	var doc crmmodels.CrmActivityHistory
	if err := s.Collection().FindOne(ctx, filter, nil).Decode(&doc); err != nil {
		return nil
	}
	return &doc
}
