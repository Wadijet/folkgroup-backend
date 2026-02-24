# Hệ Thống Phân Loại Khách Hàng FolkForm — Thiết Kế Đầy Đủ

> Thiết kế production-grade cho hệ thống phân loại khách hàng FolkForm: Customer Journey, 4 trục phân loại, Customer Metrics, Customer Intelligence, và **merge khách hàng FB + POS**.

---

## Mục lục

1. [Tổng quan mục tiêu](#1-tổng-quan-mục-tiêu)
2. [Merge khách hàng FB + POS](#2-merge-khách-hàng-fb--pos)
3. [Kiến trúc 2 lớp phân loại](#3-kiến-trúc-2-lớp-phân-loại)
4. [Chi tiết từng thành phần](#4-chi-tiết-từng-thành-phần)
5. [Customer Metrics](#5-customer-metrics)
6. [Customer Intelligence (nâng cao)](#6-customer-intelligence-nâng-cao)
7. [Customer Notes](#7-customer-notes)
8. [Sales Assignment](#8-sales-assignment)
9. [6 nhóm khách CEO](#9-6-nhóm-khách-ceo)
10. [Visualization Dashboard](#10-visualization-dashboard)
11. [API thiết kế](#11-api-thiết-kế)
12. [Cấu trúc code (thư mục)](#12-cấu-trúc-code-thư-mục)
13. [Database & Collections](#13-database--collections)
14. [Migration & tương thích](#14-migration--tương-thích)
15. [Roadmap triển khai](#15-roadmap-triển-khai)
16. [Customer Activity History](#16-customer-activity-history)
17. [Hooks & Events — Cập nhật dữ liệu customer](#17-hooks--events--cập-nhật-dữ-liệu-customer)

---

## 1. Tổng Quan Mục Tiêu

Hệ thống phân loại khách hàng **không phải để báo cáo**, mà để:

- Biết tài sản khách hàng đang tăng hay giảm
- Biết khách nào cần chăm sóc ngay
- Biết khách nào có tiềm năng trở thành VIP
- Biết khách nào sắp rời bỏ
- Tối đa hóa Lifetime Value

**Luxury brand sống bằng VIP và Repeat customers** — phân loại chính là cách đo tài sản thật của FolkForm.

---

## 2. Merge Khách Hàng FB + POS

### 2.1. Phân tích dữ liệu mẫu

**Chạy script phân tích trước khi triển khai:**

```bash
# Cần có .env với MONGODB_CONNECTION_URI, MONGODB_DBNAME_AUTH
go run scripts/analyze_customer_merge.go
```

Script sẽ output:
- Số lượng fb_customers, pc_pos_customers, fb_conversations, pc_pos_orders
- Format customerId (UUID hay số)
- Phân bố phone numbers
- **Overlap customerId** giữa FB và POS (cùng customerId có trong cả 2 collection)
- Orders: tỷ lệ có customerId vs billPhoneNumber
- Document mẫu để xem cấu trúc

### 2.2. Cấu trúc dữ liệu hiện tại

| Collection | customerId | Phone | Email | Đặc điểm |
|------------|------------|-------|-------|----------|
| **fb_customers** | Pancake customer.id (UUID) | phoneNumbers[] | email | pageId, psid — từ Pancake FB |
| **pc_pos_customers** | POS customer.id (UUID) | phoneNumbers[] | emails[] | shopId, totalSpent, lastOrderAt — từ Pancake POS |
| **fb_conversations** | customer_id (Pancake) | — | — | Link hội thoại → khách FB |
| **pc_pos_orders** | customer.id (POS) | billPhoneNumber | billEmail | Có thể guest (customerId rỗng) |

**Quan sát từ Pancake ecosystem:**
- Pancake có thể dùng **cùng customer pool** cho FB và POS nếu tích hợp. Khi đó `customerId` trùng nhau.
- Nếu FB và POS sync riêng, `customerId` có thể khác namespace (UUID khác nhau).

**Phân tích `order_sources` (để xác định online vs offline):**

Trước khi triển khai Journey, nên chạy script phân tích `posData.order_sources` trong `pc_pos_orders` để biết giá trị thực tế (vd: `facebook`, `pos`, `in_store`, `shopee`, ...). Pancake POS hỗ trợ: Facebook, Zalo, TikTok, Shopee, Lazada, **POS (cửa hàng vật lý)**. Map các giá trị này vào `online` hoặc `offline` để logic Journey chính xác.

### 2.3. Chiến lược merge đề xuất (3 tầng)

#### Tầng 1: customerId trùng (nếu Pancake unified)

- Nếu `fb_customers.customerId` = `pc_pos_customers.customerId` → **cùng một khách**.
- Merge: lấy metrics từ POS (totalSpent, orderCount, lastOrderAt), identity từ cả hai (name, phone ưu tiên từ nguồn đầy đủ hơn).
- **Unified ID** = customerId (dùng luôn vì trùng).

#### Tầng 2: Merge theo posData.fb_id (format pageId_psid) — **Ưu tiên cao**

- `posData.fb_id` có format `pageId_psid` (vd: `157725629736743_26258510603755268`).
- Match với `fb_customers`: `pageId + "_" + psid` = posData.fb_id.
- **Kết quả khảo sát:** ~398 khách POS có thể merge qua fb_id.
- **Unified ID** = ưu tiên dùng POS customerId (có metrics mua hàng).

#### Tầng 3: Merge theo phone (chuẩn hóa)

- Chuẩn hóa SĐT: bỏ khoảng trắng, dấu gạch; `0xxx` → `84xxx`; `+84` → `84`.
- Build map: `normalized_phone → [customerIds]`.
- Khách có cùng normalized phone → merge thành 1 unified customer.
- **Kết quả khảo sát:** ~407 cặp phone trùng, ~416 khách FB link được qua order.billPhone.
- **Unified ID** = chọn customerId có nguồn "chính" (ưu tiên POS vì có metrics mua hàng).

#### Tầng 4: Khách chỉ có ở 1 nguồn

- Chỉ có FB (conversation, chưa mua) → vẫn là customer, Journey = CONVERSATION.
- Chỉ có POS (mua hàng, không có conversation) → Journey = FIRST_PURCHASE/REPEAT/VIP/...
- **Unified ID** = customerId của nguồn đó.

### 2.4. Collection `crm_customers` (mới — bắt buộc tạo)

Collection **mới** lưu khách hàng đã merge, dùng làm nguồn chính cho dashboard và phân loại. Tiền tố `crm_` cho toàn bộ module CRM.

| Field | Type | Mô tả |
|-------|------|-------|
| _id | ObjectID | |
| unifiedId | string | ID thống nhất — ưu tiên POS customerId nếu có, else FB customerId |
| ownerOrganizationId | ObjectID | Phân quyền |
| sourceIds | object | `{ "pos": "uuid-pos", "fb": "uuid-fb" }` — customerId từ từng nguồn |
| primarySource | string | `pos` \| `fb` — nguồn chính (ưu tiên POS nếu có metrics) |
| name | string | Tên (merge từ các nguồn, ưu tiên nguồn đầy đủ hơn) |
| phoneNumbers | []string | Gộp, đã chuẩn hóa (0xxx → 84xxx) |
| emails | []string | Gộp |
| hasConversation | bool | Có trong fb_conversations |
| hasOrder | bool | Có đơn completed |
| orderCountOnline | int | Số đơn qua online (pageId có) |
| orderCountOffline | int | Số đơn tại cửa hàng (pageId không) |
| firstOrderChannel | string | `online` \| `offline` — kênh mua đầu tiên |
| lastOrderChannel | string | `online` \| `offline` — kênh mua gần nhất |
| isOmnichannel | bool | Mua cả online và offline |
| mergeMethod | string | `customer_id` \| `fb_id` \| `phone` \| `single_source` — cách merge |
| mergedAt | int64 | Lần merge/cập nhật cuối |
| createdAt | int64 | |
| updatedAt | int64 | |

**Index:**
- `(ownerOrganizationId, unifiedId)` unique
- `(ownerOrganizationId, sourceIds.pos)` sparse
- `(ownerOrganizationId, sourceIds.fb)` sparse
- `(ownerOrganizationId, phoneNumbers)` multikey

**Cập nhật:** Qua hooks (xem mục 16) hoặc worker/cron định kỳ.

### 2.5. Logic merge (pseudo-code)

```
1. Load tất cả fb_customers, pc_pos_customers (theo ownerOrgId)
2. Build map: customerId -> { source: "fb"|"pos", doc }
3. Build map: normalizedPhone -> [customerIds]
4. For each customerId:
   a. Nếu đã xử lý → skip
   b. Tìm theo customerId trùng (pos.id = fb.id) → merge
   c. Tìm theo phone trùng → merge
   d. Không trùng → 1 unified record
5. Ghi vào crm_customers
```

### 2.6. Query metrics cho unified customer

- **total_spent, order_count, last_order_at**: aggregate từ `pc_pos_orders` theo `customerId` (dùng `sourceIds.pos` hoặc `unifiedId` nếu trùng với pos).
- Nếu khách chỉ có FB: total_spent=0, order_count=0, last_order_at=null.
- **hasConversation**: có record trong `fb_conversations` với `customerId` = `sourceIds.fb` hoặc `unifiedId`.
- **orderCountOnline, orderCountOffline**: COUNT orders có/không có `pageId` (hoặc dựa vào `posData.order_sources` nếu có).
- **firstOrderChannel, lastOrderChannel**: từ đơn đầu/cuối theo thời gian.
- **isOmnichannel**: orderCountOnline > 0 VÀ orderCountOffline > 0.

---

## 3. Kiến Trúc 2 Lớp Phân Loại

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│  LỚP 1 — CUSTOMER JOURNEY (Omnichannel: Online + Offline)                         │
│  VISITOR | ENGAGED_ONLINE | FIRST_ONLINE | FIRST_OFFLINE | REPEAT | VIP | OMNI |   │
│  INACTIVE | REACTIVATED                                                           │
└──────────────────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  LỚP 2 — CUSTOMER SEGMENTATION (4 trục phân loại)                 │
│  VALUE | LIFECYCLE | LOYALTY | MOMENTUM                           │
└─────────────────────────────────────────────────────────────────┘
```

---

## 4. Chi Tiết Từng Thành Phần

### 4.1. LỚP 1 — CUSTOMER JOURNEY (Omnichannel: Online + Offline)

FolkForm có **cửa hàng offline** và **kênh online** (FB/Messenger). Hành trình cần phản ánh cả hai.

#### 4.1.1. Phân loại kênh (Channel) — từ dữ liệu hiện có

| Kênh | Điều kiện | Nguồn dữ liệu |
|------|-----------|---------------|
| **ONLINE** | Đã nhắn tin FB hoặc có đơn từ FB | `hasConversation=true` HOẶC có order với `pageId` / `posData.order_sources` chứa facebook |
| **OFFLINE** | Mua tại cửa hàng | Có order với `pageId` rỗng VÀ (không có conversation HOẶC `order_sources` = pos/in_store) |
| **OMNICHANNEL** | Cả online và offline | Có đơn online (pageId có) VÀ có đơn offline (pageId không, shop khác hoặc order_sources pos) |

**Cách xác định đơn online vs offline:**

| Field | Online (FB/chat) | Offline (cửa hàng) |
|-------|------------------|---------------------|
| `pageId` | Có giá trị | Rỗng hoặc null |
| `posData.order_sources` | facebook, messenger, ... | pos, in_store, ... |
| `posData.conversation_link` | Có (link từ chat) | Rỗng |
| `shippingFee` | Thường > 0 (giao hàng) | Thường = 0 (mua tại chỗ) |

**Lưu ý:** Cần chạy script phân tích `posData.order_sources` thực tế để map chính xác. Pancake POS hỗ trợ: Facebook, Zalo, Shopee, Lazada, TikTok Shop, **POS (cửa hàng)**.

---

#### 4.1.2. Các stage hành trình (chi tiết)

| Stage | Ý nghĩa | Điều kiện | Nguồn dữ liệu |
|-------|---------|-----------|---------------|
| **UNKNOWN** | Có trong hệ thống nhưng chưa tương tác rõ | Có record (import SĐT, form) nhưng chưa conversation, chưa order | crm_customers |
| **VISITOR** | Biết đến brand, chưa tương tác | hasConversation=false, order_count=0. Có thể walk-in xem chưa mua. | |
| **ENGAGED_ONLINE** | Đã nhắn tin, chưa mua | hasConversation=true, order_count=0 | fb_conversations |
| **FIRST_ONLINE** | Mua lần đầu qua online (FB/chat) | order_count=1, đơn đầu có pageId | pc_pos_orders |
| **FIRST_OFFLINE** | Mua lần đầu tại cửa hàng | order_count=1, đơn đầu không có pageId (hoặc order_sources=pos) | pc_pos_orders |
| **REPEAT** | Mua ≥ 2 lần, chưa VIP | order_count ≥ 2, total_spent < 50M | |
| **VIP** | Khách giá trị cao | total_spent ≥ 50M | |
| **OMNI** | Mua cả online và offline | Có ≥ 1 đơn online VÀ ≥ 1 đơn offline | (subset của REPEAT/VIP) |
| **INACTIVE** | Ngừng mua > 90 ngày | days_since_last_order > 90 | |
| **REACTIVATED** | Quay lại sau inactive | Có đơn mới trong 30 ngày qua, sau khi đã > 90 ngày không mua | |

---

#### 4.1.3. Logic xác định Journey Stage (ưu tiên từ cao xuống)

```
1. Nếu order_count = 0:
   - hasConversation? → ENGAGED_ONLINE
   - Không (có record)? → VISITOR hoặc UNKNOWN

2. Nếu order_count >= 1:
   - days_since > 90:
     - Có đơn trong 30 ngày qua (sau khi đã > 90 ngày)? → REACTIVATED
     - Không → INACTIVE
   - total_spent >= 50M → VIP
   - order_count >= 2 → REPEAT (kèm flag isOmni nếu có cả đơn online + offline)
   - order_count = 1:
     - Đơn đầu có pageId? → FIRST_ONLINE
     - Đơn đầu không có pageId? → FIRST_OFFLINE
```

---

#### 4.1.4. Metrics bổ sung cho Journey

| Metric | Mô tả | Cách tính |
|--------|-------|-----------|
| `hasConversation` | Đã nhắn tin FB | Có record fb_conversations với customerId |
| `orderCountOnline` | Số đơn qua online | COUNT orders có pageId |
| `orderCountOffline` | Số đơn tại cửa hàng | COUNT orders không có pageId (hoặc order_sources=pos) |
| `firstOrderChannel` | Kênh mua đầu tiên | online \| offline — từ đơn đầu tiên |
| `isOmnichannel` | Mua cả 2 kênh | orderCountOnline > 0 VÀ orderCountOffline > 0 |
| `lastOrderChannel` | Kênh mua gần nhất | online \| offline — từ đơn cuối |

---

#### 4.1.5. Funnel visualization (Omnichannel)

```
                    ┌─ FIRST_ONLINE ─┐
VISITOR → ENGAGED_ONLINE ──────────────┼─→ REPEAT ─→ VIP
                    └─ FIRST_OFFLINE ─┘      │
                                             ├─ OMNI (nếu mua cả 2 kênh)
                    FIRST_OFFLINE ───────────┘
                    (walk-in, không qua chat)
                             │
                             └─→ INACTIVE ─→ REACTIVATED
```

---

#### 4.1.6. Ví dụ phân loại

| Khách | hasConversation | order_count | Đơn đầu pageId? | Stage |
|-------|-----------------|-------------|-----------------|-------|
| A | true | 0 | — | ENGAGED_ONLINE |
| B | false | 1 | Có | FIRST_ONLINE |
| C | false | 1 | Không | FIRST_OFFLINE |
| D | true | 3 | Có | REPEAT (có thể OMNI nếu có đơn offline) |
| E | false | 1 | Không, days_since=120 | INACTIVE |

### 4.2. TRỤC VALUE (theo total_spent VNĐ)

| Tier | total_spent (VNĐ) |
|------|-------------------|
| **VIP** | ≥ 50,000,000 |
| **High** | 20,000,000 – 49,999,999 |
| **Medium** | 5,000,000 – 19,999,999 |
| **Low** | 1,000,000 – 4,999,999 |
| **New** | < 1,000,000 (bao gồm 0) |

### 4.3. TRỤC LIFECYCLE (theo days_since_last_order)

| Stage | days_since_last_order |
|-------|------------------------|
| **Active** | ≤ 30 |
| **Cooling** | 31 – 90 |
| **Inactive** | 91 – 180 |
| **Dead** | > 180 |
| **never_purchased** | Chưa có đơn (days_since = -1) |

### 4.4. TRỤC LOYALTY (theo order_count)

| Stage | order_count |
|-------|-------------|
| **Core** | ≥ 5 |
| **Repeat** | 2 – 4 |
| **One-time** | 1 |

### 4.5. TRỤC MOMENTUM (recent vs historical)

| Stage | Công thức |
|-------|-----------|
| **Rising** | revenue_last_30d > 0 và (revenue_last_30d / max(revenue_last_90d, 1)) > 0.4 |
| **Stable** | revenue_last_30d > 0 và tỷ lệ trong [0.2, 0.5] |
| **Declining** | revenue_last_90d > 0, revenue_last_30d = 0, days_since ≤ 90 |
| **Lost** | days_since > 90 hoặc (revenue_last_90d = 0 và có historical) |

### 4.6. CUSTOMER STATE (5 thành phần)

Profile: `Journey | Value | Lifecycle | Loyalty | Momentum`

---

## 5. Customer Metrics

### 5.1. Chỉ số bắt buộc

| Nhóm | Field | Nguồn |
|------|-------|-------|
| Financial | total_spent (LTV) | Aggregate orders hoặc pc_pos_customers |
| | order_count | Aggregate orders hoặc succeedOrderCount |
| | avg_order_value | total_spent / order_count |
| Time | first_order_date | MIN(orders.insertedAt) |
| | last_order_date | MAX(orders.insertedAt) |
| | days_since_last_order | now - last_order_date |
| | avg_days_between_orders | (last - first) / (order_count - 1) nếu > 1 |
| Behavioral | orders_last_30d | COUNT orders 30 ngày qua |
| | orders_last_90d | COUNT orders 90 ngày qua |
| | revenue_last_30d | SUM orders 30 ngày qua |
| | revenue_last_90d | SUM orders 90 ngày qua |

### 5.2. Chiến lược tính toán

- **Hybrid:** Dùng pc_pos_customers khi có, aggregate orders khi thiếu hoặc cần revenue_last_30d/90d.
- **crm_customers:** Dùng làm nguồn identity; metrics vẫn aggregate từ orders theo sourceIds.pos hoặc billPhoneNumber (fallback).

---

## 6. Customer Intelligence (nâng cao)

| Metric | Mô tả |
|--------|-------|
| customer_score | Điểm 0–100 |
| churn_risk_score | Xác suất rời bỏ |
| predicted_next_order_date | Dự đoán ngày mua tiếp |
| predicted_LTV | Dự đoán LTV |

Triển khai Phase 4.

---

## 7. Customer Notes

**Collection:** `crm_notes`

| Field | Type | Mô tả |
|-------|------|-------|
| _id | ObjectID | |
| customerId | string | unifiedId |
| ownerOrganizationId | ObjectID | Phân quyền |
| created_at | int64 | |
| created_by | ObjectID | User tạo |
| note_text | string | Nội dung |
| next_action | string | Hành động tiếp theo |
| next_action_date | int64 | Ngày thực hiện |
| is_deleted | bool | Soft delete (mặc định false) |

**Index:** (ownerOrganizationId, customerId, created_at)

---

## 8. Sales Assignment

| Phương án | Mô tả |
|-----------|-------|
| **Hiện tại** | posData.assigning_seller trong pc_pos_customers |
| **Mở rộng** | Collection `crm_sales_assignments`: customerId (unifiedId), assignedSaleId, assignedAt, lastContactAt, lastContactBy, ownerOrganizationId |

Ưu tiên: dùng posData khi chưa có bảng; thêm collection khi cần override/lưu độc lập.

---

## 9. 6 Nhóm Khách CEO

| Nhóm | Điều kiện |
|------|-----------|
| VIP Active | Value=VIP, Lifecycle=Active |
| VIP Inactive | Value=VIP, Lifecycle=Inactive/Dead |
| Rising | Momentum=Rising |
| New | Journey=FIRST_PURCHASE hoặc Value=New |
| One-time | Loyalty=One-time |
| Dead | Lifecycle=Dead |

---

## 10. Visualization Dashboard

### 10.1. Journey Funnel

```
VISITOR → CONVERSATION → FIRST_PURCHASE → REPEAT → VIP
                                    ↘ INACTIVE → REACTIVATED
```

### 10.2. Customer Asset Matrix (Value × Lifecycle)

|  | Active | Cooling | Inactive | Dead |
|--|--------|---------|----------|------|
| VIP | 120 | 40 | 30 | 10 |
| High | 300 | 90 | 60 | 20 |
| Medium | 900 | 200 | 150 | 80 |
| Low | 2000 | 400 | 500 | 300 |
| New | 5000 | — | — | — |

### 10.3. VIP Monitoring Panel

- VIP total, VIP active, VIP inactive, VIP revenue share

---

## 11. API Thiết Kế

### 11.1. GET /api/v1/dashboard/customers (mở rộng)

**Query params bổ sung:**

| Param | Mô tả |
|-------|-------|
| journey | VISITOR, CONVERSATION, FIRST_PURCHASE, REPEAT, VIP, INACTIVE, REACTIVATED |
| valueTier | vip, high, medium, low, new |
| lifecycle | active, cooling, inactive, dead, never_purchased |
| loyalty | core, repeat, one_time |
| momentum | rising, stable, declining, lost |
| ceoGroup | vip_active, vip_inactive, rising, new, one_time, dead |

**Response bổ sung (CustomerItem):**

```json
{
  "customerId": "unified-id",
  "journeyStage": "REPEAT",
  "valueTier": "medium",
  "lifecycleStage": "active",
  "loyaltyStage": "repeat",
  "momentumStage": "stable",
  "revenueLast30d": 0,
  "revenueLast90d": 1500000,
  "avgOrderValue": 750000,
  "sources": ["pos", "fb"]
}
```

### 11.2. Endpoints mới

| Method | Path | Mô tả |
|--------|------|-------|
| GET | /dashboard/customers/ceo-groups | 6 nhóm CEO: count + top items |
| GET | /dashboard/customers/journey-funnel | Số lượng từng stage |
| GET | /dashboard/customers/asset-matrix | Ma trận Value × Lifecycle |
| GET | /customers/:unifiedId/profile | Profile đầy đủ (metrics + classification) |
| CRUD | /customers/:unifiedId/notes | Customer notes |

---

## 12. Cấu Trúc Code (Thư Mục)

Module CRM tuân theo convention hiện tại: mỗi domain có thư mục riêng với `handler/`, `service/`, `dto/`, `models/`, `router/`.

### 12.1. Cây thư mục

```
api/internal/api/crm/
├── handler/
│   ├── handler.crm.customer.go      # Profile khách, custom endpoints
│   └── handler.crm.note.go          # CRUD ghi chú
├── service/
│   ├── service.crm.customer.go       # Merge logic, metrics, unified
│   ├── service.crm.activity.go      # Lịch sử hoạt động
│   ├── service.crm.note.go         # CRUD notes
│   └── service.crm.hooks.go         # Event handlers (OnDataChanged)
├── dto/
│   ├── dto.crm.customer.go          # Create/Update input, profile response
│   └── dto.crm.note.go
├── models/
│   ├── model.crm.customer.go        # CrmCustomer (crm_customers)
│   ├── model.crm.activity.go       # CrmActivityHistory (crm_activity_history)
│   └── model.crm.note.go           # CrmNote (crm_notes)
└── router/
    └── routes.go                    # Register CRM routes
```

### 12.2. Naming convention (theo quy tắc dự án)

| Loại | Pattern | Ví dụ |
|------|---------|-------|
| Handler | `handler.<module>.<entity>.go` | `handler.crm.note.go` |
| Service | `service.<module>.<entity>.go` | `service.crm.customer.go` |
| Model | `model.<module>.<entity>.go` | `model.crm.customer.go` |
| DTO | `dto.<module>.<entity>.go` | `dto.crm.note.go` |
| Router | `routes.go` (trong `router/`) | `crm/router/routes.go` |

### 12.3. Phân chia trách nhiệm

| Thành phần | Trách nhiệm |
|------------|-------------|
| **Report module** | Dashboard aggregates: GET /dashboard/customers, /ceo-groups, /journey-funnel, /asset-matrix — gọi CrmService để lấy dữ liệu |
| **CRM module** | Merge, CRUD customer unified, CRUD notes, activity logging, hooks |
| **init.fiber.go** | Thêm `crmrouter.Register` vào danh sách register |

### 12.4. Đăng ký router

```go
// api/cmd/server/init.fiber.go
crmrouter.Register,  // Thêm vào danh sách
```

```go
// api/internal/api/crm/router/routes.go
package router

func Register(v1 fiber.Router, r *apirouter.Router) error {
    // GET /customers/:unifiedId/profile
    // CRUD /customers/:unifiedId/notes
    return nil
}
```

---

## 13. Database & Collections

**Module CRM — tiền tố `crm_` — tổng cộng 5 collections:**

| Collection | Mục đích |
|------------|----------|
| **crm_customers** | Khách đã merge (identity + sourceIds) — **bắt buộc tạo mới** |
| **crm_activity_history** | Lịch sử hoạt động khách (order, conversation, note, ...) — **bắt buộc tạo mới** |
| **crm_notes** | Ghi chú khách (soft delete) |
| crm_sales_assignments | (Optional) Gán sale |
| crm_classification_config | (Optional) Ngưỡng theo org |

**Đăng ký collection mới:**

```go
// init.go - initColNames()
global.MongoDB_ColNames.CrmCustomers = "crm_customers"
global.MongoDB_ColNames.CrmActivityHistory = "crm_activity_history"
global.MongoDB_ColNames.CrmNotes = "crm_notes"
global.MongoDB_ColNames.CrmSalesAssignments = "crm_sales_assignments"       // Optional
global.MongoDB_ColNames.CrmClassificationConfig = "crm_classification_config" // Optional

// init.registry.go
"crm_customers", "crm_activity_history", "crm_notes",
// Optional: "crm_sales_assignments", "crm_classification_config"
```

---

## 14. Migration & Tương Thích

### 14.1. Backward compatibility

- Giữ tier cũ (new/silver/gold/platinum) như `legacyTier` nếu frontend cần.
- Thêm `valueTier` mới song song.
- Lifecycle: map `vip_inactive` → `inactive` khi value=VIP; thêm `dead`.

### 14.2. Cấu hình ngưỡng (organization_config)

```json
{
  "customer_classification": {
    "value_tiers_vnd": {
      "vip": 50000000,
      "high": 20000000,
      "medium": 5000000,
      "low": 1000000
    },
    "lifecycle_days": {
      "active": 30,
      "cooling": 90,
      "inactive": 180
    }
  }
}
```

---

## 15. Roadmap Triển Khai

| Phase | Nội dung |
|-------|----------|
| **Phase 1** | Tạo collections `crm_customers`, `crm_activity_history`. Merge logic (fb_id, phone). Value tier, Lifecycle, Loyalty, Journey. 6 nhóm CEO. Mở rộng GET /dashboard/customers. **Đăng ký `events.OnDataChanged`** cho pc_pos_customers, fb_customers, pc_pos_orders, fb_conversations. |
| **Phase 2** | Momentum (revenue_last_30d/90d). Asset matrix. Journey funnel API. Hook vào extract/sync flow `pc_pos_*` (nếu có). |
| **Phase 3** | Customer notes (collection + CRUD). Sales assignment. Hook activity cho note_added, sale_assigned. |
| **Phase 4** | Customer Intelligence (score, churn risk, predicted LTV). Change Streams (nếu cần). |

---

## 16. Customer Activity History

### 16.1. Collection `crm_activity_history` (mới)

Lưu **lịch sử hoạt động** của khách hàng — mỗi sự kiện quan trọng tạo 1 record.

| Field | Type | Mô tả |
|-------|------|-------|
| _id | ObjectID | |
| unifiedId | string | ID khách trong crm_customers |
| ownerOrganizationId | ObjectID | Phân quyền |
| activityType | string | Loại hoạt động (xem bảng dưới) |
| activityAt | int64 | Thời điểm (Unix ms) |
| source | string | `pos` \| `fb` \| `system` |
| sourceRef | object | Tham chiếu nguồn: `{ "orderId": "...", "conversationId": "..." }` |
| metadata | object | Dữ liệu bổ sung (amount, status, ...) |
| createdAt | int64 | |

### 16.2. Các loại activityType

| activityType | Mô tả | Nguồn | sourceRef | metadata (bổ sung) |
|--------------|-------|-------|-----------|---------------------|
| `order_created` | Đơn hàng mới | pos | orderId | `channel`: online \| offline |
| `order_completed` | Đơn hoàn thành | pos | orderId | `channel`: online \| offline, `amount` |
| `conversation_started` | Bắt đầu hội thoại | fb | conversationId |
| `message_received` | Nhận tin nhắn | fb | conversationId, messageId |
| `note_added` | Thêm ghi chú | system | noteId |
| `sale_assigned` | Gán sale | system | assignedSaleId |
| `merge` | Merge khách | system | mergedFromIds |

### 16.3. Index

- `(ownerOrganizationId, unifiedId, activityAt)` — query lịch sử theo khách
- `(ownerOrganizationId, activityType, activityAt)` — thống kê theo loại
- `(ownerOrganizationId, source, activityAt)` — filter theo nguồn

### 15.4. Use case

- Timeline hoạt động trên profile khách
- Phân tích tần suất tương tác
- Audit trail cho merge và gán sale

---

## 17. Hooks & Events — Cập nhật dữ liệu customer

### 17.1. Phương án ưu tiên: Hook vào CRUD qua `events.OnDataChanged`

Hệ thống đã có **event system** (`api/internal/api/events`): `BaseServiceMongoImpl` tự động gọi `events.EmitDataChanged` sau mỗi **InsertOne, UpdateOne, DeleteOne, FindOneAndUpdate, Upsert** thành công.

→ **Mọi thay đổi qua CRUD** (API, webhook dùng BaseService, extract job dùng BaseService) đều phát event. Chỉ cần đăng ký handler.

**Pattern hiện có** (report): `service.report.hooks.go` — `init()` gọi `events.OnDataChanged(handleReportDataChange)`.

---

### 17.2. Đăng ký handler cho crm_customers

Tạo file `api/internal/api/customer/service.customer.hooks.go`:

```go
func init() {
    events.OnDataChanged(handleCustomerUnifiedDataChange)
}

func handleCustomerUnifiedDataChange(ctx context.Context, e events.DataChangeEvent) {
    switch e.CollectionName {
    case global.MongoDB_ColNames.PcPosCustomers:
        // Merge từ POS customer, upsert crm_customers
    case global.MongoDB_ColNames.FbCustomers:
        // Recompute merge từ FB customer
    case global.MongoDB_ColNames.PcPosOrders:
        // Ghi activity order_created/order_completed, refresh metrics
    case global.MongoDB_ColNames.FbConvesations:
        // Set hasConversation, ghi activity conversation_started
    default:
        return
    }
}
```

**Lưu ý:** Webhook Pancake dùng `FindOneAndUpdate` qua `BaseServiceMongoImpl` → đã emit event. Extract job nếu dùng BaseService → cũng emit. Chỉ cần đảm bảo mọi ghi dữ liệu vào các collection trên đều đi qua BaseService (không gọi `collection.InsertOne` trực tiếp).

---

### 17.3. Bảng collections cần hook

| Collection | Operation | Hành động |
|------------|-----------|-----------|
| **pc_pos_customers** | insert, update, upsert | `CustomerUnifiedService.MergeFromPosCustomer(doc)` |
| **fb_customers** | insert, update, upsert | `CustomerUnifiedService.RecomputeMergeFromFb(doc)` |
| **pc_pos_orders** | insert, update, upsert | Resolve unifiedId → `LogActivity(order_created/order_completed)` → `RefreshMetrics(unifiedId)` |
| **pc_orders** | insert, update, upsert | (Nếu dùng cho customer) Tương tự pc_pos_orders |
| **fb_conversations** | insert, update, upsert | Resolve unifiedId (customerId) → `SetHasConversation` → `LogActivity(conversation_started)` |
| **crm_notes** | insert | `LogActivity(note_added)` |
| **crm_sales_assignments** | insert, update | `LogActivity(sale_assigned)` |

---

### 17.4. Document từ event — xử lý interface{}

`e.Document` có thể là struct hoặc pointer. Dùng type assertion hoặc reflection:

```go
// Ví dụ với pc_pos_customers
if doc, ok := e.Document.(*pcmodels.PcPosCustomer); ok {
    _ = customerUnifiedSvc.MergeFromPosCustomer(ctx, doc)
}
// Hoặc dùng map nếu document từ FindOneAndUpdate trả về struct
```

---

### 17.5. Luồng dữ liệu đã cover

| Nguồn | Collection | Qua BaseService? | Event emit? |
|-------|------------|------------------|-------------|
| Pancake webhook (order) | pc_orders | FindOneAndUpdate | ✅ |
| Pancake webhook (conversation) | fb_conversations | FindOneAndUpdate | ✅ |
| Pancake webhook (customer) | fb_customers | FindOneAndUpdate | ✅ |
| Pancake webhook (message) | fb_messages | UpsertMessages (custom) | ⚠️ Cần kiểm tra |
| CRUD API (pc_pos_*) | pc_pos_customers, pc_pos_orders | InsertOne/UpdateOne | ✅ |
| Extract job (nếu dùng BaseService) | pc_pos_* | InsertOne/Upsert | ✅ |

**Ngoại lệ:** Nếu có luồng ghi trực tiếp vào collection (không qua BaseService), cần thêm `events.EmitDataChanged` tại điểm đó, hoặc dùng MongoDB Change Streams.

---

### 17.6. Service interface đề xuất

```go
// CustomerUnifiedService
type CustomerUnifiedService interface {
    MergeFromPosCustomer(ctx context.Context, doc interface{}) error
    RecomputeMergeFromFb(ctx context.Context, doc interface{}) error
    SetHasConversation(ctx context.Context, fbCustomerId string) error
    RefreshMetrics(ctx context.Context, unifiedId string) error
}

// CustomerActivityService
type CustomerActivityService interface {
    LogActivity(ctx context.Context, unifiedId string, activityType string, source string, sourceRef map[string]string, metadata map[string]interface{}) error
}
```

---

### 17.7. Fallback: MongoDB Change Streams

Nếu có luồng ghi **không qua BaseService** (vd: script, worker bên ngoài), dùng **Change Streams** để bắt mọi thay đổi:

```go
// Watch collection, on change → gọi CustomerUnifiedService
stream, _ := collection.Watch(ctx, pipeline)
for stream.Next(ctx) {
    // Xử lý change event
}
```

---

## Phụ lục: Chạy script phân tích

```bash
cd api
# Set env: MONGODB_CONNECTION_URI, MONGODB_DBNAME_AUTH (vd: folkform_auth)
go run ../scripts/analyze_customer_merge.go
```

Kết quả khảo sát (folkform_auth):
- **posData.fb_id** (format pageId_psid): ~398 khách POS merge được
- **Phone** (chuẩn hóa): ~407 cặp, ~416 khách FB link qua order.billPhone
- customerId: không trùng giữa FB và POS
