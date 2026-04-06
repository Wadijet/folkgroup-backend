# Đối Chiếu Module — Vision vs Codebase (2026-03-19)

**Mục đích:** Đối chiếu từng module theo vision với code thực tế — đã có gì, cần làm gì.

**Nguồn:** Vision `docs/architecture/vision/`, Code `api/internal/api/`, `api/pkg/approval`, `api/internal/executors/`

> **Ghi chú 2026-04-07:** **Raw→L1→L2→L3** trong bảng dưới = **pipeline rule CIX**; **L1-persist/L2-persist** = mirror/canonical (data contract). `docs/05-development/KHUNG_KHUON_MODULE_INTELLIGENCE.md` mục 0.

---

## 1. Module CIO — Customer Interaction Orchestrator

### Vision
- Universal Data Ingestion Hub: nhận mọi nguồn, chuẩn hóa, lưu trữ, emit Decision Events
- Không orchestrate, không quyết định
- Agent gửi raw qua `POST /cio/ingest/*`

### Đã có trong code

| Hạng mục | Vị trí | Ghi chú |
|----------|--------|---------|
| Module cio/ | `api/internal/api/cio/` | router, handler, service, models |
| Collections | cio_events, cio_sessions, cio_touchpoint_plans, cio_routing_decisions, cio_plan_definitions, cio_plan_executions | ✅ |
| PlanTouchpoint, ExecuteTouchpoint | service.cio.touchpoint.go | Propose qua pkg/approval |
| Plan Execution | service.cio.plan_execution.go, plan_executor.go | Tạo → Duyệt → Thực thi, resume |
| Rule Engine routing | RULE_CIO_CHANNEL_CHOICE, RULE_CIO_FREQUENCY_CHECK | service.cio.routing.go |
| OnCioEventInserted → CIX | init.registry.go | cioingestion.OnCioEventInserted → EnqueueAnalysis |
| Worker plan_resume | worker.cio.plan_resume.go | ✅ |
| API | GET /cio/sessions, /cio/routing-decisions, /cio/stats | ✅ |
| Ingestion FB | cio/ingestion/ingestion.go | IngestConversationTouchpoint (sync, backfill) |

### Cần làm

