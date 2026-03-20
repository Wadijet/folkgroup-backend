# Phương Án: Triển Khai Executor (Execution Engine)

**Ngày:** 2026-03-18  
**Tham chiếu:** [08 - executor.md](../../docs-shared/architecture/vision/08%20-%20executor.md), [PHUONG_AN_TRIEN_KHAI_AI_DECISION_VA_LEARNING.md](./PHUONG_AN_TRIEN_KHAI_AI_DECISION_VA_LEARNING.md), [PHUONG_AN_TRIEN_KHAI_AI_DECISION_CIX.md](./PHUONG_AN_TRIEN_KHAI_AI_DECISION_CIX.md)

---

## 0. Chính sách Approval-First (2026-03-18)

**Nguyên tắc:** Mọi hành động hiện nay đều phải qua duyệt để kiểm soát. Dần dần config để chuyển sang tự động và AI.

| Trạng thái | Mô tả |
|------------|-------|
| **Mặc định** | Tất cả action: Propose → chờ duyệt người |
| **Tự động** | Khi config `autoApprove: true` trong ads_meta_config (ActionRuleConfig) cho rule cụ thể |
| **AI** | (Phase sau) ApprovalModeConfig mode=ai |

**Ads:** Scheduler (Morning On, Noon Cut, Night Off…), Throttle, Circuit Breaker, Commands, Peak Matrix, Self Competition, Auto Propose — tất cả dùng `Propose` (không auto-approve). Chỉ khi `ShouldAutoApprove(ruleCode, metaCfg) == true` (config trong ads_meta_config) thì Auto Propose mới dùng `ProposeAndApprove`.

**CIO, CIX:** Luồng đã qua approval. CIO: CreateExecutionRequest → Propose → chờ ApproveExecution.

---

## 1. Vision — Executor (08 - executor)

### 1.1 Định vị theo Vision

**Executor** = **Action Control Tower + Execution Runtime + Outcome Tracker**

> Mọi module khi tạo action **phải gửi về Executor** để thực thi. Không có đường đi tắt.

| Vai trò | Mô tả |
|---------|-------|
| **Đứng sau** | Ads Intelligence, Customer Intelligence, CIX, Rule Engine, Decision Engine |
| **Đứng trước** | Adapters (Meta, Zalo, CRM, …), CIO (log), Decision Brain (outcome) |
| **Không làm** | Tự sinh chiến lược, tự tính intelligence, tự quyết "nên làm gì" |

### 1.2 Luồng chuẩn (6 bước)

```
Module nguồn → Tạo Action Proposal
        ↓
┌─────────────────────────────────────────────────────────────┐
│                      EXECUTOR                                │
│  1. Validation                                               │
│  2. Policy & Approval Routing                                │
│  3. Execution Dispatch                                       │
│  4. Execution Monitoring                                     │
│  5. Outcome Collection                                       │
│  6. Decision Brain Handoff                                   │
└─────────────────────────────────────────────────────────────┘
        ↓
Adapters → External Systems | CIO (log) | Decision Brain
```

### 1.3 7 Sub-layer của Executor (Vision)

| # | Sub-layer | Chức năng |
|---|-----------|-----------|
| 1 | Action Intake | Nhận proposal, chuẩn hóa, gán idempotency |
| 2 | Validation & Normalization | Schema, required fields, adapter tồn tại |
| 3 | Policy / Guardrail / Approval | Policy routing, guardrail check, approval orchestration |
| 4 | Dispatch Runtime | Route adapter, execute, retry, idempotency |
| 5 | Execution Monitoring | State machine, theo dõi lifecycle |
| 6 | Outcome Evaluation | Thu outcome, đánh giá business result |
| 7 | Decision Brain Handoff | Chốt hồ sơ, đẩy khi đủ outcome window |

### 1.4 Executor trong pkg/approval (implementation hiện tại)

| Khái niệm | Mô tả |
|-----------|-------|
| **Executor** | Interface `Execute(ctx, doc *ActionPending) (response, err)` — mỗi domain đăng ký |
| **Domain** | ads, cix, cio, content, ... — mỗi domain có executor riêng |
| **Deferred** | Approve → status=queued → Worker poll → gọi Executor |
| **Sync** | Approve → gọi Executor ngay |

**Mapping:** pkg/approval Engine ≈ Sub-layer 3–4 (Policy/Approval + Dispatch). Executor domain ≈ Adapter dispatch.

---

