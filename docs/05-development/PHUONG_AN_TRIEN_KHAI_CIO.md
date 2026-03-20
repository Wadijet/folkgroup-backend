# Phương Án Triển Khai CIO — Customer Interaction Orchestrator

**Ngày:** 2025-03-16  
**Cập nhật:** 2026-03-18 — Phase 1–3, 5 đã triển khai; CIO→CIX→Decision luồng đã khép vòng.  
**Tham chiếu:** [THIET_KE_MODULE_CIO.md](./THIET_KE_MODULE_CIO.md)

---

## 1. Tổng Quan

**CIO là module độc lập** — mọi touchpoint (re-engage, welcome, winback, …) đều qua CIO. Không gộp logic điều phối vào CRM.

| Phase | Tên | Thời gian | Deliverable |
|-------|-----|-----------|-------------|
| 1 | Foundation | 2–3 tuần | Collections, models, PlanTouchpoint, Execute (hardcode) |
| 2 | Rule Intelligence | 2 tuần | RULE_CIO_CHANNEL_CHOICE, FREQUENCY_CHECK, trace |
| 3 | Tích hợp CRM & Decision Brain | 1–2 tuần | CRM → CIO, BuildDecisionCaseFromCIOChoice |
| 4 | A/B Test | 1 tuần | experimentId, variant, assignment |
| 5 | Ingestion (tùy chọn) | 1 tuần | fb_conversations → cio_events |
| 6 | Mở rộng (tùy chọn) | 2–3 tuần | ROUTING_MODE, Dashboard, session states, actor |

**Tổng:** 6–8 tuần (Phase 1–4 bắt buộc); Phase 5–6 tùy nhu cầu.

---

## 1.1 Kết Nối Với Các Module Hiện Có

```
                    ┌─────────────────────────────────────────────────────────┐
                    │                      CIO Module                          │
                    └─────────────────────────────────────────────────────────┘
                                         │
         ┌───────────────────────────────┼───────────────────────────────┐
         │                               │                               │
         ▼                               ▼                               ▼
┌─────────────────┐           ┌─────────────────────┐           ┌─────────────────────┐
│   CRM (crm/)    │           │  Rule Intelligence   │           │  Decision (decision/)│
│                 │           │  (ruleintel/)        │           │                     │
└────────┬────────┘           └──────────┬──────────┘           └──────────▲──────────┘
         │                               │                               │
         │ Gọi PlanTouchpoint            │ Run(domain: cio)              │ BuildDecisionCase
         │ Đọc crm_customers             │                               │ FromCIOChoice
         ▼                               ▼                               │
┌─────────────────┐           ┌─────────────────────┐                     │
│ cio_touchpoint_ │           │ RULE_CIO_CHANNEL_   │                     │
│ plans           │           │ CHOICE, FREQUENCY   │                     │
└─────────────────┘           └─────────────────────┘                     │
                                                                         │
         ┌───────────────────────────────┼───────────────────────────────┘
         │                               │
         ▼                               ▼
┌─────────────────┐           ┌─────────────────────┐
│  FB (fb/)       │           │ Notification +      │
│                 │           │ Delivery             │
└─────────────────┘           └─────────────────────┘
```

| Module | Hướng | Cách kết nối | Chi tiết |
|--------|-------|--------------|----------|
| **CRM** | CRM → CIO | Service call | `CioTouchpointService.PlanTouchpoint(unifiedId, goalCode)` khi có recommendation |
| **CRM** | CIO → CRM | Query read | `crm_customers` — projection valueTier, lifecycleStage, lastOrderAt |
| **Rule Intelligence** | CIO → Rule | Service call | `RuleEngineService.Run(ctx, RunInput{RuleID: "RULE_CIO_CHANNEL_CHOICE", Domain: "cio", ...})` |
| **Decision** | CIO → Decision | Service call | `BuildDecisionCaseFromCIOChoice(ctx, touchpointPlan, outcome)` khi touchpoint closed |
| **Notification** | CIO → Notification | Package call | `notifytrigger.TriggerProgrammatic(ctx, eventType, payload, orgID, baseURL)` |
| **Delivery** | CIO → Delivery | Queue | `delivery.Queue.Enqueue(item)` — qua notifytrigger hoặc trực tiếp |
| **FB** | FB → CIO | Hook | `FbConversationService.SyncUpsertOne` → `CioIngestionService.OnConversationUpsert` |
| **FB** | CIO → FB | Gián tiếp | Gửi Messenger qua notifytrigger (eventType → routing → channel) |

