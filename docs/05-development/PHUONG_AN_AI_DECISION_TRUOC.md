# Phương Án: Làm AI Decision Trước — Event-Driven Orchestrator

**Ngày:** 2026-03-19  
**Mục đích:** Xây AI Decision đầy đủ 3 lớp (Event Intake, Context Aggregation, Decision Core) trước, sau đó các module khác (CIO, CIX, Ads, Customer) kết nối vào.

**Tham chiếu:** [08 - ai-decision.md](../../docs/architecture/vision/08 - ai-decision.md), [unified-data-contract.md](../../docs/architecture/data-contract/unified-data-contract.md) §2.2a

---

## 1. Tổng Quan Kiến Trúc Mục Tiêu

```
Decision Events (CIO, CIX, Ads, Customer, Executor, …)
    ↓
┌─────────────────────────────────────────────────────────────┐
│  LỚP 1: Event Intake                                         │
│  Nhận event → Normalize → Filter → Route                     │
└─────────────────────────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────────────────────────┐
│  LỚP 2: Context Aggregation                                  │
│  Phân phối event xuống domain (CIX, Ads, Customer)           │
│  Domain trả context → Tổng hợp AggregatedContext             │
└─────────────────────────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────────────────────────┐
│  LỚP 3: Decision Core                                        │
│  Policy → Arbitration → Rule/LLM → Tạo Action                │
└─────────────────────────────────────────────────────────────┘
    ↓
Action → Executor (Propose)
```

---

## 2. Contract — Decision Event Schema

**File:** `api/internal/api/aidecision/dto/dto.aidecision.event.go` (mới)

```go
// DecisionEvent schema chuẩn — AI Decision nhận từ mọi nguồn.
type DecisionEvent struct {
    EventType    string                 `json:"eventType"`    // domain.event_name
    Source       string                 `json:"source"`       // CIO, CIX, Ads, Customer, Executor
    EntityRefs   map[string]string      `json:"entityRefs"`   // customerId, sessionId, orderId, ...
    Payload      map[string]interface{} `json:"payload"`
    Timestamp    string                 `json:"timestamp"`   // ISO 8601
    TraceID      string                 `json:"traceId,omitempty"`
    CorrelationID string                `json:"correlationId,omitempty"`
    OrgID        string                 `json:"orgId,omitempty"`
}
```

**Event types hỗ trợ Phase 1:**

| eventType | Source | entityRefs | Hành động |
|-----------|--------|------------|-----------|
| `conversation.message_received` | CIO | sessionId, customerId | Gọi CIX AnalyzeSession |
| `cix.signal_update` | CIX | sessionId, customerId | Đã có context trong payload |
| `ads.performance_alert` | Ads | campaignId, adsetId | (Phase 2) Gọi Ads |
| `customer.flag_triggered` | Customer | customerId | (Phase 2) Gọi Customer |

---

## 3. Phase 1: Event Intake + Entry Point (3–4 ngày)

### 3.1 Tạo Decision Event DTO và models

| File | Nội dung |
|------|----------|
| `aidecision/dto/dto.aidecision.event.go` | DecisionEvent struct |
| `aidecision/models/model.aidecision.event.go` | (nếu cần persist) |

### 3.2 Event Intake Layer

| File | Nội dung |
|------|----------|
| `aidecision/service/service.aidecision.intake.go` | `ConsumeEvent(ctx, event, ownerOrgID)` |
| | Normalize: đảm bảo eventType, entityRefs, timestamp |
| | Filter: theo org, loại bỏ stale/dedup (idempotency key) |
| | Route: chuyển sang Context Aggregation |

### 3.3 Entry point — POST /ai-decision/events

| File | Nội dung |
|------|----------|
| `aidecision/handler/handler.aidecision.events.go` | HandleConsumeEvent |
| | Nhận DecisionEvent (JSON body) |
| | Gọi service.ConsumeEvent |
| `aidecision/router/routes.go` | Thêm POST /ai-decision/events |

**Modules khác sẽ gọi:** `POST /ai-decision/events` với body DecisionEvent. Hoặc gọi service trực tiếp nếu cùng process.

### 3.4 Backward compatibility

- **Giữ** `ReceiveCixPayload` — CIX worker vẫn gọi được trong giai đoạn chuyển đổi
- **ReceiveCixPayload** gọi nội bộ `ConsumeEvent` với event type `cix.signal_update` (chuyển CixAnalysisResult → DecisionEvent)
- Khi CIO/CIX chuyển sang emit event → gọi POST /ai-decision/events, có thể bỏ ReceiveCixPayload

