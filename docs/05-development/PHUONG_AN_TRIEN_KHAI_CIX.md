# Phương Án Triển Khai CIX — Contextual Conversation Intelligence

**Ngày:** 2026-03-18  
**Tham chiếu:** [05 - cix-contextual-conversation-intelligence.md](../../docs-shared/architecture/vision/05%20-%20cix-contextual-conversation-intelligence.md), [THIET_KE_CIX_COLLECTIONS.md](./THIET_KE_CIX_COLLECTIONS.md), [rule-intelligence.md](../02-architecture/core/rule-intelligence.md)

---

## 0. CIO Hiện Tại (Đã Có — CIX Dựa Trên)

CIO đã có event model đầy đủ. CIX chỉ cần tích hợp.

| CIO đã có | Chi tiết |
|-----------|----------|
| **cio_events** | EventCategory (interaction \| business), EventScope, Domain, Tags, CausedBy, ResultRefs |
| **conversation_updated** | OnConversationUpsert (fb_conversations) → ghi cio_event |
| **message_updated** | OnMessageUpsert (fb_messages) → ghi cio_event |
| **order_created/updated/cancelled** | InjectOrderEvent (pc_pos_orders) → ghi cio_event — CIX có thể dùng cho context "khách vừa đặt đơn" |
| **touchpoint_triggered** | InjectTouchpointTriggered |

**CIX trigger:** `conversation_updated`, `message_updated` (EventCategory=interaction). CIX **chưa** được enqueue — cần thêm hook.

---

## 0.1 Collections CIX

| Collection | Vai trò |
|------------|---------|
| `cix_analysis_results` | Kết quả phân tích L1/L2/L3/Flags/Actions |
| `cix_pending_analysis` | Hàng đợi — CIO event → enqueue → worker |
| `cix_conversations` | (Phase 4+) Merged context, versioning — xem [THIET_KE_CIX_COLLECTIONS.md](./THIET_KE_CIX_COLLECTIONS.md) |

---

## 1. Tổng Quan

**CIX = Contextual Conversation Intelligence** — "Người phiên dịch có ngữ cảnh". CIX phân tích hội thoại theo pipeline **Raw → L1 → L2 → L3 → Flag → Action**, đối chiếu với Customer Profile từ Customer Intelligence.

| Phase | Tên | Thời gian | Deliverable |
|-------|-----|-----------|-------------|
| 1 | Foundation & API | 1 tuần | Hoàn thiện API, DTO, mở rộng model |
| 2 | CIO & CRM Integration | 1–2 tuần | Đọc conversation từ CIO, profile từ CRM |
| 3 | Rule Engine Pipeline | 2–3 tuần | RULE_CIX_* (L1, L2, L3, Flag, Action) |
| 4 | Event-Driven & Decision | 1–2 tuần | Hook CIO event, gửi Decision Engine, cix_signal_update |
| 5 | Mở rộng (tùy chọn) | 1–2 tuần | LLM Layer 3, Dashboard, batch analysis |

**Tổng:** 5–9 tuần (Phase 1–4 bắt buộc); Phase 5 tùy nhu cầu.

---

## 1.1 Định Vị CIX

| Vai trò | Mô tả |
|---------|-------|
| **Input** | Raw conversation (CIO), customer context (CRM) |
| **Xử lý** | Rule Engine: Raw → L1 → L2 → L3 → Flag → Action |
| **Output** | cix_analysis_results, cix_signal_update (CRM), payload (Decision) |

**Nguyên tắc:** CIX **không thực thi** — chỉ đưa ra gợi ý cho Decision Engine.

---

## 1.2 Luồng CIX End-to-End

```
cio_events (conversation_updated, message_updated)
    │
    │ CIX hook: gọi EnqueueAnalysis sau InsertOne
    ▼
cix_pending_analysis
    │
    │ cix_analysis_worker poll
    ▼
AnalyzeSession:
  1. Đọc transcript từ fb_message_items (conversationId)
  2. Đọc customer context từ crm_customers (valueTier, journeyStage)
  3. Rule Engine: Raw → L1 → L2 → L3 → Flag → Action
  4. Lưu cix_analysis_results
    │
    ├─► cix_signal_update → CRM (làm giàu Layer 3)
    └─► CIX payload → Decision Engine
```

---

## 1.3 CIX — Input/Output

