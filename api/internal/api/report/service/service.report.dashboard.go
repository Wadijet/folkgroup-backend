// Package reportsvc - Dashboard Order Processing: funnel snapshot, recent orders (dữ liệu lũy kế, query trực tiếp DB).
package reportsvc

import (
	"context"
	"fmt"
	"sort"
	"time"

	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// OrderFunnelItem mỗi mục trong funnel (status + count). Dùng khi ?by=status.
type OrderFunnelItem struct {
	Status     int    `json:"status"`
	StatusName string `json:"statusName"`
	Count      int64  `json:"count"`
}

// StageFunnelItem mỗi stage trong funnel (6 stage). Mặc định API trả về format này.
type StageFunnelItem struct {
	Stage     string `json:"stage"`
	StageName string `json:"stageName"`
	Count     int64  `json:"count"`
}

// StageAgingBucket một bucket trong aging distribution.
type StageAgingBucket struct {
	Range string  `json:"range"`
	Count int64   `json:"count"`
	Pct   float64 `json:"pct"`
	Color string  `json:"color"` // green | yellow | red
}

// StageAgingItem metrics aging cho một stage.
type StageAgingItem struct {
	Stage        string             `json:"stage"`
	StageName    string             `json:"stageName"`
	TotalCount   int64              `json:"totalCount"`
	StuckCount   int64              `json:"stuckCount"`
	StuckRate    float64            `json:"stuckRate"`
	SlaMinutes   int64              `json:"slaMinutes"`
	Buckets      []StageAgingBucket `json:"buckets"`
	Percentiles  *struct {
		P50 int64 `json:"p50"`
		P90 int64 `json:"p90"`
		P95 int64 `json:"p95"`
		P99 int64 `json:"p99"`
	} `json:"percentiles,omitempty"`
}

// StuckOrderItem đơn vượt SLA trong stage hiện tại (dùng cho bảng Stuck Orders).
type StuckOrderItem struct {
	OrderID      int64   `json:"orderId"`
	CustomerName string  `json:"customerName"`
	Stage        string  `json:"stage"`
	StageName    string  `json:"stageName"`
	AgingMinutes int64   `json:"agingMinutes"`
	SlaMinutes   int64   `json:"slaMinutes"`
	AssignedSale string  `json:"assignedSale"`
}

// RecentOrderItem đơn hàng gần nhất cho bảng Order status.
type RecentOrderItem struct {
	OrderID               int64   `json:"orderId"`
	CustomerName          string  `json:"customerName"`
	CreatedAt             string  `json:"createdAt"`
	Status                int     `json:"status"`
	StatusName            string  `json:"statusName"`
	ProcessingTimeMinutes int64   `json:"processingTimeMinutes"`
	AssignedSale          string  `json:"assignedSale"`
}

// GetOrderFunnelSnapshot trả về funnel đơn hàng lũy kế (snapshot).
// Tham số by=stage (mặc định) hoặc by=status. Dùng cho TAB 6 Order Processing.
func (s *ReportService) GetOrderFunnelSnapshot(ctx context.Context, ownerOrganizationID primitive.ObjectID, by string) ([]OrderFunnelItem, []StageFunnelItem, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !ok {
		return nil, nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.PcPosOrders, common.ErrNotFound)
	}

	filter := bson.M{"ownerOrganizationId": ownerOrganizationID}
	pipeline := []bson.M{
		{"$match": filter},
		{"$group": bson.M{
			"_id":   bson.M{"$ifNull": bson.A{"$posData.status", -1}},
			"count": bson.M{"$sum": 1},
		}},
		{"$sort": bson.M{"_id": 1}},
	}
	cursor, err := coll.Aggregate(ctx, pipeline, options.Aggregate())
	if err != nil {
		return nil, nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	def, err := s.LoadDefinition(ctx, "order_daily")
	statusLabels := make(map[string]string)
	if err == nil && def.Metadata != nil {
		statusLabels = extractStatusLabels(def.Metadata)
	}
	if len(statusLabels) == 0 {
		statusLabels = map[string]string{
			"0": "Mới", "17": "Chờ xác nhận", "11": "Chờ hàng", "12": "Chờ in",
			"13": "Đã in", "20": "Đã đặt hàng", "1": "Đã xác nhận", "8": "Đang đóng hàng",
			"9": "Chờ lấy hàng", "2": "Đã giao hàng", "3": "Đã nhận hàng", "16": "Đã thu tiền",
			"4": "Đang trả hàng", "15": "Trả hàng một phần", "5": "Đã trả hàng",
			"6": "Đã hủy", "7": "Đã xóa gần đây", "-1": "Không xác định",
		}
	}

	var statusItems []OrderFunnelItem
	var stageCounts = make(map[string]int64)
	for cursor.Next(ctx) {
		var doc struct {
			ID    int   `bson:"_id"`
			Count int64 `bson:"count"`
		}
		if err := cursor.Decode(&doc); err != nil {
			return nil, nil, common.ConvertMongoError(err)
		}
		statusKey := fmt.Sprintf("%d", doc.ID)
		label := getStatusLabel(statusLabels, statusKey)
		statusItems = append(statusItems, OrderFunnelItem{
			Status:     doc.ID,
			StatusName: label,
			Count:      doc.Count,
		})
		stage := GetStageByStatus(doc.ID)
		stageCounts[stage] += doc.Count
	}
	if err := cursor.Err(); err != nil {
		return nil, nil, common.ConvertMongoError(err)
	}
	if statusItems == nil {
		statusItems = []OrderFunnelItem{}
	}

	stageItems := buildStageFunnelItems(stageCounts)
	if by == "stage" {
		return nil, stageItems, nil
	}
	return statusItems, nil, nil
}

// buildStageFunnelItems tạo danh sách StageFunnelItem theo thứ tự StageOrder.
func buildStageFunnelItems(counts map[string]int64) []StageFunnelItem {
	out := make([]StageFunnelItem, 0, len(StageOrder))
	for _, stage := range StageOrder {
		c := counts[stage]
		cfg := GetStageConfig(stage)
		name := stage
		if cfg != nil {
			name = cfg.StageName
		}
		out = append(out, StageFunnelItem{Stage: stage, StageName: name, Count: c})
	}
	return out
}

// GetRecentOrders trả về limit đơn hàng gần nhất (sort orderId desc). Dùng cho bảng Order status TAB 6.
func (s *ReportService) GetRecentOrders(ctx context.Context, ownerOrganizationID primitive.ObjectID, limit int) ([]RecentOrderItem, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 100 {
		limit = 100
	}

	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.PcPosOrders, common.ErrNotFound)
	}

	filter := bson.M{"ownerOrganizationId": ownerOrganizationID}
	opts := options.Find().
		SetSort(bson.D{{Key: "orderId", Value: -1}}).
		SetLimit(int64(limit)).
		SetProjection(bson.M{
			"orderId":        1,
			"billFullName":   1,
			"insertedAt":     1,
			"posData.status": 1,
			"posData.status_name": 1,
			"posData.assigning_seller": 1,
			"posData.status_history":  1,
		})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	def, _ := s.LoadDefinition(ctx, "order_daily")
	statusLabels := extractStatusLabels(def.Metadata)
	if len(statusLabels) == 0 {
		statusLabels = map[string]string{
			"0": "Mới", "1": "Đã xác nhận", "2": "Đã giao hàng", "3": "Đã nhận hàng",
			"6": "Đã hủy", "7": "Đã xóa gần đây", "-1": "Không xác định",
		}
	}

	now := time.Now()
	var result []RecentOrderItem
	for cursor.Next(ctx) {
		var doc struct {
			OrderID    int64 `bson:"orderId"`
			BillFullName string `bson:"billFullName"`
			InsertedAt int64  `bson:"insertedAt"`
			PosData    struct {
				Status        *int     `bson:"status"`
				StatusName    string   `bson:"status_name"`
				AssigningSeller *struct {
					Name string `bson:"name"`
				} `bson:"assigning_seller"`
				StatusHistory []struct {
					Status    *int   `bson:"status"`
					UpdatedAt string `bson:"updated_at"`
				} `bson:"status_history"`
			} `bson:"posData"`
		}
		if err := cursor.Decode(&doc); err != nil {
			return nil, common.ConvertMongoError(err)
		}

		status := -1
		if doc.PosData.Status != nil {
			status = *doc.PosData.Status
		}
		statusName := doc.PosData.StatusName
		if statusName == "" {
			statusName = getStatusLabel(statusLabels, fmt.Sprintf("%d", status))
		}
		assignedSale := ""
		if doc.PosData.AssigningSeller != nil {
			assignedSale = doc.PosData.AssigningSeller.Name
		}

		processingMins := computeProcessingTimeMinutes(doc.InsertedAt, doc.PosData.StatusHistory, status, now)

		createdAtStr := ""
		if doc.InsertedAt > 0 {
			t := time.UnixMilli(doc.InsertedAt)
			createdAtStr = t.Format("2006-01-02T15:04:05")
		}

		result = append(result, RecentOrderItem{
			OrderID:               doc.OrderID,
			CustomerName:          doc.BillFullName,
			CreatedAt:             createdAtStr,
			Status:                status,
			StatusName:            statusName,
			ProcessingTimeMinutes: processingMins,
			AssignedSale:          assignedSale,
		})
	}
	if err := cursor.Err(); err != nil {
		return nil, common.ConvertMongoError(err)
	}
	if result == nil {
		result = []RecentOrderItem{}
	}
	return result, nil
}