---

## 4. Phase 2: Context Aggregation (3–4 ngày)

### 4.1 Domain dispatcher interface

```go
// DomainContextProvider gọi domain để lấy context.
type DomainContextProvider interface {
    GetContext(ctx context.Context, event *DecisionEvent, ownerOrgID primitive.ObjectID) (map[string]interface{}, error)
}

// CIXProvider: eventType conversation.* → gọi CIX AnalyzeSession
// CIXProvider: eventType cix.signal_update → payload đã có context
// CustomerProvider: eventType customer.* hoặc cần customer context → gọi CRM GetProfile
// AdsProvider: eventType ads.* → gọi Ads service (Phase 3)
```

### 4.2 AggregatedContext struct

```go
type AggregatedContext struct {
    ConversationContext map[string]interface{} `json:"conversationContext,omitempty"` // từ CIX
    CustomerContext     map[string]interface{} `json:"customerContext,omitempty"`   // từ CRM
    AdsContext          map[string]interface{} `json:"adsContext,omitempty"`         // từ Ads (Phase 3)
    SystemContext       map[string]interface{} `json:"systemContext,omitempty"`     // policy, constraints
}
```

### 4.3 Context Aggregation logic

| eventType | Domain gọi | Cách gọi |
|-----------|------------|----------|
| `conversation.message_received` | CIX | `cixSvc.AnalyzeSession(ctx, entityRefs.sessionId, entityRefs.customerId, ownerOrgID)` |
| `cix.signal_update` | — | Payload đã có (layer1, layer2, layer3, flags, actionSuggestions) |
| Cần customer | CRM | `crmSvc.GetProfile(ctx, customerId, ownerOrgID)` |

### 4.4 File cần tạo/sửa

| File | Nội dung |
|------|----------|
| `aidecision/service/service.aidecision.aggregation.go` | `AggregateContext(ctx, event, ownerOrgID) → AggregatedContext` |
| | Gọi CIX, CRM theo eventType và entityRefs |
| `aidecision/service/service.aidecision.engine.go` | Refactor Execute: nhận AggregatedContext thay vì chỉ CIXPayload |
| | Execute logic hiện tại (applyPolicy, proposeCixAction, proposeAndApproveAutoCixAction) dùng AggregatedContext |

---

## 5. Phase 3: Decision Core — Policy, Arbitration (2–3 ngày)

### 5.1 Tách Decision Core

| File | Nội dung |
|------|----------|
| `aidecision/service/service.aidecision.core.go` | `Decide(ctx, event, aggregatedContext) → DecisionResult` |
| | DecisionResult: ignore | merge | wait | create Action |
| | applyPolicy: dùng env CIX_APPROVAL_ACTIONS (sau chuyển sang ApprovalModeConfig) |
| | Arbitration: khi nhiều action conflict (ưu tiên CIX > Rule > default) |

### 5.2 Quyết định 5 loại

- **ignore:** Không đủ điều kiện, noise → return
- **merge:** Gom event cùng entity → xử lý 1 lần (Phase 2)
- **wait:** Chờ thêm context (Phase 2)
- **trigger flow:** Kích hoạt workflow khác (Phase 2)
- **create Action:** Propose qua pkg/approval (đã có)

### 5.3 Rule Engine integration

- Gọi Rule Engine cho policy check (hard constraints, guardrails)
- Có thể thêm RULE_AI_DECISION_POLICY sau

---

## 6. Phase 4: Luồng hoàn chỉnh

### 6.1 ConsumeEvent flow

```
ConsumeEvent(event)
    → Event Intake: normalize, filter
    → Context Aggregation: dispatch theo eventType, gọi domain
    → Decision Core: Decide(aggregatedContext)
    → create Action → Propose (pkg/approval)
```

### 6.2 CIX flow mới (sau khi CIO kết nối)

```
CIO emit conversation.message_received
    → POST /ai-decision/events (hoặc queue)
    → AI Decision ConsumeEvent
    → eventType = conversation.message_received
    → Gọi CIX AnalyzeSession(sessionId, customerId)
    → CIX trả CixAnalysisResult
    → AggregatedContext.conversationContext = result
    → Decision Core: applyPolicy, proposeCixAction / proposeAndApproveAutoCixAction
    → Executor
```

### 6.3 CIX flow cũ (giữ backward compat)