**Collections dùng chung:**
- `crm_customers` — CIO đọc (không ghi)
- `fb_conversations`, `fb_messages` — CIO đọc qua ingestion
- `delivery_queue`, `delivery_history` — CIO ghi qua notifytrigger
- `decision_cases` — CIO ghi qua BuildDecisionCaseFromCIOChoice
- `rule_definitions`, `rule_logic_definitions`, `rule_param_sets` — CIO đọc qua Rule Engine

---

## 1.2 Đối Chiếu Với Tài Liệu Phần 3 (docs-shared)

| Tính năng (Phần 3) | Trạng thái | Phase / Ghi chú |
|-------------------|------------|-----------------|
| **Lớp 1: Omnichannel Ingestion** | | |
| — Zalo, Messenger, SMS | ✅ Có | Phase 1, 5 |
| — Web chat, WhatsApp, Email, Call | ⏳ Chưa | Phase 6+ (mở rộng) |
| — Interaction Event chuẩn hóa | ✅ Có | cio_events |
| **Lớp 2: Context & State** | | |
| — Session state (new, engaged, qualification, …) | ⚠️ Một phần | cio_sessions có state; chưa đủ 9 states chi tiết |
| — Context Memory (lịch sử chat, câu hỏi mở) | ⏳ Chưa | Phase 6+ (cần storage) |
| **Lớp 3: Dynamic Routing** | | |
| — [Value: VIP] → Senior Sales | ⏳ Chưa | RULE_CIO_ROUTING_MODE có design, chưa seed |
| — [Journey: VISITOR] → AI Greeting | ⏳ Chưa | Tương tự |
| — [Intent: Complaint] → Human Only | ⏳ Chưa | Cần AI/NLP detect intent |
| — Capacity-based (cân bằng tải) | ⏳ Chưa | Phase 6+ |
| **Lớp 4: Frequency & Channel** | | |
| — Cooldown periods | ✅ Có | RULE_CIO_FREQUENCY_CHECK |
| — max_messages_per_24h | ⏳ Chưa | Thêm vào FREQUENCY_CHECK |
| — Channel Selection (preferred_channel) | ⚠️ Một phần | Dùng valueTier; preferred_channel cần CI |
| **Lớp 5: Feedback Loop** | | |
| — Gửi raw đến CI | ⏳ Chưa | CIO chuyển raw; CI extract (đã phân tách) |
| — Decision Brain (cio_choice) | ✅ Có | Phase 3 |
| **§5 AI-Human Collaboration** | | |
| — AI Modes (Shadow, Assisted, Auto-reply, Autonomous) | ⏳ Chưa | Thuộc AI Agent layer, ngoài backend CIO |
| — Handoff (confidence < 90%, VIP, negative) | ⏳ Chưa | RULE_CIO_ROUTING_MODE có thể cover |
| **§6 Data Schema** | | |
| — Interaction Event | ✅ Có | cio_events |
| — Interaction Session | ✅ Có | cio_sessions |
| — Actor (AI_AGENT, HUMAN_SALES, …) | ⏳ Chưa | Thêm field actor vào cio_events |
| **§7 CIO Dashboard** | | |
| — Interaction Flow, Speed, Quality metrics | ⏳ Chưa | Cần API GET /cio/stats, /cio/dashboard |

---

## 2. Phase 1: Foundation (2–3 tuần)

**Mục tiêu:** Có thể gọi CIO PlanTouchpoint và Execute mà không cần Rule Engine.

| # | Công việc | File | Phụ thuộc |
|---|-----------|------|------------|
| 1 | Đăng ký 4 collections trong init.registry.go | init.registry.go | — |
| 2 | Model: CioEvent, CioSession, CioTouchpointPlan, CioRoutingDecision | models/model.cio.*.go | 1 |
| 3 | DTO: PlanTouchpointRequest, ExecuteTouchpointRequest | dto/dto.cio.*.go | 2 |
| 4 | Service: CioTouchpointService.PlanTouchpoint | service.cio.touchpoint.go | 2,3 |
| 5 | Service: CioTouchpointService.ExecuteTouchpoint (hardcode channel) | — | 4 |
| 6 | Handler: HandlePlanTouchpoint, HandleExecuteTouchpoint | handler.cio.touchpoint.go | 5 |
| 7 | Router: POST /cio/touchpoint/plan, POST /cio/touchpoint/:id/execute | router/routes.go | 6 |
| 8 | Đăng ký cio router trong main.go | main.go | 7 |
| 9 | Permission: CIO.Read, CIO.Write (init) | initsvc | — |
| 10 | Query crm_customers (valueTier, lifecycleStage) trong PlanTouchpoint | service.cio.touchpoint.go | 4 |

