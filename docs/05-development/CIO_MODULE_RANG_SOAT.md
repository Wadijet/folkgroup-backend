# Rà Soát Module CIO — Đối Chiếu Tài Liệu vs Triển Khai

**Ngày:** 2025-03-16  
**Tham chiếu:** Phần 3 CIO (docs-shared), [THIET_KE_MODULE_CIO.md](./THIET_KE_MODULE_CIO.md), [PHUONG_AN_TRIEN_KHAI_CIO.md](./PHUONG_AN_TRIEN_KHAI_CIO.md)

---

## 1. Tổng Quan

| Hạng mục | Đã làm | Chưa làm |
|----------|--------|----------|
| **Collections** | 6/6 | 0 |
| **Models** | 6/6 | 0 |
| **Phase 1** | ✅ | — |
| **Phase 2** | ✅ | — |
| **Phase 3** | ✅ | — |
| **Phase 4** | ✅ | — |
| **Phase 5** | ⚠️ Một phần | Webhook CIO |
| **Phase 6** | ⚠️ Một phần | ROUTING_MODE, max_messages_per_24h, actor, session states |
| **Plan Definition** | ✅ | Chi tiết step (inputRef, outputRef, ruleRef, actionRef) |

---

## 2. Collections & Models

| Collection | Trạng thái | Model |
|------------|------------|-------|
| `cio_events` | ✅ Đã đăng ký | CioEvent (EventAt, CreatedAt) |
| `cio_sessions` | ✅ Đã đăng ký | CioSession |
| `cio_touchpoint_plans` | ✅ Đã đăng ký | CioTouchpointPlan (experimentId, variant, variantConfig) |
| `cio_routing_decisions` | ✅ Đã đăng ký | CioRoutingDecision (experimentId, variant) |
| `cio_plan_definitions` | ✅ Đã đăng ký | CioPlanDefinition (khung, steps, TargetAudience) |
| `cio_plan_executions` | ✅ Đã đăng ký | CioPlanExecution (status, completedSteps, executedActions, resume) |

---

## 3. API Endpoints

| Endpoint (theo tài liệu) | Trạng thái | Ghi chú |
|--------------------------|------------|---------|
| POST /cio/touchpoint/plan | ✅ | HandlePlanTouchpoint |
| POST /cio/touchpoint/:id/execute | ✅ | HandleExecuteTouchpoint |
| POST /cio/webhook/:channel | ❌ | Chưa có — ingestion qua sync + IngestConversationTouchpoint |
| GET /cio/sessions | ✅ | Danh sách sessions (filter: unifiedId, channel, state) |
| GET /cio/routing-decisions | ✅ | Danh sách routing decisions (filter: unifiedId, channelChosen, experimentId) |
| GET /cio/stats | ✅ | Thống kê dashboard (touchpointByStatus, touchpointByChannel, touchpointByGoal) |
| CRUD /cio/plan-definition | ✅ | Khung CRUD |
| POST /cio/plan-execution | ✅ | Tạo yêu cầu thực thi plan |
| GET /cio/plan-execution/:id | ✅ | Chi tiết execution |
| POST /cio/plan-execution/:id/approve | ✅ | Duyệt và chạy ngay |
| POST /cio/plan-execution/:id/reject | ✅ | Từ chối |
| POST /cio/plan-execution/:id/resume | ✅ | Chạy tiếp khi server restart |

---

## 4. Kiến Trúc 5 Lớp (Phần 3)

### 4.1 Lớp 1: Omnichannel Ingestion

| Tính năng | Trạng thái | Chi tiết |
|-----------|------------|----------|
| cio_events | ✅ | Collection, model, schema |
| fb_conversations → cio_events | ✅ | IngestConversationTouchpoint (sync API + backfill + crm_ingest_worker) |
| fb_messages → cio_events | ✅ | OnMessageUpsert (sync API) |
| EventAt vs CreatedAt | ✅ | EventAt từ nguồn, CreatedAt = ingestion |
| Webhook Pancake → cio_events | ❌ | Chưa hook — webhook chưa gọi CIO |
| POST /cio/webhook/:channel | ❌ | Skeleton chưa có |
| Zalo, Messenger, SMS | ⚠️ | Messenger qua fb_*; Zalo/SMS qua delivery (outbound) |