// GetStageAgingSnapshot trả về Stage Aging Distribution: buckets, stuck rate, percentiles cho từng stage.
// Stage aging = now - stage_entered_at (thời điểm đơn vào stage hiện tại, từ status_history).
// Chỉ tính cho các stage có SLA: NEW, CONFIRMATION, FULFILLMENT, SHIPPING.
func (s *ReportService) GetStageAgingSnapshot(ctx context.Context, ownerOrganizationID primitive.ObjectID) ([]StageAgingItem, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.PcPosOrders, common.ErrNotFound)
	}

	// Chỉ lấy đơn đang ở các stage xử lý (có SLA)
	statusesToFetch := []int{0, 17, 11, 12, 13, 20, 1, 8, 9, 2}
	filter := bson.M{
		"ownerOrganizationId": ownerOrganizationID,
		"posData.status":      bson.M{"$in": statusesToFetch},
	}
	opts := options.Find().SetProjection(bson.M{
		"posData.status":        1,
		"posData.status_history": 1,
	})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	now := time.Now()
	// stage -> danh sách aging (phút)
	agingByStage := make(map[string][]int64)
	for cursor.Next(ctx) {
		var doc struct {
			PosData struct {
				Status        *int `bson:"status"`
				StatusHistory []struct {
					Status    *int   `bson:"status"`
					UpdatedAt string `bson:"updated_at"`
				} `bson:"status_history"`
			} `bson:"posData"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		status := -1
		if doc.PosData.Status != nil {
			status = *doc.PosData.Status
		}
		stage := GetStageByStatus(status)
		cfg := GetStageConfig(stage)
		if cfg == nil || len(cfg.Buckets) == 0 {
			continue
		}
		enteredAt := computeStageEnteredAt(doc.PosData.StatusHistory, cfg.Statuses)
		if enteredAt.IsZero() {
			enteredAt = now
		}
		agingMins := int64(now.Sub(enteredAt).Minutes())
		agingByStage[stage] = append(agingByStage[stage], agingMins)
	}
	if err := cursor.Err(); err != nil {
		return nil, common.ConvertMongoError(err)
	}

	// Build response cho từng stage (theo thứ tự StageOrder)
	var result []StageAgingItem
	for _, stage := range StageOrder {
		cfg := GetStageConfig(stage)
		if cfg == nil || len(cfg.Buckets) == 0 {
			continue
		}
		agings := agingByStage[stage]
		if len(agings) == 0 {
			result = append(result, StageAgingItem{
				Stage:      stage,
				StageName:  cfg.StageName,
				TotalCount: 0,
				StuckCount: 0,
				StuckRate:  0,
				SlaMinutes: cfg.SlaMinutes,
				Buckets:    buildBuckets(agings, cfg.Buckets, cfg.SlaMinutes),
			})
			continue
		}

		stuckCount := int64(0)
		for _, a := range agings {
			if a > cfg.SlaMinutes {
				stuckCount++
			}
		}
		stuckRate := float64(stuckCount) / float64(len(agings))
		if len(agings) == 0 {
			stuckRate = 0
		}

		sort.Slice(agings, func(i, j int) bool { return agings[i] < agings[j] })
		p50 := percentile(agings, 50)
		p90 := percentile(agings, 90)
		p95 := percentile(agings, 95)
		p99 := percentile(agings, 99)

		result = append(result, StageAgingItem{
			Stage:      stage,
			StageName:  cfg.StageName,
			TotalCount: int64(len(agings)),
			StuckCount: stuckCount,
			StuckRate:  stuckRate,
			SlaMinutes: cfg.SlaMinutes,
			Buckets:    buildBuckets(agings, cfg.Buckets, cfg.SlaMinutes),
			Percentiles: &struct {
				P50 int64 `json:"p50"`
				P90 int64 `json:"p90"`
				P95 int64 `json:"p95"`
				P99 int64 `json:"p99"`
			}{P50: p50, P90: p90, P95: p95, P99: p99},
		})
	}
	if result == nil {
		result = []StageAgingItem{}
	}
	return result, nil
}

// GetStuckOrders trả về danh sách đơn vượt SLA, sort theo aging giảm dần.
// Tham số: limit (mặc định 50, max 200), stage (lọc theo stage, optional).
func (s *ReportService) GetStuckOrders(ctx context.Context, ownerOrganizationID primitive.ObjectID, limit int, stageFilter string) ([]StuckOrderItem, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.PcPosOrders, common.ErrNotFound)
	}

	statusesToFetch := []int{0, 17, 11, 12, 13, 20, 1, 8, 9, 2}
	filter := bson.M{
		"ownerOrganizationId": ownerOrganizationID,
		"posData.status":      bson.M{"$in": statusesToFetch},
	}
	opts := options.Find().SetProjection(bson.M{
		"orderId":              1,
		"billFullName":         1,
		"posData.status":       1,
		"posData.status_history": 1,
		"posData.assigning_seller": 1,
	})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	now := time.Now()
	var stuck []StuckOrderItem
	for cursor.Next(ctx) {
		var doc struct {
			OrderID      int64 `bson:"orderId"`
			BillFullName string `bson:"billFullName"`
			PosData      struct {
				Status        *int `bson:"status"`
				StatusHistory []struct {
					Status    *int   `bson:"status"`
					UpdatedAt string `bson:"updated_at"`
				} `bson:"status_history"`
				AssigningSeller *struct {
					Name string `bson:"name"`
				} `bson:"assigning_seller"`
			} `bson:"posData"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		status := -1
		if doc.PosData.Status != nil {
			status = *doc.PosData.Status
		}
		stg := GetStageByStatus(status)
		cfg := GetStageConfig(stg)
		if cfg == nil || cfg.SlaMinutes == 0 {
			continue
		}
		if stageFilter != "" && stg != stageFilter {
			continue
		}
		enteredAt := computeStageEnteredAt(doc.PosData.StatusHistory, cfg.Statuses)
		if enteredAt.IsZero() {
			enteredAt = now
		}
		agingMins := int64(now.Sub(enteredAt).Minutes())
		if agingMins <= cfg.SlaMinutes {
			continue
		}
		assignedSale := ""
		if doc.PosData.AssigningSeller != nil {
			assignedSale = doc.PosData.AssigningSeller.Name
		}
		stuck = append(stuck, StuckOrderItem{
			OrderID:      doc.OrderID,
			CustomerName: doc.BillFullName,
			Stage:        stg,
			StageName:    cfg.StageName,
			AgingMinutes: agingMins,
			SlaMinutes:   cfg.SlaMinutes,
			AssignedSale: assignedSale,
		})
	}
	if err := cursor.Err(); err != nil {
		return nil, common.ConvertMongoError(err)
	}

	sort.Slice(stuck, func(i, j int) bool { return stuck[i].AgingMinutes > stuck[j].AgingMinutes })
	if len(stuck) > limit {
		stuck = stuck[:limit]
	}
	if stuck == nil {
		stuck = []StuckOrderItem{}
	}
	return stuck, nil
}

