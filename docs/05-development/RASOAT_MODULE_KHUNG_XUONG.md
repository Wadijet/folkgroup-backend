# Rà Soát Module — Cần Làm Khung Xương Trước

**Ngày:** 2025-03-18  
**Cập nhật:** 2026-03-18 — CIX, AI Decision Engine đã triển khai; luồng CIO→CIX→Decision→Executor đã khép vòng.  
**Mục đích:** Đối chiếu vision với code, xác định module nào chưa có khung (router, handler, service, models) cần tạo trước.

**Tham chiếu:** [docs-shared/architecture/vision/](../../docs-shared/architecture/vision/), [backend-module-map.md](../module-map/backend-module-map.md), [docs-shared/architecture/RA_SOAT_TRIEN_KHAI_VISION.md](../../docs-shared/architecture/RA_SOAT_TRIEN_KHAI_VISION.md)

---

## 1. Tổng Quan — Module Đã Có Khung

| Module | Router | Handler | Service | Models | Ghi chú |
|--------|:------:|:-------:|:-------:|:------:|--------|
| auth | ✅ | ✅ | ✅ | ✅ | Đầy đủ |
| ads | ✅ | ✅ | ✅ | ✅ | Đầy đủ |
| approval | ✅ | ✅ | ⚠️ | ⚠️ | Service ở `internal/approval/` |
| agent | ✅ | ✅ | ✅ | ✅ | Đầy đủ |
| ai | ✅ | ✅ | ✅ | ✅ | Đầy đủ |
| **cix** | ✅ | ✅ | ✅ | ✅ | **Đã có** — Raw→L1→L2→L3→Flag→Action, CIO→CIX→Decision |
| cio | ✅ | ✅ | ✅ | ✅ | Đầy đủ |
| content | ✅ | ✅ | ✅ | ✅ | Đầy đủ |
| crm | ✅ | ✅ | ✅ | ✅ | Đầy đủ |
| cta | ✅ | ✅ | ✅ | ✅ | Đầy đủ |
| decision | ✅ | ✅ | ✅ | ✅ | **Decision Brain + AI Decision Engine** (Execute, ReceiveCixPayload) |
| delivery | ✅ | ✅ | ✅ | ✅ | Chỉ gửi message; CIX Executor gọi ExecuteActions |
| fb | ✅ | ✅ | ✅ | ✅ | Đầy đủ |
| meta | ✅ | ✅ | ✅ | ✅ | Meta Ads đầy đủ |
| notification | ✅ | ✅ | ✅ | ✅ | Đầy đủ |
| pc | ✅ | ✅ | ✅ | ✅ | Pancake POS |
| report | ✅ | ✅ | ✅ | ✅ | Đầy đủ |
| ruleintel | ✅ | ✅ | ✅ | ✅ | Đầy đủ |
| webhook | ✅ | ✅ | ✅ | ✅ | Đầy đủ |

---

## 2. Module Chưa Có Khung — Cần Làm Trước

### 2.1 CIX — Contextual Conversation Intelligence ✅ ĐÃ TRIỂN KHAI (2026-03)

| Hạng mục | Trạng thái | Ghi chú |
|----------|------------|---------|
| **Module** | ✅ Có | `api/internal/api/cix/` — router, handler, service, models |
| **Collections** | ✅ Có | `cix_analysis_results`, **`cix_intel_compute`** (job; tên cũ trong doc: `cix_pending_analysis`) |
| **Luồng** | ✅ Khép vòng | Queue AI Decision → enqueue **`cix_intel_compute`** → **CixIntelComputeWorker** → AnalyzeSession → **`cix_intel_recomputed`** (`analysisResultId`) → **ReceiveCixPayload** → **TryExecuteIfReady** / **execute_requested** → Execute → Propose → Executor → Delivery |
| **Rule Engine** | ✅ Có | RULE_CIX_LAYER1_STAGE, LAYER2_STATE, LAYER2_ADJUST, LAYER3_SIGNALS, FLAGS, ACTIONS |
| **Tích hợp** | ✅ Có | Datachanged / orchestrate / HTTP → consumer enqueue job, CRM (getCustomerContext), Decision (**ReceiveCixPayload** qua event sau worker) |

