# Thiết Kế Module CIO — Customer Interaction Orchestrator

**Ngày:** 2025-03-16  
**Tham chiếu:** [ai-commerce-os-platform-l1.md](../02-architecture/vision/ai-commerce-os-platform-l1.md), Phần 3 CIO (docs-shared), [rule-intelligence.md](../02-architecture/core/rule-intelligence.md), [learning-engine.md](../02-architecture/core/learning-engine.md), [backend-module-map.md](../module-map/backend-module-map.md), **[PHUONG_AN_TRIEN_KHAI_CIO.md](./PHUONG_AN_TRIEN_KHAI_CIO.md)** (phương án triển khai), **[CIO_BOUNDARY_AND_EVENTS.md](./CIO_BOUNDARY_AND_EVENTS.md)** (boundary với order, unified event model)

---

## 0. Định Vị — CIO Là Module Độc Lập

**CIO là module độc lập**, không gộp logic điều phối vào CRM. Mọi touchpoint (re-engage, welcome, winback, …) đều qua CIO.

**Giá trị cốt lõi:**
- **A/B test** kịch bản tương tác (kênh, timing, nội dung)
- **Trace** đầy đủ cho Decision Brain
- **Đa nguồn trigger** (CRM, Ads, Content, Campaign)
- **Mở rộng** inbound routing, session, đa kênh

---

## 1. Tổng Quan

**CIO = Customer Interaction Orchestrator** — Hub điều phối mọi điểm chạm với khách hàng. CIO **không phải** nơi hiểu khách (Customer Intelligence) mà là **orchestrator** — quyết định kênh, routing, tần suất, context.

### 1.1 Vai Trò Trong Growth Flywheel

```
Content OS → Ads Intelligence → Conversation (CIO) → Customer Intelligence → Orders → Decision Brain → Learning → Content OS
```

- **Input:** Intent/touchpoint từ Customer Intelligence, Ads, Content, Campaign
- **Output:** Conversation feed vào Customer Intelligence; Decision Case (cio_choice) vào Decision Brain

### 1.2 Nguyên Tắc Thiết Kế

| Nguyên tắc | Mô tả |
|------------|-------|
| **Hub, không brain** | CIO điều phối — không phân tích profile, không tính LTV. Customer Intelligence cung cấp context. |
| **Không sở hữu domain** | CIO không sở hữu order, customer, ads — chỉ ghi nhận event. Order = 1 event trong timeline. Chi tiết: [CIO_BOUNDARY_AND_EVENTS.md](./CIO_BOUNDARY_AND_EVENTS.md) |
| **Rule-based + AI** | Routing, channel choice có thể dùng Rule Engine (domain `cio`) hoặc AI model. |
| **Contract-based** | Giao tiếp qua schema — input từ Customer Intelligence, output cho Delivery/Notification. |
| **Tích hợp sẵn có** | Tái sử dụng fb_conversations, notification, delivery, crm. |

---

## 2. Kiến Trúc 5 Lớp

Theo vision: **Omnichannel Ingestion → Context & State → Dynamic Routing → Frequency & Channel Control → Feedback Loop**

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│ Lớp 1: Omnichannel Ingestion                                                     │
│ Webhook, sync từ Zalo, Messenger, Website chat, Telegram, Call → cio_events      │
└─────────────────────────────────────────────────────────────────────────────────┘
                                        ↓
┌─────────────────────────────────────────────────────────────────────────────────┐
│ Lớp 2: Context & State                                                           │
│ Gắn customerId, unifiedId, session state, conversation context                   │
└─────────────────────────────────────────────────────────────────────────────────┘
                                        ↓
┌─────────────────────────────────────────────────────────────────────────────────┐
│ Lớp 3: Dynamic Routing                                                           │
│ AI vs Human, kênh ưu tiên, rule-based hoặc model-based                            │
└─────────────────────────────────────────────────────────────────────────────────┘
                                        ↓