| Hướng | Nguồn/Đích | Chi tiết |
|-------|------------|----------|
| **Input** | `fb_message_items` | Transcript theo conversationId |
| **Input** | `crm_customers` | valueTier, lifecycleStage, journeyStage, flags |
| **Input** | `cio_events` (tùy chọn) | order_created gần đây — context "khách vừa đặt đơn" |
| **Output** | `cix_analysis_results` | L1/L2/L3, flags, actionSuggestions |
| **Output** | CRM | cix_signal_update — Layer 3 signals |
| **Output** | Decision Engine | Payload cho Execute |

---

## 2. Trạng Thái Hiện Tại (Cập nhật 2026-03-18)

| Hạng mục | Trạng thái | Ghi chú |
|----------|------------|---------|
| Router | ✅ Có | `cix/router/routes.go` |
| Handler | ✅ Có | `handler.cix.analysis.go` — POST /cix/analyze |
| Service | ✅ Có | `AnalyzeSession` — Rule Engine pipeline Raw→L1→L2→L3→Flag→Action |
| Models | ✅ Có | `CixAnalysisResult`, CixLayer1/2/3, CixFlag |
| Collection | ✅ Có | `cix_analysis_results`, `cix_pending_analysis` |
| Rule Engine | ✅ Có | RULE_CIX_LAYER1_STAGE, LAYER2_STATE, LAYER2_ADJUST, LAYER3_SIGNALS, FLAGS, ACTIONS |
| CIO Integration | ✅ Có | OnCioEventInserted → EnqueueAnalysis; đọc transcript từ fb_message_items |
| CRM Integration | ✅ Có | getCustomerContext (valueTier, lifecycleStage, journeyStage); OnCixSignalUpdate |
| Decision Integration | ✅ Có | ReceiveCixPayload → DecisionEngine.Execute → Propose/ProposeAndApproveAuto → Executor → Delivery |
| Worker | ✅ Có | CixAnalysisWorker poll cix_pending_analysis (30s) |

---

## 3. Phase 1: Foundation & API (1 tuần)

**Mục tiêu:** Hoàn thiện API, DTO, mở rộng model theo schema chuẩn.

| # | Công việc | File | Chi tiết |
|---|-----------|------|----------|
| 1 | DTO Request/Response | `dto/dto.cix.analysis.go` | `AnalyzeSessionRequest`, `CixAnalysisResponse` theo schema vision |
| 2 | Mở rộng model CixLayer2 | `models/model.cix.analysis.go` | Thêm `AdjustmentReason` (đã có `AdjustmentRule`) |
| 3 | API GET kết quả theo session | `handler.cix.analysis.go` | `GET /cix/analysis/:sessionUid` — FindBySessionUid |
| 4 | Response format chuẩn | Handler | `code`, `message`, `data`, `status` theo api-structure |
| 5 | Permission | Router | `CIX.Read`, `CIX.Analyze` — tách khỏi CIO.Read tạm |

**Deliverable:** API POST /cix/analyze và GET /cix/analysis/:sessionUid hoạt động, trả kết quả mặc định.

---

## 4. Phase 2: CIO & CRM Integration (1–2 tuần)

**Mục tiêu:** CIX đọc được conversation từ CIO và profile từ CRM.

### 4.1 Đọc Conversation (CIX)

| # | Công việc | Chi tiết |
|---|-----------|----------|
| 1 | Nguồn | `fb_message_items` theo conversationId (từ cio_event) |
| 2 | Fallback | `fb_conversations` + `fb_messages` nếu chưa có message_items |
| 3 | Contract | `[]ConversationTurn{From, Content, Timestamp, Channel}` |

**Ghi chú:** cio_event có ConversationID, CustomerID. CIX dùng conversationId làm key.

### 4.2 CRM — Đọc Customer Profile

| # | Công việc | Chi tiết |
|---|-----------|----------|
| 1 | CrmService method | `GetProfileForCix(ctx, customerUid, ownerOrgID)` — projection: valueTier, lifecycleStage, journeyStage, flags |
| 2 | Lookup | Hỗ trợ uid, unifiedId (buildCustomerFilterByIdOrUid) |
| 3 | Contract | `CixCustomerContext{ValueTier, LifecycleStage, JourneyStage, Flags}` |

### 4.3 CixAnalysisService — Tích Hợp

```go
// AnalyzeSession — logic mới
// 1. Lấy conversation từ CIO (hoặc fb nếu CIO chưa có)
// 2. Lấy customer context từ CRM (nếu customerUid != "")
// 3. Build Raw input: conversation + customerContext
// 4. (Phase 3) Gọi Rule Engine pipeline
// 5. Lưu cix_analysis_results
```

