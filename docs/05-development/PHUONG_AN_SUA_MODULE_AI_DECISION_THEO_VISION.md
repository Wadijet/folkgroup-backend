# Phương Án Sửa Module AI Decision Theo Vision

**Ngày:** 2026-03-19  
**Cập nhật:** 2026-03-19 — Bám sát PLATFORM_L1_EVENT_DECISION_SUPPLEMENT  
**Nguồn canonical:** [PLATFORM_L1_EVENT_DECISION_SUPPLEMENT.md](../../docs/architecture/vision/PLATFORM_L1_EVENT_DECISION_SUPPLEMENT.md)  
**Tham chiếu:** [08 - ai-decision.md](../../docs/architecture/vision/08%20-%20ai-decision.md), [DE_XUAT_SUA_CODE_THEO_VISION.md](./DE_XUAT_SUA_CODE_THEO_VISION.md)

---

## 1. Tổng Quan Vision (PLATFORM_L1_EVENT_DECISION_SUPPLEMENT)

### 1.1 Kiến trúc tổng thể

```
External → CIO → Domain Collections → Event Hook
    → Event Queue (MongoDB)
    → Worker Pool (3 lane: fast / normal / batch)

    → AI Decision
        → Decision Case (resolve/create)
        → Context Aggregation
        → Decision

    → Executor
    → Outcome

    → Decision Case Closed
    → Learning Engine (build learning_case)
```

**Câu chốt:**
> **Decision Case = đơn vị sống của bộ não AI**  
> **Learning Case = ký ức được rút ra từ Decision Case**

### 1.2 Bốn thành phần chuẩn hóa

| Thành phần | Mô tả |
|------------|-------|
| **Event System** | MongoDB-based, event envelope chuẩn, 3 lane (fast/normal/batch) |
| **AI Decision Orchestration** | Nhận event → Resolve decision_case → Context Aggregation → Decision → Action |
| **Decision Case** | Đơn vị vận hành từ trigger đến outcome; gom event, context, decision, action |
| **Runtime → Learning Loop** | Case closed → build learning_case (1 per action) |

### 1.3 Luồng AI Decision chuẩn (Supplement §3.2)

```
1. Nhận event
2. Resolve decision_case (tìm case cũ hoặc tạo mới)
3. Load context hiện tại
4. Đánh giá urgency

5. Nếu đủ context → quyết định
6. Nếu thiếu → emit Work Request tới domain (cix.analysis_requested, customer.context_requested, …)

7. Nhận Result events (cix.analysis_completed, customer.context_ready, …)
8. Update decision_case (merge context)

9. Khi đủ context → tạo decision_packet
10. Sinh action
11. Gửi Executor
```

---

## 2. Gap: Code Hiện Tại vs Vision

| Hạng mục | Vision (Supplement) | Code hiện tại | Gap |
|----------|---------------------|---------------|-----|
| **Event System** | MongoDB queue, event envelope, 3 lane | Không có | ❌ Chưa có |
| **Decision Case** | Collection `decision_cases_runtime`, lifecycle, merge/reopen | Không có | ❌ Chưa có |
| **Event types** | Source, Work Request, Result, Decision, Execution | Chỉ sync ReceiveCixPayload | ❌ Chưa có taxonomy |
| **Domain workers** | Consume Work Request → emit Result | CIX gọi AI Decision sync | ⚠️ Ngược luồng |
| **Context Aggregation** | Context Builder gom customer, conversation, order, ads | Chỉ CIXPayload | ❌ Thiếu |
| **Learning Case** | 1 per action, sau outcome | Có BuildLearningCaseFromAction | ⚠️ Cần gắn với closure_type |

---

## 3. Collections (Supplement §10)

| Collection | Tên đề xuất | Mô tả |
|------------|-------------|-------|
| Event queue | `decision_events_queue` | Hàng đợi event chờ AI Decision xử lý |
| Decision case (runtime) | `decision_cases_runtime` | Case đang vận hành — từ trigger đến outcome |
| Learning case | `learning_cases` | Ký ức học tập — 1 case per action, sau khi có outcome |
| Action pending | `action_pending_approval` | Giữ nguyên (đã có) — action chờ duyệt/thực thi |

**Lưu ý:** `decision_cases` hiện tại (dùng cho Learning) → migrate/rename thành `learning_cases`.

---

## 4. Event Envelope & Taxonomy (Supplement §2)

### 4.1 Event Envelope (BẮT BUỘC)

