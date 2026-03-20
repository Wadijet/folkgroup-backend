// Package reportsvc - Cấu hình lịch chạy ReportDirtyWorker theo từng domain (ads, order, customer).
// Mỗi domain có interval và batch riêng, config qua env hoặc API runtime.
package reportsvc

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ReportScheduleConfig cấu hình lịch chạy cho một nhóm reportKeys (vd: ads_daily, order_daily, customer_daily).
type ReportScheduleConfig struct {
	ReportKeys []string      // reportKeys thuộc domain (vd: ["ads_daily"])
	Interval   time.Duration // Khoảng thời gian giữa các lần chạy (vd: 2*time.Minute, 24*time.Hour)
	BatchSize  int           // Số dirty periods mỗi lần lấy từ DB
	Name       string        // Tên để log (vd: "ads", "order", "customer")
}

// ReportScheduleOverride override lịch chạy qua API. Interval lưu dạng string ("2m", "15m", "24h").
type ReportScheduleOverride struct {
	Interval string `json:"interval"` // duration string: "2m", "15m", "24h"
	BatchSize int   `json:"batchSize"`
}

// reportScheduleOverrides override qua API runtime. Key: ads, order, customer.
var (
	reportScheduleOverrides   = make(map[string]ReportScheduleOverride)
	reportScheduleOverridesMu sync.RWMutex
)

// GetReportScheduleOverrides trả về override hiện tại (để API GET).
func GetReportScheduleOverrides() map[string]ReportScheduleOverride {
	reportScheduleOverridesMu.RLock()
	defer reportScheduleOverridesMu.RUnlock()
	out := make(map[string]ReportScheduleOverride, len(reportScheduleOverrides))
	for k, v := range reportScheduleOverrides {
		out[k] = v
	}
	return out
}

// SetReportScheduleOverride đặt override lịch chạy cho domain (ads, order, customer).
// interval: duration string ("2m", "15m", "24h"). batchSize: 0 = không đổi.
func SetReportScheduleOverride(domain string, interval string, batchSize int) {
	domain = strings.TrimSpace(strings.ToLower(domain))
	if domain == "" {
		return
	}
	reportScheduleOverridesMu.Lock()
	defer reportScheduleOverridesMu.Unlock()
	v, ok := reportScheduleOverrides[domain]
	if !ok {
		v = ReportScheduleOverride{}
	}
	if interval != "" {
		v.Interval = strings.TrimSpace(interval)
	}
	if batchSize > 0 {
		v.BatchSize = batchSize
	}
	reportScheduleOverrides[domain] = v
}

// ClearReportScheduleOverride xóa override cho domain (dùng lại env/default).
func ClearReportScheduleOverride(domain string) {
	domain = strings.TrimSpace(strings.ToLower(domain))
	reportScheduleOverridesMu.Lock()
	defer reportScheduleOverridesMu.Unlock()
	delete(reportScheduleOverrides, domain)
}

// getEffectiveScheduleConfig trả về interval và batchSize hiệu dụng cho domain.
// Thứ tự: API override > env > default.
func getEffectiveScheduleConfig(domain string, envInterval string, envBatch string, defaultInterval time.Duration, defaultBatch int) (time.Duration, int) {
	reportScheduleOverridesMu.RLock()
	ov, hasOverride := reportScheduleOverrides[domain]
	reportScheduleOverridesMu.RUnlock()

	var interval time.Duration
	var batch int
	if hasOverride && ov.Interval != "" {
		d, err := time.ParseDuration(ov.Interval)
		if err == nil && d >= time.Minute {
			interval = d
		} else {
			interval = parseReportInterval(envInterval, defaultInterval)
		}
	} else {
		interval = parseReportInterval(envInterval, defaultInterval)
	}
	if hasOverride && ov.BatchSize > 0 {
		batch = ov.BatchSize
	} else {
		batch = parseReportBatch(envBatch, defaultBatch)
	}
	return interval, batch
}

// GetReportScheduleConfigs trả về cấu hình lịch chạy cho 3 domain.
// Thứ tự: API override > env > default. Dùng cho worker và API GET.
func GetReportScheduleConfigs() []ReportScheduleConfig {
	adsInterval, adsBatch := getEffectiveScheduleConfig("ads", "REPORT_ADS_INTERVAL", "REPORT_ADS_BATCH", 2*time.Minute, 20)
	orderInterval, orderBatch := getEffectiveScheduleConfig("order", "REPORT_ORDER_INTERVAL", "REPORT_ORDER_BATCH", 5*time.Minute, 15)
	customerInterval, customerBatch := getEffectiveScheduleConfig("customer", "REPORT_CUSTOMER_INTERVAL", "REPORT_CUSTOMER_BATCH", 10*time.Minute, 10)

	return []ReportScheduleConfig{
		{Name: "ads", ReportKeys: []string{"ads_daily"}, Interval: adsInterval, BatchSize: adsBatch},
		{Name: "order", ReportKeys: []string{"order_daily"}, Interval: orderInterval, BatchSize: orderBatch},
		{Name: "customer", ReportKeys: []string{"customer_daily"}, Interval: customerInterval, BatchSize: customerBatch},
	}
}

// parseReportInterval đọc duration từ env. Hỗ trợ: "2m", "15m", "1h", "24h".
func parseReportInterval(envKey string, defaultVal time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(envKey))
	if v == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return defaultVal
	}
	if d < time.Minute {
		return defaultVal
	}
	return d
}

// parseReportBatch đọc số từ env.
func parseReportBatch(envKey string, defaultVal int) int {
	v := strings.TrimSpace(os.Getenv(envKey))
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return defaultVal
	}
	return n
}