**Deliverable:** AnalyzeSession nhận conversation thật và customer context; vẫn trả L1/L2/L3 mặc định (chưa Rule Engine).

---

## 5. Phase 3: Rule Engine Pipeline (2–3 tuần)

**Mục tiêu:** Chạy pipeline Raw → L1 → L2 → L3 → Flag → Action qua Rule Engine.

### 5.1 Layers & Rules Cần Seed

| Bước | Layer | Rule ID | Input | Output |
|------|-------|---------|-------|--------|
| 1 | L1 | RULE_CIX_LAYER1_STAGE | raw_conversation, customer_context | stage (new, engaged, consulting, negotiating, waiting, stalled) |
| 2 | L2 | RULE_CIX_LAYER2_STATE | L1, raw, customer_context | intentStage, urgencyLevel, riskLevelRaw |
| 3 | L2 Adj | RULE_CIX_LAYER2_ADJUST | L2.riskLevelRaw, customer_context | riskLevelAdj, adjustmentReason, rule_id |
| 4 | L3 | RULE_CIX_LAYER3_SIGNALS | raw, L1, L2 | buyingIntent, objectionLevel, sentiment |
| 5 | Flag | RULE_CIX_FLAGS | L2.adj, L3, customer_context | flags[] |
| 6 | Action | RULE_CIX_ACTIONS | flags, L2, L3 | actionSuggestions[] |

### 5.2 Output Contract — CIX

| Output ID | Schema |
|-----------|--------|
| OUT_CIX_LAYER1 | `{ stage: string }` |
| OUT_CIX_LAYER2 | `{ intentStage, urgencyLevel, riskLevelRaw }` |
| OUT_CIX_LAYER2_ADJ | `{ riskLevelAdj, adjustmentReason, ruleId }` |
| OUT_CIX_LAYER3 | `{ buyingIntent, objectionLevel, sentiment }` |
| OUT_CIX_FLAGS | `{ flags: [{ name, severity, triggeredByRule }] }` |
| OUT_CIX_ACTIONS | `{ actionSuggestions: string[] }` |

### 5.3 Logic Script Ví Dụ — RULE_CIX_LAYER2_ADJUST (VIP)

```javascript
// ADJUST_RISK_VIP_v1 — docs vision
function evaluate(ctx) {
  var L2 = ctx.layers.cix_layer2 || {};
  var customer = ctx.layers.cix_customer_context || {};
  var report = { input: { L2, customer }, log: '' };
  
  if (L2.riskLevelRaw === 'warning' && customer.valueTier === 'top') {
    report.log = 'VIP + warning → danger';
    return {
      output: {
        riskLevelAdj: 'danger',
        adjustmentReason: 'vip_customer_complaint',
        ruleId: 'ADJUST_RISK_VIP_v1'
      },
      report: report
    };
  }
  
  return {
    output: {
      riskLevelAdj: L2.riskLevelRaw || 'safe',
      adjustmentReason: '',
      ruleId: ''
    },
    report: report
  };
}
```

### 5.4 Migration Seed

| File | Nội dung |
|------|----------|
| `ruleintel/migration/seed_rule_cix_system.go` | Seed RULE_CIX_LAYER1_STAGE, LAYER2_STATE, LAYER2_ADJUST, LAYER3_SIGNALS, FLAGS, ACTIONS + Logic + Param + Output |

### 5.5 CixAnalysisService — Gọi Rule Engine

```go
// RunPipeline — gọi từng rule theo thứ tự
// ctx.layers: cix_raw, cix_customer_context, cix_layer1, cix_layer2, cix_layer3, cix_flags
// Mỗi bước: Run(domain: "cix", ruleID, input) → output → merge vào layers
```

**Deliverable:** AnalyzeSession chạy pipeline thật, L1/L2/L3/Flags/Actions từ Rule Engine.

---

## 6. Phase 4: Event-Driven & Decision (1–2 tuần)

**Mục tiêu:** CIO event → enqueue → worker xử lý → gửi Decision Engine, push cix_signal_update về CRM.

### 6.1 Hàng đợi — Option B (Đã chốt)