**Checkpoint Phase 1:** API PlanTouchpoint + Execute chạy được, channel hardcode (vd: luôn zalo).

---

## 3. Phase 2: Rule Intelligence (2 tuần)

**Mục tiêu:** Channel choice và frequency qua Rule Engine.

| # | Công việc | File | Phụ thuộc |
|---|-----------|------|------------|
| 1 | Output Contract: OUT_CIO_CHANNEL_CHOICE | ruleintel migration/seed | Phase 1 |
| 2 | Logic Script: LOGIC_CIO_CHANNEL_CHOICE | seed_rule_cio_system.go | 1 |
| 3 | Param Set: PARAM_CIO_CHANNEL_DEFAULT (cooldownHours) | — | 2 |
| 4 | Rule Definition: RULE_CIO_CHANNEL_CHOICE | — | 3 |
| 5 | Logic Script: LOGIC_CIO_FREQUENCY_CHECK | — | 4 |
| 6 | Rule Definition: RULE_CIO_FREQUENCY_CHECK | — | 5 |
| 7 | Service: CioRoutingService — gọi Rule Engine | service.cio.routing.go | Phase 1.4 |
| 8 | PlanTouchpoint: thay hardcode bằng CioRoutingService.ChooseChannel | service.cio.touchpoint.go | 7 |
| 9 | Ghi cio_routing_decisions sau mỗi quyết định | — | 8 |
| 10 | Bind params.nowMs khi gọi Rule Engine | — | 7 |
| 11 | (Optional) RULE_CIO_ROUTING_MODE — AI vs Human (VIP→Human, Visitor→AI) | seed_rule_cio_system.go | — |
| 12 | Param max_messages_per_24h trong FREQUENCY_CHECK | Param Set | — |

**Checkpoint Phase 2:** Channel chọn theo valueTier/lifecycleStage, cooldown hoạt động.

---

## 4. Phase 3: Tích Hợp CRM & Decision Brain (1–2 tuần)

**Mục tiêu:** CRM gọi CIO khi có recommendation; Decision Brain nhận cio_choice.

| # | Công việc | File | Phụ thuộc |
|---|-----------|------|------------|
| 1 | CRM: khi trigger_follow_up → gọi CioTouchpointService.PlanTouchpoint | service.crm.* | Phase 2 |
| 2 | Inject CioTouchpointService vào CRM (hoặc package-level) | — | 1 |
| 3 | Decision: BuildDecisionCaseFromCIOChoice | service.decision.builder.go | — |
| 4 | Schema decision case: caseType=cio_choice, experimentId, variant | model.decision.case.go | 3 |
| 5 | CIO: khi touchpoint executed → gọi BuildDecisionCaseFromCIOChoice (async) | service.cio.feedback.go | 3,4 |
| 6 | EventType notifytrigger cho CIO (cio_re_engage_zalo, cio_re_engage_sms) | notification init | — |

**Checkpoint Phase 3:** CRM recommendation → CIO → Delivery; Decision case được tạo.

---

## 5. Phase 4: A/B Test (1 tuần)

**Mục tiêu:** Có thể chạy experiment Zalo vs SMS.

| # | Công việc | File | Phụ thuộc |
|---|-----------|------|------------|
| 1 | Model: thêm experimentId, variant, variantConfig vào CioTouchpointPlan | model.cio.touchpoint_plan.go | Phase 3 |
| 2 | Model: thêm experimentId, variant vào CioRoutingDecision | model.cio.routing_decision.go | Phase 3 |
| 3 | DTO: PlanTouchpointRequest thêm ExperimentId, Variants | dto.cio.touchpoint.go | 1 |
| 4 | Service: AssignVariant(unifiedId, experimentId, variants) — hash hoặc random | service.cio.touchpoint.go | 3 |
| 5 | PlanTouchpoint: khi có experimentId → assign variant, ghi variantConfig | — | 4 |
| 6 | BuildDecisionCaseFromCIOChoice: thêm experimentId, variant | service.decision.builder.go | Phase 3.3 |
| 7 | (Optional) Collection cio_experiments — config experiment | — | — |

**Checkpoint Phase 4:** Có thể gửi với experimentId, variant được ghi, Decision Brain có experimentId.

