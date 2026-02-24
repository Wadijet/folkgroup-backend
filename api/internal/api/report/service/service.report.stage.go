// Package reportsvc - Stage config và Stage Aging cho Dashboard Order Processing.
// 6 stage: NEW, CONFIRMATION, FULFILLMENT, SHIPPING, COMPLETED, TERMINATED, UNKNOWN.
package reportsvc

import (
	"fmt"
)

// StageOrder thứ tự stage trong funnel (để sort).
var StageOrder = []string{"NEW", "CONFIRMATION", "FULFILLMENT", "SHIPPING", "COMPLETED", "TERMINATED", "UNKNOWN"}

// StageConfig cấu hình từng stage: status codes, SLA, buckets.
type StageConfig struct {
	Stage      string
	StageName  string
	Statuses   []int
	SlaMinutes int64  // 0 = không có SLA
	Buckets    []int64 // Các ngưỡng phút (ví dụ: 15, 30, 60, 120 → buckets 0-15, 15-30, 30-60, 60-120, >120)
}

// StageConfigs mapping 6 stage chuẩn production.
var StageConfigs = []StageConfig{
	{Stage: "NEW", StageName: "Đơn mới", Statuses: []int{0, 17}, SlaMinutes: 60, Buckets: []int64{15, 30, 60, 120}},
	{Stage: "CONFIRMATION", StageName: "Xác nhận & chuẩn bị", Statuses: []int{11, 12, 13, 20, 1}, SlaMinutes: 12 * 60, Buckets: []int64{60, 180, 360, 720}},
	{Stage: "FULFILLMENT", StageName: "Đóng gói & chờ giao", Statuses: []int{8, 9}, SlaMinutes: 24 * 60, Buckets: []int64{180, 360, 720, 1440}},
	{Stage: "SHIPPING", StageName: "Đang giao hàng", Statuses: []int{2}, SlaMinutes: 72 * 60, Buckets: []int64{720, 1440, 2880, 4320}},
	{Stage: "COMPLETED", StageName: "Hoàn tất", Statuses: []int{3, 16}, SlaMinutes: 0, Buckets: nil},
	{Stage: "TERMINATED", StageName: "Hủy / trả hàng", Statuses: []int{4, 15, 5, 6, 7}, SlaMinutes: 0, Buckets: nil},
	{Stage: "UNKNOWN", StageName: "Không xác định", Statuses: []int{-1}, SlaMinutes: 60, Buckets: []int64{15, 30, 60, 120}},
}

// statusToStage map status code → stage.
var statusToStage map[int]string

func init() {
	statusToStage = make(map[int]string)
	for _, c := range StageConfigs {
		for _, st := range c.Statuses {
			statusToStage[st] = c.Stage
		}
	}
}

// GetStageByStatus trả về stage cho status code.
func GetStageByStatus(status int) string {
	if s, ok := statusToStage[status]; ok {
		return s
	}
	return "UNKNOWN"
}

// GetStageConfig trả về config cho stage.
func GetStageConfig(stage string) *StageConfig {
	for i := range StageConfigs {
		if StageConfigs[i].Stage == stage {
			return &StageConfigs[i]
		}
	}
	return nil
}

// BucketLabel tạo label cho bucket (vd: "0-15m", "15-30m", ">120m", "0-1h", ">12h").
func BucketLabel(bounds []int64, idx int) string {
	f := func(m int64) string {
		if m < 60 {
			return fmt.Sprintf("%dm", m)
		}
		if m < 1440 {
			return fmt.Sprintf("%dh", m/60)
		}
		return fmt.Sprintf("%dd", m/1440)
	}
	if len(bounds) == 0 {
		return "all"
	}
	if idx == 0 {
		return "0-" + f(bounds[0])
	}
	if idx < len(bounds) {
		return f(bounds[idx-1]) + "-" + f(bounds[idx])
	}
	return ">" + f(bounds[len(bounds)-1])
}

// AssignBucket gán aging (phút) vào bucket index. Trả về index 0..len(bounds).
func AssignBucket(agingMinutes int64, bounds []int64) int {
	if len(bounds) == 0 {
		return 0
	}
	for i, b := range bounds {
		if agingMinutes <= b {
			return i
		}
	}
	return len(bounds)
}
