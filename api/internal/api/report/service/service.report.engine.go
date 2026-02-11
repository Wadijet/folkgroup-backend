// Package reportsvc - Compute engine (xem service.report.go cho package doc).
// File: service.report.engine.go - giữ tên cấu trúc cũ.
package reportsvc

import (
	"context"
	"fmt"
	"time"

	reportmodels "meta_commerce/internal/api/report/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Timezone cố định cho cắt chu kỳ (báo cáo theo chu kỳ).
const ReportTimezone = "Asia/Ho_Chi_Minh"

// Compute chạy engine tính báo cáo: load definition, aggregation nguồn, upsert snapshot.
// Hỗ trợ báo cáo có tagDimension trong metadata: thống kê theo posData.tags, chia đều khi đơn có nhiều tag.
func (s *ReportService) Compute(ctx context.Context, reportKey, periodKey string, ownerOrganizationID primitive.ObjectID) error {
	def, err := s.LoadDefinition(ctx, reportKey)
	if err != nil {
		return fmt.Errorf("load report definition: %w", err)
	}

	loc, err := time.LoadLocation(ReportTimezone)
	if err != nil {
		return fmt.Errorf("load timezone %s: %w", ReportTimezone, err)
	}
	var startSec, endSec int64
	switch def.PeriodType {
	case "day":
		t, err := time.ParseInLocation("2006-01-02", periodKey, loc)
		if err != nil {
			return fmt.Errorf("parse periodKey %s: %w", periodKey, err)
		}
		startSec = t.Unix()
		endSec = t.AddDate(0, 0, 1).Unix() - 1
	default:
		return fmt.Errorf("periodType %s chưa hỗ trợ", def.PeriodType)
	}

	sourceColl, ok := global.RegistryCollections.Get(def.SourceCollection)
	if !ok {
		return fmt.Errorf("không tìm thấy collection nguồn %s: %w", def.SourceCollection, common.ErrNotFound)
	}

	// Build khoảng thời gian filter theo đơn vị lưu trong collection nguồn (giây hoặc mili giây).
	timeUnit := def.TimeFieldUnit
	if timeUnit == "" {
		timeUnit = "second"
	}
	var timeFrom, timeTo int64
	switch timeUnit {
	case "millisecond":
		timeFrom = startSec * 1000
		timeTo = endSec*1000 + 999 // Cuối ngày (23:59:59.999)
	default:
		timeFrom = startSec
		timeTo = endSec
	}
	filter := bson.M{
		"ownerOrganizationId": ownerOrganizationID,
		def.TimeField:         bson.M{"$gte": timeFrom, "$lte": timeTo},
	}

	// Nếu có tagDimension trong metadata: chạy pipeline đặc biệt với $unwind tags, chia đều.
	if tagDim := extractTagDimension(def.Metadata); tagDim != nil {
		return s.computeWithTagDimension(ctx, reportKey, periodKey, ownerOrganizationID, def, sourceColl, filter, tagDim)
	}

	// Pipeline chuẩn (không có tag dimension).
	groupExpr := bson.M{"_id": nil}
	for _, m := range def.Metrics {
		switch m.AggType {
		case "sum":
			groupExpr[m.OutputKey] = bson.M{"$sum": "$" + m.FieldPath}
		case "avg":
			groupExpr[m.OutputKey] = bson.M{"$avg": "$" + m.FieldPath}
		case "count":
			groupExpr[m.OutputKey] = bson.M{"$sum": 1}
		case "countIf":
			cond := buildCountIfCond(m.CountIfExpr)
			if cond != nil {
				groupExpr[m.OutputKey] = bson.M{"$sum": bson.M{"$cond": bson.A{cond, 1, 0}}}
			}
		case "min":
			groupExpr[m.OutputKey] = bson.M{"$min": "$" + m.FieldPath}
		case "max":
			groupExpr[m.OutputKey] = bson.M{"$max": "$" + m.FieldPath}
		default:
			groupExpr[m.OutputKey] = nil
		}
	}

	pipeline := []bson.M{
		{"$match": filter},
		{"$group": groupExpr},
	}
	cursor, err := sourceColl.Aggregate(ctx, pipeline)
	if err != nil {
		return common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	var metrics map[string]interface{}
	if cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			return common.ConvertMongoError(err)
		}
		metrics = make(map[string]interface{})
		for k, v := range raw {
			if k == "_id" {
				continue
			}
			metrics[k] = v
		}
	}
	if metrics == nil {
		metrics = make(map[string]interface{})
		for _, m := range def.Metrics {
			metrics[m.OutputKey] = 0
		}
	}

	return s.upsertSnapshot(ctx, reportKey, periodKey, ownerOrganizationID, metrics)
}

// tagDimensionConfig cấu hình thống kê theo tag trong report.
type tagDimensionConfig struct {
	FieldPath       string // vd: posData.tags
	NameField       string // vd: name
	SplitMode       string // equal = chia đều khi nhiều tag
	TotalAmountPath string // vd: posData.total_price_after_sub_discount
}

