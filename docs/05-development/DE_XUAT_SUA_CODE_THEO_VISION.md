# Đề Xuất Sửa Codebase Theo Vision Mới

**Ngày:** 2026-03-19  
**Nguồn:** Đối chiếu [docs-shared/architecture/vision/](docs-shared/architecture/vision/) với codebase folkgroup-backend

---

## 1. Tổng Quan Vision (Đã Đọc)

### 1.1 Tài Liệu Vision Canonical

| Tài liệu | Nội dung chính |
|----------|----------------|
| **00 - ai-commerce-os-platform-l1.md** | Flow canonical: Raw→L1→L2→L3→Decision Events→AI Decision→Action→Executor→Learning Engine. Nguyên tắc: Flags ⊂ Decision Events; AI Decision = event-driven; Decision ≠ Execution; Learning ≠ Decision |
| **07 - ai-decision.md** | AI Decision 3 lớp: Event Intake → Context Aggregation → Decision Core. Không approval, không execution. Output: 1 Action entity duy nhất |
| **08 - executor.md** | Executor 7 sub-layer: Intake→Validation→Policy/Approval→Dispatch→Monitoring→Outcome→Learning Handoff. Approval modes: manual_required, auto_by_rule, ai_recommend, fully_auto |
| **11 - learning-engine.md** | Learning Engine: chỉ học khi lifecycle kết thúc. Không tham gia runtime |
| **foundational/system-boundary.md** | Boundary rõ: AI Decision không approval; Executor không tạo Action |
| **foundational/event-system.md** | Event-driven: emit Decision Events → AI Decision consume → Action |

### 1.2 Gap Tổng Quan (Vision vs Code)

| Hạng mục | Vision | Code hiện tại | Gap |
|----------|--------|--------------|-----|
| **AI Decision 3 lớp** | Event Intake, Context Aggregation, Decision Core | Chỉ có applyPolicy + Propose/ProposeAndApproveAuto | Thiếu Event Intake, Context Aggregation, Arbitration |
| **Approval mode** | Config-driven (ApprovalModeConfig), ResolveImmediate | Rải rác: ads_meta_config, CIX_APPROVAL_ACTIONS env, ShouldAutoApprove trong scheduler | Thiếu ApprovalModeConfig, ResolveImmediate |
| **Ads auto-approve** | Scheduler chỉ Propose; Engine ResolveImmediate | Scheduler gọi ProposeAndApprove trực tiếp khi ShouldAutoApprove | Vi phạm: logic approval nằm trong domain |
| **Delivery Gate** | Chỉ nhận từ Executor (source=APPROVAL_GATE) | DELIVERY_ALLOW_DIRECT_USE=true mới cho phép; logic đảo (block khi false) | Cần validate source, deprecate direct |
| **Decision Events** | Emit event → AI Decision consume | **Đã bổ sung (2026-03-23):** `aidecision.execute_requested`, `POST /ai-decision/execute` → 202; **ReceiveCixPayload** chỉ enqueue. **(2026-03-24)** Đề xuất Executor thống nhất: `executor.propose_requested` (payload `domain`); `POST /executor/actions/propose` không gọi `approval.Propose` trực tiếp. Alias queue cũ `ads.propose_requested`. | Còn gap: Intake/Aggregation đầy đủ theo vision 3 lớp, Arbitration |
| **Executor 7 sub-layer** | Policy Registry, Guardrail, Outcome Registry | Chỉ có Propose/Approve/Execute cơ bản | Thiếu Policy Registry, Guardrail, Outcome |

---

## 2. Đối Chiếu Chi Tiết Code vs Vision

### 2.1 AI Decision Engine

**Vision (07 - ai-decision.md):**
- Lớp 1: Event Intake — nhận Decision Events, normalize, filter
- Lớp 2: Context Aggregation — gom Customer, Conversation, Ads, System context
- Lớp 3: Decision Core — Policy, Arbitration, Rule/LLM path, tạo Action
- Không approval, không execution

**Code hiện tại (`api/internal/api/aidecision/service/`):**
- `Execute()` / `ExecuteWithCase()`: vẫn là lõi quyết định — được gọi **chỉ từ worker** khi consume event **`aidecision.execute_requested`** (và các luồng nội bộ khác nếu có). **HTTP** `POST /ai-decision/execute` không gọi trực tiếp; parse actionSuggestions, applyPolicy (env CIX_APPROVAL_ACTIONS), proposeCixAction / proposeAndApproveAutoCixAction chạy trong worker.
- Không có Event Intake (sync call, không consume event)
- Không có Context Aggregation (chỉ dùng CIXPayload, CustomerCtx chưa merge đầy đủ)
- Không có Arbitration
- Policy: env CIX_APPROVAL_ACTIONS thay vì ApprovalModeConfig