### 4.2 Lớp 2: Context & State

| Tính năng | Trạng thái | Chi tiết |
|-----------|------------|----------|
| cio_sessions | ✅ | Model có, chưa dùng trong flow |
| Session state (new, engaged, …) | ⚠️ | Model có state; chưa đủ 9 states chi tiết |
| Context Memory | ❌ | Chưa — Phase 6+ |

### 4.3 Lớp 3: Dynamic Routing

| Tính năng | Trạng thái | Chi tiết |
|-----------|------------|----------|
| RULE_CIO_CHANNEL_CHOICE | ✅ | Seed, PlanTouchpoint gọi |
| RULE_CIO_FREQUENCY_CHECK | ✅ | Seed, chạy trước CHANNEL_CHOICE |
| RULE_CIO_ROUTING_MODE | ❌ | Design có, chưa seed — AI vs Human |
| [Value: VIP] → Human | ❌ | Cần ROUTING_MODE |
| [Intent: Complaint] → Human | ❌ | Cần AI/NLP + rule |
| Capacity-based | ❌ | Phase 6+ |

### 4.4 Lớp 4: Frequency & Channel Control

| Tính năng | Trạng thái | Chi tiết |
|-----------|------------|----------|
| Cooldown | ✅ | RULE_CIO_FREQUENCY_CHECK |
| Channel choice (valueTier, lifecycleStage) | ✅ | RULE_CIO_CHANNEL_CHOICE |
| max_messages_per_24h | ❌ | Chưa thêm vào FREQUENCY_CHECK |
| preferred_channel | ⚠️ | CI chưa có field; rule dùng valueTier |

### 4.5 Lớp 5: Feedback Loop

| Tính năng | Trạng thái | Chi tiết |
|-----------|------------|----------|
| BuildDecisionCaseFromCIOChoice | ✅ | ExecuteTouchpoint → async gọi |
| experimentId, variant trong decision case | ✅ | SubmitDecisionCaseFromTouchpoint |
| Trace (rule_id, logic_version, param_version) | ✅ | cio_routing_decisions |
| Gửi raw đến CI | ❌ | CIO chuyển raw; CI extract — đã phân tách |

---

## 5. Tích Hợp Module

| Tích hợp | Trạng thái | Chi tiết |
|----------|------------|----------|
| **CRM → CIO** | ✅ | trigger-follow-up → PlanTouchpoint |
| **CIO → Rule Engine** | ✅ | CioRoutingService.ChooseChannel |
| **CIO → Decision Brain** | ✅ | BuildDecisionCaseFromCIOChoice |
| **CIO → notifytrigger** | ✅ | ExecuteTouchpoint |
| **FB sync → CIO** | ✅ | DataChangeEvent → crm_ingest_worker → IngestConversationTouchpoint |
| **FB sync → cio_events** | ✅ | OnConversationUpsert, OnMessageUpsert (trong IngestConversationTouchpoint + fb_message handler) |
| **Backfill → CIO** | ✅ | IngestConversationTouchpoint đồng bộ CRM + CIO |
| **Webhook Pancake → CIO** | ❌ | Chưa hook |

---

## 6. Phase Triển Khai (PHUONG_AN)

### Phase 1: Foundation — ✅ Hoàn thành

| # | Công việc | Trạng thái |
|---|-----------|------------|
| 1 | Collections | ✅ |
| 2 | Models | ✅ |
| 3 | DTO | ✅ |
| 4 | PlanTouchpoint | ✅ |
| 5 | ExecuteTouchpoint | ✅ |
| 6 | Handler, Router | ✅ |
| 7–8 | main.go, permissions | ✅ |
| 9 | Query crm_customers | ✅ |