## 2. Trạng Thái Hiện Tại

### 2.1 Executor Đã Có

| Domain | File | Action types | Deferred? |
|--------|------|--------------|-----------|
| **ads** | `api/internal/api/ads/executor.go` + `service.ads.executor.go` | KILL, PAUSE, RESUME, ARCHIVE, DELETE, SET_BUDGET, SET_LIFETIME_BUDGET, INCREASE, DECREASE, SET_NAME | ✅ Có (worker) |
| **cix** | `api/internal/executors/cix/executor.go` | trigger_fast_response → SEND_MESSAGE; escalate_to_senior, assign_to_human_sale → ASSIGN_TO_AGENT | ❌ Sync (execute ngay) |
| **cio** | `api/internal/executors/cio/executor.go` | run_cio_plan (executionId) → RunExecution; send_cio_touchpoint (touchpointPlanId) → ExecuteTouchpoint | ❌ Sync (execute ngay) |

**Luồng ads:** Approve → status=queued → AdsExecutionWorker → adssvc.ExecuteAdsAction → Meta API

**Luồng cix:** Approve → executeCixAction ngay → map payload → Delivery.ExecuteActions (SEND_MESSAGE) hoặc stub (ASSIGN_TO_AGENT — Delivery chưa hỗ trợ)

**Luồng cio:** CreateExecutionRequest/ExecuteTouchpoint → Propose → ApproveExecution/approval.Approve → Executor cio → RunExecution hoặc ExecuteTouchpoint

### 2.2 Executor Chưa Có

| Domain | Trạng thái | Ghi chú |
|--------|------------|---------|
| **content** | ❌ Chưa có | PUBLISH_CONTENT chưa có executor |
| **crm** | ❌ Chưa có | TAG_CUSTOMER, ASSIGN_TO_AGENT chưa có executor |

### 2.3 Delivery vs Executor

| Thành phần | Vai trí | Ghi chú |
|------------|---------|---------|
| **Delivery** | Service nhận `ExecutionActionInput[]`, route SEND_MESSAGE → delivery_queue | Chỉ xử lý SEND_MESSAGE, các action khác bỏ qua |
| **Executor** | Thực thi ActionPending (đã approve) theo domain | ads → Meta API; cix/cio → gọi Delivery cho SEND_MESSAGE |

**Quan hệ:** Executor domain cix/cio **gọi** Delivery service khi actionType = SEND_MESSAGE. Delivery là **adapter** bên trong Execution Engine, không phải Executor.

---

## 3. Đề Xuất Phương Án Executor

### 3.1 Nguyên Tắc Thiết Kế

| Nguyên tắc | Mô tả |
|------------|-------|
| **Một domain một Executor** | Mỗi domain đăng ký `RegisterExecutor(domain, ex)`. Executor nhận ActionPending, đọc payload, thực thi. |
| **Executor gọi adapter** | Executor không gọi API bên ngoài trực tiếp. Gọi service: Delivery (SEND_MESSAGE), Meta (ads), CRM (TAG_CUSTOMER), ... |
| **Payload chuẩn** | ActionPending.Payload chứa đủ thông tin để Executor thực thi. Map từ ExecutionActionInput khi Propose. |
| **Decision Brain** | Sau Execute (thành công/thất bại/reject) → BuildDecisionCaseFromAction → CreateDecisionCase |

### 3.2 Kiến Trúc Executor Tổng Quan

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         APPROVAL GATE (pkg/approval)                          │
│  Propose → Resolve → Approve → status=queued (deferred) hoặc Execute ngay    │
└─────────────────────────────────────────────────────────────────────────────┘
                                        │
                    ┌───────────────────┼───────────────────┐
                    ▼                   ▼                   ▼
            ┌───────────────┐   ┌───────────────┐   ┌───────────────┐
            │ Executor ads  │   │ Executor cix  │   │ Executor cio  │
            │ Meta API     │   │ → Delivery    │   │ → RunExecution │
            └───────────────┘   └───────────────┘   └───────────────┘
                    │                   │                   │
                    ▼                   ▼                   ▼
            ┌───────────────┐   ┌───────────────┐   ┌───────────────┐
            │ Meta Graph API│   │ delivery_queue│   │ cio_plan_     │
            │               │   │ (SEND_MESSAGE)│   │ executions    │
            └───────────────┘   └───────────────┘   └───────────────┘