┌─────────────────────────────────────────────────────────────────────────────────┐
│ Lớp 4: Frequency & Channel Control                                               │
│ Giới hạn tần suất, cooldown, channel capacity, không spam                         │
└─────────────────────────────────────────────────────────────────────────────────┘
                                        ↓
┌─────────────────────────────────────────────────────────────────────────────────┐
│ Lớp 5: Feedback Loop                                                             │
│ Ghi trace, attribution, feed vào Customer Intelligence, Decision Brain           │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Kênh Hỗ Trợ

| Kênh | Trạng thái | Nguồn dữ liệu | Ghi chú |
|------|------------|---------------|---------|
| **Messenger** | ✅ Có | fb_conversations, fb_messages | Pancake sync |
| **Zalo** | ⏳ Vision | — | Cần tích hợp API Zalo |
| **Telegram** | ⚠️ Một chiều | notification channels (chatIds) | Chỉ gửi, chưa nhận |
| **Website chat** | ⏳ Vision | — | Widget, webhook |
| **SMS** | ⚠️ Qua delivery | notification channels | Gửi qua delivery queue |
| **Call** | ⏳ Vision | — | Tích hợp tương lai |

---

## 4. Models & Collections

### 4.1 Collections Mới

| Collection | Mục đích |
|------------|----------|
| `cio_events` | Event ingestion — message in/out, session start/end, từ mọi kênh |
| `cio_sessions` | Session state — customerId, channel, routing decision, touchpoint plan |
| `cio_touchpoint_plans` | Kế hoạch touchpoint — khi nào, kênh nào, nội dung gì (từ Customer Intelligence) |
| `cio_routing_decisions` | Lịch sử quyết định routing — AI vs Human, channel chosen, rule_id |

**Tái sử dụng:**
- `fb_conversations`, `fb_messages` — Messenger
- `crm_customers` — unifiedId, classification
- `notification_channels`, `notification_routing_rules` — delivery
- `delivery_queue`, `delivery_history` — gửi message
- `decision_cases` — cio_choice

### 4.2 Schema cio_events

Hai loại event: **(A) Interaction** (message, conversation, click, view) và **(B) External business** (order_created, payment_success, …). Tất cả vào event stream thống nhất. CIO chỉ log — không xử lý logic order. Chi tiết: [CIO_BOUNDARY_AND_EVENTS.md](./CIO_BOUNDARY_AND_EVENTS.md)

```javascript
{
  _id: ObjectId,
  eventType: String,        // "message_in" | "message_out" | "order_created" | "payment_success" | ...
  channel: String,          // "messenger" | "zalo" | "pos" | "web" | ...
  ownerOrganizationId: ObjectId,
  customerId: String,       // ID từ kênh (fb customerId, zalo userId, ...)
  unifiedId: String,        // crm_customers.unifiedId (sau khi resolve)
  conversationId: String,  // fb_conversations.conversationId hoặc tương đương
  sessionId: String,       // cio_sessions._id hoặc ref
  payload: Object,          // Raw payload từ webhook/sync — snapshot nhẹ, không full domain object
  sourceRef: {
    refType: String,       // "fb_conversation" | "zalo_message" | "pos_order" | "webhook"
    refId: String
  },
  eventAt: Number,          // Thời gian sự kiện thực tế (quan trọng khi sync trễ)
  createdAt: Number
}
```

### 4.3 Schema cio_sessions

```javascript
{
  _id: ObjectId,
  ownerOrganizationId: ObjectId,
  unifiedId: String,
  channel: String,
  routingMode: String,       // "ai" | "human" | "hybrid"
  state: String,            // "active" | "waiting" | "closed"
  context: {
    lastIntent: String,
    lastTouchpointAt: Number,
    touchpointCount: Number
  },
  createdAt: Number,
  updatedAt: Number
}
```

### 4.4 Schema cio_touchpoint_plans