### Phase 2: Rule Intelligence — ✅ Hoàn thành

| # | Công việc | Trạng thái |
|---|-----------|------------|
| 1–6 | OUT_CIO_*, LOGIC_CIO_*, PARAM_CIO_*, RULE_CIO_* | ✅ |
| 7 | CioRoutingService | ✅ |
| 8 | PlanTouchpoint gọi ChooseChannel | ✅ |
| 9 | cio_routing_decisions | ✅ |
| 10 | params.nowMs | ✅ |
| 11 | RULE_CIO_ROUTING_MODE | ❌ Optional |
| 12 | max_messages_per_24h | ❌ |

### Phase 3: CRM & Decision Brain — ✅ Hoàn thành

| # | Công việc | Trạng thái |
|---|-----------|------------|
| 1 | CRM trigger_follow_up → PlanTouchpoint | ✅ |
| 2 | Inject CioTouchpointService | ✅ |
| 3 | BuildDecisionCaseFromCIOChoice | ✅ |
| 4 | Schema decision case cio_choice | ✅ |
| 5 | ExecuteTouchpoint → feedback async | ✅ |
| 6 | EventType notifytrigger CIO | ⚠️ | Dùng eventType generic |

### Phase 4: A/B Test — ✅ Hoàn thành

| # | Công việc | Trạng thái |
|---|-----------|------------|
| 1 | experimentId, variant, variantConfig trong model | ✅ |
| 2 | experimentId, variant trong CioRoutingDecision | ✅ |
| 3 | PlanTouchpointRequest thêm ExperimentId, Variants | ✅ |
| 4 | AssignVariant logic | ✅ |
| 5 | PlanTouchpoint gọi AssignVariant | ✅ |
| 6 | BuildDecisionCaseFromCIOChoice experimentId, variant | ✅ |
| 7 | cio_experiments collection | ❌ Optional |

### Phase 5: Ingestion — ⚠️ Một phần

| # | Công việc | Trạng thái |
|---|-----------|------------|
| 1 | OnConversationUpsert | ✅ (cio/ingestion) |
| 2 | Hook SyncUpsertOne | ⚠️ | Qua DataChangeEvent → crm_ingest_worker → IngestConversationTouchpoint |
| 3 | IngestConversationTouchpoint đồng bộ CRM + CIO | ✅ |
| 4 | fb_message → OnMessageUpsert | ✅ |
| 5 | Webhook handler POST /cio/webhook/:channel | ❌ |
| 6 | Webhook Pancake → CIO | ❌ |

### Phase 6: Mở Rộng — ⚠️ Một phần

| # | Công việc | Trạng thái |
|---|-----------|------------|
| 1 | RULE_CIO_ROUTING_MODE | ❌ |
| 2 | max_messages_per_24h | ❌ |
| 3 | Field actor vào cio_events | ❌ |
| 4 | Session states mở rộng | ❌ |
| 5 | GET /cio/stats | ✅ |
| 6 | preferred_channel | ❌ |

---

## 7. Plan Definition (DE_XUAT_CIO_PLAN_DEFINITION)

| Hạng mục | Trạng thái | Chi tiết |
|----------|------------|----------|
| Collection cio_plan_definitions | ✅ | |
| Model CioPlanDefinition, CioPlanStepRef | ✅ | Khung |
| Target Audience | ✅ | lifecycleStages, valueTiers, customFilter — filter KH khi tạo execution |
| CRUD API /cio/plan-definition | ✅ | |
| Plan Executor | ✅ | Chạy steps (rule, action, condition), tích hợp Rule Engine |
| Plan Execution flow | ✅ | Tạo → Duyệt → Thực thi. cio_plan_executions, cập nhật tiến độ, executedActions |
| Worker auto-resume | ✅ | cio_plan_resume worker (api/internal/api/cio/worker/) — resume stale running |
| Chi tiết step (inputRef, outputRef, ruleRef, actionRef) | ❌ | Làm sau |
| Experiment gắn planId+planVersion | ❌ | |