**Chi tiết:** Xem [PHUONG_AN_TRIEN_KHAI_CIX.md](./PHUONG_AN_TRIEN_KHAI_CIX.md) §2 Trạng Thái Hiện Tại (đã cập nhật).

---

### 2.2 AI Decision Engine ✅ ĐÃ TRIỂN KHAI (2026-03)

| Hạng mục | Trạng thái | Ghi chú |
|----------|------------|---------|
| **Module** | ✅ Có | `decision/` — DecisionEngineService, Execute, ReceiveCixPayload |
| **Endpoint** | ✅ Có | `POST /decision/execute` — nhận context, trả Execution Plan |
| **Logic** | ✅ Có | applyPolicy (CIX_APPROVAL_ACTIONS), proposeCixAction, proposeAndApproveAutoCixAction |
| **Executor** | ✅ Có | `executors/cix/` — trigger_fast_response, escalate_to_senior, assign_to_human_sale, prioritize_followup → Delivery |
| **Thiếu** | ⏳ | ApprovalModeConfig, ResolveImmediate (config-driven); Context Aggregation đầy đủ từ Ads/CIO |

**Chi tiết:** Xem [PHUONG_AN_TRIEN_KHAI_AI_DECISION_VA_LEARNING.md](./PHUONG_AN_TRIEN_KHAI_AI_DECISION_VA_LEARNING.md).

---

### 2.3 Execution Engine (Phần 5)

| Hạng mục | Trạng thái | Ghi chú |
|----------|------------|---------|
| **Module** | ⚠️ Một phần | `delivery/` gửi message; Executor (ads, cio, cix) gọi Delivery |
| **Vision** | [05 - execution-engine.md](../../docs-shared/architecture/vision/05%20-%20execution-engine.md) | Tầng thực thi duy nhất |
| **Hiện tại** | delivery_queue, delivery_history | SEND_MESSAGE, ASSIGN_TO_AGENT, TAG_CUSTOMER (qua CIX Executor) |
| **Executor CIX** | ✅ Có | `executors/cix/` → ExecuteActions → Delivery |
| **Thiếu** | PAUSE_ADSET, UPDATE_AD, CREATE_ORDER, PUBLISH_CONTENT | Adapters cho Ads, Content |

**Ưu tiên:** **Trung bình** — Luồng CIX đã qua Executor → Delivery.

---

### 2.4 Google Ads Engine

| Hạng mục | Trạng thái | Ghi chú |
|----------|------------|---------|
| **Module** | ❌ Chưa có | Không có `googleads/` hay `google/` |
| **Vision** | [09 - google-ads-intelligence.md](../../docs-shared/architecture/vision/09%20-%20google-ads-intelligence.md) | Capture demand, search intent |
| **Tham chiếu** | `meta/` | Cấu trúc tương tự: account, campaign, ad-group, ad, insight |

**Khung cần tạo:**
- `api/internal/api/googleads/` — router, handler, service, models
- Collections: google_ad_accounts, google_campaigns, google_ad_groups, google_ads, google_ad_insights
- Ingestion: sync từ Google Ads API
- Rule: RULE_GOOGLE_ADS_* (Layer 1, 2, 3, Flag, Action)

**Ưu tiên:** **Trung bình** — Meta đã đủ, Google mở rộng thị trường.

---

### 2.5 Input Factory (Content OS)

| Hạng mục | Trạng thái | Ghi chú |
|----------|------------|---------|
| **Module** | ❌ Chưa có | Không có `inputfactory/` |
| **Vision** | [12 - content-expansion-system.md](../../docs-shared/architecture/vision/12%20-%20content-expansion-system.md), CES | Nơi tạo insight từ nhiều nguồn |
| **Nguồn** | Customer Intelligence, Ads Intelligence, Decision Brain | Feed vào Content Node L3 (insight) |

**Khung cần tạo:**
- **Option A:** Sub-module trong `content/` — `content/service/service.content.input_factory.go`
- **Option B:** Module mới `inputfactory/` — nếu logic phức tạp, nhiều job
- Pipeline: Aggregate từ crm, meta_ad_insights, decision_cases → tạo/update content_nodes (insight)
- Có thể là worker + service, không bắt buộc API public