```
CIX worker → AnalyzeSession (như hiện tại)
    → ReceiveCixPayload (khi có ActionSuggestions)
    → Chuyển sang ConsumeEvent với eventType=cix.signal_update
    → Decision Core (bỏ qua Context Aggregation vì đã có)
    → Propose
```

---

## 7. Cấu Trúc Thư Mục Đề Xuất

```
api/internal/api/aidecision/
├── dto/
│   ├── dto.aidecision.event.go      # DecisionEvent (mới)
│   └── dto.aidecision.execute.go   # ExecuteRequest, ExecuteResponse (giữ)
├── handler/
│   ├── handler.aidecision.execute.go   # POST /ai-decision/execute (giữ)
│   └── handler.aidecision.events.go     # POST /ai-decision/events (mới)
├── service/
│   ├── service.aidecision.engine.go     # Execute (refactor dùng AggregatedContext)
│   ├── service.aidecision.cix.go       # ReceiveCixPayload (giữ, gọi ConsumeEvent)
│   ├── service.aidecision.intake.go     # Event Intake (mới)
│   ├── service.aidecision.aggregation.go # Context Aggregation (mới)
│   └── service.aidecision.core.go      # Decision Core (mới, tách từ engine)
└── router/
    └── routes.go
```

---

## 8. Thứ Tự Triển Khai (Checklist)

### Phase 1: Event Intake (3–4 ngày)

- [ ] Tạo `dto.aidecision.event.go` — DecisionEvent struct
- [ ] Tạo `service.aidecision.intake.go` — ConsumeEvent, normalize, filter
- [ ] Tạo `handler.aidecision.events.go` — HandleConsumeEvent
- [ ] Thêm POST /ai-decision/events vào router
- [ ] ReceiveCixPayload: chuyển CixAnalysisResult → DecisionEvent, gọi ConsumeEvent (eventType=cix.signal_update)
- [ ] Test: POST /ai-decision/events với event cix.signal_update → Propose như cũ

### Phase 2: Context Aggregation (3–4 ngày)

- [ ] Tạo `service.aidecision.aggregation.go` — AggregateContext
- [ ] conversation.message_received → gọi CIX AnalyzeSession
- [ ] cix.signal_update → dùng payload
- [ ] Lấy CustomerContext từ CRM (GetProfile)
- [ ] Refactor Execute: nhận AggregatedContext
- [ ] Test: POST /ai-decision/events với conversation.message_received → CIX được gọi → Propose

### Phase 3: Decision Core (2–3 ngày)

- [ ] Tạo `service.aidecision.core.go` — Decide
- [ ] Tách applyPolicy, Arbitration
- [ ] Hỗ trợ ignore | create Action
- [ ] (Optional) Rule Engine policy check

### Phase 4: Kết nối CIO (sau khi AI Decision xong)

- [ ] CIO: thay OnCioEventInserted → CIX bằng emit → AI Decision
- [ ] Có thể dùng POST /ai-decision/events hoặc queue (MongoDB collection, Redis)

---

## 9. Ước Lượng Effort

| Phase | Effort | Phụ thuộc |
|-------|--------|-----------|
| Phase 1: Event Intake | 3–4 ngày | — |
| Phase 2: Context Aggregation | 3–4 ngày | Phase 1 |
| Phase 3: Decision Core | 2–3 ngày | Phase 2 |
| **Tổng** | **8–11 ngày** | |

---

## 10. Rủi Ro & Mitigation

| Rủi ro | Mitigation |
|--------|------------|
| CIX flow cũ bị break | ReceiveCixPayload giữ nguyên, gọi ConsumeEvent nội bộ |
| CIX AnalyzeSession chậm | Có thể chạy async trong worker; AI Decision gọi sync trước |
| Import cycle | CIX không import aidecision; AI Decision import CIX (one-way) |
| Event queue | Phase 1: sync API. Phase 4: thêm queue (MongoDB/Redis) nếu cần async |

---

## 11. Tài Liệu Tham Chiếu

| Tài liệu | Nội dung |
|----------|----------|
| [08 - ai-decision.md](../../docs/architecture/vision/08 - ai-decision.md) | Vision 3 lớp |
| [unified-data-contract.md](../../docs/architecture/data-contract/unified-data-contract.md) | Decision Event Schema |
| [PHUONG_AN_TRIEN_KHAI_AI_DECISION_VA_LEARNING.md](./PHUONG_AN_TRIEN_KHAI_AI_DECISION_VA_LEARNING.md) | Approval, Executor |

---

## Changelog

- 2026-03-19: Tạo phương án làm AI Decision trước
