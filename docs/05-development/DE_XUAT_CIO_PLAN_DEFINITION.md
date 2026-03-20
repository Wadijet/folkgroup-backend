# Đề Xuất: CIO Plan Definition — Định Nghĩa Plan Chi Tiết, Tích Hợp Rule Engine

**Ngày:** 2025-03-16  
**Tham chiếu:** [THIET_KE_MODULE_CIO.md](./THIET_KE_MODULE_CIO.md), [rule-intelligence.md](../02-architecture/core/rule-intelligence.md)

---

## 1. Mục Tiêu

- **Plan định nghĩa chi tiết từng bước** — mỗi step có input, output, params rõ ràng
- **Input/Output/Params mở** — kết nối module khác (Rule Engine, Delivery, CRM, Decision Brain)
- **Tái sử dụng Rule Engine** — step có thể gọi rule thay vì hardcode logic
- **Versioning** — theo dõi version để A/B test plan v1 vs v2

---

## 2. Kiến Trúc Tổng Quan

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ cio_plan_definitions                                                        │
│ planId, version, goalCode, steps[]                                         │
│ Mỗi step: type, input_ref, output_ref, param_ref, rule_ref (nếu type=rule) │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ Plan Executor (CIO Service)                                                  │
│ For each step:                                                               │
│   - type=rule  → Rule Engine.Run(rule_id, context) → output → next step     │
│   - type=action → Gọi module (Delivery, notifytrigger) → next step           │
│   - type=condition → Rule/branch → chọn next_step_id                         │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Schema Plan Definition

### 3.1 Document cio_plan_definitions

```javascript
{
  _id: ObjectId,
  planId: String,           // "re_engage_single_zalo"
  version: Number,          // 1, 2, 3 — tăng khi sửa, A/B test theo version
  goalCode: String,         // "re_engage" | "welcome" | "abandoned_cart"
  ownerOrganizationId: ObjectId,
  
  // Target audience — đối tượng plan áp dụng (đầu vào ban đầu). Rỗng = mọi KH.
  targetAudience: {
    lifecycleStages: [String],  // ["cooling", "inactive"] — chỉ KH có stage trong list
    valueTiers: [String],      // ["top", "high"] — chỉ KH có tier trong list
    customFilter: Object       // Mở rộng
  },
  
  // Các bước — thứ tự thực thi (mỗi execution chạy cho 1 đối tượng cụ thể = unifiedId)
  steps: [
    {
      stepId: String,       // "step_1_frequency_check"
      order: Number,        // 1, 2, 3
      stepType: String,     // "rule" | "action" | "condition"
      
      // Input — mở, tham chiếu schema hoặc mapping
      inputRef: {
        schemaRef: String,        // "schema_cio_context" — đăng ký trong registry
        sourceMapping: Object,    // { "cio_context": "ctx.layers.cio_context", "unifiedId": "ctx.entity_ref.objectId" }
        requiredFields: [String]
      },
      
      // Output — mở, contract
      outputRef: {
        outputId: String,         // "OUT_CIO_FREQUENCY_OK" | "OUT_CIO_CHANNEL_CHOICE"
        outputVersion: Number,
        targetLayer: String       // Layer ghi output (để step sau đọc)
      },
      
      // Params — mở, override hoặc tham chiếu
      paramRef: {
        paramSetId: String,      // "PARAM_CIO_FREQUENCY_DEFAULT"
        paramVersion: Number,
        override: Object         // Optional override cho step này
      },
      
      // Chỉ khi stepType = "rule"
      ruleRef: {
        ruleId: String,          // "RULE_CIO_FREQUENCY_CHECK"
        domain: String           // "cio"
      },
      
      // Chỉ khi stepType = "action"
      actionRef: {
        moduleRef: String,       // "delivery" | "notifytrigger"
        actionType: String,      // "send_touchpoint"
        config: Object           // { recipientRef, channel, templateId, ... }
        // recipientRef: "unifiedId" (mặc định — từ execution context)
        // recipientFromLayer: "path" — lấy từ output step trước (hiếm dùng)
      },
      
      // Chỉ khi stepType = "condition" — branching
      conditionRef: {
        ruleId: String,
        branchMapping: Object    // { "allowed": "step_2_channel", "filtered": "end" }
      },
      
      nextStepId: String,    // stepId bước tiếp theo (null = end)
      metadata: Object
    }
  ],
  
  createdAt: Number,
  updatedAt: Number,
  changeReason: String
}
```

