# Phương Án Làm Lại Executor — Bám Sát Vision

**Ngày:** 2026-03-19  
**Nguồn:** Vision 08 (Executor), DOI_CHIEU_3_DIEM_VISION_CODE, DE_XUAT_EXECUTOR_TIEP_THEO  
**Ràng buộc:** Chỉ AI Decision tạo action; domain modules không gọi Propose trực tiếp.

---

## 1. Nguyên Tắc Vision (Bắt Buộc)

### 1.1 Nguồn Action Duy Nhất

| Thành phần | Vai trò |
|------------|---------|
| **AI Decision** | Nguồn duy nhất tạo Action — nhận event, aggregate context, quyết định, gọi Propose |
| **Domain (CIX, Ads, CIO, Customer, Order)** | Emit event, trả context; **không** tạo action |
| **Executor** | Nhận action từ AI Decision; approval + execution; **không** tạo action |

### 1.2 Luồng Chuẩn Vision

```
Domain (CIX, Ads, CIO, …) → emit event / trả context
        ↓
AI Decision (Event Intake → Context Aggregation → Decision Core)
        ↓
AI Decision → Propose (domain, actionType, payload)
        ↓
Executor (7 sub-layer)
        ↓
Delivery / Adapters
```

---

## 2. Vision 08 — 7 Sub-layer Executor

| # | Sub-layer | Chức năng | Trạng thái hiện tại |
|---|-----------|-----------|---------------------|
| 1 | **Action Intake** | Nhận proposal, chuẩn hóa, gán idempotency | ⚠️ Có Propose, chưa chuẩn hóa rõ |
| 2 | **Validation & Normalization** | Schema, required fields, adapter tồn tại | ❌ Chưa có |
| 3 | **Policy / Guardrail / Approval** | Policy routing, guardrail check, approval orchestration | ⚠️ Có Propose/Approve, thiếu ApprovalModeConfig, ResolveImmediate |
| 4 | **Dispatch Runtime** | Route adapter, execute, retry, idempotency | ✅ Có executors/domain |
| 5 | **Execution Monitoring** | State machine, theo dõi lifecycle | ⚠️ Có pending/queued/executed, chưa đủ |
| 6 | **Outcome Evaluation** | Thu outcome, đánh giá business result | ❌ Chưa có |
| 7 | **Decision Brain Handoff** | Chốt hồ sơ, đẩy Learning khi đủ outcome | ✅ Có OnActionClosed, BuildLearningCaseFromAction |

---

## 3. Phương Án Làm Lại — 5 Phase

### Phase 0: Chuẩn Hóa Nguồn (Điều Kiện Tiên Quyết)

**Mục tiêu:** Chỉ AI Decision gọi Propose. Domain không gọi Propose trực tiếp.

| # | Công việc | Module | Chi tiết |
|---|-----------|--------|----------|
| 1 | **Ads: bỏ Propose trực tiếp** | ads | worker.ads.auto_propose → emit `ads.performance_alert` (hoặc tương đương) → AI Decision consume → AI Decision Propose |
| 2 | **CIO: bỏ Propose trực tiếp** | cio | CreateExecutionRequest, ExecuteTouchpoint → emit event → AI Decision consume → AI Decision Propose |
| 3 | **CIX: đã đúng** | cix | CIX trả context qua ReceiveCixPayload → AI Decision Propose. Giữ sync flow hoặc chuyển event-driven |
| 4 | **API Propose: chỉ nội bộ** | executor | POST /executor/actions/propose: deprecate cho external; chỉ AI Decision (internal) gọi. Hoặc giữ API nhưng document rõ: "Chỉ dùng bởi AI Decision" |

**Lưu ý:** Phase 0 có thể chạy song song Phase 1 nếu chưa có event-driven đầy đủ. Tạm thời: Ads/CIO vẫn gọi Propose nhưng **qua AI Decision** (AI Decision làm facade) — refactor nội bộ trước.

---

### Phase 1: Approval Gate Thống Nhất (2–3 ngày)

**Sub-layer 3:** Policy / Approval orchestration — config-driven.

| # | Công việc | File | Chi tiết |
|---|-----------|------|----------|
| 1 | **ApprovalModeConfig** | model mới | `approval_mode_config`: domain, scopeKey, ownerOrganizationId, mode, actionOverrides |
| 2 | **GetApprovalMode** | internal/approval/config.go | Query config; fallback ads_meta_config, CIX_APPROVAL_ACTIONS |
| 3 | **ResolveImmediate** | pkg/approval/resolver.go | Sau Propose: GetApprovalMode → mode in [auto_by_rule, fully_auto] → Approve nội bộ |
| 4 | **Propose gọi ResolveImmediate** | pkg/approval/engine.go | Propose → Insert → ResolveImmediate |
| 5 | **Ads refactor** | service.ads.auto_propose.go | Bỏ ShouldAutoApprove; luôn Propose. ResolveImmediate quyết định auto |
| 6 | **Migration config** | Script | ads_meta_config.ActionRuleConfig → approval_mode_config (MigrateAdsMetaConfigToApprovalMode) |