| Thành phần | Mô tả |
|------------|-------|
| **Collection** | `cix_pending_analysis` — queue riêng |
| **Enqueue** | **CIX thêm:** Sau khi CIO ingestion ghi cio_event (conversation_updated, message_updated) → gọi `CixService.EnqueueAnalysis(ctx, event)` (fire-and-forget) |
| **Worker** | Poll `cix_pending_analysis` (processedAt = null) → gọi AnalyzeSession → đánh dấu processedAt |
| **Dedupe** | BusinessKey = conversationId — upsert, cùng conversation chỉ 1 job mới nhất |

### 6.2 Schema cix_pending_analysis

| Field | Mô tả |
|-------|-------|
| conversationId | Từ cio_event |
| customerId | ID từ kênh (resolve → customerUid trong worker) |
| channel | messenger \| zalo \| … |
| cioEventUid | Ref event gốc |
| ownerOrganizationId | Org |
| processedAt | null = chưa xử lý; set khi xong |
| processError | Lỗi nếu có |
| retryCount | Số lần retry |
| createdAt | Thời điểm enqueue |

Chi tiết: [THIET_KE_CIX_COLLECTIONS.md](./THIET_KE_CIX_COLLECTIONS.md) §cix_pending_analysis

### 6.3 Worker CIX

| # | Công việc | Chi tiết |
|---|-----------|----------|
| 1 | Worker | `worker/cix_analysis_worker.go` — poll `cix_pending_analysis` (processedAt = null) |
| 2 | Batch | Xử lý N job mỗi lần (limit 50–100) |
| 3 | Resolve | conversationId → sessionUid, customerUid (qua cio_sessions hoặc fb_conversations) |
| 4 | Idempotent | Gọi AnalyzeSession; set processedAt khi xong |

### 6.4 Gửi Sang Decision Engine

| # | Công việc | Chi tiết |
|---|-----------|----------|
| 1 | Decision service | `DecisionEngineService.ReceiveCixPayload` hoặc `BuildDecisionCaseFromCix` |
| 2 | Input | CIX payload (sessionUid, customerUid, Layer1/2/3, Flags, ActionSuggestions) |
| 3 | Output | Tạo decision case (caseType: "cix_analysis") hoặc đưa vào context cho Execute |

**Ghi chú:** Decision Engine hiện chỉ có Decision Brain (learning cases). Cần mở rộng để nhận CIX payload — xem [RASOAT_MODULE_KHUNG_XUONG.md](./RASOAT_MODULE_KHUNG_XUONG.md) §2.2.

### 6.5 cix_signal_update — Push Về CRM

| # | Công việc | Chi tiết |
|---|-----------|----------|
| 1 | Event schema | `cix_signal_update{ customerUid, layer3: { buyingIntent, sentiment }, traceId }` |
| 2 | CRM nhận | `CrmService.OnCixSignalUpdate` — cập nhật psychographic tags, intent signals |
| 3 | Storage | Có thể lưu trong crm_customers (field mới) hoặc crm_activity_history |

**Deliverable:** CIO event → enqueue cix_pending_analysis → worker xử lý → gửi payload sang Decision, push signal về CRM.

---

## 7. Phase 5: Mở Rộng (Tùy Chọn)

| Hạng mục | Mô tả | Effort |
|----------|-------|--------|
| **LLM Layer 3** | Layer 3 (Micro Signals) dùng LLM thay Rule — extract buyingIntent, sentiment từ nội dung | ✅ Đã triển khai |

### 7.1 LLM Layer 3 — Đã triển khai (2026-03-18)

- **File:** `api/internal/api/cix/service/service.cix.llm.go` — `CixLLMService.ExtractLayer3Signals`
- **API key và model:** Ưu tiên từ `ai_provider_profiles` (DB) → fallback env
  - **DB:** Profile của org (ownerOrgID, provider=openai, status=active) hoặc system org "OpenAI Production"
  - **Env (fallback):** `OPENAI_API_KEY`, `CIX_LLM_MODEL` (mặc định: gpt-4o-mini)
- **Env:**
  - `CIX_LAYER3_MODE` — `rule` \| `llm` \| `hybrid` (mặc định: rule)
- **Mode:**
  - `rule`: Chỉ dùng Rule Engine (mặc định)
  - `llm`: Ưu tiên LLM, fallback Rule nếu không có API key
  - `hybrid`: Rule trước; nếu Rule trả giá trị mặc định (inquiring, neutral, none) → thử LLM
| **Dashboard** | GET /cix/stats, /cix/dashboard — thống kê phân tích theo org, kênh | 1 tuần |
| **Batch Analysis** | POST /cix/analyze/batch — phân tích nhiều session cùng lúc | 3–5 ngày |
| **Trace UI** | Hiển thị trace_id, rule_id trong từng bước pipeline | 1 tuần |