---

## 8. File Structure — Đối Chiếu

| File (theo THIET_KE) | Trạng thái |
|----------------------|------------|
| model.cio.event.go | ✅ |
| model.cio.session.go | ✅ |
| model.cio.touchpoint_plan.go | ✅ |
| model.cio.routing_decision.go | ✅ |
| model.cio.plan_definition.go | ✅ |
| model.cio.plan_execution.go | ✅ |
| dto.cio.touchpoint.go | ✅ |
| dto.cio.plan_definition.go | ✅ (mới) |
| service.cio.routing.go | ✅ |
| service.cio.touchpoint.go | ✅ |
| service.cio.feedback.go | ✅ |
| service.cio.plan_definition.go | ✅ |
| service.cio.plan_execution.go | ✅ |
| service.cio.plan_executor.go | ✅ |
| ingestion/ingestion.go | ✅ (tách package) |
| handler.cio.touchpoint.go | ✅ |
| handler.cio.plan_definition.go | ✅ |
| handler.cio.plan_execution.go | ✅ |
| handler.cio.webhook.go | ❌ |
| handler.cio.list.go | ✅ | HandleListSessions, HandleListRoutingDecisions, HandleGetStats |
| worker/worker.cio.plan_resume.go | ✅ | CIO Plan Resume Worker — auto-resume stale running executions |

---

## 9. Tiêu Chí Hoàn Thành (PHUONG_AN §11)

| Tiêu chí | Trạng thái |
|----------|------------|
| API PlanTouchpoint, Execute hoạt động | ✅ |
| Channel choice qua Rule Engine | ✅ |
| Frequency check qua Rule Engine | ✅ |
| CRM gọi CIO khi có recommendation | ✅ |
| Decision case cio_choice được tạo | ✅ |
| A/B test: experimentId, variant được ghi và attribution | ✅ |

---

## 10. Tóm Tắt Ưu Tiên Chưa Làm

| Ưu tiên | Công việc |
|---------|-----------|
| **Cao** | Webhook Pancake → CIO (khi cần realtime từ webhook) |
| **Trung bình** | POST /cio/webhook/:channel (skeleton) |
| **Thấp** | RULE_CIO_ROUTING_MODE, max_messages_per_24h |
| **Thấp** | actor field, session states mở rộng |
| **Optional** | cio_experiments collection (A/B vẫn chạy khi truyền variants trong request) |

---

## 11. Đánh Giá: CIO Tạm Đủ Dùng

**Kết luận:** CIO đủ dùng cho luồng chính (CRM → PlanTouchpoint → Execute, channel choice, frequency check, Plan Execution, A/B test, feedback Decision Brain). Có thể dùng production nếu ingestion qua sync/backfill đủ, chưa cần webhook realtime.

---

## 12. Changelog

- 2025-03-16: Tạo bản rà soát toàn bộ module CIO.
- 2025-03-16: Hoàn thành Phase 4 A/B test (AssignVariant, PlanTouchpointRequest.ExperimentId/Variants).
- 2025-03-16: Thêm API GET /cio/sessions, GET /cio/routing-decisions, GET /cio/stats.
- 2025-03-16: Plan Executor — service.cio.plan_executor.go, step types rule/action/condition, tích hợp PlanTouchpoint (planId) và POST /cio/plan/execute.
- 2025-03-16: Plan Execution flow — Tạo yêu cầu → Duyệt → Thực thi. Collection cio_plan_executions, cập nhật tiến độ, resume khi restart (không gửi lại action đã thực hiện).
- 2025-03-16: Target Audience — CioPlanTargetAudience (lifecycleStages, valueTiers), customerMatchesTarget khi tạo execution.
- 2025-03-16: Worker auto-resume — cio_plan_resume worker (api/internal/api/cio/worker/), quản lý như Ads workers. ListStaleRunningExecutions + ResumeExecution.
