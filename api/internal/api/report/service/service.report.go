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

	reportmodels "meta_commerce/internal/api/report/models"
	"meta_commerce/internal/common"
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
// Nếu đã có bản ghi cùng reportKey, periodKey, ownerOrganizationId và processedAt = null (đang chờ) thì bỏ qua, không ghi thêm.
// Không tạo dirty period cho chu kỳ customer/order bị tắt (điểm chặn cuối cùng — bất kể ai gọi).
func (s *ReportService) MarkDirty(ctx context.Context, reportKey, periodKey string, ownerOrganizationID primitive.ObjectID) error {
	if IsCustomerReportKeyDisabled(reportKey) {
		return nil
	}
	if IsOrderReportKeyDisabled(reportKey) {
		return nil
	}
	if IsAdsReportKeyDisabled(reportKey) {
		return nil
	}
	filter := bson.M{
		"reportKey":           reportKey,
		"periodKey":           periodKey,
		"ownerOrganizationId": ownerOrganizationID,
		"processedAt":         nil,
	}
	var existing reportmodels.ReportDirtyPeriod
	err := s.dirtyColl.FindOne(ctx, filter).Decode(&existing)
	if err == nil {
		// Đã có bản ghi cùng loại đang chờ xử lý — không tạo thêm
		return nil
	}
	if err != mongo.ErrNoDocuments {
		return common.ConvertMongoError(err)
	}

	now := time.Now().Unix()
	doc := reportmodels.ReportDirtyPeriod{
		ReportKey:           reportKey,
		PeriodKey:           periodKey,
		OwnerOrganizationID: ownerOrganizationID,
		MarkedAt:            now,
		ProcessedAt:         nil,
	}
	upsertFilter := bson.M{
		"reportKey":           reportKey,
		"periodKey":           periodKey,
		"ownerOrganizationId": ownerOrganizationID,
	}
	opts := options.Replace().SetUpsert(true)
	_, err = s.dirtyColl.ReplaceOne(ctx, upsertFilter, doc, opts)
	return common.ConvertMongoError(err)
}