// computeStageEnteredAt trả về thời điểm cuối cùng đơn vào stage hiện tại.
// Logic: MAX(updated_at) WHERE status IN stage_statuses.
func computeStageEnteredAt(history []struct {
	Status    *int   `bson:"status"`
	UpdatedAt string `bson:"updated_at"`
}, stageStatuses []int) time.Time {
	statusSet := make(map[int]bool)
	for _, s := range stageStatuses {
		statusSet[s] = true
	}
	var maxAt time.Time
	formats := []string{"2006-01-02T15:04:05.999999", "2006-01-02T15:04:05", time.RFC3339}
	for _, h := range history {
		if h.UpdatedAt == "" {
			continue
		}
		st := -1
		if h.Status != nil {
			st = *h.Status
		}
		if !statusSet[st] {
			continue
		}
		var t time.Time
		for _, layout := range formats {
			if parsed, err := time.Parse(layout, h.UpdatedAt); err == nil {
				t = parsed
				break
			}
		}
		if !t.IsZero() && t.After(maxAt) {
			maxAt = t
		}
	}
	return maxAt
}

// buildBuckets tạo buckets cho aging distribution.
func buildBuckets(agings []int64, bounds []int64, slaMinutes int64) []StageAgingBucket {
	bucketCounts := make([]int64, len(bounds)+1)
	for _, a := range agings {
		idx := AssignBucket(a, bounds)
		bucketCounts[idx]++
	}
	total := int64(len(agings))
	out := make([]StageAgingBucket, 0, len(bounds)+1)
	for i := range bucketCounts {
		c := bucketCounts[i]
		pct := float64(0)
		if total > 0 {
			pct = float64(c) / float64(total)
		}
		color := bucketColor(bounds, i, slaMinutes)
		label := BucketLabel(bounds, i)
		out = append(out, StageAgingBucket{Range: label, Count: c, Pct: pct, Color: color})
	}
	return out
}

