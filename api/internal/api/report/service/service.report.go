// Package reportsvc chứa service báo cáo theo chu kỳ (Phase 1).
// File: service.report.go - giữ tên cấu trúc cũ (service.<domain>.<entity>.go).
package reportsvc

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"meta_commerce/internal/common"
	reportmodels "meta_commerce/internal/api/report/models"
	"meta_commerce/internal/global"
)

// ReportService xử lý định nghĩa báo cáo, đánh dấu dirty và truy vấn snapshot (báo cáo theo chu kỳ Phase 1).
type ReportService struct {
	defColl   *mongo.Collection
	snapColl  *mongo.Collection
	dirtyColl *mongo.Collection
}

// NewReportService tạo mới ReportService.
func NewReportService() (*ReportService, error) {
	defColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ReportDefinitions)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.ReportDefinitions, common.ErrNotFound)
	}
	snapColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ReportSnapshots)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.ReportSnapshots, common.ErrNotFound)
	}
	dirtyColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ReportDirtyPeriods)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.ReportDirtyPeriods, common.ErrNotFound)
	}
	return &ReportService{
		defColl:   defColl,
		snapColl:  snapColl,
		dirtyColl: dirtyColl,
	}, nil
}

// LoadDefinition lấy một report definition theo key, isActive = true.
func (s *ReportService) LoadDefinition(ctx context.Context, reportKey string) (*reportmodels.ReportDefinition, error) {
	filter := bson.M{"key": reportKey, "isActive": true}
	var def reportmodels.ReportDefinition
	err := s.defColl.FindOne(ctx, filter).Decode(&def)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, common.ErrNotFound
		}
		return nil, common.ConvertMongoError(err)
	}
	return &def, nil
}

// GetReportKeysByCollection trả về danh sách report key có sourceCollection = collectionName, isActive = true.
func (s *ReportService) GetReportKeysByCollection(ctx context.Context, collectionName string) ([]string, error) {
	filter := bson.M{"sourceCollection": collectionName, "isActive": true}
	cursor, err := s.defColl.Find(ctx, filter, options.Find().SetProjection(bson.M{"key": 1}))
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	var keys []string
	for cursor.Next(ctx) {
		var doc struct {
			Key string `bson:"key"`
		}
		if err := cursor.Decode(&doc); err != nil {
			return nil, common.ConvertMongoError(err)
		}
		keys = append(keys, doc.Key)
	}
	if err := cursor.Err(); err != nil {
		return nil, common.ConvertMongoError(err)
	}
	return keys, nil
}