```json
{
  "event_id": "evt_xxx",
  "event_type": "conversation.message_inserted",
  "event_source": "cio",
  "entity_type": "message",
  "entity_id": "msg_xxx",
  "org_id": "org_xxx",
  "priority": "high",
  "lane": "fast",
  "status": "pending",
  "parent_event_id": null,
  "root_event_id": "evt_root",
  "trace_id": "trace_xxx",
  "correlation_id": "corr_xxx",
  "payload": {},
  "scheduled_at": null,
  "attempt_count": 0,
  "max_attempts": 5,
  "leased_by": null,
  "leased_until": null,
  "created_at": 1234567890
}
```

### 4.2 Event Types chính

| Loại | event_type | Nguồn/Đích |
|------|------------|------------|
| **Source** | `conversation.message_inserted`, `message.batch_ready`, `cio_event.inserted`, `order.inserted`, `customer.updated`, `ads.updated` | CRUD hook |
| **Work Request** | `cix.analysis_requested`, `customer.context_requested`, `ads.context_requested`, `order.recompute_requested` | AI Decision → Domain |
| **Result** | `cix.analysis_completed`, `customer.context_ready`, `ads.context_ready`, `order.flags_emitted` | Domain → AI Decision |
| **Execution** | `execution.completed`, `execution.failed`, `execution.rejected` | Executor → Learning |

### 4.3 Debounce (Supplement §2.6)

- **Debounce key:** `org_id:conversation_id:event_group` (ví dụ `org_001:conv_123:message_burst`)
- **Worker:** `worker.debounce_aggregator` — sau window (ví dụ 30s) không có message mới → emit `message.batch_ready`
- **Critical pattern:** Match "huỷ đơn", "cancel", CIX `urgent_intent` → flush ngay

---

## 5. Decision Case (Supplement §4)

### 5.1 Lifecycle

```
opened
  → context_collecting
  → ready_for_decision
  → decided
  → actions_created
  → executing
  → outcome_waiting
  → closed (closed_complete | closed_timeout | closed_manual)

  → cancelled | expired | dropped | merged | reopened
```

### 5.2 Schema chuẩn

```json
{
  "decision_case_id": "dcs_xxx",
  "org_id": "org_xxx",
  "root_event_id": "evt_root",
  "trigger_event_ids": [],
  "latest_event_id": "",
  "entity_refs": { "customer_id": "", "conversation_id": "", "order_id": "" },
  "case_type": "conversation_response_decision",
  "priority": "high",
  "urgency": "realtime",
  "status": "context_collecting",
  "required_contexts": ["cix", "customer"],
  "received_contexts": ["customer"],
  "context_packets": {},
  "decision_packet": null,
  "action_ids": [],
  "execution_ids": [],
  "outcome_summary": null,
  "closure_type": "closed_complete | closed_timeout | closed_manual",
  "opened_at": "",
  "closed_at": null
}
```

### 5.3 Case Types

| case_type | Mô tả |
|-----------|-------|
| `conversation_response_decision` | Trả lời / xử lý hội thoại |
| `customer_state_decision` | Trạng thái khách (winback, churn) |
| `order_risk_decision` | Đơn hàng (risk, follow-up) |
| `ads_optimization_decision` | Tối ưu ads |
| `execution_recovery_decision` | Xử lý execution failed |

### 5.4 Closure types (Supplement §7.2) — BẮT BUỘC tách rõ

| Loại đóng | Điều kiện | Learning |
|-----------|-----------|----------|
| **closed_complete** | Tất cả action có outcome (executed/failed/rejected) | ✅ Sinh learning_case đầy đủ |
| **closed_timeout** | Timeout khi chưa đủ outcome (config, ví dụ 24h) | ⚠️ Skip hoặc flag `incomplete` |
| **closed_manual** | Đóng thủ công (cancel, drop) | ⚠️ Skip hoặc flag `incomplete` |

**Chỉ `closed_complete` mới sinh learning_case đầy đủ.**

### 5.5 Merge & Reopen

- **Merge:** Cùng entity, case_type, chưa closed, trong time window (config ví dụ 30 phút)
- **Reopen:** Case closed nhưng trong `reopen_window` (ví dụ 5 phút) → REOPEN thay vì tạo case mới

---

## 6. Domain Workers (Supplement §11)

| Domain | Worker | Consume | Emit |
|--------|--------|---------|------|
| CIX | `worker.cix.request` | `cix.analysis_requested` | `cix.analysis_completed` |
| CRM | `worker.crm.request` | `customer.context_requested` | `customer.context_ready` |
| Ads | `worker.ads.request` | `ads.context_requested` | `ads.context_ready` |
| Order | `worker.order.request` | `order.recompute_requested` | `order.flags_emitted` |

