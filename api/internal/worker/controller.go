// Package worker - WorkerController quản lý throttle workers theo tải CPU, RAM, disk.
package worker

import (
	"context"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"meta_commerce/internal/logger"
)

// ThrottleState trạng thái tài nguyên (CPU/RAM): Normal, Throttled, Paused.
type ThrottleState string

const (
	StateNormal    ThrottleState = "normal"
	StateThrottled ThrottleState = "throttled"
	StatePaused    ThrottleState = "paused"
)

// Priority mức ưu tiên worker khi CPU/RAM quá tải. Số nhỏ = ưu tiên cao hơn.
type Priority int

const (
	// Order > Ads > Customer; trong từng nhóm Report ưu tiên hơn
	PriorityCritical Priority = 1 // Report: ReportDirtyWorker (order + customer reports) — Report ưu tiên nhất
	PriorityHigh     Priority = 2 // Order: CrmIngest, Delivery Processor
	PriorityNormal   Priority = 3 // Ads: tất cả Ads workers
	PriorityLow      Priority = 4 // Customer: CrmBulkWorker
	PriorityLowest   Priority = 5 // Command Cleanup, Agent Cleanup, Classification Refresh
)

// ResourceMetrics chứa CPU, RAM, disk hiện tại.
type ResourceMetrics struct {
	CPUPercent  float64
	RAMPercent  float64
	DiskPercent float64
}

// OverloadAlertCallback được gọi khi tài nguyên quá tải. state: throttled|paused.
type OverloadAlertCallback func(metrics ResourceMetrics, state string)

var (
	overloadAlertCallback OverloadAlertCallback
	overloadCallbackMu    sync.RWMutex
)

// RegisterOverloadAlertCallback đăng ký callback khi CPU/RAM/disk quá tải.
func RegisterOverloadAlertCallback(fn OverloadAlertCallback) {
	overloadCallbackMu.Lock()
	overloadAlertCallback = fn
	overloadCallbackMu.Unlock()
}

// Controller singleton quản lý throttle workers theo CPU và RAM.
type Controller struct {
	mu                   sync.RWMutex
	state                ThrottleState
	cpuPercent           float64
	ramPercent           float64
	diskPercent          float64
	lastSampleAt         time.Time
	enabled              bool
	thresholdThrottle    float64
	thresholdPause       float64
	thresholdCPUAlert    float64
	thresholdRAMThrottle float64
	thresholdRAMPause    float64
	thresholdRAMAlert    float64
	thresholdDiskAlert   float64
	sampleInterval       time.Duration
	intervalMultiplier   int
	batchDivisor         int
	lastAlertAt          int64 // Unix sec — cooldown
}

var (
	controller     *Controller
	controllerOnce sync.Once
)

// DefaultController trả về singleton Controller.
func DefaultController() *Controller {
	controllerOnce.Do(func() {
		controller = newController()
	})
	return controller
}

func newController() *Controller {
	enabled := true
	if v := os.Getenv("WORKER_CPU_THROTTLE_ENABLED"); v != "" {
		enabled = v == "true" || v == "1"
	}
	// Ngưỡng thấp (40/60) để throttle sớm, tránh CPU chạm 100% trước khi phản ứng.
	thresholdThrottle := 40.0
	if v := os.Getenv("WORKER_CPU_THRESHOLD_THROTTLE"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
			thresholdThrottle = n
		}
	}
	thresholdPause := 60.0
	if v := os.Getenv("WORKER_CPU_THRESHOLD_PAUSE"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
			thresholdPause = n
		}
	}
	// Ngưỡng CPU để gửi cảnh báo (mặc định 95% — chỉ cảnh báo khi CPU rất cao)
	thresholdCPUAlert := 95.0
	if v := os.Getenv("WORKER_CPU_THRESHOLD_ALERT"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
			thresholdCPUAlert = n
		}
	}
	// Mặc định 3s/lần — phản ứng nhanh hơn để phát hiện spike CPU/RAM sớm.
	sampleInterval := 3 * time.Second
	if v := os.Getenv("WORKER_CPU_SAMPLE_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			sampleInterval = time.Duration(n) * time.Second
		}
	}
	// Multiplier/divisor mạnh hơn để giảm tải nhanh khi vừa chạm ngưỡng.
	intervalMultiplier := 4
	if v := os.Getenv("WORKER_THROTTLE_INTERVAL_MULTIPLIER"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			intervalMultiplier = n
		}
	}
	batchDivisor := 3
	if v := os.Getenv("WORKER_THROTTLE_BATCH_DIVISOR"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			batchDivisor = n
		}
	}
	// Ngưỡng RAM để throttle/pause — kiểm soát tràn RAM như CPU.
	// Hạ ngưỡng để phản ứng sớm trước khi swap, tránh tràn.
	thresholdRAMThrottle := 60.0
	if v := os.Getenv("WORKER_RAM_THRESHOLD_THROTTLE"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
			thresholdRAMThrottle = n
		}
	}
	thresholdRAMPause := 75.0
	if v := os.Getenv("WORKER_RAM_THRESHOLD_PAUSE"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
			thresholdRAMPause = n
		}
	}
	thresholdRAMAlert := 95.0
	if v := os.Getenv("WORKER_RAM_THRESHOLD_ALERT"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
			thresholdRAMAlert = n
		}
	}
	// Ngưỡng Disk để gửi cảnh báo (mặc định 90%)
	thresholdDiskAlert := 90.0
	if v := os.Getenv("WORKER_DISK_THRESHOLD_ALERT"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
			thresholdDiskAlert = n
		}
	}

	return &Controller{
		state:                StateNormal,
		enabled:              enabled,
		thresholdThrottle:    thresholdThrottle,
		thresholdPause:       thresholdPause,
		thresholdCPUAlert:    thresholdCPUAlert,
		thresholdRAMThrottle: thresholdRAMThrottle,
		thresholdRAMPause:    thresholdRAMPause,
		thresholdRAMAlert:    thresholdRAMAlert,
		thresholdDiskAlert:   thresholdDiskAlert,
		sampleInterval:       sampleInterval,
		intervalMultiplier:   intervalMultiplier,
		batchDivisor:         batchDivisor,
	}
}