### 2.2 Approval & Executor

**Vision (08 - executor.md):**
- Approval modes: manual_required, auto_by_rule, ai_recommend_human_confirm, fully_auto
- Policy & Approval Orchestration — risk-based routing
- ResolveImmediate: đọc config → auto Approve nếu mode=auto

**Code hiện tại:**
- `service.ads.auto_propose.go` L495–519: `ShouldAutoApprove(ruleCode, metaCfg)` → ProposeAndApprove hoặc Propose
- Logic approval nằm trong Ads module — **vi phạm boundary**: Executor/Approval Gate mới quyết định auto
- `pkg/approval`: có Propose, ProposeAndApproveAuto, Approve — chưa có ResolveImmediate
- Không có ApprovalModeConfig collection

### 2.3 Delivery Gate

**Vision:** Mọi action qua Executor. Không đường đi tắt.

**Code hiện tại (`handler.delivery.execute.go`):**
- `allowDirectDeliveryUse()` = true khi env DELIVERY_ALLOW_DIRECT_USE=true
- Khi false → block 403 (đúng: mặc định block direct)
- Chưa validate `source=APPROVAL_GATE` hoặc `actionPendingId` trong payload

### 2.4 CIO Flow

**Vision:** CIO qua pkg/approval thống nhất.

**Code hiện tại:**
- `service.cio.plan_execution.go`: Propose(domain=cio) khi CreateExecutionRequest; Approve khi user duyệt
- `service.cio.touchpoint.go`: Propose(domain=cio) cho touchpoint; ApproveTouchpoint gọi approval.Approve
- Đã dùng pkg/approval — ✅ đúng hướng

### 2.5 Learning Engine

**Vision:** Chỉ học khi lifecycle kết thúc. Không runtime.

**Code hiện tại:**
- `worker.ads.execution.go`: CreateLearningCaseFromAction khi executed/failed ✅
- `handler.executor.action.go`: CreateLearningCaseFromAction khi reject ✅
- `service.aidecision.engine.go`: CreateLearningCaseFromAction khi proposeAndApproveAuto ✅
- Đúng nguyên tắc

---

## 3. Phương Án Sửa Code (Theo Thứ Tự Ưu Tiên)

### Phase 1: Approval Gate Thống Nhất (2–3 ngày)

**Mục tiêu:** Tách logic auto-approve khỏi domain (Ads), đưa vào Approval Gate. Config-driven.

| # | Công việc | File | Chi tiết |
|---|-----------|------|----------|
| 1 | **ApprovalModeConfig** | `api/internal/api/approval/models/model.approval.config.go` (mới) | Struct: domain, scopeKey, ownerOrganizationId, mode, actionOverrides. Collection `approval_mode_config` |
| 2 | **GetApprovalMode** | `api/internal/approval/config.go` (mới) | `GetApprovalMode(ctx, domain, scopeKey, actionType) (mode string, err error)`. Fallback: ads_meta_config, CIX_APPROVAL_ACTIONS |
| 3 | **ResolveImmediate** | `api/pkg/approval/resolver.go` (mới) | Sau Propose: đọc GetApprovalMode; mode=auto → Approve nội bộ |
| 4 | **ProposeWithResolve** | `api/pkg/approval/engine.go` | Propose → Insert → ResolveImmediate (hoặc gọi trong Propose) |
| 5 | **Ads refactor** | `api/internal/api/ads/service/service.ads.auto_propose.go` | Bỏ `ShouldAutoApprove`; luôn gọi `Propose` (không ProposeAndApprove). Engine/Propose gọi ResolveImmediate |
| 6 | **Migration config** | Script | Map ads_meta_config.ActionRules[].autoApprove, ads_approval_config → approval_mode_config |

**Backward compat:** Nếu không có ApprovalModeConfig, fallback đọc ads_meta_config (giữ hành vi cũ). Sau migration chuyển sang config mới.

### Phase 2: Delivery Gate Cứng (1–2 ngày)

| # | Công việc | File | Chi tiết |
|---|-----------|------|----------|
| 1 | **Validate source** | `handler.delivery.execute.go` | Chỉ nhận khi payload có `source=APPROVAL_GATE` và `actionPendingId` (hoặc qua Executor route) |
| 2 | **Deprecation** | Cùng file | Khi DELIVERY_ALLOW_DIRECT_USE=true: log warning, cho phép tạm. Phase 3 bắt buộc bỏ |