**Ưu tiên:** **Trung bình** — Content OS có pipeline L1–L8, thiếu nguồn insight tự động.

---

### 2.6 Cross Ads Intelligence

| Hạng mục | Trạng thái | Ghi chú |
|----------|------------|---------|
| **Module** | ❌ Chưa có | Không cần module riêng |
| **Vision** | Creative thắng Meta → feed Content OS, Google, TikTok | |
| **Cách làm** | Service trong `ads/` hoặc `content/` | `ads/service/service.ads.cross_intel.go` |
| **Worker** | Job phát hiện creative winner | Có thể trong `ads/worker/` |

**Khung cần tạo:**
- Service: `service.ads.cross_intel.go` — detect winner từ meta_ad_insights, emit event/job
- Consumer: Content OS (tạo insight node) — có thể qua Input Factory
- **Không cần** module mới, chỉ cần service + worker.

**Ưu tiên:** **Thấp** — Có thể làm sau Cross Ads.

---

### 2.7 Segment API (Customer Intelligence)

| Hạng mục | Trạng thái | Ghi chú |
|----------|------------|---------|
| **Module** | ⚠️ Một phần | `crm/` có, chưa có segment động |
| **Vision** | GET `/crm/segments/:id/customers` | Filter theo valueTier, lifecycleStage, intentStage |
| **Cách làm** | Thêm endpoint trong crm | Không cần module mới |

**Khung cần tạo:**
- Collection: `crm_segments` (definition: filter criteria)
- Handler: `HandleListSegmentCustomers` — query crm_customers theo segment definition
- **Không cần** module mới, mở rộng crm.

**Ưu tiên:** **Thấp** — CRM đã có filter, segment là mở rộng.

---

## 3. Thứ Tự Ưu Tiên Làm Khung Xương

| # | Module / Hạng mục | Lý do | Effort |
|---|-------------------|-------|--------|
| 1 | ~~**CIX**~~ | ✅ Đã triển khai | — |
| 2 | ~~**AI Decision Engine**~~ | ✅ Đã triển khai (CIX flow) | — |
| 3 | **Approval Gate thống nhất** | ApprovalModeConfig, ResolveImmediate (PHUONG_AN_DECISION_BRAIN) | Trung bình |
| 4 | **Google Ads** | Mở rộng Ads Intelligence, độc lập với luồng CIO | Lớn |
| 5 | **Input Factory** | Feed insight vào Content OS, có thể là service trong content | Trung bình |
| 6 | **Cross Ads** | Service + worker, không cần module mới | Nhỏ |
| 7 | **Segment API** | Mở rộng crm, thêm endpoint | Nhỏ |

---

## 4. Luồng Phụ Thuộc (Đã Khép Vòng)

```
CIO (✅)  →  CIX (✅)  →  AI Decision Engine (✅)  →  Executor → Delivery (✅)
    |              |                    |                        |
    |              |                    |                        └→ delivery.ExecuteActions
    |              |                    └→ Propose / ProposeAndApproveAuto
    |              └→ Rule Engine (✅), Customer (✅)
    └→ (luồng hiện tại) queue AI Decision → **`cix_intel_compute`** → worker CIX
```

**Luồng đã đủ để chạy:** CIO ingestion → CIX worker → ReceiveCixPayload → Execute → Executor → Delivery.

---

## 5. Tóm Tắt

| Loại | Số lượng | Module |
|------|----------|--------|
| **Cần module mới** | 1 | Google Ads |
| **Cần mở rộng** | 1 | Approval Gate (ApprovalModeConfig, ResolveImmediate) |
| **Cần service/endpoint** | 3 | Input Factory (trong content), Cross Ads (trong ads), Segment (trong crm) |

**Ưu tiên tiếp theo:** Approval Gate thống nhất → Google Ads → Input Factory.

---

## Changelog

- 2026-03-18: Cập nhật — CIX, AI Decision Engine đã triển khai; luồng CIO→CIX→Decision→Executor đã khép vòng
- 2025-03-18: Tạo tài liệu rà soát module khung xương