### 3.2 Ví Dụ Plan: re_engage_single_zalo v1

```json
{
  "planId": "re_engage_single_zalo",
  "version": 1,
  "goalCode": "re_engage",
  "steps": [
    {
      "stepId": "step_1_frequency",
      "order": 1,
      "stepType": "rule",
      "inputRef": {
        "schemaRef": "schema_cio_context",
        "sourceMapping": { "cio_context": "ctx.layers.cio_context" },
        "requiredFields": ["unifiedId", "lastTouchpointAt", "goalCode"]
      },
      "outputRef": {
        "outputId": "OUT_CIO_FREQUENCY_OK",
        "outputVersion": 1,
        "targetLayer": "frequency_ok"
      },
      "paramRef": {
        "paramSetId": "PARAM_CIO_FREQUENCY_DEFAULT",
        "paramVersion": 1
      },
      "ruleRef": {
        "ruleId": "RULE_CIO_FREQUENCY_CHECK",
        "domain": "cio"
      },
      "nextStepId": "step_2_channel"
    },
    {
      "stepId": "step_2_channel",
      "order": 2,
      "stepType": "rule",
      "inputRef": {
        "schemaRef": "schema_cio_context",
        "sourceMapping": { "cio_context": "ctx.layers.cio_context", "frequency_ok": "ctx.layers.frequency_ok" }
      },
      "outputRef": {
        "outputId": "OUT_CIO_CHANNEL_CHOICE",
        "outputVersion": 1,
        "targetLayer": "channel_choice"
      },
      "paramRef": {
        "paramSetId": "PARAM_CIO_CHANNEL_DEFAULT",
        "paramVersion": 1
      },
      "ruleRef": {
        "ruleId": "RULE_CIO_CHANNEL_CHOICE",
        "domain": "cio"
      },
      "nextStepId": "step_3_send"
    },
    {
      "stepId": "step_3_send",
      "order": 3,
      "stepType": "action",
      "inputRef": {
        "sourceMapping": { "channel_choice": "ctx.layers.channel_choice", "unifiedId": "ctx.entity_ref.objectId" }
      },
      "actionRef": {
        "moduleRef": "notifytrigger",
        "actionType": "send_touchpoint",
        "config": {
          "channelFromLayer": "channel_choice.channelChosen",
          "templateId": "re_engage_zalo_v1"
        }
      },
      "nextStepId": null
    }
  ]
}
```

### 3.3 Ví Dụ Plan: re_engage_sequence_sms v1 (2 bước)

```json
{
  "planId": "re_engage_sequence_sms",
  "version": 1,
  "goalCode": "re_engage",
  "steps": [
    {
      "stepId": "step_1_frequency",
      "order": 1,
      "stepType": "rule",
      "ruleRef": { "ruleId": "RULE_CIO_FREQUENCY_CHECK", "domain": "cio" },
      "nextStepId": "step_2_send_first"
    },
    {
      "stepId": "step_2_send_first",
      "order": 2,
      "stepType": "action",
      "actionRef": {
        "moduleRef": "notifytrigger",
        "actionType": "send_touchpoint",
        "config": { "channel": "sms", "templateId": "re_engage_sms_1" }
      },
      "nextStepId": "step_3_schedule_second"
    },
    {
      "stepId": "step_3_schedule_second",
      "order": 3,
      "stepType": "action",
      "actionRef": {
        "moduleRef": "cio",
        "actionType": "schedule_touchpoint",
        "config": { "delayDays": 3, "channel": "sms", "templateId": "re_engage_sms_2" }
      },
      "nextStepId": null
    }
  ]
}
```