**Retry backoff (BẮT BUỘC):** Attempt 1→5s, 2→30s, 3→2 phút, 4→10 phút. `scheduled_at = now + delay` → event chuyển `deferred`.

---

## 7. Phương Án Triển Khai — Theo Supplement §12

### Phase 1 (bắt buộc)

| # | Công việc | Module | Chi tiết |
|---|-----------|--------|----------|
| 1 | **Event system + worker** | aidecision, internal | Collection `decision_events_queue`, event envelope, worker consume theo lane |
| 2 | **Decision case** | aidecision | Collection `decision_cases_runtime`, model, ResolveOrCreate, lifecycle |
| 3 | **AI Decision orchestration** | aidecision | Luồng: nhận event → resolve case → context → emit request nếu thiếu → nhận result → decision → action |
| 4 | **CIX + Customer domain workers** | cix, crm | Worker consume Work Request, emit Result |
| 5 | **Debounce worker** | aidecision hoặc cio | `worker.debounce_aggregator` — emit `message.batch_ready` |
| 6 | **Basic executor** | executor | Giữ nguyên, gắn outcome → case closure |

### Phase 2

| # | Công việc |
|---|-----------|
| 1 | Order intelligence + worker |
| 2 | Ads intelligence + worker |
| 3 | Decision log + learning_case (1 per action) |
| 4 | Closure type: chỉ closed_complete sinh learning đầy đủ |

### Phase 3

| # | Công việc |
|---|-----------|
| 1 | Auto rule generation |
| 2 | Cross-merchant learning |

---

## 8. Cấu Trúc Module & File Đề Xuất

### 8.1 Module aidecision (mở rộng)

```
api/internal/api/aidecision/
├── dto/
│   ├── dto.aidecision.event.go        # Event envelope (evt_xxx)
│   ├── dto.aidecision.decision_case.go # Decision case schema
│   └── dto.aidecision.execute.go      # ExecuteRequest (backward compat)
├── models/
│   └── model.aidecision.decision_case.go
├── handler/
│   ├── handler.aidecision.execute.go  # POST /ai-decision/execute (giữ)
│   └── handler.aidecision.events.go    # POST /ai-decision/events (ingest event)
├── router/
│   └── routes.go
├── service/
│   ├── service.aidecision.engine.go    # Orchestration chính
│   ├── service.aidecision.case.go      # ResolveOrCreate, UpdateCase, CloseCase
│   ├── service.aidecision.context.go  # Context Aggregation, Context Policy Matrix
│   └── service.aidecision.cix.go       # ReceiveCixPayload (backward compat → emit event)
└── worker/
    └── worker.aidecision.consumer.go  # Consume decision_events_queue
```

### 8.2 Module mới / mở rộng

| Module | Thêm |
|--------|------|
| **internal/eventqueue** | Event queue service, emit, lease, complete, fail |
| **worker.debounce** | worker.debounce_aggregator |
| **cix/worker** | worker.cix.request — consume cix.analysis_requested |
| **crm/worker** | worker.crm.request — consume customer.context_requested |

### 8.3 Collections cần tạo

| Collection | Index đề xuất |
|------------|---------------|
| `decision_events_queue` | (status, lane, org_id, scheduled_at), (event_id) unique |
| `decision_cases_runtime` | (org_id, status), (org_id, entity_refs.conversation_id, case_type, status) |

---

## 9. Context Policy Matrix (Supplement §3.4)

| Decision | Required | Optional |
|----------|----------|----------|
| reply | conversation, customer | order |
| winback | customer, order | conversation |
| ads_opt | ads, order | customer |
| escalate | conversation (CIX), customer | — |

---

## 10. Urgency & Fallback Policy (Supplement §3.5, §3.6)

| Mức | Timeout | Ví dụ |
|-----|---------|-------|
| `realtime` | 30s | Message VIP, order mới |
| `near_realtime` | 2 phút | CIX analysis, customer update |
| `deferred` | Batch | Ads insight |

| Fallback | Mô tả |
|----------|-------|
| `proceed_partial` | Quyết định với context có |
| `defer` | Chờ thêm, đánh dấu waiting_children |
| `drop` | Bỏ qua event |
| `escalate_human` | Chuyển human |

---

## 11. Action Idempotency (Supplement §6.2) — BẮT BUỘC

Action payload bắt buộc có:

