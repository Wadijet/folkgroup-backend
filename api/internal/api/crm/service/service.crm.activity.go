// Package crmvc - Service lịch sử hoạt động CRM (crm_activity_history).
package crmvc

import (
	"context"
	"fmt"
	"time"

	"meta_commerce/internal/common/activity"
	crmmodels "meta_commerce/internal/api/crm/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/api/events"
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
	Changes        []activity.ActivityChangeItem
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
	coll, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.CustomerActivityHistory)
	if !exist {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.CustomerActivityHistory, common.ErrNotFound)
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
	// Không có sourceRef → insert mới
	snapshot, metaChanges, metadataClean := splitSnapshotFromMetadata(input.Metadata)
	changes := input.Changes
	if len(changes) == 0 && len(metaChanges) > 0 {
		changes = metaChanges
	}
	doc := crmmodels.CrmActivityHistory{
		ActivityBase: activity.ActivityBase{
			UnifiedId:           input.UnifiedId,
			OwnerOrganizationID: input.OwnerOrgID,
			Domain:              domain,
			ActivityType:        input.ActivityType,
			ActivityAt:          activityAt,
			Source:              input.Source,
			SourceRef:           input.SourceRef,
			Actor:               activity.ToActor(input.ActorId, input.ActorName, activity.ActorTypeSystem),
			Display:             activity.Display{Label: input.DisplayLabel, Icon: input.DisplayIcon, Subtext: input.DisplaySubtext},
			Snapshot:            snapshot,
			Changes:             changes,
			Metadata:            mergeMetadata(metadataClean, input.Reason, input.ClientIp, input.UserAgent, input.Status),
			CreatedAt:           now,
		},
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
	snapshot, metaChanges, metadataClean := splitSnapshotFromMetadata(input.Metadata)
	changes := input.Changes
	if len(changes) == 0 && len(metaChanges) > 0 {
		changes = metaChanges
	}
	metadata := mergeMetadata(metadataClean, input.Reason, input.ClientIp, input.UserAgent, input.Status)
	actor := activity.ToActor(input.ActorId, input.ActorName, activity.ActorTypeSystem)
	display := activity.Display{Label: input.DisplayLabel, Icon: input.DisplayIcon, Subtext: input.DisplaySubtext}
	setFields := bson.M{
		"activityAt":   activityAt,
		"activityType": input.ActivityType,
		"metadata":     metadata,
		"display":      display,
		"actor":        actor,
		"snapshot":     snapshot,
		"changes":      changes,
	}
	update := bson.M{
		"$set":        setFields,
		"$setOnInsert": buildSetOnInsert(input, domain, now),
	}
	opts := mongoopts.Update().SetUpsert(true)
	_, err := s.Collection().UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return err
	}
	// Phát event để report hook MarkDirty báo cáo customer (customer_daily, customer_weekly, ...).
	// logActivityUpsert dùng Collection().UpdateOne trực tiếp nên không qua BaseServiceMongoImpl → không tự emit.
	doc := &crmmodels.CrmActivityHistory{
		ActivityBase: activity.ActivityBase{
			OwnerOrganizationID: input.OwnerOrgID,
			ActivityAt:          activityAt,
			CreatedAt:           now,
		},
	}
	events.EmitDataChanged(ctx, events.DataChangeEvent{
		CollectionName: global.MongoDB_ColNames.CustomerActivityHistory,
		Operation:      events.OpUpsert,
		Document:       doc,
	})
	return nil
}

// buildSetOnInsert chỉ chứa field chỉ set khi insert. Không trùng path với $set (MongoDB báo conflict).
func buildSetOnInsert(input LogActivityInput, domain string, now int64) bson.M {
	return bson.M{
		"unifiedId":            input.UnifiedId,
		"ownerOrganizationId":  input.OwnerOrgID,
		"domain":               domain,
		"source":               input.Source,
		"sourceRef":            input.SourceRef,
		"createdAt":            now,
	}
}

