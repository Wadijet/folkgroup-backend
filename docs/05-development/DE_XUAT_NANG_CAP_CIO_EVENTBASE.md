# Đề Xuất Phương Án Nâng Cấp CIO và Sửa Luồng Đáp Ứng Eventbase

**Ngày:** 2026-03-18  
**Tham chiếu:** [CIO_EVENT_MIGRATION_ASSESSMENT.md](./CIO_EVENT_MIGRATION_ASSESSMENT.md), [CIO_BOUNDARY_AND_EVENTS.md](./CIO_BOUNDARY_AND_EVENTS.md), [THIET_KE_MODULE_CIO.md](./THIET_KE_MODULE_CIO.md), [identity-links-model.md](../../docs-shared/architecture/data-contract/identity-links-model.md)  
**Tham khảo thêm:** Quy tắc lọc event — CIO chỉ lưu event thuộc interaction ledger; Ref model — CIO không store full result, chỉ giữ pointer + snapshot nhỏ (causedBy, resultRefs, links)

**Đã cập nhật vision:** [docs-shared/architecture/vision/04 - cio-customer-interaction-hub.md](../../docs-shared/architecture/vision/04%20-%20cio-customer-interaction-hub.md) §6–8; [unified-data-contract.md](../../docs-shared/architecture/data-contract/unified-data-contract.md) §2.5. **Tổng hợp:** [TOM_TAT_THAY_DOI_CIO_EVENTBASE.md](./TOM_TAT_THAY_DOI_CIO_EVENTBASE.md)

**Thuật ngữ (chống nhầm):** **CIO-T1 / CIO-T2 / CIO-T3** chỉ là **ba tầng lọc** xem event có được ghi vào `cio_events` hay không — **không** phải **bản ghi chạy intel**, **không** phải **Pha ghi thô / merge / intel** trong [KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md](./KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md). Chi tiết: [KHUNG_KHUON_MODULE_INTELLIGENCE.md](./KHUNG_KHUON_MODULE_INTELLIGENCE.md) mục 0.

---

## 0. Tóm Tắt

**Eventbase** = `cio_events` — event stream **có chọn lọc** cho tương tác và sự kiện business quanh khách hàng. CIO là **hub ghi nhận**, không xử lý logic domain. **Chỉ lưu event thuộc interaction ledger** — không biến CIO thành "system event dump".

| Mục tiêu | Trạng thái hiện tại | Hành động đề xuất |
|----------|---------------------|-------------------|
| Order vào cio_events | ❌ Không có | Thêm `InjectOrderEvent` + hook `handleCrmDataChange` |
| Conversation/Message | ✅ Đã có | Giữ nguyên, bổ sung schema chuẩn |
| Schema CioEvent chuẩn | ⚠️ Thiếu eventCategory, links, source | Nâng cấp model theo Unified Data Contract |
| Touchpoint triggered | ⚠️ Chưa ghi | Ghi `touchpoint_triggered` khi ExecuteTouchpoint |
| CIX enqueue từ CIO | ⚠️ Chưa có | Hook OnConversationUpsert/OnMessageUpsert → CIX enqueue |
| Resolve customerUid | ⚠️ Một phần | Worker reconciliation (tùy chọn Phase 2) |

---

## 1. Định Nghĩa Eventbase

**Eventbase** = `cio_events` collection — nơi ghi nhận **có chọn lọc** các sự kiện thuộc **interaction ledger** (xem **CIO-T1–T3** ở §1.1):

- **CIO-T1 — Tương tác trực tiếp** — message, conversation, click, view, session, touchpoint
- **CIO-T2 — Business gắn khách** — order_created, order_updated, payment_success (gắn với customer/conversation)
- **CIO-T3 — Nội bộ kề timeline** — agent_assigned, delivery_failed, handoff_completed, … (chỉ khi đủ điều kiện lọc §1.1)

**Nguyên tắc:** CIO chỉ **log**, không xử lý logic. CIX, Customer Intelligence, Decision Engine **đọc** từ eventbase để build context.

