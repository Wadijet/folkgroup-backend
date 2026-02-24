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
	case "week":
		// periodKey: YYYY-MM-DD (thứ Hai đầu tuần) hoặc bất kỳ ngày nào trong tuần — engine chuẩn hóa về thứ Hai
		t, err := time.ParseInLocation("2006-01-02", periodKey, loc)
		if err != nil {
			return fmt.Errorf("parse periodKey %s (cần YYYY-MM-DD): %w", periodKey, err)
		}
		// Lùi về thứ Hai (weekday 1 trong Go: Mon=1, Sun=7)
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7 // Chủ nhật = 7
		}
		monday := t.AddDate(0, 0, -(weekday - 1))
		startSec = monday.Unix()
		endSec = monday.AddDate(0, 0, 7).Unix() - 1 // Hết Chủ nhật 23:59:59
	case "month":
		// periodKey: YYYY-MM (vd: 2025-02)
		t, err := time.ParseInLocation("2006-01", periodKey, loc)
		if err != nil {
			return fmt.Errorf("parse periodKey %s (cần YYYY-MM): %w", periodKey, err)
		}
		startSec = t.Unix()
		// Ngày cuối tháng
		endOfMonth := t.AddDate(0, 1, 0).Add(-time.Second)
		endSec = endOfMonth.Unix()
	case "year":
		// periodKey: YYYY (vd: 2026)
		t, err := time.ParseInLocation("2006", periodKey, loc)
		if err != nil {
			return fmt.Errorf("parse periodKey %s (cần YYYY): %w", periodKey, err)
		}
		startSec = t.Unix()
		// 31/12 23:59:59 của năm đó
		endOfYear := t.AddDate(1, 0, 0).Add(-time.Second)
		endSec = endOfYear.Unix()
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
	// Loại trừ đơn hàng đã hủy (status 6) và đã xóa gần đây (status 7) khỏi doanh thu
	if statusPath := extractStatusDimensionField(def.Metadata); statusPath != "" {
		if exclude := extractExcludeStatuses(def.Metadata); len(exclude) > 0 {
			filter[statusPath] = bson.M{"$nin": exclude}
		}
	}

	// Nếu có tagDimension trong metadata: chạy pipeline đặc biệt với $unwind tags, chia đều.
	if tagDim := extractTagDimension(def.Metadata); tagDim != nil {
		return s.computeWithTagDimension(ctx, reportKey, periodKey, ownerOrganizationID, def, sourceColl, filter, tagDim)
	}

	// Pipeline chuẩn (không có tag dimension).
	groupExpr := bson.M{"_id": nil}
	for _, m := range def.Metrics {
		if m.Type == "derived" {
			continue
		}
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
			if m.Type != "derived" {
				metrics[m.OutputKey] = 0
			}
		}
	}

	// Áp dụng derived metrics (pipeline chuẩn: scope total, context = metrics)
	applyDerivedMetricsFromDef(metrics, nil, metrics, def.Metrics)

	return s.upsertSnapshot(ctx, reportKey, periodKey, def.PeriodType, ownerOrganizationID, metrics)
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

// extractStatusDimensionField đọc statusDimension.fieldPath từ metadata. Trả về rỗng nếu không có.
func extractStatusDimensionField(metadata map[string]interface{}) string {
	if metadata == nil {
		return ""
	}
	raw, ok := metadata["statusDimension"]
	if !ok || raw == nil {
		return ""
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return ""
	}
	path, _ := m["fieldPath"].(string)
	return path
}

// extractWarehouseDimensionField đọc warehouseDimension.fieldPath từ metadata. Trả về rỗng nếu không có.
func extractWarehouseDimensionField(metadata map[string]interface{}) string {
	if metadata == nil {
		return ""
	}
	raw, ok := metadata["warehouseDimension"]
	if !ok || raw == nil {
		return ""
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return ""
	}
	path, _ := m["fieldPath"].(string)
	return path
}

// extractAssigningSellerDimensionField đọc assigningSellerDimension.fieldPath từ metadata.
func extractAssigningSellerDimensionField(metadata map[string]interface{}) string {
	if metadata == nil {
		return ""
	}
	raw, ok := metadata["assigningSellerDimension"]
	if !ok || raw == nil {
		return ""
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return ""
	}
	path, _ := m["fieldPath"].(string)
	return path
}

// extractExcludeStatuses đọc excludeStatuses từ metadata (danh sách status cần loại trừ khỏi doanh thu, vd: 6=Đã hủy, 7=Đã xóa gần đây).
func extractExcludeStatuses(metadata map[string]interface{}) []interface{} {
	if metadata == nil {
		return nil
	}
	raw, ok := metadata["excludeStatuses"]
	if !ok || raw == nil {
		return nil
	}
	arr, ok := raw.([]interface{})
	if !ok || len(arr) == 0 {
		return nil
	}
	return arr
}