// Start chạy goroutine lấy mẫu CPU định kỳ.
func (c *Controller) Start(ctx context.Context) {
	if !c.enabled {
		c.mu.Lock()
		c.state = StateNormal
		c.mu.Unlock()
		return
	}
	log := logger.GetAppLogger()
	// Lấy mẫu đầu tiên (cpu.Percent cần 1 giây để tính)
	go func() {
		time.Sleep(2 * time.Second)
		c.sampleCPU()
	}()
	ticker := time.NewTicker(c.sampleInterval)
	defer ticker.Stop()
	log.WithFields(map[string]interface{}{
		"sampleInterval":         c.sampleInterval.String(),
		"thresholdThrottle":      c.thresholdThrottle,
		"thresholdPause":         c.thresholdPause,
		"thresholdCPUAlert":      c.thresholdCPUAlert,
		"thresholdRAMThrottle":   c.thresholdRAMThrottle,
		"thresholdRAMPause":      c.thresholdRAMPause,
	}).Info("⚙️ [WORKER_CONTROLLER] Worker CPU/RAM throttle controller started")
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.sampleCPU()
		}
	}
}

// sampleCPU lấy mẫu CPU, RAM, disk và cập nhật trạng thái (throttle/pause theo cả CPU và RAM).
// Dùng gopsutil (cross-platform: Windows + Linux).
func (c *Controller) sampleCPU() {
	cpuPct := 0.0
	if percent, err := cpu.Percent(time.Second, false); err == nil && len(percent) > 0 {
		cpuPct = percent[0]
	}
	ramPct := 0.0
	if v, err := mem.VirtualMemory(); err == nil && v.Total > 0 {
		ramPct = v.UsedPercent
	}
	diskPct := 0.0
	if v, err := disk.Usage("/"); err == nil && v.Total > 0 {
		diskPct = v.UsedPercent
	} else if v, err := disk.Usage("."); err == nil && v.Total > 0 {
		diskPct = v.UsedPercent
	}

	c.mu.Lock()
	prev := c.state
	c.cpuPercent = cpuPct
	c.ramPercent = ramPct
	c.diskPercent = diskPct
	c.lastSampleAt = time.Now()
	// Quyết định trạng thái theo cả CPU và RAM — bất kỳ tài nguyên nào vượt ngưỡng đều kích hoạt throttle/pause.
	cpuPaused := cpuPct >= c.thresholdPause
	ramPaused := ramPct >= c.thresholdRAMPause
	cpuThrottled := cpuPct >= c.thresholdThrottle
	ramThrottled := ramPct >= c.thresholdRAMThrottle
	if cpuPaused || ramPaused {
		c.state = StatePaused
	} else if cpuThrottled || ramThrottled {
		c.state = StateThrottled
	} else {
		c.state = StateNormal
	}
	if prev != c.state {
		logger.GetAppLogger().WithFields(map[string]interface{}{
			"cpuPercent":  cpuPct,
			"ramPercent":  ramPct,
			"diskPercent": diskPct,
			"state":       string(c.state),
			"prev":        string(prev),
		}).Info("⚙️ [WORKER_CONTROLLER] Trạng thái CPU/RAM thay đổi")
	}
	now := time.Now().Unix()
	shouldAlert := (cpuPct >= c.thresholdCPUAlert || ramPct >= c.thresholdRAMAlert || diskPct >= c.thresholdDiskAlert) &&
		(now-c.lastAlertAt >= 1800) // Cooldown 30 phút
	if shouldAlert {
		c.lastAlertAt = now
	}
	metrics := ResourceMetrics{CPUPercent: cpuPct, RAMPercent: ramPct, DiskPercent: diskPct}
	stateStr := string(c.state)
	c.mu.Unlock()

	if shouldAlert {
		c.trySendOverloadAlert(metrics, stateStr)
	}
}