// splitSnapshotFromMetadata tách profileSnapshot, metricsSnapshot, snapshotChanges từ metadata.
// SnapshotChanges → changes (top-level); không để trong metadata.
func splitSnapshotFromMetadata(metadata map[string]interface{}) (activity.Snapshot, []activity.ActivityChangeItem, map[string]interface{}) {
	clean := make(map[string]interface{})
	for k, v := range metadata {
		if k != "profileSnapshot" && k != "metricsSnapshot" && k != "snapshotChanges" {
			clean[k] = v
		}
	}
	var snap activity.Snapshot
	if p, ok := metadata["profileSnapshot"].(map[string]interface{}); ok {
		snap.Profile = p
	}
	if m, ok := metadata["metricsSnapshot"].(map[string]interface{}); ok {
		snap.Metrics = m
	}
	changes := parseSnapshotChangesFromMetadata(metadata["snapshotChanges"])
	return snap, changes, clean
}

// parseSnapshotChangesFromMetadata chuyển snapshotChanges (từ BSON/map) sang []ActivityChangeItem.
func parseSnapshotChangesFromMetadata(v interface{}) []activity.ActivityChangeItem {
	if v == nil {
		return nil
	}
	sl, ok := v.([]interface{})
	if !ok || len(sl) == 0 {
		return nil
	}
	out := make([]activity.ActivityChangeItem, 0, len(sl))
	for _, item := range sl {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		field, _ := m["field"].(string)
		if field == "" {
			continue
		}
		out = append(out, activity.ActivityChangeItem{
			Field:    field,
			OldValue: m["oldValue"],
			NewValue: m["newValue"],
		})
	}
	return out
}

