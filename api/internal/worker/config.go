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
	WorkerReportDirtyAds      = "report_dirty_ads"
	WorkerReportDirtyOrder    = "report_dirty_order"
	WorkerReportDirtyCustomer = "report_dirty_customer"
	// WorkerReportRedisTouchFlush — quét touch trong RAM (ff:rt:*) → MarkDirty.
	WorkerReportRedisTouchFlush    = "report_redis_touch_flush"
	WorkerDelivery                 = "notification_delivery_processor"
	WorkerDeliveryCleanup          = "notification_delivery_cleanup"
	WorkerCommandCleanup           = "notification_command_cleanup"
	WorkerAgentCommandCleanup      = "notification_agent_command_cleanup"
	WorkerAgentActivityCleanup     = "notification_agent_activity_cleanup"
	WorkerCrmPendingMerge          = "customer_job_pending_merge"
	WorkerCrmBulk                  = "customer_bulk"
	WorkerAdsExecution             = "ads_execution"
	WorkerAdsAutoPropose           = "ads_auto_propose"
	WorkerAdsCircuitBreaker        = "ads_circuit_breaker"
	WorkerAdsDailyScheduler        = "ads_daily_scheduler"
	WorkerAdsPancakeHeartbeat      = "ads_pancake_heartbeat"
	WorkerAdsCounterfactual        = "ads_counterfactual"
	WorkerClassificationFull       = "crm_classification_full"
	WorkerClassificationSmart      = "crm_classification_smart"
	WorkerCixIntelCompute          = "cix_job_intel"
	WorkerAIDecisionConsumer       = "ai_decision_consumer"
	WorkerAIDecisionDebounce       = "ai_decision_debounce"
	WorkerAIDecisionClosure        = "ai_decision_closure"
	WorkerOrderIntelCompute = "order_job_intel"
	WorkerAdsIntelCompute = "ads_job_intel"
	WorkerCrmContext               = "customer_context"
	WorkerCrmIntelCompute   = "customer_job_intel"
	WorkerLearningRuleSuggestion   = "learning_rule_suggestion"
	WorkerLearningEvaluation       = "learning_evaluation"
	WorkerLearningInsightAggregate = "learning_insight_aggregate"
	WorkerIdentityBackfill         = "identity_backfill"
)

// WorkerMetadata mô tả worker (module, domain, mô tả chức năng).
// Domain: ads, order, customer, notification, system — để nhóm/filter theo domain nghiệp vụ.
type WorkerMetadata struct {
	Module      string `json:"module"`
	Domain      string `json:"domain"` // Phân loại theo domain: ads, order, customer, notification, system
	Description string `json:"description"`
}