```javascript
{
  _id: ObjectId,
  ownerOrganizationId: ObjectId,
  unifiedId: String,
  goalCode: String,         // "re_engage" | "welcome" | "abandoned_cart" | "vip_reactivation"
  suggestedChannels: [String],  // ["zalo", "sms"] — ưu tiên
  suggestedContentRef: String,  // templateId hoặc contentId
  sourceRef: {
    refType: String,       // "crm_recommendation" | "rule_engine" | "ai_workflow"
    refId: String
  },
  // A/B Test — khi có experiment
  experimentId: String,    // "exp_re_engage_zalo_vs_sms_202503"
  variant: String,         // "A" | "B" | "C" — variant được assign
  variantConfig: Object,   // { channel: "zalo", templateId: "xxx" } — config của variant
  status: String,          // "pending" | "scheduled" | "sent" | "skipped" | "expired"
  scheduledAt: Number,
  executedAt: Number,
  routingDecisionId: ObjectId,  // → cio_routing_decisions
  createdAt: Number,
  updatedAt: Number
}
```

### 4.5 Schema cio_routing_decisions

```javascript
{
  _id: ObjectId,
  ownerOrganizationId: ObjectId,
  touchpointPlanId: ObjectId,
  unifiedId: String,
  channelChosen: String,   // "zalo" | "sms" | "messenger" | ...
  ruleId: String,         // RULE_CIO_CHANNEL_CHOICE (nếu từ Rule Engine)
  logicVersion: Number,
  paramVersion: Number,
  inputSnapshot: Object,
  outputSnapshot: Object,
  traceId: String,        // rule_execution_logs
  experimentId: String,   // A/B test — để attribution
  variant: String,        // "A" | "B" | "C"
  createdAt: Number
}
```

---

## 5. Rule Intelligence — Domain CIO

### 5.1 Rule Types Cho CIO

| Rule | from_layer | to_layer | Mô tả |
|------|------------|----------|-------|
| **RULE_CIO_CHANNEL_CHOICE** | cio_context | channel_choice | Chọn kênh (Zalo vs SMS) dựa trên context, preference, cost |
| **RULE_CIO_ROUTING_MODE** | cio_context | routing_mode | AI vs Human — dựa trên complexity, VIP, escalation |
| **RULE_CIO_FREQUENCY_CHECK** | cio_context | frequency_ok | Kiểm tra cooldown, không spam |

### 5.2 Layers CIO

| Layer | Nội dung |
|-------|----------|
| `cio_context` | unifiedId, valueTier, lifecycleStage, lastTouchpointAt, channelPreference, goalCode |
| `channel_choice` | channelChosen, reason, confidence |
| `routing_mode` | routingMode (ai | human) |
| `frequency_ok` | allowed (boolean), reason |

### 5.3 Output Contract — CIO Choice

```json
{
  "output_id": "OUT_CIO_CHANNEL_CHOICE",
  "output_version": 1,
  "domain": "cio",
  "output_type": "channel_choice",
  "schema_definition": {
    "type": "object",
    "properties": {
      "channelChosen": { "type": "string", "enum": ["zalo", "sms", "messenger", "telegram"] },
      "reason": { "type": "string" },
      "confidence": { "type": "number", "minimum": 0, "maximum": 1 }
    },
    "required": ["channelChosen", "reason"]
  }
}
```

### 5.4 Logic Script — LOGIC_CIO_CHANNEL_CHOICE (ví dụ)

**Lưu ý:** CIO đọc context (valueTier, lifecycleStage) từ crm_customers — **không tính**. CI đã tính sẵn.