---

## 1.1 Quy Tắc Lọc — Chỉ Lưu Có Chọn Lọc

**CIO không phải nơi lưu mọi event nội bộ.** Chỉ lưu event là **một phần của interaction ledger**.

### Phân event thành ba tầng lọc CIO (CIO-T1 / CIO-T2 / CIO-T3)

| Mã | Mô tả | Ví dụ | Lưu trong CIO? |
|----|-------|-------|----------------|
| **CIO-T1** | Tương tác trực tiếp — khách chạm, hệ thống gửi, delivery result | `message_received`, `message_sent`, `touchpoint_triggered`, `conversation_updated` | ✅ Có |
| **CIO-T2** | Business gắn khách / hội thoại | `order_created`, `order_updated`, `payment_success` | ✅ Có |
| **CIO-T3** | Nội bộ nhưng ảnh hưởng timeline/audit tương tác (có điều kiện §1.1) | `agent_assigned`, `handoff_completed`, `delivery_failed`, `session_closed_by_timeout`, `conversation_viewed`, `note_added_to_thread` | ✅ Có (khi đủ 2 câu hỏi) |
| *(loại trừ)* | **Tính toán domain thuần** — không phục vụ trace interaction | `customer.ltv_recomputed`, `ads.creative_scored`, `rule.batch_evaluated`, `dashboard.snapshot_built` | ❌ Không |

### Hai Câu Hỏi Trước Khi Ingest Internal Event

1. **Event này có gắn được với `customer/session/thread/conversation` không?**
2. **Nếu không lưu nó trong CIO, sau này có mất khả năng audit interaction không?**

- **Có / Có** → nên vào CIO
- **Không / Không** → để domain khác giữ (Event Backbone chung, domain store riêng)

### Ví Dụ Chốt

| Event | Lưu CIO? | Lý do |
|-------|----------|-------|
| Nhân viên xem cuộc chat | ✅ | Gắn thread, cần audit |
| Hệ thống retry gửi tin | ✅ | Ảnh hưởng delivery trace |
| Hệ thống tính lại LTV | ❌ | Pure domain CRM |
| Ads engine chấm điểm adset | ❌ | Pure domain Ads |

### Schema Gợi Ý Cho Phân Loại

| Field | Giá trị | Mô tả |
|-------|---------|-------|
| `eventScope` | `customer_external` \| `operator_internal` \| `system_operational` | Nguồn event |
| `isInteractionRelevant` | `true` \| `false` | Chỉ ingest khi `true` |

---

## 1.2 Ref Model — Pointer + Snapshot Nhỏ, Không Store Full Result

**CIO không sở hữu kết quả nghiệp vụ, nhưng phải giữ đủ reference để nối interaction với nguyên nhân và outcome.**

### Nguyên Tắc Chốt

| Làm | Không làm |
|-----|-----------|
| Lưu **pointer/link** tới entity gốc | Copy full order, full customer, full execution payload |
| Lưu **statusSnapshot** nhỏ (vd: `deliveryStatus: "sent"`) | Nhét 200 field từ domain gốc |
| Ref đủ để **trace nhân quả** (conversation → order, decision → execution) | Log độc lập không nối được outcome |

**Source of truth** của result luôn ở **domain gốc**. CIO chỉ giữ khóa nối.

### Mô Hình 3 Tầng Ref

| Tầng | Field | Mô tả | Ví dụ |
|------|-------|-------|-------|
| **1. CausedBy** | `causedBy` | Event này xảy ra vì cái gì? | `{ module: "execution_engine", entityType: "action_execution", entityUid: "exe_123" }` |
| **2. Links** | `links` | Event gắn với đối tượng nào? | `conversation`, `customer`, `session` (đã có) |
| **3. ResultRefs** | `resultRefs` | Event dẫn tới outcome nào? | `orderUid`, `deliveryUid`, `decisionUid` |

### Khi Nào Cần Ref Sâu?