// workerMetadataMap map tên worker → metadata (module + domain + mô tả).
var workerMetadataMap = map[string]WorkerMetadata{
	WorkerReportDirtyAds:           {Module: "report", Domain: "ads", Description: "Tính toán lại báo cáo ads_daily khi có dirty periods"},
	WorkerReportDirtyOrder:         {Module: "report", Domain: "order", Description: "Tính toán lại báo cáo order_daily khi có dirty periods"},
	WorkerReportDirtyCustomer:      {Module: "report", Domain: "customer", Description: "Tính toán lại báo cáo customer_daily khi có dirty periods"},
	WorkerReportRedisTouchFlush:    {Module: "report", Domain: "system", Description: "Một worker, ba nhịp flush Redis→MarkDirty (ads/order/customer); env REPORT_REDIS_TOUCH_FLUSH_INTERVAL_*_SEC + POLL_TICK"},
	WorkerDelivery:                 {Module: "notification", Domain: "notification", Description: "Xử lý hàng đợi gửi thông báo (email, Telegram, SMS...)"},
	WorkerDeliveryCleanup:          {Module: "notification", Domain: "notification", Description: "Dọn các item bị kẹt trong hàng đợi delivery"},
	WorkerCommandCleanup:           {Module: "notification", Domain: "system", Description: "Dọn command cũ hết hạn"},
	WorkerAgentCommandCleanup:      {Module: "notification", Domain: "system", Description: "Dọn agent command cũ hết hạn"},
	WorkerAgentActivityCleanup:     {Module: "notification", Domain: "system", Description: "Dọn agent activity log cũ"},
	WorkerCrmPendingMerge:          {Module: "crm", Domain: "customer", Description: "Queue merge L1→L2 khách hàng (customer_pending_merge)"},
	WorkerCrmBulk:                  {Module: "crm", Domain: "customer", Description: "Xử lý bulk job cập nhật customer hàng loạt"},
	WorkerAdsExecution:             {Module: "ads", Domain: "ads", Description: "Thực thi các đề xuất quảng cáo đã được duyệt"},
	WorkerAdsAutoPropose:           {Module: "ads", Domain: "aidecision", Description: "Auto propose (aidecision/adsautop → executor.propose_requested); code AID, đăng ký cạnh worker ads"},
	WorkerAdsCircuitBreaker:        {Module: "ads", Domain: "ads", Description: "Giám sát và tạm dừng account khi lỗi Meta API liên tục"},
	WorkerAdsDailyScheduler:        {Module: "ads", Domain: "ads", Description: "Lên lịch chạy mode detection và các task ads hàng ngày"},
	WorkerAdsPancakeHeartbeat:      {Module: "ads", Domain: "ads", Description: "Gửi heartbeat đến Pancake để đồng bộ trạng thái"},
	WorkerAdsCounterfactual:        {Module: "ads", Domain: "ads", Description: "Đánh giá kill đã qua 4h → counterfactual outcomes (FolkForm v4.1)"},
	WorkerClassificationFull:       {Module: "crm", Domain: "customer", Description: "Refresh toàn bộ phân loại khách hàng (lifecycle, journey, momentum) — 24h"},
	WorkerClassificationSmart:      {Module: "crm", Domain: "customer", Description: "Refresh phân loại thông minh — chỉ khách gần ngưỡng lifecycle — 6h"},
	WorkerCixIntelCompute:          {Module: "cix", Domain: "cix", Description: "Poll cix_intel_compute — Raw→L1→L2→L3 qua Rule Engine (cùng quy ước *_intel_compute); enqueue từ AI Decision consumer (cix.analysis_requested)"},
	WorkerAIDecisionConsumer:       {Module: "aidecision", Domain: "aidecision", Description: "Consume decision_events_queue (PriorityCritical). Bypass pause/throttle: WORKER_AI_DECISION_CONSUMER_IGNORE_RESOURCE_THROTTLE=1"},
	WorkerAIDecisionDebounce:       {Module: "aidecision", Domain: "aidecision", Description: "Flush debounce state hết window → emit message.batch_ready"},
	WorkerAIDecisionClosure:        {Module: "aidecision", Domain: "aidecision", Description: "Đóng case quá hạn với closed_timeout"},
	WorkerOrderIntelCompute: {Module: "orderintel", Domain: "order", Description: "Poll order_intel_compute — tính Raw→L1→L2→L3→Flags, emit order_intel_recomputed"},
	WorkerAdsIntelCompute: {Module: "ads", Domain: "ads", Description: "Poll ads_intel_compute — ApplyAdsIntelligenceRecompute / RecalculateAll (không tính trong consumer AI Decision)"},
	WorkerCrmContext:               {Module: "crm", Domain: "customer", Description: "Consume customer.context_requested → load customer → emit customer.context_ready"},
	WorkerCrmIntelCompute:   {Module: "crm", Domain: "customer", Description: "Poll customer_intel_compute — RefreshMetrics / Recalculate* / classification_refresh (không tính trong consumer AI Decision)"},
	WorkerLearningRuleSuggestion:   {Module: "learning", Domain: "learning", Description: "Phân tích learning_cases → tạo rule suggestions (Phase 3, LEARNING_RULE_SUGGESTION_ENABLED=true)"},
	WorkerLearningEvaluation:       {Module: "learning", Domain: "learning", Description: "Batch tính evaluation (outcome_class, error_attribution) cho learning_cases"},
	WorkerLearningInsightAggregate: {Module: "learning", Domain: "learning", Description: "Aggregate anonymized learning stats cross-merchant (Phase 3)"},
	WorkerIdentityBackfill:         {Module: "identity", Domain: "system", Description: "Backfill uid, sourceIds, links cho document cũ (4 lớp identity)"},
}

// GetAllWorkerMetadata trả về metadata tất cả workers (để API GET).
func GetAllWorkerMetadata() map[string]WorkerMetadata {
	out := make(map[string]WorkerMetadata, len(AllWorkerNames))
	for _, name := range AllWorkerNames {
		if m, ok := workerMetadataMap[name]; ok {
			out[name] = m
		} else {
			out[name] = WorkerMetadata{Module: "unknown", Domain: "system", Description: "Chưa có mô tả"}
		}
	}
	return out
}