---

## 4. Tích Hợp Rule Engine

### 4.1 Step type = "rule"

- Plan Executor build `RunInput` từ `inputRef.sourceMapping` + context hiện tại
- Gọi `RuleEngineService.Run(rule_id, domain, layers, params)`
- Output ghi vào `targetLayer` (ctx.layers[targetLayer] = output)
- Chuyển sang `nextStepId`

### 4.2 Rule Engine đã có sẵn

| Rule | Domain | Input | Output |
|------|--------|-------|--------|
| RULE_CIO_FREQUENCY_CHECK | cio | cio_context | frequency_ok (allowed, reason) |
| RULE_CIO_CHANNEL_CHOICE | cio | cio_context, frequency_ok | channel_choice (channelChosen, reason) |

**Lợi ích:** Không cần logic mới — plan chỉ **orchestrate** các rule có sẵn. Thêm step = thêm rule mới (domain cio).

### 4.3 Step type = "action"

- Không gọi Rule Engine
- Gọi module tương ứng: `notifytrigger.TriggerProgrammatic`, `delivery.Send`, ...
- Config có thể tham chiếu output step trước: `channelFromLayer: "channel_choice.channelChosen"`

### 4.4 Step type = "condition"

- Gọi Rule Engine
- Output quyết định nhánh: `branchMapping[output.result]` → next_step_id
- Ví dụ: frequency filtered → end; allowed → step_2_channel

---

## 5. Input/Output Mở — Kết Nối Module

### 5.1 Input Sources

| Source | Mô tả | Ví dụ |
|--------|-------|-------|
| `ctx.layers.*` | Output từ step trước | channel_choice, frequency_ok |
| `ctx.entity_ref` | Entity đang xử lý | unifiedId, ownerOrganizationId |
| `ctx.params` | Params từ param_ref | cooldownHours, th_* |
| External | CRM, CI — Plan Executor fetch trước khi chạy | valueTier, lifecycleStage |

### 5.2 Output Targets

| Target | Mô tả |
|--------|-------|
| `targetLayer` | Ghi vào ctx.layers cho step sau |
| `moduleRef` | action step — gọi Delivery, notifytrigger |
| Decision Brain | Feedback step — gọi BuildDecisionCaseFromCIOChoice |

### 5.3 Param Override

- Plan có thể override param cho từng step (A/B test timing, template)
- `paramRef.override`: { "cooldownHours": 48 } — chỉ step này dùng 48h

---

## 6. Versioning & A/B Test

| Trường | Mục đích |
|--------|----------|
| `planId` + `version` | Định danh plan. Experiment variant = (planId, planVersion) |
| `changeReason` | Lý do thay đổi — audit |
| Experiment | variants: [ { id: "A", planId: "re_engage_single_zalo", planVersion: 1 }, { id: "B", planId: "re_engage_sequence_sms", planVersion: 1 } ] |

**Attribution:** cio_touchpoint_plans, cio_routing_decisions ghi `planId`, `planVersion` → Decision Brain phân tích theo plan.

---

## 7. Luồng Thực Thi

```
1. CRM recommendation → CIO PlanTouchpoint(unifiedId, goalCode, experimentId?)
2. Assign variant (nếu có experiment) → (planId, planVersion)
3. Load cio_plan_definitions { planId, version }
4. Build initial context: layers.cio_context từ crm_customers, cio_touchpoint_plans
5. For each step (theo order):
   a. stepType=rule: RuleEngine.Run(ruleRef) → output → layers[targetLayer]
   b. stepType=action: Gọi actionRef.moduleRef với config
   c. stepType=condition: RuleEngine.Run → branchMapping[output] → nextStepId
6. Ghi trace: planId, planVersion, stepId, rule_id, output
7. Feedback → Decision Brain (planId, planVersion, variant)
```

---

## 8. So Sánh: Dùng Rule Engine vs Không