```javascript
function evaluate(ctx) {
  var input = ctx.layers.cio_context || {};  // Đã có từ crm_customers
  var params = ctx.params || {};
  var report = { input: input, params: params, log: '' };
  
  // Cooldown: không gửi nếu vừa touch < 24h
  var now = params.nowMs || 0;  // Caller truyền vào (service gọi Rule Engine với params.nowMs = time.Now().UnixMilli())
  var lastTouch = input.lastTouchpointAt || 0;
  var cooldownMs = (params.cooldownHours || 24) * 3600 * 1000;
  if (now - lastTouch < cooldownMs) {
    report.result = 'filtered';
    report.log = 'Cooldown: vừa touch ' + Math.round((now - lastTouch) / 3600000) + 'h trước';
    return { output: null, report: report };
  }
  
  // Rule: VIP/High → Zalo (đọc valueTier, không tính)
  if (input.valueTier === 'top' || input.valueTier === 'high') {
    report.result = 'match';
    report.log = 'VIP/High → Zalo (valueTier=' + input.valueTier + ')';
    return { output: { channelChosen: 'zalo', reason: 'VIP ưu tiên Zalo', confidence: 0.9 }, report: report };
  }
  
  // Rule: Cooling/Inactive → Zalo (re-engage)
  if (input.lifecycleStage === 'cooling' || input.lifecycleStage === 'inactive') {
    report.result = 'match';
    report.log = 'Cooling/Inactive → Zalo (lifecycleStage=' + input.lifecycleStage + ')';
    return { output: { channelChosen: 'zalo', reason: 'Re-engage qua Zalo', confidence: 0.8 }, report: report };
  }
  
  // Default: SMS
  report.result = 'match';
  report.log = 'Default → SMS (chi phí thấp)';
  return { output: { channelChosen: 'sms', reason: 'SMS phù hợp', confidence: 0.7 }, report: report };
}
```

---

## 6. API & Endpoints

### 6.1 Router: `cio/router/routes.go`

| Method | Path | Handler | Mô tả |
|--------|------|---------|-------|
| POST | /cio/webhook/:channel | HandleWebhook | Ingestion — nhận event từ Zalo, Messenger, Website |
| POST | /cio/touchpoint/plan | HandlePlanTouchpoint | Tạo touchpoint plan (từ CRM, rule, AI) |
| POST | /cio/touchpoint/:id/execute | HandleExecuteTouchpoint | Thực thi — routing → delivery |
| GET | /cio/sessions | HandleListSessions | Danh sách session (filter unifiedId, channel) |
| GET | /cio/routing-decisions | HandleListRoutingDecisions | Lịch sử quyết định routing |

### 6.2 Permission

- `CIO.Read` — xem sessions, routing decisions
- `CIO.Write` — plan touchpoint, execute
- `CIO.Webhook` — webhook (có thể dùng API key riêng)

---

## 7. Tích Hợp Với Module Hiện Có

### 7.1 Customer Intelligence (CRM)

- **Input:** CRM gọi CIO khi có recommendation (repeat_gap_risk, vip_at_risk) → tạo `cio_touchpoint_plans`
- **Context:** CIO lấy classification (valueTier, lifecycleStage) từ `crm_customers` để routing

### 7.2 Notification & Delivery

- **Output:** CIO chọn channel → gọi `notifytrigger` hoặc `delivery.Queue` với channelType, content, recipient
- **Tái sử dụng:** notification_routing_rules (có thể mở rộng cho cio), delivery_queue

### 7.3 Decision Brain

- **CIO Choice:** Khi touchpoint hoàn thành (gửi xong, có outcome) → `BuildDecisionCaseFromCIOChoice` tạo decision case `caseType: "cio_choice"`
- **Schema:** goalCode, targetType=customer, targetId=unifiedId, result, text.aiText (situation, decisionRationale, actualOutcome)

### 7.4 FB (Messenger)

- **Ingestion:** fb_conversations sync → map sang cio_events (eventType: message_in)
- **Outbound:** Gửi qua Messenger API (Pancake hoặc Meta) — cần adapter

### 7.5 Order (POS, Shopify)