| Trường hợp | Ref cần có |
|------------|------------|
| Event là **hệ quả** của action/domain flow (gửi tin bởi Execution, assign bởi workflow) | `causedBy`, `resultRefs` |
| Cần **audit** "nói gì dẫn đến chuyện gì" (conversation → order, message → payment) | `resultRefs.orderUid`, `links.conversation` |
| Cần **join** cho learning loop (Decision Brain, CIX) | `causedBy.entityUid`, `resultRefs.decisionUid` |
| Event **vận hành nội bộ** nhẹ (`conversation_viewed`, `typing_started`) | Chỉ `links.conversationUid`, `actor`, `timestamp` |

**Quy tắc:** Event càng gần boundary interaction → ref càng nhẹ. Event càng là cầu nối sang domain khác → ref càng phải rõ.

### Schema Tối Thiểu Mỗi CIO Event

- `eventUid`, `eventType`, `occurredAt` (eventAt)
- `sourceModule` (domain), `entityType`, `entityUid` (sourceRef)
- `links`: conversationUid, customerUid, sessionUid
- `causedBy`: module, entityType, entityUid (khi có)
- `resultRefs`: orderUid, deliveryUid, decisionUid, … (khi có)
- `statusSnapshot`: snapshot nhỏ (vd: `{ deliveryStatus: "sent" }`), không full result

### Câu Chốt

**CIO không cần sở hữu kết quả của module gốc, nhưng phải giữ đủ reference để nối interaction với nguyên nhân và outcome.**

- Không cần mang cả order vào CIO — nhưng nên có `orderUid` nếu interaction dẫn tới order
- Không cần mang full execution record vào CIO — nhưng nên có `executionUid` / `deliveryUid` để trace

---

## 2. Luồng Hiện Tại — Gaps

### 2.1 Sơ Đồ Luồng Hiện Tại

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│ pc_pos_orders Insert/Update (sync, webhook)                                       │
│   → BaseServiceMongoImpl → EmitDataChanged                                         │
│       ├─► handleReportDataChange → MarkDirty (report)                             │
│       └─► handleCrmDataChange → EnqueueCrmIngest → crm_pending_ingest               │
│               └─► crm_ingest_worker → IngestOrderTouchpoint (CRM)                 │
│                                                                                   │
│   CIO: KHÔNG NHẬN ❌                                                              │
└─────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────┐
│ fb_conversations / fb_messages sync                                               │
│   → IngestConversationTouchpoint (CRM) → OnConversationUpsert → cio_events ✅     │
│   → fb_message handler → OnMessageUpsert → cio_events ✅                         │
└─────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────┐
│ CIO ExecuteTouchpoint / Plan Executor                                              │
│   → notifytrigger.TriggerProgrammatic (cio_touchpoint)                             │
│   → CIO: KHÔNG GHI touchpoint_triggered vào cio_events ⚠️                         │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### 2.2 Bảng Gaps Chi Tiết

| Luồng | Nguồn | Đích | Hiện tại | Cần sửa |
|-------|-------|------|----------|---------|
| Order | pc_pos_orders | cio_events | ❌ Không | Thêm InjectOrderEvent |
| Conversation | fb_conversations | cio_events | ✅ OnConversationUpsert | Bổ sung eventCategory, links |
| Message | fb_messages | cio_events | ✅ OnMessageUpsert | Bổ sung eventCategory, links |
| Touchpoint | ExecuteTouchpoint | cio_events | ❌ Không | Ghi touchpoint_triggered |
| CIX | cio_events | cix_pending_analysis | ❌ Chưa | Hook enqueue (Phase 2) |

---

## 3. Phương Án Nâng Cấp

### 3.1 Phase 1 — Order Inject (Ưu tiên cao)

#### 3.1.1 Thêm `CioIngestionService.InjectOrderEvent`

**File:** `api/internal/api/cio/ingestion/ingestion.go`

