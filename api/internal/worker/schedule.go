// Package worker - Cấu hình lịch chạy (interval, batchSize) cho tất cả workers.
// Cho phép thay đổi qua env hoặc API runtime.
package worker

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	reportsvc "meta_commerce/internal/api/report/service"
)

// WorkerScheduleOverride override interval/batch qua API runtime.
type WorkerScheduleOverride struct {
	Interval  string `json:"interval"`  // duration: "30s", "2m", "24h"
	BatchSize int    `json:"batchSize"` // 0 = không đổi
}

// workerScheduleOverrides override qua API. Key: worker name (lowercase).
var (
	workerScheduleOverrides   = make(map[string]WorkerScheduleOverride)
	workerScheduleOverridesMu sync.RWMutex
)

// defaultWorkerSchedules mặc định interval và batchSize cho từng worker.
var defaultWorkerSchedules = map[string]struct {
	Interval  time.Duration
	BatchSize int
}{
	WorkerCommandCleanup:      {1 * time.Minute, 300},
	WorkerAgentCommandCleanup: {1 * time.Minute, 300},
	WorkerAgentActivityCleanup: {1 * time.Hour, 1},
	WorkerCrmIngest:           {30 * time.Second, 50},
	WorkerCrmBulk:             {2 * time.Minute, 2},
	WorkerAdsExecution:        {30 * time.Second, 10},
	WorkerAdsAutoPropose:      {30 * time.Minute, 0},
	WorkerAdsCircuitBreaker:   {10 * time.Minute, 0},
	WorkerAdsDailyScheduler:   {1 * time.Minute, 0},
	WorkerAdsPancakeHeartbeat: {15 * time.Minute, 0},
	WorkerAdsCounterfactual:   {30 * time.Minute, 0},
	WorkerClassificationFull:  {24 * time.Hour, 200},
	WorkerClassificationSmart: {6 * time.Hour, 200},
	WorkerCixAnalysis:        {30 * time.Second, 50},  // poll cix_pending_analysis, batch 50
	WorkerCixRequest:         {5 * time.Second, 1},    // consume cix.analysis_requested → EnqueueAnalysis
	WorkerAIDecisionConsumer: {1 * time.Second, 1},    // idle giữa các lần queue trống; khi có hàng dùng busy-poll + burst (batchSize không dùng)
	WorkerAIDecisionDebounce: {5 * time.Second, 1},    // flush debounce state hết window → message.batch_ready
	WorkerAIDecisionClosure:  {10 * time.Minute, 1},   // đóng case quá hạn với closed_timeout
	WorkerOrderIntelligencePending: {3 * time.Second, 1}, // poll order_intelligence_pending, 1 job/tick
	WorkerCrmContext:         {5 * time.Second, 1},   // consume customer.context_requested → emit customer.context_ready
	WorkerLearningRuleSuggestion: {1 * time.Hour, 1},   // Phase 3: phân tích learning_cases → rule suggestions
	WorkerLearningEvaluation:     {5 * time.Minute, 50}, // Batch tính evaluation cho learning_cases
	WorkerLearningInsightAggregate: {6 * time.Hour, 1}, // Phase 3: aggregate cross-merchant (anonymized)
	WorkerIdentityBackfill:   {10 * time.Minute, 500}, // interval 10 phút, batch 500 doc/collection
	// report_redis_touch_flush: poll tick ~3s; flush touch trong RAM ff:rt:* → MarkDirty (chu kỳ theo REPORT_REDIS_TOUCH_*)
	WorkerReportRedisTouchFlush: {3 * time.Second, 0},
}

// GetWorkerScheduleOverrides trả về override hiện tại (để API GET).
func GetWorkerScheduleOverrides() map[string]WorkerScheduleOverride {
	workerScheduleOverridesMu.RLock()
	defer workerScheduleOverridesMu.RUnlock()
	out := make(map[string]WorkerScheduleOverride, len(workerScheduleOverrides))
	for k, v := range workerScheduleOverrides {
		out[k] = v
	}
	return out
}

