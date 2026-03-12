// Package worker - Config cho ngưỡng throttle và mức ưu tiên worker.
// Cho phép chỉnh qua biến môi trường (env) hoặc API runtime.
package worker

import (
	"os"
	"strconv"
	"strings"
	"sync"
)

// WorkerName tên worker đăng ký trong Registry (format module_suffix).
const (
	WorkerReportDirty          = "report_dirty"
	WorkerDelivery             = "notification_delivery_processor"
	WorkerDeliveryCleanup      = "notification_delivery_cleanup"
	WorkerCommandCleanup       = "notification_command_cleanup"
	WorkerAgentCommandCleanup  = "notification_agent_command_cleanup"
	WorkerAgentActivityCleanup = "notification_agent_activity_cleanup"
	WorkerCrmIngest            = "crm_ingest"
	WorkerCrmBulk              = "crm_bulk"
	WorkerAdsExecution         = "ads_execution"
	WorkerAdsAutoPropose       = "ads_auto_propose"
	WorkerAdsCircuitBreaker    = "ads_circuit_breaker"
	WorkerAdsDailyScheduler    = "ads_daily_scheduler"
	WorkerAdsPancakeHeartbeat  = "ads_pancake_heartbeat"
	WorkerAdsCounterfactual    = "ads_counterfactual"
	WorkerClassificationFull   = "crm_classification_full"
	WorkerClassificationSmart  = "crm_classification_smart"
)

// WorkerMetadata mô tả worker (module, mô tả chức năng).
type WorkerMetadata struct {
	Module      string `json:"module"`
	Description string `json:"description"`
}

// workerMetadataMap map tên worker → metadata (module + mô tả).
var workerMetadataMap = map[string]WorkerMetadata{
	WorkerReportDirty:          {Module: "report", Description: "Tính toán lại báo cáo khi có dirty periods (order, customer, ads)"},
	WorkerDelivery:             {Module: "notification", Description: "Xử lý hàng đợi gửi thông báo (email, Telegram, SMS...)"},
	WorkerDeliveryCleanup:      {Module: "notification", Description: "Dọn các item bị kẹt trong hàng đợi delivery"},
	WorkerCommandCleanup:       {Module: "notification", Description: "Dọn command cũ hết hạn"},
	WorkerAgentCommandCleanup:  {Module: "notification", Description: "Dọn agent command cũ hết hạn"},
	WorkerAgentActivityCleanup: {Module: "notification", Description: "Dọn agent activity log cũ"},
	WorkerCrmIngest:           {Module: "crm", Description: "Đồng bộ dữ liệu customer từ agent vào hệ thống"},
	WorkerCrmBulk:             {Module: "crm", Description: "Xử lý bulk job cập nhật customer hàng loạt"},
	WorkerAdsExecution:        {Module: "ads", Description: "Thực thi các đề xuất quảng cáo đã được duyệt"},
	WorkerAdsAutoPropose:      {Module: "ads", Description: "Tạo đề xuất quảng cáo tự động theo rule"},
	WorkerAdsCircuitBreaker:   {Module: "ads", Description: "Giám sát và tạm dừng account khi lỗi Meta API liên tục"},
	WorkerAdsDailyScheduler:   {Module: "ads", Description: "Lên lịch chạy mode detection và các task ads hàng ngày"},
	WorkerAdsPancakeHeartbeat: {Module: "ads", Description: "Gửi heartbeat đến Pancake để đồng bộ trạng thái"},
	WorkerAdsCounterfactual:   {Module: "ads", Description: "Đánh giá kill đã qua 4h → counterfactual outcomes (FolkForm v4.1)"},
	WorkerClassificationFull:  {Module: "crm", Description: "Refresh toàn bộ phân loại khách hàng (lifecycle, journey, momentum) — 24h"},
	WorkerClassificationSmart: {Module: "crm", Description: "Refresh phân loại thông minh — chỉ khách gần ngưỡng lifecycle — 6h"},
}

// GetAllWorkerMetadata trả về metadata tất cả workers (để API GET).
func GetAllWorkerMetadata() map[string]WorkerMetadata {
	out := make(map[string]WorkerMetadata, len(AllWorkerNames))
	for _, name := range AllWorkerNames {
		if m, ok := workerMetadataMap[name]; ok {
			out[name] = m
		} else {
			out[name] = WorkerMetadata{Module: "unknown", Description: "Chưa có mô tả"}
		}
	}
	return out
}

