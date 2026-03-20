# Phương Án: Triển Khai AI Decision Engine & Learning Engine (Approval là phần của Executor)

**Ngày:** 2026-03-18  
**Cập nhật:** 2026-03-19 — Đổi tên module: decision→ai-decision+learning, approval+delivery→executor. API: /ai-decision/execute, /learning/cases, /executor/actions/*, /executor/send, /executor/execute, /executor/history.  
**Nguyên tắc:** Mọi quyết định cần được duyệt mới chuyển sang thực hiện. Có config để sau này duyệt tự động hoặc duyệt bằng AI.

**Tham chiếu vision:** [rule-intelligence.md](../02-architecture/core/rule-intelligence.md), [learning-engine.md](../02-architecture/core/learning-engine.md), [RASOAT_MODULE_KHUNG_XUONG.md](./RASOAT_MODULE_KHUNG_XUONG.md), [07 - ai-decision.md](../../docs-shared/architecture/vision/07%20-%20ai-decision.md)

---

## 0. Trạng Thái Triển Khai (2026-03-19)

| Hạng mục | Trạng thái | Chi tiết |
|----------|------------|----------|
| **Learning engine (Decision Brain)** | ✅ Có | `learning/` — LearningCase, BuildLearningCaseFromAction, BuildLearningCaseFromCIOChoice, collection decision_cases |
| **AI Decision Engine** | ✅ Có | `aidecision/` — Execute, ReceiveCixPayload, applyPolicy, proposeCixAction, proposeAndApproveAutoCixAction |
| **Executor** | ✅ Có | `executor/` — Approval Gate + Execution: /executor/actions/*, /executor/send, /executor/execute, /executor/history |
| **CIX Executor** | ✅ Có | executors/cix → trigger_fast_response, escalate_to_senior, assign_to_human_sale, prioritize_followup → Delivery |
| **ApprovalModeConfig** | ❌ Chưa | Config-driven mode (human/auto/ai) theo domain/org |
| **ResolveImmediate** | ❌ Chưa | Engine tự resolve sau Propose theo config |
| **Delivery Gate** | ✅ Có | POST /executor/execute — handler.delivery chỉ nhận từ Executor (DELIVERY_ALLOW_DIRECT_USE deprecate) |

---

## 1. Tổng Quan

### 1.1 AI Decision Engine là gì?

**AI Decision Engine** là tầng ra quyết định thống nhất của hệ thống AI Commerce (theo vision 07 - ai-decision.md):
- **Input:** Context từ nhiều nguồn (CIX, Ads, CIO, CRM, Rule Engine)
- **Output:** Execution Plan (danh sách action cần thực thi)
- **Gate:** Mọi action phải qua **Approval** trước khi gửi sang Execution Engine (Delivery)

### 1.2 Nguyên Tắc Thiết Kế

| Nguyên tắc | Mô tả |
|------------|-------|
| **Approval Gate** | Không có action nào đi thẳng từ AI Decision Engine → Delivery. Luôn qua Approval (Propose → Approve/Reject → Execute). |
| **Config Approval Mode** | Mỗi domain/org có thể cấu hình: `human` (duyệt tay), `auto` (tự động duyệt), `ai` (AI duyệt — Phase sau). |
| **Unified Flow** | Một luồng approval duy nhất cho tất cả domain (ads, cio, cix, delivery). |
| **Audit Trail** | Mọi quyết định (approve/reject/auto) đều ghi vào Decision Brain (DecisionCase) để học tập. |

---

## 2. Đánh Giá Module Approval Hiện Có

### 2.1 Cấu Trúc Hiện Tại

```
api/pkg/approval/           # Engine generic
├── engine.go               # Propose, Approve, Reject, ExecuteOne
├── types.go                # ActionPending, Status
└── interfaces.go           # Storage, Notifier, Executor

api/internal/approval/       # Bridge (pkg/approval)
├── service.go              # Delegate sang pkg/approval
├── init.go                 # NewEngine(storage, notifier)
├── executor.go             # RegisterExecutor
└── bridge/
    ├── storage.go          # MongoDB
    └── notifier.go         # NotifyTrigger

api/internal/api/executor/   # API layer — Approval Gate + Execution (thay approval + delivery router)
├── handler/handler.executor.action.go
└── router/routes.go        # /executor/actions/*, /executor/send, /executor/execute, /executor/history
```

**Điểm mạnh:**
- Engine độc lập, domain-agnostic
- Propose → Approve/Reject → Execute rõ ràng
- Deferred execution (queue) cho domain ads
- Executor registry theo domain

### 2.2 Vấn Đề Hiện Tại

| Vấn đề | Chi tiết |
|--------|----------|
| **Phân mảnh approval** | CIO có luồng riêng (`cio_plan_executions`: pending_approval → approve) **không dùng** pkg/approval. Content draft có approval trên node. |
| **Delivery không qua approval** | Đã sửa: `POST /executor/execute` — handler có gate (DELIVERY_ALLOW_DIRECT_USE deprecate). |
| **Config rải rác** | Ads: `ads_approval_config`, `ads_meta_config.account.automationConfig.autoProposeEnabled`, `ActionRules[].autoApprove`. Không có config thống nhất. |
| **Auto-approve logic nằm trong scheduler** | `service.ads.scheduler.go` gọi `approval.Approve()` trực tiếp khi rule có `autoApprove`. Logic không tách biệt. |
| **AI Decision Engine** | Đã tích hợp: `AIDecisionService.Execute` — Propose, gọi executor/approval, Delivery qua pkg/approval. |

### 2.3 Bảng So Sánh Luồng Approval Theo Domain

| Domain | Collection/Entity | Luồng hiện tại | Dùng pkg/approval? |
|--------|-------------------|----------------|--------------------|
| **ads** | action_pending_approval | Propose → Approve/Reject → Execute (worker) | ✅ Có |
| **cio** | cio_plan_executions | CreateExecutionRequest → ApproveExecution/RejectExecution → RunExecution | ❌ Không |
| **cix** | — | Chưa triển khai | Kế hoạch dùng |
| **delivery** | — | Nhận actions trực tiếp, không approval | ❌ Không |
| **content** | content_draft_nodes | approvalStatus: draft→pending→approved | ❌ Không (status trên node) |

---

## 3. Đề Xuất Cấu Trúc Lại

### 3.1 Kiến Trúc Mới: AI Decision Engine + Approval Gate

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    AI DECISION ENGINE (Tầng ra quyết định)                  │
├─────────────────────────────────────────────────────────────────────────────┤
│  Input: CIXPayload, AdsContext, CIOContext, CustomerCtx, RuleOutput           │
│  Output: ProposedActions[] (mỗi action có approvalMode: human|auto|ai)       │
└─────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                    APPROVAL GATE (Cổng duyệt thống nhất)                      │
├─────────────────────────────────────────────────────────────────────────────┤
│  • Propose(action) → ActionPending (status=pending)                           │
│  • Resolve(actionId, mode):                                                  │
│      - mode=human: Chờ user Approve/Reject qua API                           │
│      - mode=auto:  Tự động Approve (theo config domain/org)                  │
│      - mode=ai:    Gọi AI Approver (Phase sau)                               │
│  • Sau khi Approved → Execute (worker hoặc sync)                             │
└─────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                    EXECUTION ENGINE (Delivery)                                │
├─────────────────────────────────────────────────────────────────────────────┤
│  Chỉ nhận actions từ Approval Gate (đã Approved). Không nhận trực tiếp.      │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Config Approval Mode Thống Nhất

**Collection mới:** `approval_mode_config` (hoặc mở rộng `ads_approval_config` → `approval_config` generic)

```go
// ApprovalModeConfig cấu hình chế độ duyệt theo domain/org.
type ApprovalModeConfig struct {
    ID                   primitive.ObjectID `bson:"_id,omitempty"`
    Domain               string             `bson:"domain"`      // ads | cio | cix | delivery
    ScopeKey             string             `bson:"scopeKey"`   // adAccountId | planId | "" (org-level)
    OwnerOrganizationID  primitive.ObjectID `bson:"ownerOrganizationId"`
    Mode                 string             `bson:"mode"`       // human | auto | ai
    // Override theo actionType (tùy chọn)
    ActionOverrides      map[string]string  `bson:"actionOverrides,omitempty"` // actionType -> mode
    CreatedAt            int64              `bson:"createdAt"`
    UpdatedAt            int64              `bson:"updatedAt"`
}
```

**Mode:**
- `human`: Chờ user duyệt qua API (mặc định an toàn)
- `auto`: Tự động approve ngay sau Propose (dùng khi đã tin tưởng rule)
- `ai`: Gọi AI để quyết định approve/reject (Phase sau)

**Ví dụ config:**
- Org X, domain=ads, scopeKey=act_123, mode=auto, actionOverrides: {"PAUSE":"human"}
- Org Y, domain=cio, scopeKey="", mode=human

### 3.3 Luồng Propose Mới (có Resolve)

```
Propose(domain, actionType, payload, ...)
    │
    ├─► Lưu ActionPending (status=pending)
    │
    └─► ResolveImmediate()  // Gọi ngay sau Propose
            │
            ├─► Đọc ApprovalModeConfig(domain, scopeKey, actionType)
            │
            ├─► mode=auto  → Approve() nội bộ → status=queued/executed
            ├─► mode=ai    → Gọi AI Approver → Approve/Reject
            └─► mode=human → Giữ pending, notify user
```

**Thay đổi:** Không còn logic auto-approve nằm trong scheduler. Scheduler chỉ Propose; Engine gọi `ResolveImmediate` và quyết định theo config.

### 3.4 Cấu Trúc Lại Module Approval

| Thành phần | Thay đổi |
|------------|----------|
| **pkg/approval** | Thêm `ResolveImmediate(ctx, doc)` — đọc config, nếu auto/ai thì Approve ngay. Tách logic resolve khỏi Propose. |
| **ApprovalModeConfig** | Collection mới (hoặc mở rộng ads_approval_config). Service `GetApprovalMode(domain, scopeKey, actionType)`. |
| **Ads scheduler** | Chỉ gọi `Propose`. Bỏ `approval.Approve()` trực tiếp. Engine tự resolve theo config. |
| **CIO Plan Execution** | **Option A:** Giữ flow riêng, thêm sync với Decision Brain (ghi DecisionCase khi approve/reject). **Option B:** Chuyển sang dùng pkg/approval — CreateExecutionRequest → Propose(domain=cio) → Resolve. |
| **Delivery** | **Gate:** Chỉ nhận actions từ Approval Gate. Thêm nguồn `source=APPROVAL_GATE`, `actionPendingId` trong payload. Deprecate nhận trực tiếp từ caller. |

### 3.5 AI Decision Engine Integration

```
AI Decision Engine.Execute(ctx, req, ownerOrgID)
    │
    ├─► Parse context (CIX, Ads, CIO, ...)
    ├─► Policy: tách actions cần approval vs auto (theo config)
    │
    ├─► Với mỗi action:
    │       approval.Propose(domain, ProposeInput{...})
    │       → Engine.ResolveImmediate() → auto approve nếu config
    │
    └─► Actions đã approved (status=queued/executed) → Worker/Executor xử lý
        Actions còn pending → Notify user, chờ Approve/Reject
```

---

## 4. Logic Bên Trong AI Decision Engine (Theo Vision)

> **Nguồn:** rule-intelligence.md (pipeline, Rule chỉ trả output), decision-brain.md (Context→Choice→Goal→Outcome), RASOAT (Context Aggregation, Policy, Arbitration)

### 4.1 Pipeline Theo Vision (rule-intelligence)

```
Raw Data → Rule → L1 → Rule → L2 → Rule → L3 → Rule → Flag → Rule → Action/Recommendation
                                                                           │
                                                                           ▼
                                                              AI Decision Engine (Context Aggregation, Policy, Arbitration)
                                                                           │
                                                                           ▼
                                                              ProposedActions → Approval Gate → Execute → Outcome
                                                                           │
                                                                           ▼
                                                              Decision Brain (học từ outcome)
```

**Nguyên tắc:** Rule Engine **chỉ trả output**, không thực thi. AI Decision Engine nhận output từ Rule + các nguồn khác → quyết định → Propose.

### 4.2 Bốn Thành Phần Logic

| Thành phần | Mô tả | Input | Output |
|------------|-------|-------|--------|
| **Context Aggregation** | Gom context từ nhiều nguồn thành một unified context | CIXPayload, AdsContext, CIOContext, CustomerCtx, RuleOutput | `AggregatedContext` |
| **Policy Evaluation** | Áp dụng policy: action nào cần approval, mode nào (human/auto/ai) | AggregatedContext, ProposedActions, ApprovalModeConfig | Mỗi action có `approvalMode` |
| **Arbitration** | Giải quyết xung đột khi nhiều nguồn đề xuất khác nhau | Nhiều ProposedAction trùng target/conflict | Một ProposedAction cuối (hoặc bỏ qua) |
| **Action Builder** | Chuyển output từ nguồn → ProposedAction chuẩn | Rule output, CIX actionSuggestions, CIO step output | `ProposedAction[]` |

### 4.3 Context Aggregation — Chi Tiết

**Schema AggregatedContext (đề xuất):**

```go
type AggregatedContext struct {
    SessionUid    string                 // Từ CIX
    CustomerUid   string                 // Unified customer ID
    Domain        string                 // ads | cio | cix | ...
    
    // Layers (từ Rule Engine / CIX)
    Layer1        map[string]interface{}  // Metrics: spend, mess, orders, ...
    Layer2        map[string]interface{}  // Scores, healthState
    Layer3        map[string]interface{}  // Classification, cell
    Flags         []string               // Alert flags: sl_a, chs_critical, ...
    
    // Customer (từ CRM)
    CustomerCtx   map[string]interface{} // valueTier, lifecycleStage, journeyStage
    
    // Nguồn đề xuất
    CIXActions    []string               // actionSuggestions từ CIX
    RuleOutputs   []RuleOutput           // Output từ Rule Engine (actionType, reason, ruleCode)
    CIOContext    map[string]interface{} // Plan, step, channelChosen
    AdsContext    map[string]interface{} // campaignId, adAccountId, metrics
    
    TraceID       string
    CorrelationID string
}
```

**Logic:** Merge từ ExecuteRequest.CIXPayload, CustomerCtx, và gọi thêm Rule Engine / CRM / Ads nếu cần bổ sung context.

### 4.4 Policy Evaluation — Chi Tiết

**Policy config (env hoặc DB):**

- `APPROVAL_ACTIONS_<DOMAIN>` — danh sách actionType bắt buộc approval (vd: `escalate_to_senior,assign_to_human_sale`)
- `ApprovalModeConfig` — mode mặc định và override theo actionType

**Logic:**

```
Với mỗi ProposedAction:
  1. actionType có trong APPROVAL_ACTIONS? → mode = human (trừ khi override)
  2. Đọc ApprovalModeConfig(domain, scopeKey, actionType)
  3. Gán approvalMode = human | auto | ai
```

### 4.5 Arbitration — Chi Tiết

**Tình huống:** CIX đề xuất `trigger_fast_response`, Rule Ads đề xuất `PAUSE` cho cùng campaign. Hoặc nhiều Rule cùng trigger cho một target.

**Chiến lược (đề xuất):**

| Ưu tiên | Nguồn | Ghi chú |
|---------|-------|---------|
| 1 | Manual override | User đã chọn |
| 2 | CIX (realtime) | Hội thoại đang diễn ra |
| 3 | Rule Engine (scheduled) | Ads, CIO |
| 4 | Mặc định | Bỏ qua nếu conflict không giải được |

**Logic:** Nếu nhiều action cùng target (vd: campaignId) và conflict (PAUSE vs INCREASE) → chọn theo priority, hoặc bỏ qua và log.

### 4.6 Action Builder — Chi Tiết

**Map từ nguồn → ProposedAction:**

| Nguồn | Output | ProposedAction |
|-------|--------|----------------|
| **CIX** | actionSuggestions: `["trigger_fast_response","escalate_to_senior"]` | actionType=SEND_MESSAGE, ASSIGN_TO_AGENT; payload chứa sessionUid, customerUid |
| **Rule Ads** | ShouldPropose, ActionType, Reason, RuleCode | actionType=PAUSE/DECREASE/INCREASE; payload chứa campaignId, adAccountId |
| **CIO** | Step output (channelChosen, contentRef) | actionType=SEND_MESSAGE; payload chứa executionId, planId, channel |

**ExecutionActionInput** (contract với Delivery) — xem `dto.execution.action.go`.

### 4.7 Luồng Execute Đầy Đủ (Có Logic)

```
AI Decision Engine.Execute(ctx, req, ownerOrgID)
    │
    ├─ 1. Context Aggregation
    │     Merge CIXPayload, CustomerCtx, gọi Rule/CRM/Ads nếu cần → AggregatedContext
    │
    ├─ 2. Action Builder
    │     Parse CIXActions, RuleOutputs, CIOContext → ProposedAction[]
    │
    ├─ 3. Arbitration
    │     Resolve conflict nếu có → ProposedAction[] đã filter
    │
    ├─ 4. Policy Evaluation
    │     Mỗi action → approvalMode (human|auto|ai)
    │
    └─ 5. Propose + Resolve
          Với mỗi action: approval.Propose(domain, ...) → ResolveImmediate()
          mode=auto → Approve nội bộ
          mode=human → Notify, chờ user
          mode=ai → Gọi AI Approver (Phase sau)
```

### 4.8 Tích Hợp Decision Brain (Learning)

Theo learning-engine.md: **Context → Choice → Goal → Outcome → Lesson**

- Khi ActionPending đóng (executed/rejected/failed): gọi `BuildDecisionCaseFromAction(doc)` → `CreateDecisionCase`
- DecisionCase lưu: situation, decisionRationale, intendedGoal, actualOutcome, lesson
- Phục vụ: analytics, retrieval cho AI, tuning rule/param

---

## 5. Phân Tích Option: CIO Có Nên Dùng pkg/approval?

### 5.1 Option A: CIO Giữ Flow Riêng

**Ưu điểm:**
- Ít thay đổi code
- cio_plan_executions có lifecycle phức tạp (resume, completedSteps, ...)

**Nhược điểm:**
- Hai luồng approval song song
- Config approval mode phải maintain ở hai nơi

### 5.2 Option B: CIO Chuyển Sang pkg/approval

**Ưu điểm:**
- Một luồng duy nhất
- Config thống nhất
- ActionPending.payload chứa executionId, planId, ...; Executor domain=cio gọi RunExecution

**Nhược điểm:**
- Refactor CreateExecutionRequest → Propose
- Cần map cio_plan_executions status với ActionPending status

**Đề xuất:** **Option B** — Chuyển CIO sang pkg/approval. Lý do: nguyên tắc "mọi quyết định qua approval" và config thống nhất. Payload có thể chứa `executionId`, Executor load execution và gọi `RunExecution`.

---

## 6. Cấu Trúc Thư Mục (Đã Triển Khai 2026-03-19)

```
api/
├── pkg/
│   └── approval/
│       ├── engine.go          # Propose, Approve, Reject, ExecuteOne
│       ├── types.go
│       └── interfaces.go
│       # TODO: + resolver.go (ResolveImmediate), GetApprovalMode
│
├── internal/
│   ├── approval/              # Bridge (MongoDB, NotifyTrigger)
│   │   ├── service.go
│   │   ├── config.go          # MỚI: GetApprovalMode, ApprovalModeConfig
│   │   └── ...
│   │
│   └── api/
│       ├── aidecision/        # AI Decision Engine
│       │   ├── service/
│       │   │   ├── service.aidecision.engine.go   # Execute → Propose
│       │   │   └── service.aidecision.cix.go      # ReceiveCixPayload
│       │   └── ...
│       │
│       ├── learning/          # Learning engine (Decision Brain)
│       │   ├── service/
│       │   │   ├── service.learning.builder.go    # BuildLearningCaseFromAction, FromCIOChoice
│       │   │   └── service.learning.case.go       # CreateLearningCaseFromAction
│       │   └── ...
│       │
│       ├── executor/          # Approval Gate + Execution (thay approval + delivery router)
│       │   ├── handler/handler.executor.action.go
│       │   └── router/routes.go   # /executor/actions/*, /executor/send, /executor/execute, /executor/history
│       │
│       └── delivery/
│           └── handler/      # Nội bộ — chỉ nhận từ Executor
```

---

## 7. Lộ Trình Triển Khai

### Phase 1: Config & Resolve (2–3 ngày)

| # | Công việc | Chi tiết |
|---|-----------|----------|
| 1 | Tạo ApprovalModeConfig | Model, collection, migration từ ads_approval_config/ads_meta_config |
| 2 | ResolveImmediate | Thêm vào pkg/approval, đọc config, auto Approve nếu mode=auto |
| 3 | Ads scheduler refactor | Bỏ approval.Approve() trực tiếp; Propose xong gọi ResolveImmediate (hoặc Engine gọi trong Propose) |
| 4 | GetApprovalMode service | Resolve domain, scopeKey, actionType → mode |

### Phase 2: Delivery Gate (1–2 ngày)

| # | Công việc | Chi tiết |
|---|-----------|----------|
| 1 | Delivery chỉ nhận từ Approval | Validate source=APPROVAL_GATE hoặc actionPendingId. Deprecate direct call. |
| 2 | Executor gọi Delivery | Khi Execute ActionPending, Executor build ExecutionActionInput và gọi Delivery |

### Phase 3: AI Decision Engine + CIX (2–3 ngày)

| # | Công việc | Chi tiết |
|---|-----------|----------|
| 1 | AI Decision Engine Execute mở rộng | Parse CIXPayload, Propose cho từng action, ResolveImmediate |
| 2 | ReceiveCixPayload | Entry từ CIX, gọi AI Decision Engine |
| 3 | Domain cix Executor | RegisterExecutor(cix), ExecuteCixAction → Delivery |

### Phase 4: CIO Migration (2–3 ngày)

| # | Công việc | Chi tiết |
|---|-----------|----------|
| 1 | CreateExecutionRequest → Propose | Thay vì tạo cio_plan_executions status=pending_approval, gọi approval.Propose(domain=cio) |
| 2 | Executor domain=cio | Load execution, gọi RunExecution |
| 3 | ApproveExecution/RejectExecution | Chuyển thành approval.Approve/Reject (hoặc giữ API cũ, map sang approval) |

### Phase 5: AI Approval (Sau này)

| # | Công việc | Chi tiết |
|---|-----------|----------|
| 1 | AI Approver service | Nhận ActionPending, gọi LLM/rule để quyết định approve/reject |
| 2 | mode=ai | ResolveImmediate gọi AI Approver khi config mode=ai |
| 3 | Feedback loop | Ghi kết quả vào DecisionCase để học |

---

## 8. Rà Soát Đối Chiếu Codebase

> **Ngày rà soát:** 2026-03-19

### 8.1 Đã Có Trong Code

| Thành phần | Vị trí | Ghi chú |
|------------|--------|---------|
| **pkg/approval** | `api/pkg/approval/` | Engine: Propose, Approve, Reject, ExecuteOne, NotifyExecuted, NotifyFailed |
| **internal/approval** | `api/internal/approval/` | Bridge MongoDB + NotifyTrigger |
| **Executor (API)** | `api/internal/api/executor/` | Handler actions, router: /executor/actions/*, /executor/send, /executor/execute, /executor/history |
| **AI Decision Engine** | `api/internal/api/aidecision/` | service.aidecision.engine.go, service.aidecision.cix.go — Execute, ReceiveCixPayload |
| **Learning engine** | `api/internal/api/learning/` | service.learning.builder.go, service.learning.case.go — BuildLearningCaseFromAction, CreateLearningCaseFromAction |
| **Domain ads Executor** | `api/internal/api/ads/executor.go` | `RegisterExecutor("ads", ...)` |
| **Domain cix Executor** | `api/internal/executors/cix/` | trigger_fast_response, escalate_to_senior, assign_to_human_sale, prioritize_followup → Delivery |
| **AdsExecutionWorker** | `api/internal/api/ads/worker/worker.ads.execution.go` | Xử lý status=queued, gọi learningsvc.CreateLearningCaseFromAction khi executed/failed |
| **BuildLearningCase khi Reject** | `handler.executor.action.go` | Gọi learningsvc.CreateLearningCaseFromAction khi reject |
| **CIO feedback** | `service.cio.feedback.go` | Gọi learningsvc khi touchpoint executed |
| **CIX AnalyzeSession** | `service.cix.analysis.go` | L1→L2→L3→Flags→ActionSuggestions, gọi aidecisionsvc.ReceiveCixPayload |
| **Delivery HandleExecute** | `handler.delivery.execute.go` | Gate: chỉ nhận từ Executor (DELIVERY_ALLOW_DIRECT_USE deprecate) |

### 8.2 Chưa Có / Chưa Đúng

| Thành phần | Trạng thái | Chi tiết |
|------------|------------|----------|
| **Domain cio Executor** | ❌ Chưa có | CIO dùng flow riêng (cio_plan_executions), không qua pkg/approval |
| **ResolveImmediate** | ❌ Chưa có | pkg/approval chưa có logic đọc config → auto Approve |
| **ApprovalModeConfig** | ❌ Chưa có | Config nằm rải rác: ads_meta_config, ads_approval_config |
| **Context Aggregation** | ❌ Chưa có | Execute() chưa merge đầy đủ CIX/Ads/CIO |
| **Policy / Arbitration / Action Builder** | ❌ Chưa có | Chỉ TODO trong comment |

### 8.3 Điểm Cần Sửa Gấp (Quick Wins)

| # | Công việc | File | Trạng thái |
|---|-----------|------|------------|
| 1 | BuildLearningCase khi ActionPending executed/failed | `worker.ads.execution.go` | ✅ Đã có |
| 2 | BuildLearningCase khi Reject | `handler.executor.action.go` | ✅ Đã có |
| 3 | CIX AnalyzeSession → ReceiveCixPayload | `service.cix.analysis.go` | ✅ Đã có |
| 4 | ReceiveCixPayload | `service.aidecision.cix.go` | ✅ Đã có |

### 8.4 Sơ Đồ Gap (Phương Án vs Code)

```
PHƯƠNG ÁN                              CODEBASE
─────────────────────────────────────────────────────────────
AI Decision Engine                     ✅ aidecision/ (Execute, ReceiveCixPayload)
  ├─ Context Aggregation                ❌ Chưa có
  ├─ Action Builder                     ❌ Chưa có
  ├─ Arbitration                        ❌ Chưa có
  ├─ Policy Evaluation                  ❌ Chưa có
  └─ Propose + Resolve                  ✅ Có (proposeCixAction, proposeAndApproveAutoCixAction)

Approval Gate / Executor                ✅ executor/ + pkg/approval
  ├─ ResolveImmediate                   ❌ Chưa có
  ├─ ApprovalModeConfig                 ❌ Chưa có
  └─ GetApprovalMode                    ❌ Chưa có

BuildLearningCase khi ActionPending đóng ✅ learning/
  ├─ executed/failed (worker)           ✅ worker.ads.execution
  └─ rejected                           ✅ handler.executor.action

CIX → AI Decision Engine               ✅ service.cix.analysis → aidecisionsvc
Domain cix Executor                     ✅ executors/cix/
Delivery gate                           ✅ handler.delivery (qua /executor/execute)
```

---

## 9. Checklist Nhanh

- [ ] ApprovalModeConfig model & collection
- [ ] ResolveImmediate trong pkg/approval
- [ ] GetApprovalMode(domain, scopeKey, actionType)
- [ ] Ads scheduler: bỏ Approve trực tiếp, dùng ResolveImmediate
- [ ] Delivery: gate chỉ nhận từ Approval (Phase 2)
- [ ] AI Decision Engine: Execute → Propose → Resolve
- [ ] CIO: migrate sang pkg/approval (Phase 4)
- [ ] **BuildDecisionCase khi ActionPending executed/failed/rejected** (Quick Win — worker + approval.Reject)
- [ ] **CIX AnalyzeSession → ReceiveCixPayload → AI Decision Engine** (Quick Win)

---

## 10. Rủi Ro & Mitigation

| Rủi ro | Mitigation |
|--------|------------|
| Breaking change cho Ads | Phase 1 giữ backward compat: nếu không có config, mặc định human (pending). Migration config từ ads_meta_config. |
| CIO migration phức tạp | Có thể giữ Option A tạm thời, chỉ sync DecisionCase. Option B làm Phase 4. |
| Delivery gate chặn caller cũ | Phase 2: cho phép cả direct và approval. Deprecation warning. Phase 3: bắt buộc approval. |
| Config trùng lặp | Consolidate ads_approval_config, ads_meta_config.automationConfig vào ApprovalModeConfig. Migration script. |

---

## Changelog

- 2026-03-19: Cập nhật §0, §2, §6, §8 — đồng bộ với đổi tên module (aidecision, learning, executor)
- 2026-03-18: Tạo phương án Decision Brain, đánh giá approval hiện có, đề xuất cấu trúc lại
- 2026-03-18: Bổ sung §4 Logic bên trong AI Decision Engine (Context Aggregation, Policy, Arbitration, Action Builder) theo vision docs
- 2026-03-18: Bổ sung §8 Rà soát đối chiếu codebase — gap analysis, quick wins
- 2026-03-18: Thống nhất tên gọi: **AI Decision Engine** (tầng ra quyết định), **Decision Brain** (tầng học tập)