**Backward compat:** Không có ApprovalModeConfig → fallback ads_meta_config.

---

### Phase 2: Action Intake + Validation (1–2 ngày)

**Sub-layer 1–2:** Chuẩn hóa, validate.

| # | Công việc | Chi tiết |
|---|-----------|----------|
| 1 | **Intake chuẩn hóa** | Propose: bắt buộc domain, actionType, payload; tự gán idempotency_key nếu thiếu |
| 2 | **Validation schema** | Validate payload theo domain+actionType (required fields); reject nếu thiếu |
| 3 | **Adapter tồn tại** | Check executors[domain] != nil trước Insert; reject nếu domain chưa đăng ký |
| 4 | **Explainability Snapshot** | Lưu payload.reason, payload.traceId, payload.decisionCaseId vào ActionPending (phục vụ sub-layer 7) |

---

### Phase 3: Delivery Gate Cứng + Idempotency (1 ngày)

**Sub-layer 4:** Idempotency enforce.

| # | Công việc | File | Chi tiết |
|---|-----------|------|----------|
| 1 | **Validate source** | handler.delivery.execute | Chỉ nhận khi `source=APPROVAL_GATE` và `actionPendingId` (hoặc qua Executor route) |
| 2 | **Deprecation** | Cùng file | DELIVERY_ALLOW_DIRECT_USE=true: log warning |
| 3 | **Idempotency enforce** | pkg/approval/engine.go | Trước Execute: lookup idempotency_key đã xử lý → skip nếu trùng |
| 4 | **Index** | action_pending_approval | Index idempotency_key (hoặc payload.idempotencyKey) |

---

### Phase 4: Outcome + Handoff (1–2 ngày)

**Sub-layer 6–7:** Outcome Evaluation, Decision Brain Handoff.

| # | Công việc | Chi tiết |
|---|-----------|----------|
| 1 | **Outcome snapshot** | Sau Execute: lưu executeResponse, executedAt vào ActionPending (đã có) |
| 2 | **Closure type** | OnActionClosed: truyền closure_type (executed/rejected/failed) → Learning |
| 3 | **1 action = 1 learning case** | Đã có BuildLearningCaseFromAction; đảm bảo gắn decision_case_id |
| 4 | **Outcome window** | (Tùy chọn) Config timeout chờ outcome trước khi đóng case |

---

### Phase 5: Policy Registry + Guardrail (Dài hạn)

**Sub-layer 3 mở rộng.**

| # | Công việc | Chi tiết |
|---|-----------|----------|
| 1 | **Action Policy Registry** | actionType → requiredApproval, guardrailIds |
| 2 | **Guardrail interface** | Check(ctx, action) (allowed bool, reason string) |
| 3 | **Rate limit, budget cap** | Guardrails cụ thể |
| 4 | **Conflict / Concurrency** | Nhiều action cùng target → arbitration |

---

## 4. Lộ Trình Tổng Hợp

```
Phase 0 (điều kiện)   Phase 1 (2–3 ngày)    Phase 2 (1–2 ngày)    Phase 3 (1 ngày)
─────────────────    ─────────────────    ─────────────────    ─────────────────
Ads/CIO qua          ApprovalModeConfig  →  Intake + Validation → Delivery gate
AI Decision          ResolveImmediate        Schema, adapter      Idempotency enforce
(tùy event-driven)   Ads: bỏ ShouldAuto      Explainability
                     GetApprovalMode

Phase 4 (1–2 ngày)   Phase 5 (dài hạn)
─────────────────    ─────────────────
Outcome + Handoff    Policy Registry
Closure type         Guardrail Engine
```

---

## 5. Sơ Đồ Kiến Trúc Sau Khi Làm Lại