- **Inject:** Order system gửi `order_created`, `order_updated`, `order_cancelled`, `payment_success` vào cio_events
- **CIO chỉ:** Log event, gắn customerUid — **không** tính LTV, phân loại, đánh giá đơn
- **Lý do:** Gắn conversation với outcome; feed CIX context; feed Customer Intelligence
- Chi tiết: [CIO_BOUNDARY_AND_EVENTS.md](./CIO_BOUNDARY_AND_EVENTS.md)

---

## 8. Luồng End-to-End (Ví Dụ Re-engagement)

```
1. CRM Rule: repeat_gap_risk → recommendation { flowId: "re_engage", unifiedId }
2. CRM Service: Gọi CIO PlanTouchpoint(unifiedId, goalCode: "re_engage", suggestedChannels: ["zalo","sms"])
3. CIO Service:
   a. Lấy context từ crm_customers (valueTier, lastTouchpointAt)
   b. Gọi Rule Engine RULE_CIO_CHANNEL_CHOICE → channelChosen: "zalo"
   c. Gọi RULE_CIO_FREQUENCY_CHECK → allowed: true
   d. Tạo cio_routing_decisions, cập nhật cio_touchpoint_plans
   e. Gọi Delivery/NotifyTrigger (channel: zalo, templateId, recipient)
4. Delivery: Gửi Zalo
5. Feedback: cio_events (message_out), cập nhật lastTouchpointAt
6. (Sau lifecycle) Decision Brain: BuildDecisionCaseFromCIOChoice
```

### 8.1 Luồng A/B Test (khi có experiment)

```
1. Tạo experiment: exp_re_engage_zalo_vs_sms { variants: [ { id: "A", channel: "zalo" }, { id: "B", channel: "sms" } ] }
2. CRM recommendation → CIO PlanTouchpoint(unifiedId, goalCode, experimentId)
3. CIO: Assign variant (random 50/50 hoặc hash unifiedId % 2)
4. CIO: Ghi cio_touchpoint_plans { experimentId, variant: "A", variantConfig: { channel: "zalo" } }
5. CIO: Gửi qua channel của variant
6. Outcome (mở, click, mua) → Decision Brain: BuildDecisionCaseFromCIOChoice { experimentId, variant: "A", result }
7. Phân tích: Zalo (A) vs SMS (B) — conversion rate, cost per conversion
```

---

## 9. Cấu Trúc Code Đề Xuất

```
api/internal/api/cio/
├── handler/
│   ├── handler.cio.webhook.go      # HandleWebhook
│   ├── handler.cio.touchpoint.go   # Plan, Execute
│   ├── handler.cio.session.go      # ListSessions
│   └── handler.cio.routing.go      # ListRoutingDecisions
├── service/
│   ├── service.cio.ingestion.go    # Lớp 1: Ingest event
│   ├── service.cio.context.go      # Lớp 2: Resolve context
│   ├── service.cio.routing.go      # Lớp 3: Dynamic routing (gọi Rule Engine)
│   ├── service.cio.frequency.go    # Lớp 4: Frequency check
│   ├── service.cio.touchpoint.go   # Plan, Execute
│   └── service.cio.feedback.go     # Lớp 5: Feedback, trace
├── models/
│   ├── model.cio.event.go
│   ├── model.cio.session.go
│   ├── model.cio.touchpoint_plan.go
│   └── model.cio.routing_decision.go
├── dto/
│   ├── dto.cio.webhook.go
│   ├── dto.cio.touchpoint.go
│   └── dto.cio.routing.go
└── router/
    └── routes.go
```

---

## 10. Phase Triển Khai Đề Xuất

### Phase 1: CIO Foundation (2–3 tuần)

| Bước | Hành động |
|------|-----------|
| 1 | Tạo collections: cio_events, cio_sessions, cio_touchpoint_plans, cio_routing_decisions |
| 2 | Models, DTO, router skeleton |
| 3 | Webhook ingestion cho Messenger (map fb_conversations → cio_events) |
| 4 | Service PlanTouchpoint, ExecuteTouchpoint (chưa Rule Engine, hardcode channel) |