// MarkDirtyAdsDaily đánh dấu chu kỳ ads_daily cần tính lại, theo adAccountId (dimensions).
func (s *ReportService) MarkDirtyAdsDaily(ctx context.Context, periodKey string, ownerOrganizationID primitive.ObjectID, adAccountId string) error {
	if IsAdsReportKeyDisabled("ads_daily") || adAccountId == "" {
		return nil
	}
	filter := bson.M{
		"reportKey":           "ads_daily",
		"periodKey":           periodKey,
		"ownerOrganizationId": ownerOrganizationID,
		"adAccountId":         adAccountId,
		"processedAt":         nil,
	}
	var existing reportmodels.ReportDirtyPeriod
	err := s.dirtyColl.FindOne(ctx, filter).Decode(&existing)
	if err == nil {
		return nil
	}
	if err != mongo.ErrNoDocuments {
		return common.ConvertMongoError(err)
	}
	now := time.Now().Unix()
	doc := reportmodels.ReportDirtyPeriod{
		ReportKey:           "ads_daily",
		PeriodKey:           periodKey,
		OwnerOrganizationID: ownerOrganizationID,
		AdAccountId:         adAccountId,
		MarkedAt:            now,
		ProcessedAt:         nil,
	}
	upsertFilter := bson.M{
		"reportKey":           "ads_daily",
		"periodKey":           periodKey,
		"ownerOrganizationId": ownerOrganizationID,
		"adAccountId":         adAccountId,
	}
	opts := options.Replace().SetUpsert(true)
	_, err = s.dirtyColl.ReplaceOne(ctx, upsertFilter, doc, opts)
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

// FindSnapshotsForTrendByDayRange truy vấn report_snapshots theo khoảng ngày [startMs, endMs].
// Đơn vị cơ sở là ngày; thử chu kỳ dài hơn (yearly→monthly→weekly→daily) để thay thế nếu có, tuy nhiên chu kỳ dài phải khớp từ ngày đầu đến ngày cuối.
// reportKeyOrder: thứ tự ưu tiên (vd: GetReportKeyOrderForDomain("order")).
func (s *ReportService) FindSnapshotsForTrendByDayRange(ctx context.Context, ownerOrganizationID primitive.ObjectID, startMs, endMs int64, reportKeyOrder []string) ([]reportmodels.ReportSnapshot, error) {
	candidates := getCandidateReportKeysAndRanges(startMs, endMs, reportKeyOrder)
	for _, c := range candidates {
		list, err := s.FindSnapshotsForTrend(ctx, c.reportKey, ownerOrganizationID, c.fromStr, c.toStr)
		if err != nil {
			return nil, err
		}
		if len(list) > 0 {
			return list, nil
		}
	}
	return []reportmodels.ReportSnapshot{}, nil
}

// GetOrderTrendFromDb trả về order trend aggregate trực tiếp từ order_canonical (PHỤ, đối chiếu — query DB nặng).
// Cùng format với FindSnapshotsForTrendByDayRange: []ReportSnapshot (reportKey, periodKey, periodType, metrics).
func (s *ReportService) GetOrderTrendFromDb(ctx context.Context, ownerOrganizationID primitive.ObjectID, startMs, endMs int64) ([]reportmodels.ReportSnapshot, error) {
	candidates := getCandidateReportKeysAndRanges(startMs, endMs, reportKeyOrderOrder)
	if len(candidates) == 0 {
		return []reportmodels.ReportSnapshot{}, nil
	}
	c := candidates[0]
	periodKeys := getPeriodKeysInRange(c.reportKey, c.fromStr, c.toStr)
	periodType := reportKeyToPeriodType(c.reportKey)
	list := make([]reportmodels.ReportSnapshot, 0, len(periodKeys))
	now := time.Now().Unix()
	for _, pk := range periodKeys {
		metrics, err := s.AggregateOrderReportForPeriod(ctx, c.reportKey, pk, ownerOrganizationID)
		if err != nil {
			return nil, fmt.Errorf("aggregate order %s/%s: %w", c.reportKey, pk, err)
		}
		list = append(list, reportmodels.ReportSnapshot{
			ReportKey:           c.reportKey,
			PeriodKey:           pk,
			PeriodType:          periodType,
			OwnerOrganizationID: ownerOrganizationID,
			Metrics:             metrics,
			ComputedAt:          now,
			CreatedAt:           now,
			UpdatedAt:           now,
		})
	}
	return list, nil
}

// FindSnapshotsForTrend truy vấn report_snapshots theo reportKey, ownerOrganizationId, periodKey trong [from, to].
func (s *ReportService) FindSnapshotsForTrend(ctx context.Context, reportKey string, ownerOrganizationID primitive.ObjectID, from, to string) ([]reportmodels.ReportSnapshot, error) {
	return s.FindSnapshotsForTrendWithDimensions(ctx, reportKey, ownerOrganizationID, from, to, nil)
}

// FindSnapshotsForTrendWithDimensions giống FindSnapshotsForTrend, thêm filter dimensions (vd: adAccountId cho ads_daily).
func (s *ReportService) FindSnapshotsForTrendWithDimensions(ctx context.Context, reportKey string, ownerOrganizationID primitive.ObjectID, from, to string, dimensions map[string]interface{}) ([]reportmodels.ReportSnapshot, error) {
	filter := bson.M{
		"reportKey":            reportKey,
		"ownerOrganizationId": ownerOrganizationID,
		"periodKey":            bson.M{"$gte": from, "$lte": to},
	}
	if len(dimensions) > 0 {
		for k, v := range dimensions {
			filter["dimensions."+k] = v
		}
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

// GetAdsTrendFromDb trả về ads trend aggregate trực tiếp từ meta_ad_insights + meta_campaigns (PHỤ, đối chiếu — query DB nặng).
// Cùng format với FindSnapshotsForAdsTrendByDayRange: []ReportSnapshot. adAccountId optional — rỗng thì aggregate tất cả accounts.
func (s *ReportService) GetAdsTrendFromDb(ctx context.Context, ownerOrganizationID primitive.ObjectID, startMs, endMs int64, adAccountId string) ([]reportmodels.ReportSnapshot, error) {
	if IsAdsReportKeyDisabled("ads_daily") {
		return []reportmodels.ReportSnapshot{}, nil
	}
	reportKeyOrder := GetReportKeyOrderForDomain("ads")
	candidates := getCandidateReportKeysAndRanges(startMs, endMs, reportKeyOrder)
	if len(candidates) == 0 {
		return []reportmodels.ReportSnapshot{}, nil
	}
	c := candidates[0]
	periodKeys := getPeriodKeysInRange(c.reportKey, c.fromStr, c.toStr)
	now := time.Now().Unix()
	list := make([]reportmodels.ReportSnapshot, 0)

	if adAccountId != "" {
		for _, pk := range periodKeys {
			metrics, err := s.AggregateAdsDailyForPeriod(ctx, pk, ownerOrganizationID, adAccountId)
			if err != nil {
				return nil, fmt.Errorf("aggregate ads %s/%s: %w", c.reportKey, pk, err)
			}
			list = append(list, reportmodels.ReportSnapshot{
				ReportKey:           c.reportKey,
				PeriodKey:           pk,
				PeriodType:          "day",
				OwnerOrganizationID: ownerOrganizationID,
				Dimensions:          map[string]interface{}{"adAccountId": adAccountId},
				Metrics:             metrics,
				ComputedAt:          now,
				CreatedAt:           now,
				UpdatedAt:           now,
			})
		}
		return list, nil
	}

	accColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdAccounts)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection meta_ad_accounts")
	}
	cursor, err := accColl.Find(ctx, bson.M{"ownerOrganizationId": ownerOrganizationID}, nil)
	if err != nil {
		return nil, fmt.Errorf("lấy ad accounts: %w", err)
	}
	defer cursor.Close(ctx)
	var accountIds []string
	for cursor.Next(ctx) {
		var doc struct {
			AdAccountId string `bson:"adAccountId"`
		}
		if err := cursor.Decode(&doc); err != nil || doc.AdAccountId == "" {
			continue
		}
		accountIds = append(accountIds, doc.AdAccountId)
	}
	for _, pk := range periodKeys {
		for _, accId := range accountIds {
			metrics, err := s.AggregateAdsDailyForPeriod(ctx, pk, ownerOrganizationID, accId)
			if err != nil {
				return nil, fmt.Errorf("aggregate ads %s/%s account %s: %w", c.reportKey, pk, accId, err)
			}
			list = append(list, reportmodels.ReportSnapshot{
				ReportKey:           c.reportKey,
				PeriodKey:           pk,
				PeriodType:          "day",
				OwnerOrganizationID: ownerOrganizationID,
				Dimensions:          map[string]interface{}{"adAccountId": accId},
				Metrics:             metrics,
				ComputedAt:          now,
				CreatedAt:           now,
				UpdatedAt:           now,
			})
		}
	}
	return list, nil
}

// FindSnapshotsForAdsTrendByDayRange truy vấn ads_daily từ report_snapshots theo khoảng ngày.
// adAccountId: optional — nếu có thì filter theo dimensions.adAccountId; nếu rỗng trả về tất cả ad accounts.
func (s *ReportService) FindSnapshotsForAdsTrendByDayRange(ctx context.Context, ownerOrganizationID primitive.ObjectID, startMs, endMs int64, adAccountId string) ([]reportmodels.ReportSnapshot, error) {
	if IsAdsReportKeyDisabled("ads_daily") {
		return []reportmodels.ReportSnapshot{}, nil
	}
	reportKeyOrder := GetReportKeyOrderForDomain("ads")
	candidates := getCandidateReportKeysAndRanges(startMs, endMs, reportKeyOrder)
	var dimensions map[string]interface{}
	if adAccountId != "" {
		dimensions = map[string]interface{}{"adAccountId": adAccountId}
	}
	for _, c := range candidates {
		list, err := s.FindSnapshotsForTrendWithDimensions(ctx, c.reportKey, ownerOrganizationID, c.fromStr, c.toStr, dimensions)
		if err != nil {
			return nil, err
		}
		if len(list) > 0 {
			return list, nil
		}
	}
	return []reportmodels.ReportSnapshot{}, nil
}

// GetUnprocessedDirtyPeriods lấy tối đa limit bản ghi từ report_dirty_periods có processedAt = null.
func (s *ReportService) GetUnprocessedDirtyPeriods(ctx context.Context, limit int) ([]reportmodels.ReportDirtyPeriod, error) {
	return s.GetUnprocessedDirtyPeriodsByReportKeys(ctx, limit, nil)
}

// GetUnprocessedDirtyPeriodsByReportKeys lấy dirty periods chưa xử lý, filter theo reportKeys.
// Nếu reportKeys = nil hoặc rỗng thì lấy tất cả (giống GetUnprocessedDirtyPeriods).
func (s *ReportService) GetUnprocessedDirtyPeriodsByReportKeys(ctx context.Context, limit int, reportKeys []string) ([]reportmodels.ReportDirtyPeriod, error) {
	if limit <= 0 {
		limit = 50
	}
	filter := bson.M{"processedAt": nil}
	if len(reportKeys) > 0 {
		filter["reportKey"] = bson.M{"$in": reportKeys}
	}
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
func (s *ReportService) SetDirtyProcessed(ctx context.Context, reportKey, periodKey string, ownerOrganizationID primitive.ObjectID, adAccountId string) error {
	now := time.Now().Unix()
	filter := bson.M{
		"reportKey":           reportKey,
		"periodKey":           periodKey,
		"ownerOrganizationId": ownerOrganizationID,
	}
	if reportKey == "ads_daily" && adAccountId != "" {
		filter["adAccountId"] = adAccountId
	}
	update := bson.M{"$set": bson.M{"processedAt": now}}
	_, err := s.dirtyColl.UpdateOne(ctx, filter, update)
	return common.ConvertMongoError(err)
}

// DeleteDirtyPeriod xóa dirty period (dùng khi chu kỳ bị tắt bởi config — không tạo chu kỳ báo cáo).
func (s *ReportService) DeleteDirtyPeriod(ctx context.Context, reportKey, periodKey string, ownerOrganizationID primitive.ObjectID, adAccountId string) error {
	filter := bson.M{
		"reportKey":           reportKey,
		"periodKey":           periodKey,
		"ownerOrganizationId": ownerOrganizationID,
	}
	if reportKey == "ads_daily" && adAccountId != "" {
		filter["adAccountId"] = adAccountId
	}
	_, err := s.dirtyColl.DeleteMany(ctx, filter)
	return common.ConvertMongoError(err)
}

// DeleteReportSnapshot xóa snapshot theo reportKey, periodKey, ownerOrganizationId.
func (s *ReportService) DeleteReportSnapshot(ctx context.Context, reportKey, periodKey string, ownerOrganizationID primitive.ObjectID, adAccountId string) error {
	filter := bson.M{
		"reportKey":           reportKey,
		"periodKey":           periodKey,
		"ownerOrganizationId": ownerOrganizationID,
	}
	if reportKey == "ads_daily" && adAccountId != "" {
		filter["dimensions.adAccountId"] = adAccountId
	}
	_, err := s.snapColl.DeleteMany(ctx, filter)
	return common.ConvertMongoError(err)
}