```

### 3.3 Mapping ActionType → Executor Domain

| ActionType (ExecutionActionInput) | Executor domain | Adapter/Service |
|----------------------------------|-----------------|----------------|
| SEND_MESSAGE | cix, cio | Delivery.ExecuteActions |
| PAUSE_ADSET, UPDATE_AD, KILL, ... | ads | adssvc.ExecuteAdsAction (Meta API) |
| ASSIGN_TO_AGENT | cix, cio | (tương lai) CRM/CIO service |
| TAG_CUSTOMER | crm | (tương lai) crm service |
| PUBLISH_CONTENT | content | (tương lai) content service |
| CREATE_ORDER, SCHEDULE_TASK | — | (tương lai) |

**Lưu ý:** Khi Propose, `domain` xác định Executor nào sẽ chạy. Cùng actionType SEND_MESSAGE có thể từ domain cix hoặc cio — payload khác nhau (sessionUid vs executionId).

---

## 4. Chi Tiết Triển Khai Từng Executor

### 4.1 Executor CIX (Đã có — cải thiện theo Vision)

**Vị trí:** `api/internal/executors/cix/executor.go` — package tách riêng để tránh import cycle.

**Mapping actionType → ExecutionActionInput:**
| actionType | ExecutionActionType | Adapter |
|------------|---------------------|---------|
| trigger_fast_response | SEND_MESSAGE | Delivery.ExecuteActions |
| escalate_to_senior, assign_to_human_sale | ASSIGN_TO_AGENT | Stub (Delivery chưa hỗ trợ) |

**Cải thiện theo Vision 08:**
- Lấy `content`, `channel` từ payload (trigger_fast_response)
- Thêm `idempotencyKey`, `traceId`, `correlationId` vào ExecutionActionInput
- ASSIGN_TO_AGENT: tích hợp CRM/CIO khi có adapter

**Payload chuẩn khi Propose (domain=cix):**
```json
{
  "sessionUid": "sess_xxx",
  "customerUid": "cust_xxx",
  "channel": "messenger",
  "content": "Template phản hồi nhanh",
  "traceId": "trace_xxx",
  "idempotencyKey": "act_xxx_20260318_001"
}
```

### 4.2 Executor CIO (Đã triển khai)

**Vị trí:** `api/internal/executors/cio/executor.go`

**Mục đích:** Thực thi cio_plan_execution hoặc cio_touchpoint đã approve.

| actionType | Payload | Xử lý |
|------------|---------|-------|
| run_cio_plan | executionId | Load CioPlanExecution, gọi RunExecution |
| send_cio_touchpoint | touchpointPlanId | Load CioTouchpointPlan, gọi ExecuteTouchpoint logic |

**Luồng:** CreateExecutionRequest → Propose → ApproveExecution → Executor; ExecuteTouchpoint → Propose → approval.Approve → Executor.

### 4.3 Executor CRM (Tương lai)

**ActionType:** TAG_CUSTOMER, ASSIGN_TO_AGENT  
**Adapter:** CRM service — tag customer, assign conversation to agent.

### 4.4 Executor Content (Tương lai)

**ActionType:** PUBLISH_CONTENT  
**Adapter:** Content service — publish node, schedule.

---

## 5. Lộ Trình Triển Khai

### Phase 1: Executor CIX (2–3 ngày)

| # | Công việc | File | Chi tiết |
|---|-----------|------|----------|
| 1 | Tạo executor.go | `api/internal/api/cix/executor.go` | RegisterExecutor, executeCixAction |
| 2 | Tạo service.cix.executor.go | `api/internal/api/cix/service/` | ExecuteCixAction, map payload → ExecutionActionInput |
| 3 | Import cix trong main | `main.go` hoặc init | Đảm bảo init() chạy |
| 4 | Decision Engine Propose cix | `service.decision.engine.go` | Khi action cần approval → Propose(domain=cix) |
| 5 | ReceiveCixPayload | `service.decision.cix.go` | CIX AnalyzeSession → gọi Decision Engine |

### Phase 2: BuildDecisionCase khi Executor đóng (1 ngày)

| # | Công việc | File | Chi tiết |
|---|-----------|------|----------|
| 1 | Worker ads: gọi BuildDecisionCaseFromAction | `worker.ads.execution.go` | Sau NotifyExecuted, NotifyFailed |
| 2 | Reject: gọi BuildDecisionCaseFromAction | `pkg/approval` hoặc handler | Khi Reject |
| 3 | CIX executor: tương tự nếu dùng deferred | Worker cix (nếu có) | |

### Phase 3: Executor CIO (2–3 ngày — sau Phase 4 Decision Brain)

| # | Công việc | Chi tiết |
|---|-----------|----------|
| 1 | CreateExecutionRequest → Propose(domain=cio) | Thay flow cio_plan_executions |
| 2 | Executor cio | Load execution, RunExecution |
| 3 | ApproveExecution/RejectExecution | Map sang approval.Approve/Reject |

### Phase 4: Executor CRM, Content (Sau này)

Khi có nhu cầu TAG_CUSTOMER, PUBLISH_CONTENT từ Decision Engine.

---

## 6. Contract Payload Chuẩn

### 6.1 ActionPending.Payload — Domain ads

```json
{
  "adAccountId": "act_xxx",
  "campaignId": "123",
  "adSetId": "456",
  "adId": "789",
  "value": 100
}
```

### 6.2 ActionPending.Payload — Domain cix

```json
{
  "sessionUid": "sess_xxx",
  "customerUid": "cust_xxx",
  "channel": "messenger",
  "content": "Nội dung tin nhắn",
  "traceId": "trace_xxx",
  "correlationId": "corr_xxx"
}
```

### 6.3 ActionPending.Payload — Domain cio

**run_cio_plan:**
```json
{
  "executionId": "ObjectId",
  "planId": "planId",
  "unifiedId": "cust_xxx",
  "goalCode": "goal_code"
}
```

**send_cio_touchpoint:**
```json
{
  "touchpointPlanId": "ObjectId",
  "unifiedId": "cust_xxx",
  "goalCode": "goal_code"
}
```

---

## 7. Rủi Ro & Mitigation

| Rủi ro | Mitigation |
|--------|------------|
| Import cycle (approval ↔ delivery ↔ cix) | Executor cix gọi deliverysvc qua interface hoặc qua handler. Đặt service trong package phù hợp. |
| CIX execute ngay vs deferred | Mặc định execute ngay (realtime). Nếu cần retry, đăng ký RegisterDeferredExecutionDomain + worker. |
| Payload không đủ | Mở rộng ProposeInput, document payload cho từng actionType. |

---

## 8. Checklist Nhanh

- [x] Executor CIX: `executors/cix/executor.go` (đã có)
- [x] Import cix executor trong main (init)
- [x] CIX executor: content, channel, idempotencyKey, traceId từ payload
- [x] Decision Engine: Propose/ProposeAndApproveAuto(domain=cix) — không gọi Delivery trực tiếp
- [x] BuildDecisionCaseFromAction khi executed/failed/rejected (worker ads, handler reject, decision engine)
- [x] Executor CIO: `executors/cio/executor.go` — run_cio_plan, send_cio_touchpoint
- [x] CreateExecutionRequest → Propose; ApproveExecution/RejectExecution → approval
- [x] ExecuteTouchpoint → Propose → approval.Approve → Executor
- [x] Delivery gate: 403 khi DELIVERY_ALLOW_DIRECT_USE=false
- [ ] Action Policy Registry, Guardrail Engine (Vision 08)


---

## 9. Gap Vision 08 vs Codebase

| Hạng mục Vision 08 | Trạng thái |
|--------------------|------------|
| pkg/approval (Propose, Approve, Execute) | ✅ Đã có |
| CIX Executor, Ads Executor, CIO Executor | ✅ Đã đăng ký |
| Delivery gate (POST /delivery/execute) | ✅ 403 khi DELIVERY_ALLOW_DIRECT_USE=false |
| Action Policy Registry | ⏳ Chưa có |
| Guardrail Engine | ⏳ Chưa có |
| Conflict / Concurrency | ⏳ Chưa có |
| Outcome Registry + Evaluation | ⏳ Chưa có |
| State machine đầy đủ | ⏳ Chưa đủ |
| Explainability Snapshot | ⏳ Chưa có |
| Idempotency | ⏳ Một phần (payload có, chưa enforce) |

---

## Changelog

- 2026-03-18: Tạo phương án triển khai Executor dựa trên Vision, PHUONG_AN_TRIEN_KHAI_AI_DECISION_VA_LEARNING, RASOAT
- 2026-03-18: Cập nhật theo Vision 08 (08 - executor.md); ghi nhận Executor CIX đã có
- 2026-03-18: Executor CIO: run_cio_plan (CreateExecutionRequest), send_cio_touchpoint (ExecuteTouchpoint); Approval-first