```
                    ┌─────────────────────────────────────────┐
                    │           AI DECISION ENGINE             │
                    │  Event Intake → Context → Decision      │
                    │  → Propose(domain, actionType, payload) │
                    └─────────────────────────────────────────┘
                                        │
                                        ▼
┌───────────────────────────────────────────────────────────────────────────────┐
│                           EXECUTOR (7 sub-layer)                              │
│                                                                               │
│  1. Action Intake      → Chuẩn hóa, gán idempotency                           │
│  2. Validation        → Schema, adapter tồn tại                               │
│  3. Policy/Approval   → GetApprovalMode, ResolveImmediate                     │
│  4. Dispatch          → executors[domain].Execute                             │
│  5. Monitoring        → pending→queued→executed/rejected/failed               │
│  6. Outcome           → executeResponse, executedAt                            │
│  7. Handoff           → OnActionClosed → BuildLearningCaseFromAction         │
└───────────────────────────────────────────────────────────────────────────────┘
                                        │
                    ┌───────────────────┼───────────────────┐
                    ▼                   ▼                   ▼
            ┌───────────────┐   ┌───────────────┐   ┌───────────────┐
            │ Executor ads  │   │ Executor cix  │   │ Executor cio  │
            │ (worker)      │   │ (sync)        │   │ (sync)        │
            └───────────────┘   └───────────────┘   └───────────────┘
                    │                   │                   │
                    ▼                   ▼                   ▼
            Meta API            Delivery (source=     CIO RunExecution
                                APPROVAL_GATE)
```

---

## 6. Rủi Ro & Mitigation

| Rủi ro | Mitigation |
|--------|------------|
| Phase 0 chưa xong (Ads/CIO vẫn Propose trực tiếp) | Phase 1 vẫn làm được: ResolveImmediate + ApprovalModeConfig. Ads refactor: bỏ ShouldAutoApprove, luôn Propose — logic approval chuyển sang Executor. |
| Breaking Ads auto-propose | Fallback ads_meta_config khi chưa có approval_mode_config |
| Import cycle (approval ↔ ads) | GetApprovalMode trong internal/approval, inject hoặc lazy load ads config |
| Event-driven chưa có | Phase 0 có thể làm từng bước: Ads emit event → AI Decision worker consume → AI Decision Propose. Hoặc tạm: Ads gọi AI Decision service.ProposeForAds(context) — AI Decision làm facade |

---

## 7. Checklist Thực Hiện

### Phase 1
- [x] ApprovalModeConfig model + collection
- [x] GetApprovalMode + fallback (ads_meta_config, CIX_APPROVAL_ACTIONS)
- [x] ResolveImmediate (Resolver interface, SetResolver)
- [x] Propose gọi ResolveImmediate
- [x] Ads: bỏ ShouldAutoApprove, luôn Propose
- [x] Migration ads_meta_config → approval_mode_config (Step 14 InitDefaultData)

### Phase 2
- [x] Check adapter tồn tại trước Insert
- [x] Intake chuẩn hóa (idempotency_key) — Ads, CIO thêm vào payload
- [x] Validation schema theo domain+actionType (pkg/approval/validator.go)
- [x] Explainability Snapshot (TraceID, DecisionCaseID trong ActionPending)

### Phase 3
- [x] Delivery validate source=APPROVAL_GATE (khi DELIVERY_ALLOW_DIRECT_USE=true)
- [x] Log warning khi DELIVERY_ALLOW_DIRECT_USE=true (deprecation)
- [x] Idempotency enforce trước Execute (FindByIdempotencyKey)
- [x] Index payload.idempotencyKey (CreateActionPendingIdempotencyIndex)

### Phase 4
- [ ] Closure type trong OnActionClosed
- [ ] Outcome window (tùy chọn)

### Phase 0 (song song hoặc trước)
- [x] AI Decision facade: ProposeForAds, ProposeForCio (service.aidecision.propose_facade.go)
- [x] **Event-driven:** Ads (và mọi domain) emit **`executor.propose_requested`** (`payload.domain`); tương thích `ads.propose_requested`
- [x] AI Decision consumer xử lý `executor.propose_requested` → `processExecutorProposeRequested` → ProposeForAds / `approval.Propose`
- [x] Ads Propose → EmitAdsProposeRequest (trả eventID)
- [x] CIO CreateExecutionRequest, ExecuteTouchpoint → EmitCioProposeRequest
- [x] Executor HandlePropose: domain=ads|cio emit event, trả 202 Accepted

---

## 8. Tài Liệu Tham Chiếu

| Tài liệu | Đường dẫn |
|----------|-----------|
| Vision 08 Executor | docs-shared/architecture/vision/08 - executor.md |
| Đề xuất Executor tiếp theo | [DE_XUAT_EXECUTOR_TIEP_THEO.md](./DE_XUAT_EXECUTOR_TIEP_THEO.md) |
| Đối chiếu 3 điểm | [scripts/reports/DOI_CHIEU_3_DIEM_VISION_CODE_2026-03-19.md](../scripts/reports/DOI_CHIEU_3_DIEM_VISION_CODE_2026-03-19.md) |
| Phương án AI Decision | [PHUONG_AN_SUA_MODULE_AI_DECISION_THEO_VISION.md](./PHUONG_AN_SUA_MODULE_AI_DECISION_THEO_VISION.md) |

---

## Changelog

- 2026-03-19: Tạo phương án làm lại Executor bám sát Vision; ràng buộc chỉ AI Decision tạo action
