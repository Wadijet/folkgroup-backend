// Package reportsvc - Compute engine (xem service.report.go cho package doc).
// File: service.report.engine.go - giữ tên cấu trúc cũ.
package reportsvc

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
)

// Timezone cố định cho cắt chu kỳ (báo cáo theo chu kỳ).
const ReportTimezone = "Asia/Ho_Chi_Minh"

// Compute chạy engine tính báo cáo: load definition, aggregation nguồn, upsert snapshot.
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
	_, err = s.snapColl.UpdateOne(ctx, filterSnap, update, opts)
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