// trySendOverloadAlert gọi callback nếu đã đăng ký.
func (c *Controller) trySendOverloadAlert(metrics ResourceMetrics, state string) {
	overloadCallbackMu.RLock()
	fn := overloadAlertCallback
	overloadCallbackMu.RUnlock()
	if fn != nil {
		go fn(metrics, state)
	}
}

// getIntervalMultiplier trả về multiplier theo priority khi Throttled. Ưu tiên cao = multiplier nhỏ.
func (c *Controller) getIntervalMultiplier(p Priority) int {
	switch p {
	case PriorityCritical:
		return 1
	case PriorityHigh:
		return 2
	case PriorityNormal:
		return c.intervalMultiplier // 3
	case PriorityLow:
		return 4
	case PriorityLowest:
		return 5
	default:
		return c.intervalMultiplier
	}
}

// getBatchDivisor trả về divisor theo priority khi Throttled. Ưu tiên cao = divisor nhỏ (giữ nhiều batch hơn).
func (c *Controller) getBatchDivisor(p Priority) int {
	switch p {
	case PriorityCritical:
		return 2 // batch/2 = 50%
	case PriorityHigh:
		return 2
	case PriorityNormal:
		return c.batchDivisor // 2
	case PriorityLow:
		return 4
	case PriorityLowest:
		return 4
	default:
		return c.batchDivisor
	}
}

// ShouldThrottle trả về true nếu worker nên bỏ qua chu kỳ này.
// Khi Paused (CPU hoặc RAM vượt ngưỡng): chỉ Critical chạy; High/Normal/Low/Lowest skip.
// (Report Dirty/High load nhiều data — dừng khi Paused để tránh tràn RAM.)
// Khi Throttled: Lowest skip (để dành CPU/RAM cho ưu tiên cao hơn).
func (c *Controller) ShouldThrottle(p Priority) bool {
	if !c.enabled {
		return false
	}
	c.mu.RLock()
	s := c.state
	c.mu.RUnlock()
	if s == StatePaused {
		return p > PriorityCritical
	}
	if s == StateThrottled && p == PriorityLowest {
		return true
	}
	return false
}

// GetEffectiveInterval trả về interval hiệu dụng theo priority (khi Throttled: base * multiplier).
func (c *Controller) GetEffectiveInterval(base time.Duration, p Priority) time.Duration {
	if !c.enabled {
		return base
	}
	c.mu.RLock()
	s := c.state
	mul := c.getIntervalMultiplier(p)
	c.mu.RUnlock()
	if s == StateThrottled {
		return base * time.Duration(mul)
	}
	if s == StatePaused && (p == PriorityCritical || p == PriorityHigh) {
		return base * 2 // Khi Paused vẫn chạy nhưng chậm hơn
	}
	return base
}

// GetEffectiveBatchSize trả về batchSize hiệu dụng theo priority (khi Throttled/Paused: base / divisor).
// Khi Paused: Critical/High dùng divisor 4 để giảm tải mạnh.
func (c *Controller) GetEffectiveBatchSize(base int, p Priority) int {
	if !c.enabled {
		return base
	}
	c.mu.RLock()
	s := c.state
	div := c.getBatchDivisor(p)
	c.mu.RUnlock()
	if s == StatePaused && (p == PriorityCritical || p == PriorityHigh) {
		div = 4
	}
	if (s == StateThrottled || s == StatePaused) && div > 0 {
		n := base / div
		if n < 1 {
			n = 1
		}
		return n
	}
	return base
}

// GetEffectivePoolSize trả về pool size hiệu dụng theo trạng thái CPU/RAM.
// Khi Throttled: base/2 (tối thiểu 1); khi Paused: 1 (chạy tuần tự để giảm tải).
func (c *Controller) GetEffectivePoolSize(base int, p Priority) int {
	if !c.enabled || base <= 1 {
		return base
	}
	c.mu.RLock()
	s := c.state
	c.mu.RUnlock()
	if s == StatePaused {
		return 1
	}
	if s == StateThrottled {
		n := base / 2
		if n < 1 {
			n = 1
		}
		return n
	}
	return base
}

