// Package decisionlive — Timeline theo trace + feed theo org (RAM process), WebSocket, metrics trung tâm chỉ huy.
//
// Luồng dữ liệu timeline (một traceId trong một org), khớp với Publish trong publish.go:
//
//	Ghi (server, khi AI_DECISION_LIVE_ENABLED bật): Publish bước 4 gắn Seq vào ring (orgHex:traceId),
//	  bước 6a broadcast WS theo trace; mỗi DecisionLiveEvent = một hàng timeline (field phase = giai đoạn pipeline).
//	Đọc REST: Timeline(org, traceId) = snapshot ring + backfill field suy ra (opsTier, feed…).
//	Đọc WebSocket: Subscribe(org, traceId) trước → Timeline replay → gửi client → drain trùng Seq → nhận tiếp từ liveCh.
//
// Live tắt: không append ring / không WS; client chỉ thấy mốc cũ trong RAM (nếu có) tới khi restart — metrics vẫn cập nhật qua nhánh Publish chỉ metrics.
//
// Org-live: Publish bước 6b đẩy cùng mốc vào ring org (FeedSeq); GET/WS org dùng OrgTimeline / OrgTimelineForAPI / SubscribeOrg (xem handler live).
//
// Mỗi Publish tạo đúng một DecisionLiveEvent trên timeline; DetailBullets / DetailSections chỉ là nội dung trong cùng mốc, không tách thành live_event khác.
package decisionlive

// Phase — Giá trị gợi ý cho field DecisionLiveEvent.phase (một phase = một mốc timeline).
// UI sắp theo tsMs + Seq; các hằng dưới đây giúp thống nhất ngữ nghĩa giữa service gọi Publish và màn drill-down.
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

	// PhaseIntelDomainCompute* — worker ngoài consumer AID (crm/order/cix/ads_intel_compute, CRM context…) — nối timeline theo traceId.
	PhaseIntelDomainComputeStart = "intel_domain_compute_start"
	PhaseIntelDomainComputeDone  = "intel_domain_compute_done"
	PhaseIntelDomainComputeError = "intel_domain_compute_error"
)

// Severity — mức độ hiển thị.
const (
	SeverityInfo  = "info"
	SeverityWarn  = "warn"
	SeverityError = "error"
)

// Nhãn kind cho DecisionLiveProcessNode — gợi ý render UI (cây / timeline phụ).
const (
	ProcessTraceKindBranch   = "branch"   // Nhóm bước con
	ProcessTraceKindStep     = "step"     // Bước thực thi
	ProcessTraceKindDecision = "decision" // Điểm rẽ nhánh (if/else nghiệp vụ)
	ProcessTraceKindOutcome  = "outcome"  // Kết quả tốt / kết thúc nhánh
	ProcessTraceKindSkip     = "skip"     // Bỏ qua có chủ đích
	ProcessTraceKindError    = "error"    // Lỗi / ngoại lệ
)

// DecisionLiveProcessNode — Một nút trong cây «quá trình xử lý» của đúng một mốc timeline (giải thích vì sao xử lý như vậy).
// Client có thể render phẳng theo DFS hoặc indent theo children.
type DecisionLiveProcessNode struct {
	Kind     string                    `json:"kind"`
	Key      string                    `json:"key,omitempty"`
	LabelVi  string                    `json:"labelVi"`
	DetailVi string                    `json:"detailVi,omitempty"`
	Children []DecisionLiveProcessNode `json:"children,omitempty"`
}

// DecisionLiveDetailSection — accordion mở rộng; ưu tiên **ít section**, mỗi section ngắn — tránh nhiều khối trùng ý với detailBullets.
type DecisionLiveDetailSection struct {
	Title string   `json:"title"`
	Items []string `json:"items"`
}

// TraceStep — Chi tiết tùy chọn trong đúng một mốc (cùng Seq/phase); không thay thế chuỗi các DecisionLiveEvent trên timeline.
type TraceStep struct {
	Index     int                    `json:"index"`
	Kind      string                 `json:"kind"` // rule | llm | policy | propose | io
	Title     string                 `json:"title"`
	Reasoning string                 `json:"reasoning,omitempty"`
	InputRef  map[string]interface{} `json:"inputRef,omitempty"`
	OutputRef map[string]interface{} `json:"outputRef,omitempty"`
}

