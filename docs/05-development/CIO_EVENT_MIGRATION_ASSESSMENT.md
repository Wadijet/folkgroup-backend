# Đánh Giá Luồng Hiện Tại vs Event-Based CIO

**Ngày:** 2026-03-18  
**Tham chiếu:** [CIO_BOUNDARY_AND_EVENTS.md](./CIO_BOUNDARY_AND_EVENTS.md), [THIET_KE_MODULE_CIO.md](./THIET_KE_MODULE_CIO.md)

---

## 1. Luồng Hiện Tại

### 1.1 Order (pc_pos_orders)

```
pc_pos_orders Insert/Update (sync, webhook)
    │
    ├─► events.EmitDataChanged
    │       │
    │       ├─► AI Decision queue (datachanged) → applyDatachangedSideEffects → RecordReportTouchFromDataChange → Redis `ff:rt:*` → worker report_redis_touch_flush → MarkDirty
    │       └─► (cùng consumer) CRM ingest / … → EnqueueCrmIngest → crm_pending_ingest
    │                   │
    │                   └─► crm_ingest_worker → IngestOrderTouchpoint (CRM)
    │
    └─► CIO: KHÔNG nhận
```

**Kết luận:** Order **không** đi qua cio_events.

### 1.2 Conversation (fb_conversations, fb_messages)

```
fb_conversations sync → OnConversationUpsert → cio_events (conversation_updated)
fb_messages sync → OnMessageUpsert → cio_events (message_updated)
```

**Kết luận:** Conversation **đã** đi qua cio_events.

### 1.3 Customer (pc_pos_customers, fb_customers)

```
pc_pos_customers / fb_customers Insert/Update
    │
    └─► handleCrmDataChange → EnqueueCrmIngest → crm_pending_ingest
            │
            └─► crm_ingest_worker → MergeFromPosCustomer / MergeFromFbCustomer (CRM)
```

**Kết luận:** Customer **không** đi qua cio_events. CRM là owner.

---

## 2. Có Cần Sửa Không?

| Luồng | Hiện tại | Cần sửa? | Lý do |
|-------|----------|----------|-------|
| **Order** | pc_pos_orders → crm_pending_ingest → CRM | **Có — THÊM** | Inject order_created/order_updated vào cio_events để: timeline thống nhất, CIX context, gắn conversation với outcome. **Không thay** luồng CRM. |
| **Conversation** | fb_* → cio_events | **Không** | Đã đúng. |
| **Customer** | pos/fb_customers → crm_pending_ingest → CRM | **Không bắt buộc** | Customer là domain của CRM. CIO không sở hữu. Event customer_merged có thể thêm sau nếu cần timeline. |

---

## 3. Điều Chỉnh Cần Làm

### 3.1 Order — Thêm Inject Vào cio_events

**Cách 1: Hook trong handleCrmDataChange**

Khi `collectionName == "pc_pos_orders"` và EnqueueCrmIngest xong → gọi `CioIngestionService.InjectOrderEvent(ctx, doc, operation)`.

**Cách 2: Handler riêng OnDataChanged**

Đăng ký handler mới: khi pc_pos_orders → gọi CioIngestionService.InjectOrderEvent.

**Cách 3: Trong PcPosOrderService (override InsertOne/UpdateOne)**

Sau khi base Insert/Update xong → gọi InjectOrderEvent. Tránh vì trùng logic với events.

**Đề xuất:** Cách 1 hoặc 2 — dùng sự kiện có sẵn, không sửa PcPosOrderService.

### 3.2 Schema Event Order Trong cio_events

```json
{
  "eventType": "order_created",
  "eventCategory": "business",
  "channel": "pos",
  "source": "pancake",
  "ownerOrganizationId": ObjectId,
  "customerId": "pos_customer_uuid",
  "conversationId": "",
  "payload": {
    "orderId": 12345,
    "amount": 3200000,
    "items": 2
  },
  "sourceRef": { "refType": "pos_order", "refId": "12345" },
  "eventAt": 1710003600000,
  "createdAt": 1710003601000
}
```

- `eventType`: `order_created` | `order_updated` | `order_cancelled` (suy từ operation + status)
- `customerId`: từ order doc (posData.customer_id hoặc field tương ứng)
- `payload`: snapshot nhẹ — không full order

### 3.3 Resolve customerUid

CIO event có thể chưa có customerUid lúc ghi (resolve async). Có thể:
- Ghi trước với customerId (từ kênh), để trống customerUid
- Worker reconciliation sau resolve và cập nhật links.customer.uid
- Hoặc CIX/CI đọc customerId và resolve khi cần

---

## 4. Luồng Sau Khi Điều Chỉnh

```
pc_pos_orders Insert/Update
    │
    ├─► handleCrmDataChange → crm_pending_ingest → IngestOrderTouchpoint (GIỮ NGUYÊN)
    │
    └─► handleCrmDataChange (hoặc handler mới) → CioIngestionService.InjectOrderEvent → cio_events
```

**Nguyên tắc:** Thêm path, không bỏ path cũ. CRM vẫn là nơi aggregate order metrics.

---

## 5. Customer — Có Cần Inject?

| Event | Cần? | Ghi chú |
|-------|------|---------|
| customer_merged | Tùy chọn | Timeline "khách A merge với B" — có thể thêm Phase 2 |
| customer_created | Tùy chọn | Ít giá trị cho CIO — CIO quan tâm interaction |
| customer_updated | Không | CRM internal — tránh noise |

**Đề xuất:** Chưa cần. Ưu tiên Order trước.

---

## 6. Tóm Tắt Hành Động

| # | Hành động | Effort | Ưu tiên |
|---|-----------|--------|---------|
| 1 | Thêm `CioIngestionService.InjectOrderEvent` | 1–2 ngày | Cao |
| 2 | Hook handleCrmDataChange (pc_pos_orders) → gọi InjectOrderEvent | 0.5 ngày | Cao |
| 3 | (Tùy chọn) Worker resolve customerUid cho cio_events order_* | 1 ngày | Trung bình |
| 4 | Customer event (customer_merged) | — | Thấp, sau |

---

## 7. Rủi Ro Nếu Không Sửa

| Rủi ro | Mức độ |
|--------|--------|
| CIX thiếu context "khách vừa đặt đơn" | Trung bình — CIX có thể đọc từ crm_customers (lastOrderAt, orderCount) |
| Timeline CIO không đủ | Trung bình — khó query "mọi thứ xảy ra quanh khách" từ 1 nơi |
| Gắn conversation với outcome | Trung bình — cần join fb_conversations + pc_pos_orders qua posData.conversation_id |

**Kết luận:** Nên thêm order inject — effort nhỏ, lợi ích kiến trúc dài hạn.

---

## Changelog

- 2026-03-18: Tạo tài liệu đánh giá luồng vs event-based CIO