| # | Hạng mục | Chi tiết |
|---|----------|----------|
| 1 | **POST /cio/ingest/*** | Vision: Agent gửi raw qua `POST /cio/ingest/interaction`, `/cio/ingest/order`, `/cio/ingest/ads`, `/cio/ingest/crm`. Chưa có endpoint |
| 2 | **Zalo, Web chat, Telegram, Call** | Chưa có kênh |
| 3 | **POST /cio/webhook/:channel** | Ingestion qua sync/backfill, chưa webhook |
| 4 | **RULE_CIO_ROUTING_MODE** | AI vs Human routing — chưa seed |
| 5 | **cio_orders, cio_ads** | Vision: CIO lưu order/ads events. Hiện order qua webhook Pancake, ads qua meta/ |

---

## 2. Module CIX — Contextual Conversation Intelligence

### Vision
- Raw→L1→L2→L3→Flags, KHÔNG tạo Action
- Emit Decision Event cix.signal_update
- Được AI Decision gọi (không subscribe trực tiếp)

### Đã có trong code

| Hạng mục | Vị trí | Ghi chú |
|----------|--------|---------|
| Module cix/ | `api/internal/api/cix/` | router, handler, service, models |
| Collections | cix_analysis_results, cix_pending_analysis | ✅ |
| Rule Engine pipeline | RULE_CIX_LAYER1_STAGE, LAYER2_*, LAYER3_*, FLAGS, ACTIONS | Đã seed |
| CIO → CIX | OnCioEventInserted → EnqueueAnalysis | init.registry.go |
| CIX → AI Decision | ReceiveCixPayload (sync) | service.cix.analysis.go L351 |
| CRM integration | getCustomerContext | service.cix.analysis.go |
| Executor CIX | executors/cix/ | trigger_fast_response, escalate_to_senior, assign_to_human_sale, prioritize_followup → Delivery |
| API | POST /cix/analyze, GET /cix/analysis/:sessionUid | ✅ |

### Cần làm

| # | Hạng mục | Chi tiết |
|---|----------|----------|
| 1 | **Event-driven** | Vision: AI Decision nhận event → gọi CIX. Hiện: CIX worker chạy → gọi ReceiveCixPayload sync. Có thể giữ sync cho CIX flow |
| 2 | **cix_conversations** | Vision: merged context — chưa rõ collection |

---

## 3. Module AI Decision

### Vision
- 3 lớp: Event Intake, Context Aggregation, Decision Core
- Nhận Decision Events, phân phối xuống domain, tổng hợp context, tạo Action
- Không approval, không execution

### Đã có trong code

| Hạng mục | Vị trí | Ghi chú |
|----------|--------|---------|
| Module aidecision/ | `api/internal/api/aidecision/` | router, handler, service |
| Execute | service.aidecision.engine.go | Nhận CIXPayload, parse actionSuggestions |
| ReceiveCixPayload | service.aidecision.cix.go | Entry từ CIX |
| applyPolicy | env CIX_APPROVAL_ACTIONS | Phân tách needApproval vs auto |
| proposeCixAction, proposeAndApproveAutoCixAction | service.aidecision.cix.go | Propose / ProposeAndApproveAuto qua pkg/approval |
| API | POST /ai-decision/execute | ✅ |

### Cần làm

| # | Hạng mục | Chi tiết |
|---|----------|----------|
| 1 | **Event Intake** | Vision: nhận Decision Events, normalize, filter. Hiện: sync ReceiveCixPayload, không consume event |
| 2 | **Context Aggregation** | Vision: merge CIX + Customer + Ads + Order. Hiện: chỉ CIXPayload, CustomerCtx chưa merge đầy đủ |
| 3 | **Arbitration** | Vision: resolve conflict khi nhiều action cùng target. Chưa có |
| 4 | **Policy từ config** | Vision: ApprovalModeConfig. Hiện: env CIX_APPROVAL_ACTIONS |
| 5 | **Phân phối event** | Vision: AI Decision nhận event → gọi CIX/Ads/Customer. Hiện: chỉ CIX gọi qua ReceiveCixPayload |

---

## 4. Module Executor

### Vision
- 7 sub-layer: Intake, Validation, Policy/Approval, Dispatch, Monitoring, Outcome, Learning Handoff
- Approval modes: manual_required, auto_by_rule, ai_recommend_human_confirm, fully_auto
- ResolveImmediate: đọc config → auto Approve nếu mode=auto

### Đã có trong code

| Hạng mục | Vị trí | Ghi chú |
|----------|--------|---------|
| Module executor/ | `api/internal/api/executor/` | router, handler |
| pkg/approval | Propose, Approve, Reject, ExecuteOne, ProposeAndApproveAuto | engine.go |
| action_pending_approval | Collection | ✅ |
| Executor CIX, Ads | executors/cix, ads/worker | Đăng ký Execute |
| /executor/actions/* | propose, approve, reject, execute, cancel, find, pending | ✅ |
| /executor/send, /executor/execute | delivery handler | ✅ |
| /executor/history | notification history | ✅ |

### Cần làm

| # | Hạng mục | Chi tiết |
|---|----------|----------|
| 1 | **ApprovalModeConfig** | Vision: collection config (domain, scopeKey, mode, actionOverrides). Chưa có |
| 2 | **ResolveImmediate** | Vision: sau Propose, đọc config → auto Approve nếu mode=auto. Chưa có |
| 3 | **Action Policy Registry** | Vision: định nghĩa action type, approval mode, risk tier. Chưa có |
| 4 | **Guardrail Engine** | Vision: kiểm tra trước khi duyệt/chạy. Chưa có |
| 5 | **Outcome Registry** | Vision: outcome cần đo, evaluation window. Chưa có |

---

## 5. Module Learning Engine

### Vision
- Chỉ học khi lifecycle kết thúc
- Build learning_case từ context + action + outcome
- Không tham gia runtime

### Đã có trong code

| Hạng mục | Vị trí | Ghi chú |
|----------|--------|---------|
| Module learning/ | `api/internal/api/learning/` | router, handler, service |
| decision_cases | Collection | ✅ |
| BuildLearningCaseFromAction | service.learning.builder.go | action_pending (executed/rejected/failed) |
| BuildDecisionCaseFromCIOChoice | service.learning.builder.go | cio_choice |
| CreateLearningCaseFromAction | service.learning.case.go | ✅ |
| API | GET/POST /learning/cases | ✅ |
| Trigger | worker.ads.execution, handler.executor.action, service.aidecision.engine | ✅ |

### Cần làm

| # | Hạng mục | Chi tiết |
|---|----------|----------|
| 1 | **Content Choice cases** | Chọn creative, content line — tương lai |
| 2 | **Creative lifecycle cases** | Case khi creative đóng — chưa pipeline |
| 3 | **Retrieval cho AI** | Embedding/search — chưa có |

---

## 6. Module Delivery

### Vision
- Chỉ nhận từ Executor (source=APPROVAL_GATE)
- Không đường đi tắt

### Đã có trong code

| Hạng mục | Vị trí | Ghi chú |
|----------|--------|---------|
| handler.delivery.execute | allowDirectDeliveryUse() | DELIVERY_ALLOW_DIRECT_USE=true mới cho phép direct |
| Khi false | Block 403 | "Mọi action phải qua Executor" |
| ExecuteActions | delivery service | executors/cix gọi |

### Cần làm

| # | Hạng mục | Chi tiết |
|---|----------|----------|
| 1 | **Validate source** | Chỉ nhận payload có source=APPROVAL_GATE, actionPendingId |
| 2 | **Deprecation** | DELIVERY_ALLOW_DIRECT_USE: log warning khi true, bỏ ở Phase 3 |

---

## 7. Module Ads

### Vision
- Meta tạo demand, Google capture demand, Cross feed lẫn nhau
- Raw→L1→L2→L3→Flags, KHÔNG tạo Action (AI Decision tạo)
- Approval qua Executor (ResolveImmediate)

### Đã có trong code

| Hạng mục | Vị trí | Ghi chú |
|----------|--------|---------|
| Module ads/, meta/ | Đầy đủ | campaign, adset, ad, insight, activity |
| Rule Engine | RULE_ADS_LAYER1/2/3, RULE_ADS_FLAG_* | ✅ |
| Auto propose worker | worker.ads.auto_propose.go | Scheduler |
| ShouldAutoApprove | service.ads.meta_config.go | **Vi phạm boundary**: logic approval trong domain |
| Propose/ProposeAndApproveAuto | service.ads.propose.go | Gọi pkg/approval |

### Cần làm

| # | Hạng mục | Chi tiết |
|---|----------|----------|
| 1 | **Bỏ ShouldAutoApprove** | Ads luôn Propose; Executor/ResolveImmediate quyết định auto |
| 2 | **Google Ads** | Module mới — chưa có |
| 3 | **Cross Ads** | Service detect creative winner → feed Content |

---

## 8. Module CRM (Customer Intelligence)

### Vision
- Raw→L1→L2→L3→Flags, emit customer.flag_triggered
- KHÔNG tạo Action

### Đã có trong code

| Hạng mục | Vị trí | Ghi chú |
|----------|--------|---------|
| Module crm/ | Đầy đủ | customer, classification, ingest, merge |
| RULE_CRM_CLASSIFICATION | valueTier, lifecycleStage, journeyStage | ✅ |
| Activity Framework | crm_activity_history | ✅ |

### Cần làm

| # | Hạng mục | Chi tiết |
|---|----------|----------|
| 1 | **Intent (I1–I4)** | Chưa có |
| 2 | **Psychographic (Lớp 4)** | ai_personality_tags — chưa có |
| 3 | **Lớp 5** | churn_risk, next_best_action — chưa có |
| 4 | **Segment API** | GET /crm/segments/:id/customers — chưa có |

---

## 9. Module Order Intelligence

### Vision
- Raw→L1→L2→L3→Flags cho order
- AI Decision gọi khi cần
- Trace creative/keyword/conversation → order

### Đã có trong code

| Hạng mục | Vị trí | Ghi chú |
|----------|--------|---------|
| Orders (Pancake) | pc/models/model.pc.pos.order.go | ✅ |
| posData.ad_id, post_id | Link ads, conversation | ✅ |
| Webhook | handler.pancake.pos.webhook | ✅ |

### Cần làm

| # | Hạng mục | Chi tiết |
|---|----------|----------|
| 1 | **Module orderintel/** | Chưa có — cần tạo |
| 2 | **Raw→L1→L2→L3→Flags** | Rule Engine cho Order |
| 3 | **AI Decision gọi** | Khi event commerce.order_* |

---

## 10. Module Content OS

### Vision
- Pipeline L1–L8, Input Factory, insight từ Customer/Ads/Learning

### Đã có trong code

| Hạng mục | Vị trí | Ghi chú |
|----------|--------|---------|
| Content Node L1–L8 | model.content.node, video, publication | ✅ |
| CRUD | content/router, handler, service | ✅ |

### Cần làm

| # | Hạng mục | Chi tiết |
|---|----------|----------|
| 1 | **Insight feed** | Customer/Ads/Learning → Content — chưa pipeline |
| 2 | **Input Factory** | Chưa có |
| 3 | **POST /content/insights/ingest** | Nhận insight từ các module |
| 4 | **Cross Ads** | Creative winner → Content OS |

---

## 11. Module Rule Intelligence

### Đã có trong code

| Hạng mục | Vị trí | Ghi chú |
|----------|--------|---------|
| ruleintel/ | definition, logic, param-set, output-contract | ✅ |
| rule_execution_logs | trace_id | ✅ |
| Seed | Ads, CRM, CIO, CIX | ✅ |

---

## 12. Tổng Hợp — Ưu Tiên Cần Làm

### Phase 1: Approval Gate (2–3 ngày)

| # | Công việc | Module |
|---|-----------|--------|
| 1 | ApprovalModeConfig | executor hoặc internal/approval |
| 2 | GetApprovalMode | internal/approval/config.go |
| 3 | ResolveImmediate | pkg/approval/resolver.go |
| 4 | Ads: bỏ ShouldAutoApprove, luôn Propose | ads/service |

### Phase 2: Delivery Gate (1–2 ngày)

| # | Công việc |
|---|-----------|
| 1 | Validate source=APPROVAL_GATE, actionPendingId |
| 2 | Deprecation warning khi DELIVERY_ALLOW_DIRECT_USE |

### Phase 3: AI Decision 3 lớp (2–3 ngày)

| # | Công việc |
|---|-----------|
| 1 | Event Intake (normalize, filter) |
| 2 | Context Aggregation (merge Ads, Customer) |
| 3 | Arbitration |
| 4 | Policy từ config |

### Phase 4: CIO Ingestion (2–3 ngày)

| # | Công việc |
|---|-----------|
| 1 | POST /cio/ingest/interaction, order, ads, crm |
| 2 | Agent migration |

### Phase 5: Module mới (dài hạn)

| # | Module | Effort |
|---|--------|--------|
| 1 | Order Intelligence | Lớn |
| 2 | Google Ads | Lớn |
| 3 | Input Factory | Trung bình |
| 4 | Cross Ads (service) | Nhỏ |
| 5 | Segment API (crm) | Nhỏ |

---

## 13. Luồng Đã Khép Vòng

```
cio_events (OnCioEventInserted)
    → cix_pending_analysis
    → CIX worker AnalyzeSession
    → ReceiveCixPayload (khi có ActionSuggestions)
    → aidecisionsvc.Execute
    → Propose / ProposeAndApproveAuto
    → executor (action_pending_approval)
    → executors/cix Execute
    → delivery.ExecuteActions
```

**Kết luận:** Luồng CIX đã chạy. Cần bổ sung Approval Gate thống nhất và Delivery Gate validate.

---

## Changelog

- 2026-03-19: Tạo báo cáo đối chiếu module vision vs code