// bucketColor green = within SLA, yellow = near/straddle SLA, red = SLA breach.
func bucketColor(bounds []int64, bucketIdx int, slaMinutes int64) string {
	if len(bounds) == 0 {
		return "green"
	}
	var lowEnd, highEnd int64
	if bucketIdx == 0 {
		lowEnd, highEnd = 0, bounds[0]
	} else if bucketIdx < len(bounds) {
		lowEnd, highEnd = bounds[bucketIdx-1], bounds[bucketIdx]
	} else {
		lowEnd = bounds[len(bounds)-1]
		highEnd = lowEnd + 1
	}
	if highEnd <= slaMinutes {
		return "green"
	}
	if lowEnd <= slaMinutes {
		return "yellow"
	}
	return "red"
}

// percentile trả về giá trị tại percentile p (0-100). agings phải đã sort.
func percentile(agings []int64, p int) int64 {
	if len(agings) == 0 {
		return 0
	}
	idx := (p * len(agings)) / 100
	if idx >= len(agings) {
		idx = len(agings) - 1
	}
	return agings[idx]
}

// computeProcessingTimeMinutes tính thời gian xử lý (phút) từ status_history.
// Đơn đang xử lý (status 1,8,9,11,12,13,17,20): now - thời điểm chuyển sang submitted/processing.
// Đơn completed (3,16): thời điểm completed - thời điểm bắt đầu xử lý.
func computeProcessingTimeMinutes(insertedAtMs int64, history []struct {
	Status    *int   `bson:"status"`
	UpdatedAt string `bson:"updated_at"`
}, currentStatus int, now time.Time) int64 {
	// Trạng thái đang xử lý (chưa hoàn thành)
	processingStatuses := map[int]bool{1: true, 8: true, 9: true, 11: true, 12: true, 13: true, 17: true, 20: true}
	completedStatuses := map[int]bool{3: true, 16: true}

	var startTime time.Time
	var endTime time.Time

	if insertedAtMs > 0 {
		startTime = time.UnixMilli(insertedAtMs)
	}

	for _, h := range history {
		if h.UpdatedAt == "" {
			continue
		}
		t, err := time.Parse("2006-01-02T15:04:05.999999", h.UpdatedAt)
		if err != nil {
			t, err = time.Parse("2006-01-02T15:04:05", h.UpdatedAt)
		}
		if err != nil {
			t, err = time.Parse(time.RFC3339, h.UpdatedAt)
		}
		if err != nil {
			continue
		}
		st := -1
		if h.Status != nil {
			st = *h.Status
		}
		if processingStatuses[st] && startTime.IsZero() {
			startTime = t
		}
		if completedStatuses[st] {
			endTime = t
			break
		}
	}

	if startTime.IsZero() {
		return 0
	}
	if completedStatuses[currentStatus] && !endTime.IsZero() {
		return int64(endTime.Sub(startTime).Minutes())
	}
	return int64(now.Sub(startTime).Minutes())
}