// DecisionLiveEvent — Một mốc trên timeline của traceId (traceId + seq + phase + tsMs). Chuỗi theo thời gian = luồng xử lý đã Publish.
type DecisionLiveEvent struct {
	SchemaVersion int    `json:"schemaVersion"`
	Stream        string `json:"stream"`
	Seq           int64  `json:"seq"`
	// FeedSeq thứ tự trong buffer live theo tổ chức (màn hình event stream toàn org).
	FeedSeq int64  `json:"feedSeq,omitempty"`
	TsMs    int64  `json:"tsMs"`
	Phase   string `json:"phase"`
	// PhaseLabelVi — nhãn phase tiếng Việt (một dòng phụ trên thẻ swimlane / tooltip).
	PhaseLabelVi  string `json:"phaseLabelVi,omitempty"`
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
	// E2EStage / E2EStepID — tham chiếu luồng chuẩn G1–G6 (docs/flows/bang-pha-buoc-event-e2e.md); enrich trong Publish.
	E2EStage       string `json:"e2eStage,omitempty"`
	E2EStepID      string `json:"e2eStepId,omitempty"`
	E2EStepLabelVi string `json:"e2eStepLabelVi,omitempty"`
	OrgIDHex       string `json:"orgId,omitempty"`
	// SourceKind: sau enrich = nhóm hiển thị (conversation | order | pos_sync | meta_sync | decision | queue | unknown | …); client cũ chỉ đọc field này cho chip.
	SourceKind  string `json:"sourceKind,omitempty"`
	SourceTitle string `json:"sourceTitle,omitempty"`
	// FeedSourceCategory / FeedSourceLabelVi — nhóm chip “Nguồn” trên UI (mở rộng: meta_sync, pos_sync, crm…).
	// «Khác» / other ở đây là nhóm nguồn dữ liệu, không phải tên module xử lý — cột swimlane theo module dùng businessDomain + businessDomainLabelVi.
	FeedSourceCategory string `json:"feedSourceCategory"`
	FeedSourceLabelVi  string `json:"feedSourceLabelVi"`
	// BusinessDomain — mã module đang xử lý mốc (queue/worker phát Publish): consumer AID → aidecision; worker intel miền → crm|order|cix|ads; khớp docs/flows/bang-pha-buoc-event-e2e.md §1.2.
	BusinessDomain string `json:"businessDomain,omitempty"`
	// BusinessDomainLabelVi — nhãn tiếng Việt + mã trong ngoặc (vd. «AI Decision (aidecision)») để UI không gộp nhầm vào «Khác» của feedSource.
	BusinessDomainLabelVi string `json:"businessDomainLabelVi,omitempty"`
	// OpsTier — phân loại vận hành (backend): decision | pipeline | operational | unknown (luôn serialize để client không phụ thuộc omitempty).
	OpsTier          string `json:"opsTier"`
	OpsTierLabelVi   string `json:"opsTierLabelVi"`
	Summary          string `json:"summary"`
	ReasoningSummary string `json:"reasoningSummary,omitempty"`
	// UiTitle — tiêu đề một dòng ưu tiên cho node swimlane (step → phase + nguồn); đồng bộ logic với persist Mongo.
	UiTitle string `json:"uiTitle,omitempty"`
	// UiSummary — tóm tắt ngắn cùng node (ưu tiên summary, fallback reasoningSummary).
	UiSummary    string                 `json:"uiSummary,omitempty"`
	DecisionMode string                 `json:"decisionMode,omitempty"`
	Confidence   float64                `json:"confidence,omitempty"`
	Refs         map[string]string      `json:"refs,omitempty"`
	Step         *TraceStep             `json:"step,omitempty"`
	Detail       map[string]interface{} `json:"detail,omitempty"`
	// DetailBullets: gạch đầu dòng trong cùng một mốc (không phải mốc timeline riêng) — không lặp metadata kỹ thuật (để ở refs/detail).
	DetailBullets []string `json:"detailBullets,omitempty"`
	// DetailSections: nhóm nội dung theo mục trong cùng một mốc (vd. danh sách gợi ý). Không thay thế chuỗi live_event.
	DetailSections []DecisionLiveDetailSection `json:"detailSections,omitempty"`
	// ProcessTrace: cây bước / nhánh logic đã đi qua cho mốc này (consumer queue, sau này có thể bổ sung engine…).
	ProcessTrace []DecisionLiveProcessNode `json:"processTrace,omitempty"`
	// OutcomeKind — Phân loại kết quả (ổn định để lọc); xem hằng Outcome* trong outcome.go.
	OutcomeKind string `json:"outcomeKind,omitempty"`
	// OutcomeAbnormal — true nếu là trường hợp bất thường / cần chú ý (lỗi, bỏ qua, thiếu dữ liệu, …).
	OutcomeAbnormal bool `json:"outcomeAbnormal,omitempty"`
	// OutcomeLabelVi — Nhãn ngắn tiếng Việt cho chip (điền trong EnrichLiveOutcomeMetadata nếu trống).
	OutcomeLabelVi string `json:"outcomeLabelVi,omitempty"`
}