// mergeMetadata gộp reason, clientIp, userAgent, status vào metadata.
func mergeMetadata(metadata map[string]interface{}, reason, clientIp, userAgent, status string) map[string]interface{} {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	if reason != "" {
		metadata["reason"] = reason
	}
	if clientIp != "" {
		metadata["clientIp"] = clientIp
	}
	if userAgent != "" {
		metadata["userAgent"] = userAgent
	}
	if status != "" {
		metadata["status"] = status
	}
	return metadata
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

// GetLastSnapshotForCustomer lấy profile và metrics từ activity gần nhất có snapshot.
// Dùng để so sánh và chỉ lưu snapshot mới khi có thay đổi.
func (s *CrmActivityService) GetLastSnapshotForCustomer(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID, excludeSource string, excludeSourceRef map[string]interface{}) (profile, metrics map[string]interface{}, ok bool) {
	filter := bson.M{
		"unifiedId":           unifiedId,
		"ownerOrganizationId": ownerOrgID,
		"$or": []bson.M{
			{"snapshot.profile": bson.M{"$exists": true}},
			{"snapshot.metrics": bson.M{"$exists": true}},
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
	return doc.Snapshot.Profile, doc.Snapshot.Metrics, doc.Snapshot.Profile != nil || doc.Snapshot.Metrics != nil
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

// GetLastSnapshotBeforeActivityAt lấy profile và metrics từ activity gần nhất có activityAt < beforeActivityAt.
func (s *CrmActivityService) GetLastSnapshotBeforeActivityAt(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID, beforeActivityAt int64) (profile, metrics map[string]interface{}, ok bool) {
	filter := bson.M{
		"unifiedId":           unifiedId,
		"ownerOrganizationId": ownerOrgID,
		"activityAt":          bson.M{"$lt": beforeActivityAt},
		"$or": []bson.M{
			{"snapshot.profile": bson.M{"$exists": true}},
			{"snapshot.metrics": bson.M{"$exists": true}},
		},
	}
	opts := mongoopts.FindOne().SetSort(bson.D{{Key: "activityAt", Value: -1}})
	var doc crmmodels.CrmActivityHistory
	if err := s.Collection().FindOne(ctx, filter, opts).Decode(&doc); err != nil {
		return nil, nil, false
	}
	return doc.Snapshot.Profile, doc.Snapshot.Metrics, doc.Snapshot.Profile != nil || doc.Snapshot.Metrics != nil
}

// UpdateActivityMetadata cập nhật snapshot của activity theo _id (dùng cho recalc).
// metadataUpdates: profileSnapshot → snapshot.profile, metricsSnapshot → snapshot.metrics, snapshotChanges → changes
func (s *CrmActivityService) UpdateActivityMetadata(ctx context.Context, activityID primitive.ObjectID, metadataUpdates map[string]interface{}) error {
	if len(metadataUpdates) == 0 {
		return nil
	}
	set := bson.M{}
	for k, v := range metadataUpdates {
		path := "metadata." + k
		if k == "profileSnapshot" {
			path = "snapshot.profile"
		} else if k == "metricsSnapshot" {
			path = "snapshot.metrics"
		} else if k == "snapshotChanges" {
			path = "changes"
		}
		set[path] = v
	}
	_, err := s.Collection().UpdateOne(ctx, bson.M{"_id": activityID}, bson.M{"$set": set})
	return err
}

// GetActivitiesInPeriod lấy các activity trong khoảng [startMs, endMs] có snapshot.metrics.
func (s *CrmActivityService) GetActivitiesInPeriod(ctx context.Context, ownerOrgID primitive.ObjectID, startMs, endMs int64) ([]crmmodels.CrmActivityHistory, error) {
	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"activityAt":          bson.M{"$gte": startMs, "$lte": endMs},
		"snapshot.metrics":    bson.M{"$exists": true},
	}
	opts := mongoopts.Find().SetSort(bson.D{{Key: "unifiedId", Value: 1}, {Key: "activityAt", Value: 1}})
	return s.Find(ctx, filter, opts)
}

// GetLastSnapshotPerCustomerBeforeEndMs lấy snapshot.metrics cuối cùng của mỗi khách trước endMs.
func (s *CrmActivityService) GetLastSnapshotPerCustomerBeforeEndMs(ctx context.Context, ownerOrgID primitive.ObjectID, endMs int64) (map[string]map[string]interface{}, error) {
	pipe := []bson.M{
		{"$match": bson.M{
			"ownerOrganizationId": ownerOrgID,
			"activityAt":          bson.M{"$lte": endMs},
			"snapshot.metrics":    bson.M{"$exists": true},
		}},
		{"$sort": bson.M{"activityAt": -1}},
		{"$group": bson.M{
			"_id":             "$unifiedId",
			"metricsSnapshot": bson.M{"$first": "$snapshot.metrics"},
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

// CopySnapshotToMetadata ghi snapshot và changes từ activity vào metadata (format cũ).
// Dùng khi upsert "giữ snapshot cũ" — ingest cần metadata có profileSnapshot, metricsSnapshot, snapshotChanges.
func CopySnapshotToMetadata(metadata map[string]interface{}, act *crmmodels.CrmActivityHistory) {
	if metadata == nil || act == nil {
		return
	}
	// Cấu trúc mới: Snapshot + Changes
	if act.Snapshot.Profile != nil && metadata["profileSnapshot"] == nil {
		metadata["profileSnapshot"] = act.Snapshot.Profile
	}
	if act.Snapshot.Metrics != nil && metadata["metricsSnapshot"] == nil {
		metadata["metricsSnapshot"] = act.Snapshot.Metrics
	}
	if len(act.Changes) > 0 && metadata["snapshotChanges"] == nil {
		sl := make([]interface{}, len(act.Changes))
		for i, c := range act.Changes {
			sl[i] = map[string]interface{}{"field": c.Field, "oldValue": c.OldValue, "newValue": c.NewValue}
		}
		metadata["snapshotChanges"] = sl
	}
	if (act.Snapshot.Profile != nil || act.Snapshot.Metrics != nil) && metadata["snapshotAt"] == nil && act.ActivityAt > 0 {
		metadata["snapshotAt"] = act.ActivityAt
	}
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