```go
// InjectOrderEvent ghi event order vào cio_events khi pc_pos_orders được sync/upsert.
// eventType: order_created | order_updated | order_cancelled (suy từ operation + status).
func (s *Service) InjectOrderEvent(ctx context.Context, orderDoc interface{}, operation string) error
```

**Logic:**
- Parse order doc → lấy customerId, orderId, amount, channel (pos), source (pancake)
- Xác định eventType: `order_created` (OpInsert), `order_updated` (OpUpdate), `order_cancelled` (nếu status cancelled)
- Tạo CioEvent với:
  - `eventType`, `eventCategory: "business"`
  - `channel: "pos"`, `source: "pancake"`
  - `customerId` (từ posData), `customerUid` để trống (resolve sau)
  - `sourceRef: { refType: "pos_order", refId: orderId }`
  - `payload`: snapshot nhẹ (amount, items, orderId)
  - `links.order`: externalRefs khi chưa có order uid

#### 3.1.2 Hook handleCrmDataChange

**File:** `api/internal/api/crm/service/service.crm.hooks.go` (hoặc package crmvc tương ứng)

Khi `e.CollectionName == global.MongoDB_ColNames.PcPosOrders`:
- Sau `EnqueueCrmIngest` (hoặc song song, không block):
- Gọi `cioingestion.NewService()` → `InjectOrderEvent(ctx, e.Document, e.Operation)`
- Fire-and-forget: `go cioSvc.InjectOrderEvent(...)` để không block CRM flow

**Lưu ý:** Không thay thế luồng CRM. CRM vẫn EnqueueCrmIngest → IngestOrderTouchpoint. CIO nhận thêm path song song.

#### 3.1.3 Schema Event Order (theo CIO_BOUNDARY_AND_EVENTS)

Order thuộc **CIO-T2 (business gắn khách)** — gắn customer, cần audit timeline (gắn conversation với outcome). *(Không nhầm với **bản ghi chạy intel** hay **CIO-T1**.)*

```json
{
  "eventType": "order_created",
  "eventCategory": "business",
  "eventScope": "customer_external",
  "domain": "pos",
  "tags": ["order", "pos", "commerce"],
  "channel": "pos",
  "source": "pancake",
  "ownerOrganizationId": ObjectId,
  "customerId": "pos_customer_uuid",
  "customerUid": "",
  "conversationId": "",
  "payload": {
    "orderId": 12345,
    "amount": 3200000,
    "items": 2
  },
  "sourceRef": { "refType": "pos_order", "refId": "12345" },
  "causedBy": { "module": "pos", "entityType": "pos_order", "entityUid": "ord_xxx" },
  "resultRefs": { "orderUid": "ord_xxx" },
  "statusSnapshot": { "orderStatus": "created" },
  "links": {
    "customer": { "uid": null, "externalRefs": [{ "source": "pos", "id": "customer_uuid" }], "status": "pending_resolution" },
    "order": { "uid": "ord_xxx", "externalRefs": [{ "source": "pos", "id": "12345" }], "status": "resolved" }
  },
  "eventAt": 1710003600000,
  "createdAt": 1710003601000,
  "traceId": "trace_xxx",
  "correlationId": ""
}
```

**Ghi chú:** `causedBy.entityUid`, `resultRefs.orderUid` lấy từ `order.uid` (pc_pos_orders) khi có. Nếu order chưa có uid → để trống, dùng `links.order.externalRefs` + `sourceRef`.

---

### 3.2 Phase 1 — Nâng Cấp Model CioEvent

**File:** `api/internal/api/cio/models/model.cio.event.go`

Bổ sung fields theo [CIO_BOUNDARY_AND_EVENTS](./CIO_BOUNDARY_AND_EVENTS.md) và [identity-links-model](../../docs-shared/architecture/data-contract/identity-links-model.md):