```json
{
  "idempotency_key": "decision_case_id:action_type:version"
}
```

Executor check `idempotency_key` trước khi tạo mới — retry không tạo action trùng.

---

## 12. Backward Compatibility

| Hiện tại | Chuyển đổi |
|----------|------------|
| CIX gọi `ReceiveCixPayload` sync | `ReceiveCixPayload` → tạo event `cix.analysis_completed` (coi như result) → push queue → AI Decision worker consume. Hoặc gọi trực tiếp `ProcessEvent` nội bộ |
| `OnCioEventInserted` → EnqueueAnalysis | Phase 1: giữ. Phase 2: OnCioEventInserted → emit `conversation.message_inserted` → queue → AI Decision emit `cix.analysis_requested` → CIX worker |
| `decision_cases` (Learning) | Rename/migrate → `learning_cases` |

---

## 13. Checklist Thực Hiện

### Phase 1
- [x] Tạo collection `decision_events_queue`, model event envelope
- [x] Tạo collection `decision_cases_runtime`, model decision case
- [x] Event queue service: emit, lease, complete, fail
- [x] AI Decision worker: consume event, ResolveOrCreate case
- [x] Context Aggregation: merge từ context_packets
- [x] Emit Work Request khi thiếu context (cix.analysis_requested, customer.context_requested)
- [x] CIX worker: consume cix.analysis_requested → EnqueueAnalysis → emit cix.analysis_completed
- [x] CRM worker: consume customer.context_requested → emit customer.context_ready
- [x] Debounce worker: message.batch_ready
- [x] Khi đủ context: decision_packet → action → Executor (Propose)
- [x] Action idempotency_key

### Phase 2
- [x] Order intelligence + worker
- [x] Ads intelligence + worker
- [x] Closure type: closed_complete / closed_timeout / closed_manual
- [x] Learning: chỉ closed_complete sinh learning_case đầy đủ; 1 action = 1 learning case
- [x] Migrate decision_cases → learning_cases

### Phase 3
- [x] Auto rule generation (worker, service, API GET/PATCH rule-suggestions)
- [ ] Cross-merchant learning (stub: config, schedule, model; worker nil)

---

## 14. Rủi Ro & Mitigation

| Rủi ro | Mitigation |
|--------|------------|
| Breaking CIX flow | Phase 1: ReceiveCixPayload vẫn hoạt động — chuyển sang emit event nội bộ hoặc gọi ProcessEvent |
| Event queue chưa có | Dùng MongoDB collection làm queue; worker poll hoặc change stream |
| Multi-tenant block | Queue theo org, quota per org, fair scheduling |
| Learning sai khi timeout | Chỉ closed_complete sinh learning đầy đủ; closed_timeout/manual skip hoặc flag incomplete |

---

## 15. Tài Liệu Tham Chiếu

| Tài liệu | Đường dẫn |
|----------|-----------|
| **Canonical — Supplement** | [PLATFORM_L1_EVENT_DECISION_SUPPLEMENT.md](../../docs/architecture/vision/PLATFORM_L1_EVENT_DECISION_SUPPLEMENT.md) |
| **Spec code-level** | [PHUONG_AN_SUA_MODULE_AI_DECISION_SPEC_CODE.md](./PHUONG_AN_SUA_MODULE_AI_DECISION_SPEC_CODE.md) |
| Vision AI Decision | [08 - ai-decision.md](../../docs/architecture/vision/08%20-%20ai-decision.md) |
| Learning Engine | [11 - learning-engine.md](../../docs/architecture/vision/11%20-%20learning-engine.md) |
| Event flow chi tiết | [THIET_KE_EVENT_FLOW_AI_DECISION.md](../../docs/architecture/vision/THIET_KE_EVENT_FLOW_AI_DECISION.md) |
| Đề xuất sửa code | DE_XUAT_SUA_CODE_THEO_VISION.md |

---

## 16. Đã Triển Khai (Phase 1)

