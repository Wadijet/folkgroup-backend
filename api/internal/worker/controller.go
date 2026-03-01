// Package worker - WorkerController quản lý throttle workers theo tải CPU.
package worker

import (
	"context"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"meta_commerce/internal/logger"
)

// ThrottleState trạng thái CPU: Normal, Throttled, Paused.
type ThrottleState string

const (
	StateNormal    ThrottleState = "normal"
	StateThrottled ThrottleState = "throttled"
	StatePaused    ThrottleState = "paused"
)

// Priority mức ưu tiên worker khi CPU quá tải. Số nhỏ = ưu tiên cao hơn.
type Priority int

const (
	PriorityCritical Priority = 1 // CRM Ingest — real-time từ hook, cần ưu tiên cao nhất
	PriorityHigh     Priority = 2 // Report Dirty — báo cáo dashboard
	PriorityNormal   Priority = 3 // CRM Bulk — user gọi API
	PriorityLow      Priority = 4 // Command Cleanup, Agent Command Cleanup
	PriorityLowest   Priority = 5 // Classification Refresh — batch định kỳ
)

// Controller singleton quản lý throttle workers theo CPU.
type Controller struct {
	mu               sync.RWMutex
	state            ThrottleState
	cpuPercent       float64
	lastSampleAt     time.Time
	enabled          bool
	thresholdThrottle float64
	thresholdPause   float64
	sampleInterval   time.Duration
	intervalMultiplier int
	batchDivisor     int
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
	thresholdThrottle := 70.0
	if v := os.Getenv("WORKER_CPU_THRESHOLD_THROTTLE"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
			thresholdThrottle = n
		}
	}
	thresholdPause := 90.0
	if v := os.Getenv("WORKER_CPU_THRESHOLD_PAUSE"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
			thresholdPause = n
		}
	}
	// Mặc định 15s/lần — gopsutil hoạt động trên Windows và Linux
	sampleInterval := 15 * time.Second
	if v := os.Getenv("WORKER_CPU_SAMPLE_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			sampleInterval = time.Duration(n) * time.Second
		}
	}
	intervalMultiplier := 3
	if v := os.Getenv("WORKER_THROTTLE_INTERVAL_MULTIPLIER"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			intervalMultiplier = n
		}
	}
	batchDivisor := 2
	if v := os.Getenv("WORKER_THROTTLE_BATCH_DIVISOR"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			batchDivisor = n
		}
	}

	return &Controller{
		state:              StateNormal,
		enabled:            enabled,
		thresholdThrottle:  thresholdThrottle,
		thresholdPause:     thresholdPause,
		sampleInterval:     sampleInterval,
		intervalMultiplier: intervalMultiplier,
		batchDivisor:       batchDivisor,
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
		"sampleInterval":     c.sampleInterval.String(),
		"thresholdThrottle":  c.thresholdThrottle,
		"thresholdPause":     c.thresholdPause,
	}).Info("⚙️ [WORKER_CONTROLLER] Worker CPU throttle controller started")
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.sampleCPU()
		}
	}
}

// sampleCPU lấy mẫu CPU và cập nhật trạng thái.
// Dùng gopsutil (cross-platform: Windows + Linux).
func (c *Controller) sampleCPU() {
	percent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return
	}
	cpuPct := 0.0
	if len(percent) > 0 {
		cpuPct = percent[0]
	}
	c.mu.Lock()
	prev := c.state
	c.cpuPercent = cpuPct
	c.lastSampleAt = time.Now()
	if cpuPct >= c.thresholdPause {
		c.state = StatePaused
	} else if cpuPct >= c.thresholdThrottle {
		c.state = StateThrottled
	} else {
		c.state = StateNormal
	}
	if prev != c.state {
		logger.GetAppLogger().WithFields(map[string]interface{}{
			"cpuPercent": cpuPct,
			"state":     string(c.state),
			"prev":      string(prev),
		}).Info("⚙️ [WORKER_CONTROLLER] Trạng thái CPU thay đổi")
	}
	c.mu.Unlock()
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
// Khi Paused: chỉ Critical và High chạy; Normal/Low/Lowest skip.
// Khi Throttled: Lowest skip (để dành CPU cho ưu tiên cao hơn).
func (c *Controller) ShouldThrottle(p Priority) bool {
	if !c.enabled {
		return false
	}
	c.mu.RLock()
	s := c.state
	c.mu.RUnlock()
	if s == StatePaused {
		return p >= PriorityNormal
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

// GetState trả về trạng thái hiện tại (để debug).
func (c *Controller) GetState() (ThrottleState, float64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state, c.cpuPercent
}

// ShouldThrottle package-level helper. Truyền priority để xác định có skip khi CPU quá tải không.
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