### Phase 3: AI Decision Engine 3 Lớp (2–3 ngày)

| # | Công việc | File | Chi tiết |
|---|-----------|------|----------|
| 1 | **Event Intake (mở rộng)** | `service.aidecision.engine.go` | Nhận ExecuteRequest như Decision Event envelope; normalize, filter |
| 2 | **Context Aggregation** | Cùng file | Merge CIXPayload, CustomerCtx, gọi thêm CRM/Ads nếu cần → AggregatedContext |
| 3 | **Action Builder** | Cùng file | Parse CIXActions, RuleOutputs → ProposedAction[] chuẩn |
| 4 | **Arbitration** | Cùng file | Resolve conflict khi nhiều action cùng target (ưu tiên: CIX > Rule > default) |
| 5 | **Policy Evaluation** | Cùng file | Dùng GetApprovalMode thay env CIX_APPROVAL_ACTIONS |

### Phase 4: CIO Migration (Tùy chọn, 2–3 ngày)

| # | Công việc | Chi tiết |
|---|-----------|----------|
| 1 | CreateExecutionRequest → Propose | Thay cio_plan_executions pending_approval bằng approval.Propose(domain=cio) |
| 2 | Executor domain=cio | RegisterExecutor("cio", ...) load execution, gọi RunExecution |

### Phase 5: Event-Driven (Dài hạn)

| # | Công việc | Chi tiết |
|---|-----------|----------|
| 1 | Ads emit Decision Event | Khi flag trigger → emit ads.performance_alert; AI Decision consume |
| 2 | Async consume | AI Decision subscribe queue thay vì sync ReceiveCixPayload |
| 3 | Customer Intelligence | Emit customer.flag_triggered → AI Decision |

---

## 4. Lộ Trình Đề Xuất

```
Phase 1 (2–3 ngày)     Phase 2 (1–2 ngày)     Phase 3 (2–3 ngày)
─────────────────     ─────────────────     ─────────────────
ApprovalModeConfig  →  Delivery gate      →  Context Aggregation
ResolveImmediate       validate source        Action Builder
Ads: chỉ Propose       deprecation warning    Arbitration
GetApprovalMode                                Policy từ config
```

---

## 5. Checklist Cụ Thể (Phase 1)

- [ ] Tạo `ApprovalModeConfig` struct (domain, scopeKey, ownerOrganizationId, mode, actionOverrides)
- [ ] Tạo collection `approval_mode_config`, đăng ký global
- [ ] Migration: ads_meta_config.ActionRules, ads_approval_config → approval_mode_config
- [ ] `internal/approval/config.go`: GetApprovalMode với fallback ads config
- [ ] `pkg/approval/resolver.go`: ResolveImmediate(ctx, doc *ActionPending)
- [ ] Propose: sau Insert gọi ResolveImmediate (hoặc ProposeWithResolve)
- [ ] `service.ads.auto_propose.go`: bỏ ShouldAutoApprove; luôn Propose
- [ ] Test: Propose ads với config mode=auto → auto approve qua ResolveImmediate

---

## 6. Rủi Ro & Mitigation

| Rủi ro | Mitigation |
|--------|------------|
| Breaking Ads | Phase 1: fallback đọc ads_meta_config nếu chưa có approval_mode_config |
| Import cycle | ResolveImmediate trong pkg/approval — dùng callback/inject config service, không import decisionsvc |
| Config migration | Script migration; test trên staging trước |

---

## 7. Tài Liệu Tham Chiếu

| Vision | Đường dẫn |
|--------|-----------|
| Platform L1 | docs-shared/architecture/vision/00 - ai-commerce-os-platform-l1.md |
| AI Decision | docs-shared/architecture/vision/07 - ai-decision.md |
| Executor | docs-shared/architecture/vision/08 - executor.md |
| Rà soát | docs-shared/architecture/reviews/RA_SOAT_TRIEN_KHAI_VISION.md |
| System Boundary | docs-shared/architecture/foundational/system-boundary.md |
| Event System | docs-shared/architecture/foundational/event-system.md |
| Phương án Decision Brain | [PHUONG_AN_TRIEN_KHAI_AI_DECISION_VA_LEARNING.md](./PHUONG_AN_TRIEN_KHAI_AI_DECISION_VA_LEARNING.md) |

---

## Changelog

- 2026-03-19: Bổ sung đối chiếu chi tiết vision vs code; Phase 5 event-driven; tham chiếu docs
- 2026-03-19: Cập nhật §3 — đường dẫn file theo module mới (aidecision, learning, executor)
- 2026-03-19: Tạo đề xuất sửa code theo vision đã refactor