| Field | Loại | Mô tả |
|-------|------|-------|
| `EventCategory` | string | `interaction` \| `business` — phân biệt CIO-T1 vs CIO-T2 (schema; còn CIO-T3 dựa `eventScope` + bảng §1.1) |
| `EventScope` | string | `customer_external` \| `operator_internal` \| `system_operational` — nguồn event (§1.1) |
| `Domain` | string | Module/domain: `cio` \| `crm` \| `pos` \| `delivery` \| `ads` \| … — phục vụ thống kê theo domain. Index single. |
| `Tags` | []string | Tag linh hoạt: `["re_engage","zalo"]`, `["inbound","messenger"]` — phục vụ filter, drill-down, dashboard. Index multi-key. |
| `Source` | string | Nguồn: pancake, shopify, webhook, … |
| `Links` | map[string]LinkItem | Đã có — conversation, customer, session, order (§1.2 Tầng 2) |
| `CausedBy` | CausedByRef | Nguyên nhân gây event — module, entityType, entityUid (§1.2 Tầng 1) |
| `ResultRefs` | map[string]string | Outcome UID — orderUid, deliveryUid, decisionUid, executionUid (§1.2 Tầng 3) |
| `StatusSnapshot` | map[string]interface{} | Snapshot nhỏ — `{ deliveryStatus: "sent" }`, không full result |
| `Uid` | string | evt_xxx — generate khi Insert nếu chưa có |

**CausedByRef** (struct):

```go
type CausedByRef struct {
    Module     string `json:"module" bson:"module"`         // execution_engine | pos | cio | delivery | ...
    EntityType string `json:"entityType" bson:"entityType"` // action_execution | pos_order | touchpoint_plan | ...
    EntityUid  string `json:"entityUid" bson:"entityUid"`   // exe_123 | ord_456 | ...
}
```

**Quy ước:** Mọi event ingest vào CIO mặc định `isInteractionRelevant = true` (theo §1.1 — chỉ ingest khi relevant). Không cần field riêng nếu logic lọc đã áp dụng trước khi gọi ingestion.

**Domain & Tags — phục vụ thống kê:**
- `domain`: module phát sinh event — `cio`, `crm`, `pos`, `delivery`, `ads`, `fb`, … → aggregate theo domain (vd: "bao nhiêu event từ pos?", "event delivery chiếm %?")
- `tags`: mảng string linh hoạt — `["re_engage","zalo"]`, `["inbound","messenger","vip"]` → filter, drill-down, dashboard (vd: "event có tag re_engage", "touchpoint qua zalo")

Cập nhật `OnConversationUpsert`, `OnMessageUpsert`:
- Set `eventCategory: "interaction"`, `eventScope: "customer_external"`, `domain: "fb"` (hoặc `"zalo"` tùy kênh)
- Set `source: "pancake"` (Messenger) hoặc `"zalo"` tùy kênh
- Set `tags: ["inbound", channel]` (vd: `["inbound","messenger"]`)
- Generate `uid` (evt_xxx) nếu chưa có

---

### 3.3 Phase 1 — Touchpoint Triggered

**File:** `api/internal/api/cio/service/service.cio.touchpoint.go` (ExecuteTouchpoint) và `service.cio.plan_executor.go` (runActionStepWithExec)

Sau khi gọi `notifytrigger.TriggerProgrammatic` thành công:
- Gọi `cioingestion.NewService()` → `InjectTouchpointTriggered(ctx, ...)`

**File:** `api/internal/api/cio/ingestion/ingestion.go`

```go
// InjectTouchpointTriggered ghi event touchpoint_triggered khi touchpoint được gửi.
func (s *Service) InjectTouchpointTriggered(ctx context.Context, unifiedId, goalCode, channel, planId string, orgID primitive.ObjectID, templateId string) error
```

Schema:
- `eventType: "touchpoint_triggered"`
- `eventCategory: "interaction"`, `eventScope: "customer_external"` (hoặc `system_operational` nếu từ automation)
- `domain: "cio"`, `tags: [goalCode, channel]` (vd: `["re_engage","zalo"]`)
- `causedBy: { module: "cio", entityType: "touchpoint_plan", entityUid: planId }`
- `resultRefs: { deliveryUid: "dlv_xxx" }` — khi Execution/Delivery trả về (có thể cập nhật async)
- `statusSnapshot: { deliveryStatus: "queued" }` — snapshot nhỏ, không full execution payload
- `channel`, `customerUid` (unifiedId), `payload`: goalCode, planId, templateId