// defaultWorkerPriorities mức ưu tiên mặc định cho từng worker.
// Số nhỏ = ưu tiên cao hơn. 1=Critical, 2=High, 3=Normal, 4=Low, 5=Lowest.
var defaultWorkerPriorities = map[string]Priority{
	WorkerReportDirty:          PriorityCritical,
	WorkerDelivery:             PriorityHigh,
	WorkerDeliveryCleanup:      PriorityLow,
	WorkerCommandCleanup:       PriorityLow,
	WorkerAgentCommandCleanup:  PriorityLow,
	WorkerAgentActivityCleanup: PriorityLow,
	WorkerCrmIngest:            PriorityHigh,
	WorkerCrmBulk:              PriorityLow,
	WorkerAdsExecution:        PriorityNormal,
	WorkerAdsAutoPropose:       PriorityNormal,
	WorkerAdsCircuitBreaker:    PriorityNormal,
	WorkerAdsDailyScheduler:   PriorityNormal,
	WorkerAdsPancakeHeartbeat:  PriorityNormal,
	WorkerAdsCounterfactual:    PriorityLow,
	WorkerClassificationFull:   PriorityLowest,
	WorkerClassificationSmart:  PriorityLowest,
}

// priorityOverrides override mức ưu tiên qua API (runtime). Ưu tiên cao hơn env.
var (
	priorityOverrides   = make(map[string]Priority)
	priorityOverridesMu sync.RWMutex
)

// workerActiveOverrides override trạng thái active/inactive qua API (runtime).
// Key = worker name (lowercase), Value = true (active) / false (inactive).
// Mặc định: tất cả active. Khi inactive, worker vẫn chạy vòng lặp nhưng bỏ qua mỗi tick (sleep rồi continue).
var (
	workerActiveOverrides   = make(map[string]bool)
	workerActiveOverridesMu sync.RWMutex
)

// SetPriorityOverride đặt override mức ưu tiên cho worker (runtime, qua API).
// Giá trị 0 = xóa override, dùng lại env/mặc định.
// Tên worker chuẩn hóa lowercase (report_dirty, crm_ingest, ...).
func SetPriorityOverride(workerName string, priority Priority) {
	priorityOverridesMu.Lock()
	defer priorityOverridesMu.Unlock()
	name := strings.ToLower(strings.TrimSpace(workerName))
	if priority >= 1 && priority <= 5 {
		priorityOverrides[name] = priority
	} else {
		delete(priorityOverrides, name)
	}
}

// AllWorkerNames danh sách tất cả worker (để trả effective priorities).
var AllWorkerNames = []string{
	WorkerReportDirty, WorkerDelivery, WorkerDeliveryCleanup,
	WorkerCommandCleanup, WorkerAgentCommandCleanup, WorkerAgentActivityCleanup,
	WorkerCrmIngest, WorkerCrmBulk,
	WorkerAdsExecution, WorkerAdsAutoPropose, WorkerAdsCircuitBreaker,
	WorkerAdsDailyScheduler, WorkerAdsPancakeHeartbeat, WorkerAdsCounterfactual,
	WorkerClassificationFull, WorkerClassificationSmart,
}

// GetAllEffectivePriorities trả về map worker_name → priority hiệu dụng (1–5) cho tất cả workers.
// Dùng cho API GET để client biết mức ưu tiên hiện tại từng worker.
func GetAllEffectivePriorities() map[string]int {
	out := make(map[string]int, len(AllWorkerNames))
	for _, name := range AllWorkerNames {
		defaultP := defaultWorkerPriorities[name]
		p := GetPriority(name, defaultP)
		out[name] = int(p)
	}
	return out
}

// GetPriorityOverrides trả về map override hiện tại (để API GET).
func GetPriorityOverrides() map[string]int {
	priorityOverridesMu.RLock()
	defer priorityOverridesMu.RUnlock()
	out := make(map[string]int, len(priorityOverrides))
	for k, v := range priorityOverrides {
		out[k] = int(v)
	}
	return out
}

// SetWorkerActiveOverride đặt override active/inactive cho worker (runtime, qua API).
// workerName: tên worker (report_dirty, crm_bulk, ...). active: true = chạy, false = tạm dừng.
// Khi active = false, worker vẫn chạy vòng lặp nhưng mỗi tick sẽ sleep và skip (không xử lý job).
func SetWorkerActiveOverride(workerName string, active bool) {
	workerActiveOverridesMu.Lock()
	defer workerActiveOverridesMu.Unlock()
	name := strings.ToLower(strings.TrimSpace(workerName))
	workerActiveOverrides[name] = active
}