| Tiêu chí | Dùng Rule Engine | Không dùng |
|----------|------------------|------------|
| Logic channel choice, frequency | Rule có sẵn — tái sử dụng | Hardcode trong CIO service |
| Thêm step mới | Thêm rule (domain cio) + step ref | Sửa code |
| A/B test param | Param Set version — Rule Engine | Manual |
| Trace | rule_id, logic_version, param_version | Không có |
| Consistency | Cùng rule cho Ads, CRM, CIO | Mỗi module logic riêng |

**Kết luận:** Nên dùng Rule Engine cho các bước **quyết định** (frequency, channel, routing). Step **action** (gửi tin) gọi module trực tiếp.

---

## 9. Đề Xuất Triển Khai

| Phase | Công việc |
|-------|-----------|
| 1 | Collection cio_plan_definitions, model, CRUD API |
| 2 | Plan Executor — chạy steps, tích hợp Rule Engine cho step type=rule |
| 3 | Step type=action — gọi notifytrigger/delivery |
| 4 | Step type=condition — branching |
| 5 | Experiment gắn planId+planVersion, attribution |
| 6 | Migrate PlanTouchpoint hiện tại sang load plan definition |

---

## 10. Use Case — Thiết Kế Có Đáp Ứng Không?

### 10.1 Outbound Touchpoint (Đã cover)

| Use case | Mô tả | Đáp ứng? | Ghi chú |
|----------|-------|----------|---------|
| **UC-1: Re-engage đơn giản** | CRM repeat_gap_risk → 1 touchpoint Zalo/SMS | ✅ | Plan re_engage_single_zalo, step rule (frequency, channel) + action (send) |
| **UC-2: Re-engage chuỗi** | 2 touchpoint SMS cách 3 ngày | ✅ | Plan re_engage_sequence_sms, step schedule_touchpoint |
| **UC-3: Re-engage fallback** | Zalo trước, không mở → SMS sau 2 ngày | ⚠️ | Cần step condition + action schedule có điều kiện (chờ event "opened") |
| **UC-4: Welcome** | Khách mới → 1 tin chào | ✅ | Plan welcome_single, goalCode welcome |
| **UC-5: Abandoned cart** | Giỏ bỏ dở → nhắc nhở | ✅ | Plan abandoned_cart_single, goalCode abandoned_cart |
| **UC-6: VIP reactivation** | Khách VIP lâu không mua | ✅ | Plan vip_reactivation, param override (cooldown ngắn hơn) |

### 10.2 Frequency & Channel Control (Đã cover)

| Use case | Mô tả | Đáp ứng? | Ghi chú |
|----------|-------|----------|---------|
| **UC-7: Cooldown** | Không gửi nếu vừa touch < 24h | ✅ | Step rule RULE_CIO_FREQUENCY_CHECK |
| **UC-8: Chọn kênh theo valueTier** | VIP → Zalo, thường → SMS | ✅ | Step rule RULE_CIO_CHANNEL_CHOICE, input từ crm_customers |
| **UC-9: max_messages_per_24h** | Giới hạn spam | ⚠️ | Cần thêm param vào RULE_CIO_FREQUENCY_CHECK, plan chỉ ref param |
| **UC-10: preferred_channel** | Profile có preferred_channel | ⚠️ | Cần CI thêm field, input vào cio_context; rule đọc từ layer |

### 10.3 A/B Test (Đã cover)

| Use case | Mô tả | Đáp ứng? | Ghi chú |
|----------|-------|----------|---------|
| **UC-11: A/B kênh** | Zalo vs SMS cho cùng goal | ✅ | Experiment variants = 2 plan (hoặc 1 plan, 2 variantConfig) |
| **UC-12: A/B plan** | Plan single vs plan sequence | ✅ | Experiment variants = (planId1, v1) vs (planId2, v1) |
| **UC-13: A/B param** | Cooldown 24h vs 48h | ✅ | paramRef.override hoặc 2 Param Set version |
| **UC-14: A/B version plan** | Plan v1 vs v2 (sửa template) | ✅ | planId + version, attribution |