### Phase 2: Rule Intelligence (2 tuần)

| Bước | Hành động |
|------|-----------|
| 1 | Seed RULE_CIO_CHANNEL_CHOICE, LOGIC_CIO_CHANNEL_CHOICE, PARAM_CIO_CHANNEL_DEFAULT |
| 2 | Service routing gọi Rule Engine |
| 3 | RULE_CIO_FREQUENCY_CHECK |
| 4 | Trace, cio_routing_decisions |

### Phase 3: Tích Hợp CRM & Decision Brain (1–2 tuần)

| Bước | Hành động |
|------|-----------|
| 1 | CRM recommendation → PlanTouchpoint |
| 2 | BuildDecisionCaseFromCIOChoice |
| 3 | Zalo adapter (nếu có API) |

### Phase 4: Mở Rộng (tương lai)

- Website chat, Call
- AI model cho routing (thay Rule Engine khi cần)
- Dashboard CIO (sessions, routing stats)

---

## 11. Files Cần Tạo/Sửa

| File | Nội dung |
|------|----------|
| `api/internal/api/cio/models/model.cio.*.go` | 4 models |
| `api/internal/api/cio/dto/dto.cio.*.go` | DTOs |
| `api/internal/api/cio/service/service.cio.*.go` | 6 services |
| `api/internal/api/cio/handler/handler.cio.*.go` | 4 handlers |
| `api/internal/api/cio/router/routes.go` | Routes |
| `api/cmd/server/init.registry.go` | Đăng ký 4 collections |
| `api/cmd/server/main.go` | Register cio router |
| `ruleintel/migration/seed_rule_cio_system.go` | Seed rules CIO |
| `decision/service/service.decision.builder.go` | BuildDecisionCaseFromCIOChoice |

---

## 12. Phân Tách Logic CIO vs Customer Intelligence

### 12.0 Ranh Giới Rõ Ràng — Tránh Lẫn Lộn

**Câu hỏi duy nhất để phân biệt:**

| Câu hỏi | Trả lời = Customer Intelligence | Trả lời = CIO |
|---------|--------------------------------|---------------|
| **"Khách này là ai?"** | valueTier, lifecycleStage, journeyStage, LTV | — |
| **"Nên làm gì với khách?"** | recommendation: re_engage, winback, flowId | — |
| **"Khi nào, qua kênh nào, ai xử lý?"** | — | channel, timing, AI vs Human |
| **"Trạng thái cuộc hội thoại hiện tại?"** | — | session state, routing assignment |

**Nguyên tắc vàng:** Customer Intelligence **hiểu**. CIO **điều phối**. CIO không suy luận, không aggregate, không tạo recommendation.

---

### 12.1 Các Điểm Dễ Lẫn (từ tài liệu CIO docs-shared)

| Nội dung trong CIO doc | Dễ lẫn vì | Phân công đúng |
|------------------------|-----------|----------------|
| **"engagement signals, intent signals, objection signals"** (Lớp 5) | CIO "gửi" signals → CI? Ai extract? | **CI** (hoặc AI/NLP service) extract từ raw. **CIO** chỉ chuyển raw data (messages, timestamps). CIO không phát hiện intent. |
| **"Context Memory: lịch sử chat, câu hỏi đang mở"** (Lớp 2) | Giống "hiểu" khách? | **CIO** lưu để truyền cho AI/Sales (continuity). **CI** không lưu lịch sử chat — CI chỉ nhận aggregate (conversationCount, lastConversationAt). |
| **"Intent: Complaint/Negative Sentiment"** (Lớp 3) | AI detect intent → CIO? | **AI Agent** (hoặc NLP) detect, trả signal. **CIO** nhận signal và routing. CIO không chạy model. |
| **"preferred_channel của Profile"** (Lớp 4) | Ai tính preferred_channel? | **CI** tính (từ behavior). **CIO** đọc và dùng. |
| **Session state: qualification, objection_handling** | Ai cập nhật? | **AI/Sales** (handler) cập nhật sau mỗi turn. **CIO** lưu và truyền. CIO không infer từ nội dung. |