// SetWorkerScheduleOverride đặt override lịch cho worker.
func SetWorkerScheduleOverride(workerName string, interval string, batchSize int) {
	name := strings.ToLower(strings.TrimSpace(workerName))
	if name == "" {
		return
	}
	workerScheduleOverridesMu.Lock()
	defer workerScheduleOverridesMu.Unlock()
	v := workerScheduleOverrides[name]
	if interval != "" {
		v.Interval = strings.TrimSpace(interval)
	}
	if batchSize > 0 {
		v.BatchSize = batchSize
	}
	workerScheduleOverrides[name] = v
}

// ClearWorkerScheduleOverride xóa override cho worker.
func ClearWorkerScheduleOverride(workerName string) {
	name := strings.ToLower(strings.TrimSpace(workerName))
	workerScheduleOverridesMu.Lock()
	defer workerScheduleOverridesMu.Unlock()
	delete(workerScheduleOverrides, name)
}

// GetEffectiveWorkerSchedule trả về interval và batchSize hiệu dụng cho worker.
// Thứ tự: API override > env > default.
func GetEffectiveWorkerSchedule(workerName string, defaultInterval time.Duration, defaultBatchSize int) (time.Duration, int) {
	canonical := strings.ToLower(strings.TrimSpace(workerName))
	if d, ok := defaultWorkerSchedules[canonical]; ok {
		defaultInterval = d.Interval
		defaultBatchSize = d.BatchSize
	}
	envInterval := parseWorkerIntervalEnv(canonical, defaultInterval)
	envBatch := parseWorkerBatchEnv(canonical, defaultBatchSize)

	workerScheduleOverridesMu.RLock()
	ov, hasOverride := workerScheduleOverrides[canonical]
	workerScheduleOverridesMu.RUnlock()

	var interval time.Duration
	var batch int
	if hasOverride && ov.Interval != "" {
		d, err := time.ParseDuration(ov.Interval)
		if err == nil && d >= time.Second {
			interval = d
		} else {
			interval = envInterval
		}
	} else {
		interval = envInterval
	}
	if hasOverride && ov.BatchSize > 0 {
		batch = ov.BatchSize
	} else {
		batch = envBatch
	}
	return interval, batch
}

func parseWorkerIntervalEnv(workerName string, defaultVal time.Duration) time.Duration {
	envKey := "WORKER_" + strings.ToUpper(strings.ReplaceAll(workerName, "-", "_")) + "_INTERVAL"
	v := strings.TrimSpace(os.Getenv(envKey))
	if v == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return defaultVal
	}
	if d < time.Second {
		return defaultVal
	}
	return d
}

func parseWorkerBatchEnv(workerName string, defaultVal int) int {
	envKey := "WORKER_" + strings.ToUpper(strings.ReplaceAll(workerName, "-", "_")) + "_BATCH"
	v := strings.TrimSpace(os.Getenv(envKey))
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return defaultVal
	}
	return n
}

// GetAllWorkerSchedules trả về config hiệu dụng cho tất cả workers (để API GET).
// report_dirty_ads, report_dirty_order, report_dirty_customer lấy từ reportSchedules.
func GetAllWorkerSchedules() map[string]map[string]interface{} {
	out := make(map[string]map[string]interface{})
	reportConfigs := reportsvc.GetReportScheduleConfigs()
	reportWorkerDomain := map[string]string{
		WorkerReportDirtyAds:     "ads",
		WorkerReportDirtyOrder:   "order",
		WorkerReportDirtyCustomer: "customer",
	}
	for _, name := range AllWorkerNames {
		if domain, ok := reportWorkerDomain[name]; ok {
			for i := range reportConfigs {
				if reportConfigs[i].Name == domain {
					out[name] = map[string]interface{}{
						"interval":  reportConfigs[i].Interval.String(),
						"batchSize": reportConfigs[i].BatchSize,
					}
					break
				}
			}
			continue
		}
		def, ok := defaultWorkerSchedules[name]
		if !ok {
			def = struct {
				Interval  time.Duration
				BatchSize int
			}{1 * time.Minute, 0}
		}
		interval, batch := GetEffectiveWorkerSchedule(name, def.Interval, def.BatchSize)
		out[name] = map[string]interface{}{
			"interval":  interval.String(),
			"batchSize": batch,
		}
	}
	return out
}