// extractStatusLabels đọc statusLabels từ metadata (mã status -> tên tiếng Việt).
func extractStatusLabels(metadata map[string]interface{}) map[string]string {
	if metadata == nil {
		return nil
	}
	raw, ok := metadata["statusLabels"]
	if !ok || raw == nil {
		return nil
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	out := make(map[string]string)
	for k, v := range m {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

// formulaRegistry map formulaRef -> hàm tính (params đã resolve).
var formulaRegistry = map[string]func(params map[string]float64) float64{
	"pct_of_total": func(p map[string]float64) float64 {
		v, t := p["value"], p["total"]
		if t == 0 {
			return 0
		}
		return round2(v/t*100)
	},
	"avg_from_sum_count": func(p map[string]float64) float64 {
		sum, count := p["sum"], p["count"]
		if count == 0 {
			return 0
		}
		return sum / count
	},
	"ratio": func(p map[string]float64) float64 {
		v, t := p["value"], p["total"]
		if t == 0 {
			return 0
		}
		return v / t
	},
}

// resolveParam lấy giá trị float64 từ param path (vd: "orderCount", "total.orderCount").
func resolveParam(path string, item, totalMap map[string]interface{}) float64 {
	if path == "" {
		return 0
	}
	var m map[string]interface{}
	if len(path) > 6 && path[:6] == "total." {
		m = totalMap
		path = path[6:]
	} else {
		if item != nil {
			m = item
		} else {
			m = totalMap
		}
	}
	if m == nil {
		return 0
	}
	return toFloat64(m[path])
}

// applyDerivedMetricsFromDef áp dụng derived metrics từ definition vào metrics.
// totalMap: map tổng (metrics["total"] hoặc metrics cho pipeline chuẩn).
// item: map item (nil cho scope=total).
func applyDerivedMetricsFromDef(metrics map[string]interface{}, totalMap map[string]interface{}, flatOrTotal map[string]interface{}, defMetrics []reportmodels.ReportMetricDefinition) {
	for _, m := range defMetrics {
		if m.Type != "derived" || m.FormulaRef == "" {
			continue
		}
		fn, ok := formulaRegistry[m.FormulaRef]
		if !ok {
			continue
		}
		if m.Scope == "total" {
			target := totalMap
			if target == nil {
				target = flatOrTotal
			}
			if target == nil {
				continue
			}
			params := make(map[string]float64)
			for k, path := range m.Params {
				params[k] = resolveParam(path, nil, target)
			}
			target[m.OutputKey] = fn(params)
		} else if m.Scope == "perDimension" {
			if totalMap == nil {
				totalMap, _ = metrics["total"].(map[string]interface{})
			}
			if totalMap == nil {
				continue
			}
			for _, dimKey := range []string{"byTag", "byStatus", "byWarehouse", "byAssigningSeller"} {
				dimMap, _ := metrics[dimKey].(map[string]interface{})
				if dimMap == nil {
					continue
				}
				for _, itemVal := range dimMap {
					itemMap, ok := itemVal.(map[string]interface{})
					if !ok {
						continue
					}
					params := make(map[string]float64)
					for k, path := range m.Params {
						params[k] = resolveParam(path, itemMap, totalMap)
					}
					itemMap[m.OutputKey] = fn(params)
				}
			}
		}
	}
}

func toFloat64(v interface{}) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int64:
		return float64(x)
	case int:
		return float64(x)
	default:
		return 0
	}
}

func round2(f float64) float64 {
	return float64(int64(f*100+0.5)) / 100
}

// getStatusLabel trả về tên tiếng Việt cho mã status. Trả về mã nếu không có trong labels.
func getStatusLabel(labels map[string]string, statusKey string) string {
	if labels == nil {
		return statusKey
	}
	if label, ok := labels[statusKey]; ok {
		return label
	}
	if statusKey == "-1" {
		return "Không xác định"
	}
	return statusKey
}

// computeWithTagDimension chạy aggregation với $unwind tags, chia đều số lượng và số tiền khi đơn có nhiều tag.
// Thêm byStatus khi có statusDimension trong metadata (theo docs Pancake POS API).
func (s *ReportService) computeWithTagDimension(ctx context.Context, reportKey, periodKey string, ownerOrganizationID primitive.ObjectID, def *reportmodels.ReportDefinition, sourceColl *mongo.Collection, filter bson.M, tagDim *tagDimensionConfig) error {
	tagPath := tagDim.FieldPath
	nameField := tagDim.NameField
	amountPath := tagDim.TotalAmountPath
	statusFieldPath := extractStatusDimensionField(def.Metadata)
	warehouseFieldPath := extractWarehouseDimensionField(def.Metadata)
	assigningSellerFieldPath := extractAssigningSellerDimensionField(def.Metadata)

	facet := bson.M{
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
	}
	if statusFieldPath != "" {
		facet["byStatus"] = []bson.M{
			{"$addFields": bson.M{
				"__statusKey": bson.M{"$toString": bson.M{"$ifNull": bson.A{"$" + statusFieldPath, -1}}},
				"__docAmount": bson.M{"$toLong": bson.M{"$ifNull": bson.A{"$" + amountPath, 0}}},
			}},
			{"$group": bson.M{
				"_id":        "$__statusKey",
				"orderCount": bson.M{"$sum": 1},
				"totalAmount": bson.M{"$sum": "$__docAmount"},
			}},
		}
	}
	if warehouseFieldPath != "" {
		facet["byWarehouse"] = []bson.M{
			{"$addFields": bson.M{
				"__warehouseKey": bson.M{"$ifNull": bson.A{"$" + warehouseFieldPath, "Không xác định"}},
				"__docAmount":   bson.M{"$toLong": bson.M{"$ifNull": bson.A{"$" + amountPath, 0}}},
			}},
			{"$group": bson.M{
				"_id":        "$__warehouseKey",
				"orderCount": bson.M{"$sum": 1},
				"totalAmount": bson.M{"$sum": "$__docAmount"},
			}},
		}
	}
	if assigningSellerFieldPath != "" {
		facet["byAssigningSeller"] = []bson.M{
			{"$addFields": bson.M{
				"__sellerKey": bson.M{"$ifNull": bson.A{"$" + assigningSellerFieldPath, "Không xác định"}},
				"__docAmount": bson.M{"$toLong": bson.M{"$ifNull": bson.A{"$" + amountPath, 0}}},
			}},
			{"$group": bson.M{
				"_id":        "$__sellerKey",
				"orderCount": bson.M{"$sum": 1},
				"totalAmount": bson.M{"$sum": "$__docAmount"},
			}},
		}
	}
	pipeline := []bson.M{
		{"$match": filter},
		{"$facet": facet},
	}

	cursor, err := sourceColl.Aggregate(ctx, pipeline)
	if err != nil {
		return common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	metrics := make(map[string]interface{})
	metrics["total"] = map[string]interface{}{"orderCount": int64(0), "totalAmount": int64(0)}
	metrics["byTag"] = make(map[string]interface{})
	metrics["byStatus"] = make(map[string]interface{})
	metrics["byWarehouse"] = make(map[string]interface{})
	metrics["byAssigningSeller"] = make(map[string]interface{})

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
			ByStatus []struct {
				ID          string  `bson:"_id"`
				OrderCount  float64 `bson:"orderCount"`
				TotalAmount float64 `bson:"totalAmount"`
			} `bson:"byStatus"`
			ByWarehouse []struct {
				ID          string  `bson:"_id"`
				OrderCount  float64 `bson:"orderCount"`
				TotalAmount float64 `bson:"totalAmount"`
			} `bson:"byWarehouse"`
			ByAssigningSeller []struct {
				ID          string  `bson:"_id"`
				OrderCount  float64 `bson:"orderCount"`
				TotalAmount float64 `bson:"totalAmount"`
			} `bson:"byAssigningSeller"`
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

		statusLabels := extractStatusLabels(def.Metadata)
		byStatus := make(map[string]interface{})
		for _, st := range raw.ByStatus {
			avgAmount := float64(0)
			if st.OrderCount > 0 {
				avgAmount = st.TotalAmount / st.OrderCount
			}
			label := getStatusLabel(statusLabels, st.ID)
			byStatus[st.ID] = map[string]interface{}{
				"label":       label,
				"orderCount":  st.OrderCount,
				"totalAmount": st.TotalAmount,
				"avgAmount":   avgAmount,
			}
		}
		metrics["byStatus"] = byStatus

		byWarehouse := make(map[string]interface{})
		for _, w := range raw.ByWarehouse {
			avgAmount := float64(0)
			if w.OrderCount > 0 {
				avgAmount = w.TotalAmount / w.OrderCount
			}
			byWarehouse[w.ID] = map[string]interface{}{
				"label":       w.ID,
				"orderCount":  w.OrderCount,
				"totalAmount": w.TotalAmount,
				"avgAmount":   avgAmount,
			}
		}
		metrics["byWarehouse"] = byWarehouse

		byAssigningSeller := make(map[string]interface{})
		for _, s := range raw.ByAssigningSeller {
			avgAmount := float64(0)
			if s.OrderCount > 0 {
				avgAmount = s.TotalAmount / s.OrderCount
			}
			byAssigningSeller[s.ID] = map[string]interface{}{
				"label":       s.ID,
				"orderCount":  s.OrderCount,
				"totalAmount": s.TotalAmount,
				"avgAmount":   avgAmount,
			}
		}
		metrics["byAssigningSeller"] = byAssigningSeller

		// Áp dụng derived metrics từ definition (formulaRef + params)
		totalMap, _ := metrics["total"].(map[string]interface{})
		applyDerivedMetricsFromDef(metrics, totalMap, totalMap, def.Metrics)
	}

	return s.upsertSnapshot(ctx, reportKey, periodKey, def.PeriodType, ownerOrganizationID, metrics)
}

func (s *ReportService) upsertSnapshot(ctx context.Context, reportKey, periodKey, periodType string, ownerOrganizationID primitive.ObjectID, metrics map[string]interface{}) error {
	now := time.Now().Unix()
	filterSnap := bson.M{
		"reportKey":            reportKey,
		"periodKey":            periodKey,
		"ownerOrganizationId": ownerOrganizationID,
	}
	update := bson.M{
		"$set": bson.M{
			"metrics":     metrics,
			"periodType":  periodType,
			"computedAt":  now,
			"updatedAt":   now,
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