---

## 8. Cấu Trúc Code Đề Xuất

```
api/internal/api/cix/
├── handler/
│   └── handler.cix.analysis.go    # HandleAnalyzeSession, HandleGetAnalysisBySession
├── service/
│   ├── service.cix.analysis.go   # AnalyzeSession, RunPipeline, FindBySessionUid
│   └── service.cix.queue.go     # EnqueueAnalysis — gọi từ CIO ingestion
├── models/
│   ├── model.cix.analysis.go    # CixAnalysisResult, CixLayer1/2/3, CixFlag
│   └── model.cix.pending.go    # CixPendingAnalysis
├── dto/
│   └── dto.cix.analysis.go      # Request, Response
└── router/
    └── routes.go

api/internal/worker/
└── cix_analysis_worker.go        # Poll cix_pending_analysis, gọi AnalyzeSession

ruleintel/migration/
└── seed_rule_cix_system.go       # Seed RULE_CIX_*
```

---

## 9. Files CIX Cần Tạo/Sửa

| File | Hành động |
|------|-----------|
| `api/internal/api/cix/dto/dto.cix.analysis.go` | Tạo mới |
| `api/internal/api/cix/models/model.cix.analysis.go` | Sửa — bổ sung field nếu cần |
| `api/internal/api/cix/models/model.cix.pending.go` | Tạo mới — CixPendingAnalysis |
| `api/internal/api/cix/handler/handler.cix.analysis.go` | Sửa — thêm GET, dùng DTO |
| `api/internal/api/cix/service/service.cix.analysis.go` | Sửa — đọc fb_message_items, CRM, Rule Engine |
| `api/internal/api/cix/service/service.cix.queue.go` | Tạo mới — EnqueueAnalysis |
| `api/internal/api/cix/router/routes.go` | Sửa — thêm GET, permission |
| `api/internal/worker/cix_analysis_worker.go` | Tạo mới — poll cix_pending_analysis |
| `api/internal/worker/controller.go` | Sửa — đăng ký worker |
| `api/cmd/server/init.go`, `init.registry.go` | Sửa — đăng ký cix_pending_analysis |
| `api/internal/api/cio/ingestion/ingestion.go` | Sửa — gọi CixService.EnqueueAnalysis sau InsertOne (conversation_updated, message_updated) |
| `ruleintel/migration/seed_rule_cix_system.go` | Tạo mới |

**Phụ thuộc (nếu chưa có):** CrmService.GetProfileForCix, DecisionEngineService.ReceiveCixPayload.

---

## 10. Rủi Ro & Mitigation

| Rủi ro | Mitigation |
|--------|------------|
| CIO chưa có API conversation | Query trực tiếp fb_conversations + fb_messages theo sessionId |
| Rule Engine chậm (nhiều rule) | Cache customer context; batch rule execution |
| Volume cao (nhiều message) | Worker throttle; queue cix_pending_analysis |
| Decision Engine chưa sẵn sàng | Lưu payload vào cix_analysis_results; Decision đọc sau |

---

## 11. Checklist Trước Khi Bắt Đầu

- [ ] Đọc [05 - cix-contextual-conversation-intelligence.md](../../docs-shared/architecture/vision/05%20-%20cix-contextual-conversation-intelligence.md)
- [ ] Đọc [rule-intelligence.md](../02-architecture/core/rule-intelligence.md)
- [ ] Đọc [THIET_KE_CIX_COLLECTIONS.md](./THIET_KE_CIX_COLLECTIONS.md) — schema cix_conversations, cix_pending_analysis
- [ ] Xác nhận fb_message_items có conversationId để query transcript
- [ ] Xác nhận CRM có GetProfile (valueTier, journeyStage) hoặc tương đương

---

## Changelog

- 2026-03-18: Tạo phương án triển khai module CIX
- 2026-03-18: Chốt Phase 4 dùng hàng đợi Option B (cix_pending_analysis)
- 2026-03-18: **Cập nhật theo CIO mới** — chỉ tập trung CIX; CIO đã có EventCategory, order inject, touchpoint; CIX trigger từ conversation_updated, message_updated; thêm §0 đánh giá CIO hiện tại
- 2026-03-18: **LLM Layer 3** — CixLLMService, CIX_LAYER3_MODE (rule/llm/hybrid), OPENAI_API_KEY, CIX_LLM_MODEL