// GetState trả về trạng thái hiện tại (để debug).
func (c *Controller) GetState() (ThrottleState, float64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state, c.cpuPercent
}

// GetResourceMetrics trả về CPU, RAM, disk hiện tại.
func (c *Controller) GetResourceMetrics() ResourceMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return ResourceMetrics{
		CPUPercent:  c.cpuPercent,
		RAMPercent:  c.ramPercent,
		DiskPercent: c.diskPercent,
	}
}

// WorkerThresholds cấu hình ngưỡng throttle (để API GET/PUT).
type WorkerThresholds struct {
	Enabled                bool    `json:"enabled"`
	CPUThresholdThrottle   float64 `json:"cpuThresholdThrottle"`
	CPUThresholdPause      float64 `json:"cpuThresholdPause"`
	CPUThresholdAlert      float64 `json:"cpuThresholdAlert"`
	RAMThresholdThrottle   float64 `json:"ramThresholdThrottle"`
	RAMThresholdPause      float64 `json:"ramThresholdPause"`
	RAMThresholdAlert      float64 `json:"ramThresholdAlert"`
	DiskThresholdAlert     float64 `json:"diskThresholdAlert"`
	SampleIntervalSeconds  int     `json:"sampleIntervalSeconds"`
	IntervalMultiplier     int     `json:"intervalMultiplier"`
	BatchDivisor           int     `json:"batchDivisor"`
}

// GetThresholds trả về ngưỡng hiện tại (để API GET).
func (c *Controller) GetThresholds() WorkerThresholds {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return WorkerThresholds{
		Enabled:               c.enabled,
		CPUThresholdThrottle:  c.thresholdThrottle,
		CPUThresholdPause:     c.thresholdPause,
		CPUThresholdAlert:     c.thresholdCPUAlert,
		RAMThresholdThrottle:  c.thresholdRAMThrottle,
		RAMThresholdPause:     c.thresholdRAMPause,
		RAMThresholdAlert:     c.thresholdRAMAlert,
		DiskThresholdAlert:    c.thresholdDiskAlert,
		SampleIntervalSeconds: int(c.sampleInterval.Seconds()),
		IntervalMultiplier:    c.intervalMultiplier,
		BatchDivisor:          c.batchDivisor,
	}
}

// SetThresholds cập nhật ngưỡng (runtime, qua API). Chỉ cập nhật các field có giá trị hợp lệ.
// enabled: cập nhật nếu có; số: chỉ cập nhật khi > 0 (hoặc >= 1 cho multiplier/divisor).
func (c *Controller) SetThresholds(t *WorkerThresholds) {
	if t == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = t.Enabled
	if t.CPUThresholdThrottle > 0 {
		c.thresholdThrottle = t.CPUThresholdThrottle
	}
	if t.CPUThresholdPause > 0 {
		c.thresholdPause = t.CPUThresholdPause
	}
	if t.CPUThresholdAlert > 0 {
		c.thresholdCPUAlert = t.CPUThresholdAlert
	}
	if t.RAMThresholdThrottle > 0 {
		c.thresholdRAMThrottle = t.RAMThresholdThrottle
	}
	if t.RAMThresholdPause > 0 {
		c.thresholdRAMPause = t.RAMThresholdPause
	}
	if t.RAMThresholdAlert > 0 {
		c.thresholdRAMAlert = t.RAMThresholdAlert
	}
	if t.DiskThresholdAlert > 0 {
		c.thresholdDiskAlert = t.DiskThresholdAlert
	}
	if t.SampleIntervalSeconds > 0 {
		c.sampleInterval = time.Duration(t.SampleIntervalSeconds) * time.Second
	}
	if t.IntervalMultiplier >= 1 {
		c.intervalMultiplier = t.IntervalMultiplier
	}
	if t.BatchDivisor >= 1 {
		c.batchDivisor = t.BatchDivisor
	}
}

// ShouldThrottle package-level helper. Truyền priority để xác định có skip khi CPU/RAM quá tải không.
func ShouldThrottle(p Priority) bool {
	return DefaultController().ShouldThrottle(p)
}

// GetEffectiveInterval package-level helper. Truyền priority để xác định interval khi Throttled.
func GetEffectiveInterval(base time.Duration, p Priority) time.Duration {
	return DefaultController().GetEffectiveInterval(base, p)
}

// GetEffectiveBatchSize package-level helper. Truyền priority để xác định batch size khi Throttled.
func GetEffectiveBatchSize(base int, p Priority) int {
	return DefaultController().GetEffectiveBatchSize(base, p)
}

// GetEffectivePoolSize package-level helper. Truyền priority để xác định pool size khi Throttled/Paused.
func GetEffectivePoolSize(base int, p Priority) int {
	return DefaultController().GetEffectivePoolSize(base, p)
}