// GetDirtyPeriodKeysForReportKeys trả về map reportKey -> periodKey cho danh sách report keys và timestamp.
// Dùng khi hook cần mark dirty cho các report cụ thể (vd: customer_* khi pc_pos_customers thay đổi).
func (s *ReportService) GetDirtyPeriodKeysForReportKeys(ctx context.Context, reportKeys []string, unixSec int64) (map[string]string, error) {
	if len(reportKeys) == 0 {
		return nil, nil
	}
	filter := bson.M{"key": bson.M{"$in": reportKeys}, "isActive": true}
	cursor, err := s.defColl.Find(ctx, filter, options.Find().SetProjection(bson.M{"key": 1, "periodType": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	loc, err := time.LoadLocation(ReportTimezone)
	if err != nil {
		return nil, fmt.Errorf("load timezone %s: %w", ReportTimezone, err)
	}
	t := time.Unix(unixSec, 0).In(loc)

	result := make(map[string]string)
	for cursor.Next(ctx) {
		var doc struct {
			Key       string `bson:"key"`
			PeriodType string `bson:"periodType"`
		}
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		result[doc.Key] = periodKeyFromTime(t, doc.PeriodType)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// GetDirtyPeriodKeysForCollection trả về map reportKey -> periodKey cho collection và timestamp.
// Hook dùng khi dữ liệu nguồn thay đổi để mark đúng chu kỳ cho từng loại báo cáo (day/week/month/year).
func (s *ReportService) GetDirtyPeriodKeysForCollection(ctx context.Context, collectionName string, unixSec int64) (map[string]string, error) {
	filter := bson.M{"sourceCollection": collectionName, "isActive": true}
	cursor, err := s.defColl.Find(ctx, filter, options.Find().SetProjection(bson.M{"key": 1, "periodType": 1}))
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	loc, err := time.LoadLocation(ReportTimezone)
	if err != nil {
		return nil, fmt.Errorf("load timezone %s: %w", ReportTimezone, err)
	}
	t := time.Unix(unixSec, 0).In(loc)

	result := make(map[string]string)
	for cursor.Next(ctx) {
		var doc struct {
			Key       string `bson:"key"`
			PeriodType string `bson:"periodType"`
		}
		if err := cursor.Decode(&doc); err != nil {
			return nil, common.ConvertMongoError(err)
		}
		periodKey := periodKeyFromTime(t, doc.PeriodType)
		result[doc.Key] = periodKey
	}
	if err := cursor.Err(); err != nil {
		return nil, common.ConvertMongoError(err)
	}
	return result, nil
}

// periodKeyFromTime tính periodKey theo periodType từ thời điểm t (đã In(loc)).
func periodKeyFromTime(t time.Time, periodType string) string {
	switch periodType {
	case "week":
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := t.AddDate(0, 0, -(weekday - 1))
		return monday.Format("2006-01-02")
	case "month":
		return t.Format("2006-01")
	case "year":
		return t.Format("2006")
	default:
		return t.Format("2006-01-02")
	}
}

// MarkDirty đánh dấu chu kỳ cần tính lại.
func (s *ReportService) MarkDirty(ctx context.Context, reportKey, periodKey string, ownerOrganizationID primitive.ObjectID) error {
	now := time.Now().Unix()
	doc := reportmodels.ReportDirtyPeriod{
		ReportKey:           reportKey,
		PeriodKey:           periodKey,
		OwnerOrganizationID: ownerOrganizationID,
		MarkedAt:            now,
		ProcessedAt:         nil,
	}
	filter := bson.M{
		"reportKey":           reportKey,
		"periodKey":           periodKey,
		"ownerOrganizationId": ownerOrganizationID,
	}
	opts := options.Replace().SetUpsert(true)
	_, err := s.dirtyColl.ReplaceOne(ctx, filter, doc, opts)
	return common.ConvertMongoError(err)
}

// GetDefinitionsCollection trả về collection report_definitions.
func (s *ReportService) GetDefinitionsCollection() *mongo.Collection { return s.defColl }

// GetSnapshotsCollection trả về collection report_snapshots.
func (s *ReportService) GetSnapshotsCollection() *mongo.Collection { return s.snapColl }

// GetDirtyCollection trả về collection report_dirty_periods.
func (s *ReportService) GetDirtyCollection() *mongo.Collection { return s.dirtyColl }

// GetReportSnapshot lấy một snapshot theo reportKey, periodKey, ownerOrganizationId.
// Trả về nil nếu không tìm thấy (không phải lỗi).
func (s *ReportService) GetReportSnapshot(ctx context.Context, reportKey, periodKey string, ownerOrganizationID primitive.ObjectID) (*reportmodels.ReportSnapshot, error) {
	filter := bson.M{
		"reportKey":            reportKey,
		"periodKey":            periodKey,
		"ownerOrganizationId": ownerOrganizationID,
	}
	var snap reportmodels.ReportSnapshot
	err := s.snapColl.FindOne(ctx, filter).Decode(&snap)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, common.ConvertMongoError(err)
	}
	return &snap, nil
}

// FindSnapshotsForTrend truy vấn report_snapshots theo reportKey, ownerOrganizationId, periodKey trong [from, to].
func (s *ReportService) FindSnapshotsForTrend(ctx context.Context, reportKey string, ownerOrganizationID primitive.ObjectID, from, to string) ([]reportmodels.ReportSnapshot, error) {
	filter := bson.M{
		"reportKey":            reportKey,
		"ownerOrganizationId": ownerOrganizationID,
		"periodKey":            bson.M{"$gte": from, "$lte": to},
	}
	opts := options.Find().SetSort(bson.D{{Key: "periodKey", Value: 1}})
	cursor, err := s.snapColl.Find(ctx, filter, opts)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	var list []reportmodels.ReportSnapshot
	if err := cursor.All(ctx, &list); err != nil {
		return nil, common.ConvertMongoError(err)
	}
	if list == nil {
		list = []reportmodels.ReportSnapshot{}
	}
	return list, nil
}

// GetUnprocessedDirtyPeriods lấy tối đa limit bản ghi từ report_dirty_periods có processedAt = null.
func (s *ReportService) GetUnprocessedDirtyPeriods(ctx context.Context, limit int) ([]reportmodels.ReportDirtyPeriod, error) {
	if limit <= 0 {
		limit = 50
	}
	filter := bson.M{"processedAt": nil}
	opts := options.Find().SetSort(bson.D{{Key: "markedAt", Value: 1}}).SetLimit(int64(limit))
	cursor, err := s.dirtyColl.Find(ctx, filter, opts)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	var list []reportmodels.ReportDirtyPeriod
	if err := cursor.All(ctx, &list); err != nil {
		return nil, common.ConvertMongoError(err)
	}
	if list == nil {
		list = []reportmodels.ReportDirtyPeriod{}
	}
	return list, nil
}

// SetDirtyProcessed đánh dấu đã xử lý.
func (s *ReportService) SetDirtyProcessed(ctx context.Context, reportKey, periodKey string, ownerOrganizationID primitive.ObjectID) error {
	now := time.Now().Unix()
	filter := bson.M{
		"reportKey":           reportKey,
		"periodKey":           periodKey,
		"ownerOrganizationId": ownerOrganizationID,
	}
	update := bson.M{"$set": bson.M{"processedAt": now}}
	_, err := s.dirtyColl.UpdateOne(ctx, filter, update)
	return common.ConvertMongoError(err)
}