// GetWorkerActiveOverrides trả về map override active hiện tại (để API GET).
// Chỉ trả về các worker đã được override (không bao gồm mặc định).
func GetWorkerActiveOverrides() map[string]bool {
	workerActiveOverridesMu.RLock()
	defer workerActiveOverridesMu.RUnlock()
	out := make(map[string]bool, len(workerActiveOverrides))
	for k, v := range workerActiveOverrides {
		out[k] = v
	}
	return out
}

// IsWorkerActive trả về true nếu worker đang active (được phép chạy).
// Thứ tự: API override > env WORKER_ACTIVE_<NAME> > mặc định true.
func IsWorkerActive(workerName string) bool {
	canonical := strings.ToLower(strings.TrimSpace(workerName))
	// 1. Override từ API (runtime)
	workerActiveOverridesMu.RLock()
	if v, ok := workerActiveOverrides[canonical]; ok {
		workerActiveOverridesMu.RUnlock()
		return v
	}
	workerActiveOverridesMu.RUnlock()
	// 2. Env: WORKER_ACTIVE_REPORT_DIRTY, WORKER_ACTIVE_CRM_BULK...
	envKey := "WORKER_ACTIVE_" + strings.ToUpper(strings.ReplaceAll(canonical, "-", "_"))
	if v := os.Getenv(envKey); v != "" {
		return v == "true" || v == "1"
	}
	// 3. Mặc định: active
	return true
}

// GetAllWorkerActive trả về map worker_name → active (true/false) cho tất cả workers.
func GetAllWorkerActive() map[string]bool {
	out := make(map[string]bool, len(AllWorkerNames))
	for _, name := range AllWorkerNames {
		out[name] = IsWorkerActive(name)
	}
	return out
}

// poolSizeEnvKeys map worker name -> env key cho pool size (để dùng tên ngắn hơn).
// Nếu không có: dùng WORKER_POOL_SIZE_<UPPERCASE_NAME>.
var poolSizeEnvKeys = map[string]string{
	WorkerDelivery:     "WORKER_POOL_SIZE_DELIVERY", // short for notification_delivery_processor
	WorkerReportDirty:  "WORKER_POOL_SIZE_REPORT_DIRTY",
	WorkerAdsExecution: "WORKER_POOL_SIZE_ADS_EXECUTION",
}

// GetPoolSize trả về pool size cơ bản cho worker từ env.
// Env: WORKER_POOL_SIZE_<NAME> (vd: WORKER_POOL_SIZE_DELIVERY, WORKER_POOL_SIZE_REPORT_DIRTY).
// Tham số: workerName, defaultSize (mặc định khi không có env).
func GetPoolSize(workerName string, defaultSize int) int {
	canonical := strings.ToLower(strings.TrimSpace(workerName))
	envKey := poolSizeEnvKeys[canonical]
	if envKey == "" {
		envKey = "WORKER_POOL_SIZE_" + strings.ToUpper(strings.ReplaceAll(canonical, "-", "_"))
	}
	if v := os.Getenv(envKey); v != "" {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil && n >= 1 {
			return n
		}
	}
	return defaultSize
}

// GetPriority trả về mức ưu tiên cho worker. Ưu tiên: API override > env > mặc định.
//
// Env: WORKER_PRIORITY_<NAME>=1|2|3|4|5 (NAME dạng REPORT_DIRTY, CRM_INGEST...)
// API: SetPriorityOverride (runtime)
//
// Tham số:
//   - workerName: tên worker (report_dirty, crm_ingest, ...)
//   - defaultPriority: mặc định nếu không có trong map và không có env
//
// Trả về: Priority (1=Critical, 2=High, 3=Normal, 4=Low, 5=Lowest)
func GetPriority(workerName string, defaultPriority Priority) Priority {
	canonical := strings.ToLower(strings.TrimSpace(workerName))
	// 1. Override từ API (runtime)
	priorityOverridesMu.RLock()
	if p, ok := priorityOverrides[canonical]; ok {
		priorityOverridesMu.RUnlock()
		return p
	}
	priorityOverridesMu.RUnlock()
	// 2. Override từ env (WORKER_PRIORITY_REPORT_DIRTY, WORKER_PRIORITY_CRM_INGEST...)
	envKey := "WORKER_PRIORITY_" + strings.ToUpper(canonical)
	if v := os.Getenv(envKey); v != "" {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil && n >= 1 && n <= 5 {
			return Priority(n)
		}
	}
	// 3. Mặc định từ map
	if p, ok := defaultWorkerPriorities[canonical]; ok {
		return p
	}
	return defaultPriority
}