### 10.4 Dynamic Routing — AI vs Human (Chưa cover đầy đủ)

| Use case | Mô tả | Đáp ứng? | Ghi chú |
|----------|-------|----------|---------|
| **UC-15: VIP → Human** | Khách VIP chuyển Sales | ⚠️ | Cần RULE_CIO_ROUTING_MODE (chưa seed), step rule mới |
| **UC-16: Visitor → AI** | Khách mới AI greeting | ⚠️ | Tương tự, routing_mode → action khác (ai_agent vs human) |
| **UC-17: Intent Complaint → Human** | Phàn nàn → Human Only | ⚠️ | Cần step condition đọc intent từ layer; intent từ AI/NLP |
| **UC-18: Capacity-based** | Cân bằng tải Sales | ❌ | Thiết kế plan chưa có — cần module bên ngoài (queue, assignment) |

### 10.5 Inbound & Session (Chưa cover)

| Use case | Mô tả | Đáp ứng? | Ghi chú |
|----------|-------|----------|---------|
| **UC-19: Inbound routing** | Message inbound → AI vs Human | ❌ | Plan Definition là **outbound**; inbound cần flow khác (event-driven) |
| **UC-20: Session state** | new, engaged, qualification, … | ❌ | Plan không quản session state; cần cio_sessions + logic riêng |
| **UC-21: Context Memory** | Lịch sử chat, câu hỏi mở | ❌ | Ngoài scope plan — thuộc AI Agent / Session layer |

### 10.6 Feedback & Trace (Đã cover)

| Use case | Mô tả | Đáp ứng? | Ghi chú |
|----------|-------|----------|---------|
| **UC-22: Trace đầy đủ** | planId, planVersion, stepId, rule_id | ✅ | Luồng thực thi ghi trace |
| **UC-23: Decision Brain attribution** | outcome → planId, variant | ✅ | cio_touchpoint_plans, cio_routing_decisions |
| **UC-24: Cập nhật CI** | engagement signals, intent | ⚠️ | Cần step action "feedback_to_ci" — config mở có thể thêm |

### 10.7 Trigger Đa Nguồn (Đã cover)

| Use case | Mô tả | Đáp ứng? | Ghi chú |
|----------|-------|----------|---------|
| **UC-25: CRM trigger** | repeat_gap_risk → PlanTouchpoint | ✅ | PlanTouchpoint(unifiedId, goalCode) |
| **UC-26: Ads trigger** | Campaign kết thúc → chăm sóc | ⚠️ | Cùng API PlanTouchpoint, sourceRef khác |
| **UC-27: Campaign/Content trigger** | Chiến dịch thủ công | ⚠️ | Tương tự |

### 10.8 Tổng Kết

| Nhóm | Đáp ứng | Đáp ứng một phần | Chưa đáp ứng |
|------|---------|------------------|--------------|
| Outbound Touchpoint | 5 | 1 | 0 |
| Frequency & Channel | 2 | 2 | 0 |
| A/B Test | 4 | 0 | 0 |
| Dynamic Routing | 0 | 3 | 1 |
| Inbound & Session | 0 | 0 | 3 |
| Feedback & Trace | 2 | 1 | 0 |
| Trigger Đa Nguồn | 1 | 2 | 0 |

**Kết luận:**
- Thiết kế **đáp ứng tốt** outbound touchpoint, frequency/channel, A/B test, trace.
- **Cần mở rộng** cho: routing AI vs Human (rule mới), fallback có điều kiện (opened), max_messages_per_24h, preferred_channel.
- **Ngoài scope** plan definition: inbound routing, session state, context memory — cần kiến trúc khác (event-driven, session layer).

---

## 11. Changelog

- 2025-03-16: Tạo đề xuất Plan Definition, tích hợp Rule Engine.
- 2025-03-16: Thêm §10 Use Case — đối chiếu thiết kế với use case.