// defaultWorkerPriorities mức ưu tiên mặc định cho từng worker.
// Số nhỏ = ưu tiên cao hơn. 1=Critical, 2=High, 3=Normal, 4=Low, 5=Lowest.
var defaultWorkerPriorities = map[string]Priority{
	WorkerReportDirtyAds:           PriorityCritical,
	WorkerReportDirtyOrder:         PriorityCritical,
	WorkerReportDirtyCustomer:      PriorityCritical,
	WorkerReportRedisTouchFlush:    PriorityNormal,
	WorkerDelivery:                 PriorityHigh,
	WorkerDeliveryCleanup:          PriorityLow,
	WorkerCommandCleanup:           PriorityLow,
	WorkerAgentCommandCleanup:      PriorityLow,
	WorkerAgentActivityCleanup:     PriorityLow,
	WorkerCrmPendingMerge:          PriorityHigh,
	WorkerCrmBulk:                  PriorityLow,
	WorkerAdsExecution:             PriorityNormal,
	WorkerAdsAutoPropose:           PriorityNormal,
	WorkerAdsCircuitBreaker:        PriorityNormal,
	WorkerAdsDailyScheduler:        PriorityNormal,
	WorkerAdsPancakeHeartbeat:      PriorityNormal,
	WorkerAdsCounterfactual:        PriorityLow,
	WorkerClassificationFull:       PriorityLowest,
	WorkerClassificationSmart:      PriorityLowest,
	WorkerCixIntelCompute:          PriorityNormal,
	WorkerAIDecisionConsumer:       PriorityCritical, // Paused (RAM/CPU): chỉ Critical còn chạy — consumer không được để High
	WorkerAIDecisionDebounce:       PriorityNormal,
	WorkerAIDecisionClosure:        PriorityLow,
	WorkerOrderIntelCompute: PriorityHigh,
	WorkerAdsIntelCompute: PriorityHigh,
	WorkerCrmContext:               PriorityNormal,
	WorkerCrmIntelCompute:   PriorityHigh,
	WorkerLearningRuleSuggestion:   PriorityLowest,
	WorkerLearningEvaluation:       PriorityLowest,
	WorkerLearningInsightAggregate: PriorityLowest,
	WorkerIdentityBackfill:         PriorityLowest,
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
// Tên worker chuẩn hóa lowercase (report_dirty, crm_pending_merge, ...).
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
	WorkerReportDirtyAds, WorkerReportDirtyOrder, WorkerReportDirtyCustomer,
	WorkerReportRedisTouchFlush,
	WorkerDelivery, WorkerDeliveryCleanup,
	WorkerCommandCleanup, WorkerAgentCommandCleanup, WorkerAgentActivityCleanup,
	WorkerCrmPendingMerge, WorkerCrmBulk,
	WorkerAdsExecution, WorkerAdsAutoPropose, WorkerAdsCircuitBreaker,
	WorkerAdsDailyScheduler, WorkerAdsPancakeHeartbeat, WorkerAdsCounterfactual,
	WorkerClassificationFull, WorkerClassificationSmart,
	WorkerCixIntelCompute,
	WorkerAIDecisionConsumer,
	WorkerAIDecisionDebounce,
	WorkerAIDecisionClosure,
	WorkerOrderIntelCompute,
	WorkerAdsIntelCompute,
	WorkerCrmContext,
	WorkerCrmIntelCompute,
	WorkerLearningRuleSuggestion,
	WorkerLearningEvaluation,
	WorkerLearningInsightAggregate,
	WorkerIdentityBackfill,
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
var poolSizeEnvKeys = map[string]string{
	WorkerDelivery:            "WORKER_POOL_SIZE_DELIVERY",
	WorkerReportDirtyAds:      "WORKER_POOL_SIZE_REPORT_DIRTY_ADS",
	WorkerReportDirtyOrder:    "WORKER_POOL_SIZE_REPORT_DIRTY_ORDER",
	WorkerReportDirtyCustomer: "WORKER_POOL_SIZE_REPORT_DIRTY_CUSTOMER",
	WorkerAdsExecution:        "WORKER_POOL_SIZE_ADS_EXECUTION",
	WorkerAIDecisionConsumer:  "WORKER_POOL_SIZE_AI_DECISION_CONSUMER",
}

// poolSizeOverrides override pool size qua API runtime.
var (
	poolSizeOverrides   = make(map[string]int)
	poolSizeOverridesMu sync.RWMutex
)

// SetPoolSizeOverride đặt override pool size cho worker (runtime, qua API).
func SetPoolSizeOverride(workerName string, size int) {
	name := strings.ToLower(strings.TrimSpace(workerName))
	if name == "" {
		return
	}
	poolSizeOverridesMu.Lock()
	defer poolSizeOverridesMu.Unlock()
	if size >= 1 {
		poolSizeOverrides[name] = size
	} else {
		delete(poolSizeOverrides, name)
	}
}

// ClearPoolSizeOverride xóa override pool size.
func ClearPoolSizeOverride(workerName string) {
	SetPoolSizeOverride(workerName, 0)
}

// GetPoolSizeOverrides trả về override hiện tại (để API GET).
func GetPoolSizeOverrides() map[string]int {
	poolSizeOverridesMu.RLock()
	defer poolSizeOverridesMu.RUnlock()
	out := make(map[string]int, len(poolSizeOverrides))
	for k, v := range poolSizeOverrides {
		out[k] = v
	}
	return out
}

// GetPoolSize trả về pool size cơ bản cho worker. Thứ tự: API override > env > default.
func GetPoolSize(workerName string, defaultSize int) int {
	canonical := strings.ToLower(strings.TrimSpace(workerName))
	poolSizeOverridesMu.RLock()
	if n, ok := poolSizeOverrides[canonical]; ok && n >= 1 {
		poolSizeOverridesMu.RUnlock()
		return n
	}
	poolSizeOverridesMu.RUnlock()

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

// defaultPoolSizes pool size mặc định cho workers dùng pool.
var defaultPoolSizes = map[string]int{
	WorkerDelivery:            6,
	WorkerReportDirtyAds:      6,
	WorkerReportDirtyOrder:    6,
	WorkerReportDirtyCustomer: 6,
	WorkerAdsExecution:        4,
	WorkerAIDecisionConsumer:  5, // mặc định ≥ min pool; env WORKER_POOL_SIZE_AI_DECISION_CONSUMER / API; sàn tối thiểu: MinPoolSizeAIDecisionConsumer
}

// MinPoolSizeAIDecisionConsumer sàn pool tối thiểu cho ai_decision_consumer (mặc định 2).
// Override: AI_DECISION_CONSUMER_MIN_POOL=1..128.
func MinPoolSizeAIDecisionConsumer() int {
	n := 2
	if v := strings.TrimSpace(os.Getenv("AI_DECISION_CONSUMER_MIN_POOL")); v != "" {
		if x, err := strconv.Atoi(v); err == nil && x >= 1 && x <= 128 {
			n = x
		}
	}
	return n
}

// EffectiveAIDecisionConsumerParallelSlots số slot song song sau throttle + áp sàn min (đồng bộ với consumer thực tế).
func EffectiveAIDecisionConsumerParallelSlots() int {
	minP := MinPoolSizeAIDecisionConsumer()
	base := GetPoolSize(WorkerAIDecisionConsumer, minP)
	if base < minP {
		base = minP
	}
	p := GetPriority(WorkerAIDecisionConsumer, PriorityCritical)
	slots := GetEffectivePoolSizeForWorker(WorkerAIDecisionConsumer, base, p)
	if slots < minP {
		slots = minP
	}
	if slots < 1 {
		slots = 1
	}
	return slots
}

// GetAllWorkerPoolSizes trả về pool size hiệu dụng cho workers có pool (để API GET).
func GetAllWorkerPoolSizes() map[string]int {
	out := make(map[string]int)
	for name, def := range defaultPoolSizes {
		out[name] = GetPoolSize(name, def)
	}
	return out
}

// GetPriority trả về mức ưu tiên cho worker. Ưu tiên: API override > env > mặc định.
//
// Env: WORKER_PRIORITY_<NAME>=1|2|3|4|5 (NAME dạng REPORT_DIRTY, CRM_PENDING_MERGE...)
// API: SetPriorityOverride (runtime)
//
// Tham số:
//   - workerName: tên worker (report_dirty, crm_pending_merge, ...)
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
	// 2. Override từ env (WORKER_PRIORITY_REPORT_DIRTY, WORKER_PRIORITY_CRM_PENDING_MERGE...)
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

// IsWorkerBypassingResourceThrottle true nếu worker được cấu hình bỏ qua pause/throttle tài nguyên.
// Env: WORKER_RESOURCE_THROTTLE_BYPASS — danh sách tên worker (phẩy) hoặc * / all;
// WORKER_AI_DECISION_CONSUMER_IGNORE_RESOURCE_THROTTLE=1 — chỉ consumer AI Decision.
func IsWorkerBypassingResourceThrottle(workerName string) bool {
	canonical := strings.ToLower(strings.TrimSpace(workerName))
	if canonical == "" {
		return false
	}
	if v := strings.TrimSpace(os.Getenv("WORKER_AI_DECISION_CONSUMER_IGNORE_RESOURCE_THROTTLE")); v == "1" || strings.EqualFold(v, "true") {
		if canonical == strings.ToLower(WorkerAIDecisionConsumer) {
			return true
		}
	}
	raw := strings.TrimSpace(os.Getenv("WORKER_RESOURCE_THROTTLE_BYPASS"))
	if raw == "" {
		return false
	}
	if raw == "*" || strings.EqualFold(raw, "all") {
		return true
	}
	for _, part := range strings.Split(raw, ",") {
		p := strings.ToLower(strings.TrimSpace(part))
		if p != "" && p == canonical {
			return true
		}
	}
	return false
}