---

### 12.2 Nguyên Tắc: Một Nơi, Một Trách Nhiệm

| Module | Trách nhiệm | Không làm |
|--------|-------------|------------|
| **Customer Intelligence (CRM)** | Hiểu khách — aggregate metrics, classification, recommendation | Không chọn kênh, không gửi message, không quản lý session |
| **CIO** | Điều phối — chọn kênh, routing, frequency, gửi | Không aggregate, không tính classification, không tạo recommendation |

### 12.3 Bảng Phân Công Logic

| Logic | Thuộc | Chi tiết |
|-------|-------|----------|
| Aggregate orders (totalSpent, orderCount, lastOrderAt, revenueLast30d...) | **CRM** | pc_pos_orders → currentMetrics |
| Aggregate conversations (conversationCount, lastConversationAt...) | **CRM** | fb_conversations → currentMetrics |
| Classification (valueTier, lifecycleStage, journeyStage, channel, loyaltyStage, momentumStage) | **CRM** | Rule Engine RULE_CRM_CLASSIFICATION |
| Signal/flag (repeat_gap_risk, vip_at_risk) | **CRM** | Rule Engine RULE_CRM_* (Phase 2) |
| Recommendation (flowId, goalCode) | **CRM** | Rule Engine trigger_follow_up → output recommendation |
| Chọn kênh (Zalo vs SMS vs Messenger) | **CIO** | Rule Engine RULE_CIO_CHANNEL_CHOICE |
| Kiểm tra frequency/cooldown | **CIO** | Rule Engine RULE_CIO_FREQUENCY_CHECK |
| Routing AI vs Human | **CIO** | Rule Engine RULE_CIO_ROUTING_MODE |
| Gửi message qua Delivery | **CIO** | notifytrigger / delivery.Queue |
| Ghi activity (conversation_started) | **CRM** | crm_activity_history |
| Ghi event (message_in, touchpoint_triggered) | **CIO** | cio_events |

### 12.4 Contract Giao Tiếp

**CRM → CIO (khi có recommendation):**

```json
{
  "unifiedId": "xxx",
  "ownerOrganizationId": "xxx",
  "goalCode": "re_engage",
  "suggestedChannels": ["zalo", "sms"],
  "sourceRef": { "refType": "crm_recommendation", "refId": "rule_trace_xxx" }
}
```

**CIO cần context từ CRM — lấy qua API, không duplicate logic:**

```json
{
  "unifiedId": "xxx",
  "valueTier": "top",
  "lifecycleStage": "cooling",
  "lastTouchpointAt": 1710000000000,
  "goalCode": "re_engage"
}
```

- **CIO đọc** `crm_customers` (projection: valueTier, lifecycleStage, lastOrderAt) + `cio_touchpoint_plans` (lastTouchpointAt)
- **CIO không gọi** GetClassificationFromCustomer — dùng field đã denormalize (valueTier, lifecycleStage) trong crm_customers
- **CIO không aggregate** — không query pc_pos_orders, fb_conversations

### 12.5 Luồng Phân Tách Rõ