// extractTagDimension đọc tagDimension từ metadata. Trả về nil nếu không có.
func extractTagDimension(metadata map[string]interface{}) *tagDimensionConfig {
	if metadata == nil {
		return nil
	}
	raw, ok := metadata["tagDimension"]
	if !ok || raw == nil {
		return nil
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	fieldPath, _ := m["fieldPath"].(string)
	nameField, _ := m["nameField"].(string)
	if fieldPath == "" || nameField == "" {
		return nil
	}
	cfg := &tagDimensionConfig{
		FieldPath: fieldPath,
		NameField: nameField,
		SplitMode: "equal",
	}
	if s, ok := m["splitMode"].(string); ok && s != "" {
		cfg.SplitMode = s
	}
	if path, ok := metadata["totalAmountField"].(string); ok && path != "" {
		cfg.TotalAmountPath = path
	} else {
		cfg.TotalAmountPath = "posData.total_price_after_sub_discount"
	}
	return cfg
}

// computeWithTagDimension chạy aggregation với $unwind tags, chia đều số lượng và số tiền khi đơn có nhiều tag.
func (s *ReportService) computeWithTagDimension(ctx context.Context, reportKey, periodKey string, ownerOrganizationID primitive.ObjectID, def *reportmodels.ReportDefinition, sourceColl *mongo.Collection, filter bson.M, tagDim *tagDimensionConfig) error {
	tagPath := tagDim.FieldPath
	nameField := tagDim.NameField
	amountPath := tagDim.TotalAmountPath

	// $facet: vừa tính tổng, vừa tính theo tag.
	pipeline := []bson.M{
		{"$match": filter},
		{"$facet": bson.M{
			"total": []bson.M{
				{"$group": bson.M{
					"_id": nil,
					"orderCount": bson.M{"$sum": 1},
					"totalAmount": bson.M{"$sum": bson.M{"$toLong": bson.M{"$ifNull": bson.A{"$" + amountPath, 0}}}},
				}},
			},
			"byTag": []bson.M{
				{"$addFields": bson.M{
					"__tagCount":   bson.M{"$size": bson.M{"$ifNull": bson.A{"$" + tagPath, bson.A{}}}},
					"__docAmount":  bson.M{"$toLong": bson.M{"$ifNull": bson.A{"$" + amountPath, 0}}},
				}},
				{"$addFields": bson.M{
					"__effectiveTagCount": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$__tagCount", 0}}, 1, "$__tagCount"}},
				}},
				{"$unwind": bson.M{"path": "$" + tagPath, "preserveNullAndEmptyArrays": true}},
				{"$addFields": bson.M{
					"__tagName":        bson.M{"$ifNull": bson.A{"$" + tagPath + "." + nameField, "Không tag"}},
					"__splitOrderCount": bson.M{"$divide": bson.A{1, "$__effectiveTagCount"}},
					"__splitAmount":    bson.M{"$divide": bson.A{"$__docAmount", "$__effectiveTagCount"}},
				}},
				{"$group": bson.M{
					"_id":        "$__tagName",
					"orderCount": bson.M{"$sum": "$__splitOrderCount"},
					"totalAmount": bson.M{"$sum": "$__splitAmount"},
				}},
			},
		}},
	}

	cursor, err := sourceColl.Aggregate(ctx, pipeline)
	if err != nil {
		return common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	metrics := make(map[string]interface{})
	metrics["total"] = map[string]interface{}{"orderCount": int64(0), "totalAmount": int64(0)}
	metrics["byTag"] = make(map[string]interface{})

	if cursor.Next(ctx) {
		var raw struct {
			Total []struct {
				OrderCount  int64 `bson:"orderCount"`
				TotalAmount int64 `bson:"totalAmount"`
			} `bson:"total"`
			ByTag []struct {
				ID          string  `bson:"_id"`
				OrderCount  float64 `bson:"orderCount"`
				TotalAmount float64 `bson:"totalAmount"`
			} `bson:"byTag"`
		}
		if err := cursor.Decode(&raw); err != nil {
			return common.ConvertMongoError(err)
		}
		totalOrderCount := int64(0)
		totalAmount := int64(0)
		if len(raw.Total) > 0 {
			totalOrderCount = raw.Total[0].OrderCount
			totalAmount = raw.Total[0].TotalAmount
			metrics["total"] = map[string]interface{}{
				"orderCount":  totalOrderCount,
				"totalAmount": totalAmount,
			}
		}
		byTag := make(map[string]interface{})
		for _, t := range raw.ByTag {
			avgAmount := float64(0)
			if t.OrderCount > 0 {
				avgAmount = t.TotalAmount / t.OrderCount
			}
			byTag[t.ID] = map[string]interface{}{
				"orderCount":  t.OrderCount,
				"totalAmount": t.TotalAmount,
				"avgAmount":   avgAmount,
			}
		}
		metrics["byTag"] = byTag
	}

	return s.upsertSnapshot(ctx, reportKey, periodKey, ownerOrganizationID, metrics)
}

func (s *ReportService) upsertSnapshot(ctx context.Context, reportKey, periodKey string, ownerOrganizationID primitive.ObjectID, metrics map[string]interface{}) error {
	now := time.Now().Unix()
	filterSnap := bson.M{
		"reportKey":            reportKey,
		"periodKey":            periodKey,
		"ownerOrganizationId": ownerOrganizationID,
	}
	update := bson.M{
		"$set": bson.M{
			"metrics":    metrics,
			"computedAt": now,
			"updatedAt":  now,
		},
		"$setOnInsert": bson.M{"createdAt": now},
	}
	opts := options.Update().SetUpsert(true)
	_, err := s.snapColl.UpdateOne(ctx, filterSnap, update, opts)
	return common.ConvertMongoError(err)
}

func buildCountIfCond(expr string) bson.M {
	switch expr {
	case "paidAt>0":
		return bson.M{"$gt": []interface{}{"$paidAt", int64(0)}}
	default:
		return nil
	}
}