---

### 3.4 Phase 2 — CIX Enqueue (Tùy chọn)

Khi ghi cio_event (OnConversationUpsert, OnMessageUpsert):
- Fire-and-forget: `CixService.EnqueueAnalysis(ctx, event)` nếu CIX module đã sẵn sàng
- Tham chiếu: [PHUONG_AN_TRIEN_KHAI_CIX.md](./PHUONG_AN_TRIEN_KHAI_CIX.md) § Enqueue

---

### 3.5 Phase 2 — Event nội bộ kề timeline (CIO-T3 — tùy chọn)

Khi Execution Gateway, Delivery, Agent có event ảnh hưởng interaction timeline — inject vào CIO (thuộc **CIO-T3** trong §1.1, sau khi qua 2 câu hỏi lọc):

| eventType | eventScope | domain | tags ví dụ | Mô tả |
|-----------|------------|--------|-------------|-------|
| `agent_assigned` | operator_internal | cio | ["assignment","agent"] | Phân công agent cho conversation |
| `handoff_completed` | operator_internal | cio | ["handoff"] | Handoff hoàn tất |
| `delivery_failed` | system_operational | delivery | ["delivery","failed"] | Gửi tin thất bại, cần retry |
| `delivery_retry_started` | system_operational | delivery | ["delivery","retry"] | Bắt đầu retry |
| `session_closed_by_timeout` | system_operational | cio | ["session","timeout"] | Session đóng do timeout |
| `conversation_viewed` | operator_internal | cio | ["view","operator"] | Nhân viên xem cuộc chat |
| `note_added_to_thread` | operator_internal | cio | ["note","thread"] | Thêm ghi chú vào thread |

**Điều kiện:** Phải gắn với customer/session/thread/conversation. Chỉ inject khi có khả năng mất audit nếu không lưu.

**Ref cho CIO-T3:** Event vận hành nội bộ nhẹ → ref nhẹ. Chỉ cần `links.conversationUid`, `actor`, `timestamp`. Không cần `causedBy`/`resultRefs` sâu. Event từ Delivery (`delivery_failed`) → cần `causedBy.executionUid`, `resultRefs.deliveryUid`.

---

### 3.6 Phase 2 — Worker Resolve customerUid (Tùy chọn)

Worker chạy định kỳ:
- Query cio_events có `links.customer.status = "pending_resolution"` và `customerId != ""`
- Resolve customerId → customerUid (crm_customers)
- Update `links.customer.uid`, `links.customer.status = "resolved"`, `customerUid`

---

## 4. Luồng Sau Khi Nâng Cấp

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│ pc_pos_orders Insert/Update                                                      │
│   → EmitDataChanged                                                              │
│       ├─► handleCrmDataChange → EnqueueCrmIngest → crm_ingest_worker (GIỮ NGUYÊN)│
│       └─► handleCrmDataChange → CioIngestionService.InjectOrderEvent → cio_events ✅
└─────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────┐
│ fb_conversations / fb_messages                                                    │
│   → OnConversationUpsert / OnMessageUpsert → cio_events (eventCategory, source) ✅
│   → (Phase 2) CIX.EnqueueAnalysis                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────┐
│ CIO ExecuteTouchpoint / Plan Executor                                             │
│   → notifytrigger.TriggerProgrammatic                                             │
│   → InjectTouchpointTriggered → cio_events ✅                                     │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## 5. Thứ Tự Triển Khai