```
[CRM] Rule repeat_gap_risk → recommendation { unifiedId, goalCode }
         │
         │  CRM gọi: cio.PlanTouchpoint(unifiedId, goalCode)
         ▼
[CIO]  1. Query crm_customers (chỉ valueTier, lifecycleStage) — 1 doc, index unifiedId
       2. Query cio_touchpoint_plans (lastTouchpointAt cho unifiedId) — optional
       3. Build cio_context → Rule Engine RULE_CIO_CHANNEL_CHOICE
       4. Rule Engine RULE_CIO_FREQUENCY_CHECK
       5. Ghi cio_routing_decisions, tạo cio_touchpoint_plan
       6. Gọi Delivery
         │
         │  Feedback: cập nhật lastTouchpointAt (trong cio_touchpoint_plans hoặc crm_customers)
         ▼
[CRM]  (Optional) Cập nhật crm_customers.lastTouchpointAt nếu lưu ở CRM
       (Hoặc CIO tự quản lastTouchpointAt trong cio_touchpoint_plans)
```

---

## 13. Hiệu Suất

### 13.1 Tránh Duplicate Aggregation

| Hành động sai | Hành động đúng |
|---------------|----------------|
| CIO gọi RecalculateCustomerFromAllSources | CIO chỉ đọc crm_customers (valueTier, lifecycleStage đã có) |
| CIO aggregate pc_pos_orders, fb_conversations | CRM đã aggregate → currentMetrics, classification |
| CIO gọi GetClassificationFromCustomer (Rule Engine) | Dùng valueTier, lifecycleStage denormalized trong crm_customers |

### 13.2 Query Tối Ưu

| Query | Index | Projection |
|-------|-------|------------|
| CIO lấy context cho unifiedId | crm_customers: (ownerOrganizationId, unifiedId) unique | valueTier, lifecycleStage, lastOrderAt |
| CIO lấy lastTouchpointAt | cio_touchpoint_plans: (ownerOrganizationId, unifiedId, status), sort executedAt desc | executedAt |
| CRM list recommendation | (đã có) | — |

### 13.3 Async & Fire-and-Forget

| Luồng | Đồng bộ | Bất đồng bộ |
|-------|---------|-------------|
| fb_conversations sync → cio_events | — | Hook fire-and-forget: `go cioIngestion.OnConversationUpsert(...)` |
| CRM recommendation → CIO PlanTouchpoint | API call (có thể queue job) | Nếu volume cao: queue job, worker xử lý |
| CIO ExecuteTouchpoint → Delivery | Gọi notifytrigger (nhanh) | Delivery worker xử lý queue |
| CIO → Decision Brain (cio_choice) | — | Khi touchpoint closed: async BuildDecisionCase |

### 13.4 Cache (Tùy Chọn)

| Dữ liệu | Cache | TTL | Ghi chú |
|---------|-------|-----|---------|
| valueTier, lifecycleStage | Có thể | 5–15 phút | Cập nhật khi Recalculate chạy |
| RULE_CIO_* (definition, logic, param) | Rule Engine đã cache | — | Không cần |
| lastTouchpointAt | Trong cio_touchpoint_plans | — | Query mỗi lần PlanTouchpoint (1 doc) |

### 13.5 Batch & Bulk

| Tình huống | Cách xử lý |
|------------|------------|
| CRM có 100 recommendation cùng lúc | Gọi CIO PlanTouchpoint từng cái hoặc batch API (CIO nhận batch unifiedIds) |
| CIO batch PlanTouchpoint | Query crm_customers với $in unifiedIds — 1 query thay vì N |
| Backfill cio_events từ fb_conversations | Worker batch, cursor, limit 500 |

---

## 14. Changelog

- 2026-03-18: Thêm CIO boundary với order — không sở hữu domain, chỉ log event. Tham chiếu CIO_BOUNDARY_AND_EVENTS.md.
- 2025-03-16: Tạo tài liệu thiết kế module CIO theo vision và codebase hiện có.
- 2025-03-16: Thêm §12 Phân tách logic CIO vs Customer Intelligence, §13 Hiệu suất.
- 2025-03-16: Thêm schema experimentId/variant, luồng A/B test (§8.1).
- 2025-03-16: Chốt CIO độc lập — bỏ Phase 0 (CRM Only), §0 định vị CIO là module độc lập.