---

## 6. Phase 5: Ingestion (tùy chọn, 1 tuần)

**Mục tiêu:** fb_conversations sync → cio_events.

| # | Công việc | File | Phụ thuộc |
|---|-----------|------|------------|
| 1 | Service: CioIngestionService.OnConversationUpsert | service.cio.ingestion.go | Phase 1 |
| 2 | Hook: FbConversationService.SyncUpsertOne → gọi OnConversationUpsert | service.fb.conversation.go | 1 |
| 3 | Webhook handler: POST /cio/webhook/:channel (skeleton) | handler.cio.webhook.go | — |

**Ghi chú:** Phase 5 có thể làm sau khi Phase 1–4 ổn định.

---

## 7. Phase 6: Mở Rộng (tùy chọn, 2–3 tuần)

**Mục tiêu:** Bổ sung tính năng theo tài liệu Phần 3 còn thiếu.

| # | Công việc | File | Ghi chú |
|---|-----------|------|---------|
| 1 | RULE_CIO_ROUTING_MODE — AI vs Human (VIP→Human, Visitor→AI) | seed_rule_cio_system.go | Đã có trong design |
| 2 | max_messages_per_24h trong RULE_CIO_FREQUENCY_CHECK | Logic Script | Bổ sung Param Set |
| 3 | Field actor (AI_AGENT, HUMAN_SALES, CUSTOMER, SYSTEM) vào cio_events | model.cio.event.go | Schema §6 |
| 4 | Session states mở rộng: qualification, product_discussion, objection_handling, closing, follow_up | model.cio.session.go | §3.2 |
| 5 | API GET /cio/stats — Interaction Flow, Speed, Quality metrics | handler.cio.stats.go | §7 Dashboard |
| 6 | preferred_channel từ crm_customers (CI cần thêm field) | — | §4.1 |

**Ghi chú:** Phase 6 có thể tách từng mục theo ưu tiên.

---

## 8. Timeline Tổng Hợp

| Tuần | Phase | Công việc chính |
|------|-------|-----------------|
| 1–2 | 1 | Collections, models, PlanTouchpoint, Execute (hardcode) |
| 3 | 2 | Rule Engine, RULE_CIO_CHANNEL_CHOICE, FREQUENCY_CHECK |
| 4 | 3 | CRM integration, BuildDecisionCaseFromCIOChoice |
| 5 | 4 | A/B test (experimentId, variant) |
| 6 | 5 (optional) | Ingestion fb_conversations → cio_events |
| 7–8 | Buffer | Test, fix, docs |

---

## 9. Thứ Tự Thực Thi

```
Phase 1.1 → 1.2 → 1.3 → 1.4,5,6 → 1.7,8 → 1.9,10
  ↓
Phase 2.1–6 (seed rules) → 2.7,8,9,10
  ↓
Phase 3.1,2 → 3.3,4,5 → 3.6
  ↓
Phase 4.1–6
  ↓
Phase 5 (nếu cần)
```

---

## 10. Rủi Ro & Giảm Thiểu

| Rủi ro | Mức độ | Giảm thiểu |
|--------|--------|-------------|
| Rule Engine chưa hỗ trợ domain cio | Trung bình | Kiểm tra ruleintel trước Phase 2; domain mở, chỉ cần seed |
| notifytrigger chưa có eventType CIO | Thấp | Đăng ký eventType mới trong init |
| CRM chưa có recommendation (trigger_follow_up) | Trung bình | Phase 3 phụ thuộc CRM Phase 2; có thể mock |
| Zalo API chưa có | Trung bình | Phase 1–4 dùng SMS/Telegram trước; Zalo sau |

---

## 11. Tiêu Chí Hoàn Thành

- [ ] API PlanTouchpoint, Execute hoạt động
- [ ] Channel choice qua Rule Engine
- [ ] Frequency check qua Rule Engine
- [ ] CRM gọi CIO khi có recommendation
- [ ] Decision case cio_choice được tạo
- [ ] A/B test: experimentId, variant được ghi và attribution

---

## 12. Changelog

- 2025-03-16: Tạo phương án triển khai CIO.
- 2025-03-16: Chốt CIO độc lập — bỏ Nhánh A (CRM Only), chỉ triển khai module CIO.
- 2025-03-16: Thêm §1.1 Kết nối module, §1.2 Đối chiếu Phần 3, Phase 6 Mở rộng, ROUTING_MODE, max_messages_per_24h.
