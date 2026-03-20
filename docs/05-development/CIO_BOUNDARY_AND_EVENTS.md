# CIO Boundary & Unified Event Model — Ràng Buộc Kiến Trúc

**Ngày:** 2026-03-18  
**Tham chiếu:** [THIET_KE_MODULE_CIO.md](./THIET_KE_MODULE_CIO.md)

---

## 0. Rule Chốt (Rất Quan Trọng)

> **CIO = timeline của mọi thứ xảy ra quanh khách hàng**
>
> **Order = 1 event trong timeline đó, không phải logic của CIO**

**CIO không sở hữu domain nào:**
- Không sở hữu order
- Không sở hữu customer
- Không sở hữu ads

**CIO chỉ:** ghi nhận mọi sự kiện liên quan đến customer interaction.

---

## 1. CIO Với Order Event

### 1.1 Có Thể Theo Dõi Order Event

✅ CIO **có thể** nhận các event:
- `order_created`
- `order_updated`
- `payment_success`
- `order_cancelled`
- `delivery_failed`

**Nhưng** — đây là **interaction signal**, không phải business logic.

### 1.2 CIO Lưu Gì Về Order?

Chỉ lưu dạng **event log**:

```json
{
  "eventType": "order_created",
  "customerUid": "cust_123",
  "orderUid": "ord_456",
  "channel": "pos",
  "source": "shopify",
  "eventAt": 1710003600000,
  "payload": {
    "amount": 3200000,
    "items": 2
  }
}
```

- Log lại sự kiện
- Gắn với customer / conversation
- **Không xử lý gì thêm**

### 1.3 Không Được Làm Gì Trong CIO

❌ Không:
- Tính LTV
- Phân loại khách VIP
- Đánh giá đơn tốt/xấu
- Quyết định upsell/cross-sell
- Phân tích hành vi mua

→ Thuộc **Customer Intelligence** hoặc **Decision Engine**.

---

## 2. Vì Sao CIO Cần Order Event?

| Lý do | Mô tả |
|-------|-------|
| **(1) Gắn conversation với outcome** | Chat → chốt đơn → order_created → hiểu conversation có hiệu quả không |
| **(2) Feed cho CIX** | Khách vừa đặt đơn → intent khác; khách vừa hủy → sentiment khác |
| **(3) Feed Customer Intelligence** | CIO phát event → CI build LTV, repeat rate, segment |

---

## 3. Unified Event Model — 2 Loại Event

### (A) Interaction Event (gốc)

| eventType | Mô tả | Nguồn |
|-----------|-------|-------|
| `message_sent` | Tin gửi đi | Messenger, Zalo, … |
| `message_received` | Tin nhận | Messenger, Zalo, … |
| `conversation_updated` | Conversation thay đổi | fb_conversations sync |
| `message_updated` | Message metadata thay đổi | fb_messages sync |
| `click` | Click link/CTA | Web, email |
| `view` | Xem trang/sản phẩm | Web |
| `session_start` | Bắt đầu session | Webhook |
| `session_end` | Kết thúc session | Webhook |
| `touchpoint_triggered` | Touchpoint được kích hoạt | CIO PlanTouchpoint |

### (B) External Business Event (inject vào)

| eventType | Mô tả | Nguồn inject |
|-----------|-------|---------------|
| `order_created` | Đơn mới | POS, Shopify, Pancake |
| `order_updated` | Đơn cập nhật | POS, Shopify |
| `order_cancelled` | Đơn hủy | POS, Shopify |
| `payment_success` | Thanh toán thành công | Payment gateway |
| `shipment_delivered` | Giao hàng xong | Logistics |

**Tất cả** đều vào **event stream thống nhất** (`cio_events`).

---

## 4. Schema Chuẩn Cho CIO Event (Unified)

```json
{
  "_id": ObjectId,
  "uid": "evt_xxx",
  "eventType": "order_created",
  "eventCategory": "interaction",
  "channel": "pos",
  "source": "shopify",
  "ownerOrganizationId": ObjectId,
  "customerUid": "cust_123",
  "conversationId": "conv_456",
  "sessionUid": "sess_789",
  "links": {
    "customer": { "uid": "cust_123", "status": "resolved" },
    "order": { "uid": "ord_456", "externalRefs": [{ "source": "pos", "id": "order_123" }], "status": "resolved" }
  },
  "payload": {
    "amount": 3200000,
    "items": 2
  },
  "sourceRef": { "refType": "pos_order", "refId": "order_123" },
  "eventAt": 1710003600000,
  "createdAt": 1710003601000
}
```

| Field | Bắt buộc | Mô tả |
|-------|----------|-------|
| eventType | ✅ | Loại sự kiện |
| eventCategory | ⚠️ | `interaction` \| `business` — phân biệt (A) vs (B) |
| channel | ✅ | messenger \| zalo \| pos \| web \| … |
| source | ⚠️ | Nguồn inject: shopify, pancake, pos, … |
| customerUid | ⚠️ | Khi resolve được |
| links | ⚠️ | Link tới customer, order, conversation |
| payload | ⚠️ | Snapshot nhẹ — không full domain object |
| eventAt | ✅ | Thời gian sự kiện thực tế |

---

## 5. Luồng Chuẩn Order ↔ CIO ↔ Customer Intelligence

```
┌─────────────────┐     order_created      ┌─────────────────┐
│ POS / Shopify   │ ──────────────────────►│ CIO (cio_events)│
│ Order System   │     inject event        │ Log only        │
└─────────────────┘                        └────────┬────────┘
                                                    │
                                                    │ event stream
                                                    ▼
┌─────────────────┐                        ┌─────────────────┐
│ Customer        │ ◄───────────────────── │ CIX             │
│ Intelligence    │   cix_signal_update    │ (context)       │
│ (build LTV,     │                        └─────────────────┘
│  segment)       │
└────────┬────────┘
         │
         │ recommendation
         ▼
┌─────────────────┐
│ Decision Engine │
└─────────────────┘
```

**Map chuẩn:**

| Hệ thống | Vai trò với Order | Giao với CIO |
|----------|-------------------|--------------|
| **POS / Shopify** | Source of truth đơn | Inject `order_created`, `order_updated`, `order_cancelled` vào cio_events |
| **CIO** | Log event, gắn customer | Không xử lý logic đơn |
| **Customer Intelligence** | Aggregate orders → LTV, segment | Đọc cio_events (order_*) hoặc sync trực tiếp từ POS |
| **CIX** | Hiểu conversation theo context (vừa đặt đơn, vừa hủy) | Đọc cio_events + cix_conversations |

---

## 6. Nếu Làm Sai Sẽ Bị Gì?

| Sai lầm | Hậu quả |
|---------|---------|
| CIO xử lý order logic | Trùng logic với Customer Intelligence |
| CIO tính LTV, phân loại | Vỡ Data Sovereignty |
| CIO quyết định upsell | Khó scale multi-channel |
| Logic rải rác | Khó debug — không biết lỗi ở đâu |

---

## 7. Chốt 1 Câu

**CIO = timeline của mọi thứ xảy ra quanh khách hàng. Order = 1 event trong timeline đó, không phải logic của CIO.**

---

## Changelog

- 2026-03-18: Tạo tài liệu CIO boundary & unified event model
