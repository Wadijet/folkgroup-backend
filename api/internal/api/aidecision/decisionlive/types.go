// Package decisionlive — WebSocket + buffer replay cho luồng AI Decision (live timeline).
//
// Mỗi lần Publish tạo một DecisionLiveEvent: đó là một mốc trên timeline (theo phase), không lồng thêm
// “lớp bước” thứ hai. DetailBullets / DetailSections chỉ là nội dung hiển thị trong cùng một mốc đó.
package decisionlive

// Phase — các giai đoạn cố định (causal order) để UI dựng timeline.
const (
	PhaseQueued    = "queued"    // Đã ghi queue execute_requested
	PhaseConsuming = "consuming" // Worker bắt đầu ExecuteWithCase
	PhaseSkipped   = "skipped"   // Bỏ qua (không có CIX payload)
	PhaseParse     = "parse"     // Đã đọc actionSuggestions từ CIX
	PhaseLLM       = "llm"       // Đang / đã gọi LLM (rule-first + ambiguity)
	PhaseDecision  = "decision"  // Đã chọn danh sách action + mode + reasoning
	PhasePolicy    = "policy"    // Đã tách auto vs cần duyệt
	PhasePropose   = "propose"   // Đã tạo proposal / auto-approve
	PhaseEmpty     = "empty"     // Không có action sau khi quyết định
	PhaseDone      = "done"      // Hoàn tất pipeline
	PhaseError     = "error"     // Lỗi không chặn toàn bộ (ghi nhận)

	// Phase queue consumer — timeline cho mọi event_type (tránh trùng PhaseConsuming với gauge no-trace của worker).
	PhaseQueueProcessing    = "queue_processing"    // Worker bắt đầu xử lý job queue
	PhaseQueueDone          = "queue_done"          // Job queue hoàn tất (không lỗi consumer)
	PhaseQueueError         = "queue_error"         // Job queue lỗi tại consumer
	PhaseDatachangedEffects = "datachanged_effects" // Đã chạy applyDatachangedSideEffects

	// Pipeline nghiệp vụ (điều phối / CIX / đủ điều kiện execute) — ghi đủ bước cho audit + UI.
	PhaseOrchestrate   = "orchestrate"    // ResolveOrCreate + emit event con (hội thoại / đơn / …)
	PhaseCixIntegrated = "cix_integrated" // Đã nhận kết quả CIX vào case
	PhaseExecuteReady  = "execute_ready"  // Đủ context — chuẩn bị gửi execute_requested
	// PhaseAdsEvaluate — đánh giá ACTION_RULE / metrics trên chiến dịch Meta (luồng ads_optimization_decision, không qua CIX).
	PhaseAdsEvaluate = "ads_evaluate"
)

// Severity — mức độ hiển thị.
const (
	SeverityInfo  = "info"
	SeverityWarn  = "warn"
	SeverityError = "error"
)

// DecisionLiveDetailSection nhóm gạch đầu dòng trong một mốc timeline (UI accordion). Không tương đương một live_event riêng.
type DecisionLiveDetailSection struct {
	Title string   `json:"title"`
	Items []string `json:"items"`
}

// TraceStep — mô tả ngắn cho đúng một mốc DecisionLiveEvent (drill-down / builder), không phải danh sách bước timeline.
type TraceStep struct {
	Index     int                    `json:"index"`
	Kind      string                 `json:"kind"` // rule | llm | policy | propose | io
	Title     string                 `json:"title"`
	Reasoning string                 `json:"reasoning,omitempty"`
	InputRef  map[string]interface{} `json:"inputRef,omitempty"`
	OutputRef map[string]interface{} `json:"outputRef,omitempty"`
}

// DecisionLiveEvent — một mốc trên timeline (một hàng trong feed / replay). Chuỗi nhiều event = luồng thực hiện.
type DecisionLiveEvent struct {
	SchemaVersion int    `json:"schemaVersion"`
	Stream        string `json:"stream"`
	Seq           int64  `json:"seq"`
	// FeedSeq thứ tự trong buffer live theo tổ chức (màn hình event stream toàn org).
	FeedSeq       int64  `json:"feedSeq,omitempty"`
	TsMs          int64  `json:"tsMs"`
	Phase         string `json:"phase"`
	Severity      string `json:"severity"`
	TraceID       string `json:"traceId"`
	CorrelationID string `json:"correlationId,omitempty"`
	// W3CTraceID trace-id chuẩn W3C (32 hex) — neo với OTel / traceparent; sinh từ khóa nội bộ (traceId) nếu chưa set.
	W3CTraceID string `json:"w3cTraceId,omitempty"`
	// SpanID span-id chuẩn W3C (16 hex) — mỗi bước Publish một span mới nếu chưa gán.
	SpanID string `json:"spanId,omitempty"`
	// ParentSpanID span cha (16 hex) — phân nhánh / bất đồng bộ; rỗng = root trong luồng con.
	ParentSpanID string `json:"parentSpanId,omitempty"`
	// DecisionCaseID neo tới decision_cases_runtime (audit / UI theo case).
	DecisionCaseID string `json:"decisionCaseId,omitempty"`
	OrgIDHex       string `json:"orgId,omitempty"`
	// SourceKind: sau enrich = nhóm hiển thị (conversation | order | pos_sync | meta_sync | decision | queue | unknown | …); client cũ chỉ đọc field này cho chip.
	SourceKind  string `json:"sourceKind,omitempty"`
	SourceTitle string `json:"sourceTitle,omitempty"`
	// FeedSourceCategory / FeedSourceLabelVi — nhóm chip “Nguồn” trên UI (mở rộng: meta_sync, pos_sync, crm…).
	FeedSourceCategory string `json:"feedSourceCategory"`
	FeedSourceLabelVi  string `json:"feedSourceLabelVi"`
	// OpsTier — phân loại vận hành (backend): decision | pipeline | operational | unknown (luôn serialize để client không phụ thuộc omitempty).
	OpsTier          string                 `json:"opsTier"`
	OpsTierLabelVi   string                 `json:"opsTierLabelVi"`
	Summary          string                 `json:"summary"`
	ReasoningSummary string                 `json:"reasoningSummary,omitempty"`
	DecisionMode     string                 `json:"decisionMode,omitempty"`
	Confidence       float64                `json:"confidence,omitempty"`
	Refs             map[string]string      `json:"refs,omitempty"`
	Step             *TraceStep             `json:"step,omitempty"`
	Detail           map[string]interface{} `json:"detail,omitempty"`
	// DetailBullets: gạch đầu dòng trong cùng một mốc (không phải mốc timeline riêng) — không lặp metadata kỹ thuật (để ở refs/detail).
	DetailBullets []string `json:"detailBullets,omitempty"`
	// DetailSections: nhóm nội dung theo mục trong cùng một mốc (vd. danh sách gợi ý). Không thay thế chuỗi live_event.
	DetailSections []DecisionLiveDetailSection `json:"detailSections,omitempty"`
}