| # | Thành phần | Vị trí | Trạng thái |
|---|------------|--------|------------|
| 1 | Spec code-level | PHUONG_AN_SUA_MODULE_AI_DECISION_SPEC_CODE.md | ✅ |
| 2 | Model DecisionEvent | aidecision/models/model.aidecision.event.go | ✅ |
| 3 | Model DecisionCase | aidecision/models/model.aidecision.decision_case.go | ✅ |
| 4 | Collections | decision_events_queue, decision_cases_runtime, decision_debounce_state | ✅ |
| 5 | Event queue service | EmitEvent, LeaseOne, LeaseOneByEventType, CompleteEvent, FailEvent | ✅ |
| 6 | API POST /ai-decision/events | handler.aidecision.events.go | ✅ |
| 7 | API POST /ai-decision/cases/:id/close | handler.aidecision.case.go | ✅ |
| 8 | UIDPrefixDecisionCase | utility/uid.go | ✅ |
| 9 | ResolveOrCreate, UpdateCaseWithCixContext, UpdateCaseWithCustomerContext | service.aidecision.case.go | ✅ |
| 10 | AI Decision consumer worker | worker.aidecision.consumer.go | ✅ |
| 11 | CIX Request worker | worker.cix.request.go — consume cix.analysis_requested → EnqueueAnalysis | ✅ |
| 12 | CRM Context worker | worker.crm.context.go — consume customer.context_requested → emit customer.context_ready | ✅ |
| 13 | Debounce worker | worker.aidecision.debounce.go — emit message.batch_ready | ✅ |
| 14 | Closure worker | worker.aidecision.closure.go — closed_timeout | ✅ |
| 15 | CIO hook (event-driven) | init.registry.go — RegisterAIDecisionOnDataChanged (luôn queue) | ✅ |
| 16 | OnActionClosed callback | pkg/approval/engine.go — TryCloseCaseWhenAllActionsDone | ✅ |
| 17 | Learning: closed_complete | createLearningCasesForClosedCase (1 per action) | ✅ |
| 18 | Action idempotency_key | decision_case_id:action_type:version | ✅ |

**Env (tùy chọn):** `AI_DECISION_DEBOUNCE_ENABLED`, `AI_DECISION_CLOSURE_MAX_AGE_HOURS`. *Context readiness theo **Context Policy Matrix** (code `contextpolicy`), không còn `AI_DECISION_REQUIRE_BOTH_CONTEXT`.* *(Đã bỏ `AI_DECISION_EVENT_DRIVEN` / `AI_DECISION_USE_CIX_REQUEST` — luôn event-driven; CixRequest worker luôn bật.)*

**Phase 1 hoàn thành.**

### Phase 2 đã triển khai

| # | Thành phần | Vị trí | Trạng thái |
|---|------------|--------|------------|
| 1 | Order Context worker | pc/worker/worker.pc.order_context.go | ✅ |
| 2 | ads.context_requested → ready | consumer AID (`service.aidecision.ads_context_payload` + dispatch) | ✅ |
| 3 | UpdateCaseWithOrderContext, UpdateCaseWithAdsContext | service.aidecision.case.go | ✅ |
| 4 | Consumer: order.flags_emitted, ads.context_ready | worker.aidecision.consumer.go | ✅ |
| 5 | LearningCases collection | learning_cases, LearningCaseService dùng LearningCases | ✅ |
| 6 | CloseCase 404 | CloseCaseWithOrgCheck trả ErrNotFound | ✅ |

### Phase 3 đã triển khai

| # | Thành phần | Vị trí | Trạng thái |
|---|------------|--------|------------|
| 1 | Rule Suggestion worker | learning/worker/worker.learning.rule_suggestion.go | ✅ |
| 2 | AnalyzeAndSuggestRules, ListRuleSuggestions | service.learning.rule_suggestion.go | ✅ |
| 3 | GET /learning/rule-suggestions | handler.learning.rule_suggestion.go | ✅ |
| 4 | PATCH /learning/rule-suggestions/:id | handler.learning.rule_suggestion.go | ✅ |
| 5 | Collections rule_suggestions, learning_insights_aggregate | init.go | ✅ |
| 6 | Cross-merchant (stub) | WorkerLearningInsightAggregate = nil | ⏳ |

**Env:** `LEARNING_RULE_SUGGESTION_ENABLED=true` — bật worker phân tích learning_cases → rule suggestions.

---

## Changelog

- 2026-03-19: Phase 3 hoàn thành — Auto rule generation (worker, API GET/PATCH rule-suggestions)
- 2026-03-19: Phase 2 hoàn thành — Order/Ads workers, migrate learning_cases, sửa CloseCase 404
- 2026-03-19: Phase 1 hoàn thành — CIX Request worker, checklist, docs
- 2026-03-19: Triển khai Phase 1 phần đầu — models, collections, event queue, API events
- 2026-03-19: Cập nhật bám sát PLATFORM_L1_EVENT_DECISION_SUPPLEMENT — Event System, Decision Case, Domain workers, Collections, Closure types, Phase theo supplement
- 2026-03-19: Tạo phương án sửa module AI Decision theo vision