| # | Hành động | Effort | Ưu tiên | Phase |
|---|-----------|--------|---------|-------|
| 1 | Thêm `InjectOrderEvent` trong cio/ingestion | 1–2 ngày | Cao | 1 |
| 2 | Hook handleCrmDataChange (pc_pos_orders) → InjectOrderEvent | 0.5 ngày | Cao | 1 |
| 3 | Nâng cấp CioEvent model (eventCategory, eventScope, domain, tags, causedBy, resultRefs, statusSnapshot, source, uid) | 0.5–1 ngày | Cao | 1 |
| 4 | Cập nhật OnConversationUpsert, OnMessageUpsert với schema mới | 0.5 ngày | Cao | 1 |
| 5 | Thêm `InjectTouchpointTriggered` + gọi từ ExecuteTouchpoint | 1 ngày | Trung bình | 1 |
| 6 | Worker resolve customerUid cho cio_events | 1 ngày | Trung bình | 2 |
| 7 | CIX enqueue từ CIO event | 0.5–1 ngày | Thấp | 2 |
| 8 | Internal events CIO-T3 (delivery_failed, agent_assigned, …) | 1–2 ngày | Thấp | 2 |

---

## 6. Rủi Ro và Giảm Thiểu

| Rủi ro | Mức độ | Giảm thiểu |
|--------|--------|------------|
| InjectOrderEvent chậm, block handleCrmDataChange | Trung bình | Fire-and-forget `go cioSvc.InjectOrderEvent(...)` |
| Order doc schema thay đổi | Thấp | Parse defensive, fallback giá trị mặc định |
| Duplicate event (cùng orderId) | Trung bình | Idempotency: check sourceRef trước khi insert (tùy chọn) |
| CioEvent volume tăng | Thấp | Index eventAt, ownerOrganizationId, customerId; TTL nếu cần |

---

## 7. Files Cần Sửa/Tạo

| File | Hành động |
|------|-----------|
| `api/internal/api/cio/ingestion/ingestion.go` | Thêm InjectOrderEvent, InjectTouchpointTriggered |
| `api/internal/api/cio/models/model.cio.event.go` | Thêm EventCategory, EventScope, Domain, Tags, CausedByRef, ResultRefs, StatusSnapshot, Source; đảm bảo Links, Uid |
| `api/internal/api/crm/service/service.crm.hooks.go` | Hook pc_pos_orders → InjectOrderEvent |
| `api/internal/api/cio/service/service.cio.touchpoint.go` | Gọi InjectTouchpointTriggered sau ExecuteTouchpoint |
| `api/internal/api/cio/service/service.cio.plan_executor.go` | Gọi InjectTouchpointTriggered sau send_touchpoint |
| `api/internal/api/pc/models/model.pc.pos.order.go` | Tham chiếu schema posData (customer_id, order_id, amount) |

---

## 8. Changelog

- 2026-03-18: Tổng hợp thay đổi → TOM_TAT_THAY_DOI_CIO_EVENTBASE.md; cập nhật vision docs-shared (04 - cio-customer-interaction-hub §6–8, unified-data-contract §2.5)
- 2026-03-18: Bổ sung §1.2 Ref Model — causedBy, resultRefs, statusSnapshot; pointer + snapshot nhỏ, không store full result; CausedByRef struct; cập nhật schema order/touchpoint với ref 3 tầng
- 2026-03-18: Bổ sung domain, tags cho CioEvent — phục vụ thống kê (aggregate theo domain, filter/drill-down theo tags)
- 2026-03-18: Bổ sung §1.1 Quy tắc lọc — chỉ lưu có chọn lọc (CIO-T1/T2/T3 + loại trừ compute thuần domain; 2 câu hỏi, eventScope, ví dụ chốt); §3.5 Internal events CIO-T3; eventScope trong model và schema order
- 2026-04-07: Đổi danh pháp **CIO-T1/T2/T3**; bỏ nhãn A/B/C dễ trùng với intel / pha ingress — tham chiếu [KHUNG_KHUON_MODULE_INTELLIGENCE.md](./KHUNG_KHUON_MODULE_INTELLIGENCE.md) mục 0
- 2026-03-18: Tạo tài liệu đề xuất nâng cấp CIO eventbase
