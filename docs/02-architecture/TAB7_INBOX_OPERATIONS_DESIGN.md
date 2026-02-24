# Tab 7 — Inbox Operations — Thiết Kế Backend

> Thiết kế backend cho Tab 7 Dashboard: Kiểm soát xử lý lead — CEO theo dõi lượng hội thoại, backlog, tốc độ phản hồi và hiệu suất sale.

---

## 1. Mục đích (Purpose)

**Kiểm soát xử lý lead** — CEO theo dõi:
- Lượng hội thoại
- Backlog (tin chờ phản hồi)
- Tốc độ phản hồi
- Hiệu suất sale

---

## 2. Layout UI (Tóm tắt)

| Row | Nội dung | Mô tả |
|-----|----------|-------|
| Row 0 | Page Filter | Chọn Page FB khi có nhiều trang |
| Row 1 | KPI Summary | 6 ô: Hội thoại hôm nay, Backlog, TB phản hồi, P90, Chưa assign, Conversion |
| Row 2 | Bảng hội thoại | Page, Khách, Tin cuối, Snippet, Trạng thái, Thời gian chờ, Response time, Sale, Tags, Hành động |
| Row 3 | Sale Performance | Sale, Số hội thoại, TB Response, Conversion |
| Right | Alert Zone | CRITICAL: Backlog > 30p. WARNING: Backlog chưa assign |

---

## 3. API

### 3.1. Endpoint chính

```
GET /api/v1/dashboard/inbox
```

**Permission:** `Report.Read`  
**Middleware:** `OrganizationContextMiddleware`

**Query params:**

| Param | Type | Mặc định | Mô tả |
|-------|------|-----------|-------|
| pageId | string | — | Lọc theo Page FB (optional) |
| filter | string | all | `backlog` \| `unassigned` \| `all` |
| limit | int | 50 | Số dòng conversation (max 200) |
| offset | int | 0 | Phân trang |
| sort | string | updated_desc | `waiting_desc` \| `updated_desc` \| `updated_asc` |
| period | string | month | Cho Conversion: `day` \| `week` \| `month` \| `60d` \| `90d` |

**Response format (chuẩn):**

```json
{
  "code": 200,
  "message": "Thành công",
  "data": {
    "pages": [{ "pageId": "...", "pageName": "..." }],
    "summary": {
      "conversationsToday": 0,
      "backlogCount": 0,
      "medianResponseMin": 0,
      "p90ResponseMin": 0,
      "unassignedCount": 0,
      "conversionRate": 0
    },
    "conversations": [...],
    "salePerformance": [...],
    "alerts": {
      "critical": [...],
      "warning": [...]
    }
  },
  "status": "success"
}
```

---

## 4. Data Mapping

### 4.1. Nguồn dữ liệu

| Collection | Mục đích |
|------------|----------|
| fb_conversations | Hội thoại, panCakeData (last_sent_by, current_assign_users, tags, customer) |
| fb_message_items | Response time (tin khách → tin page) |
| fb_pages | Tên Page cho filter |
| pc_pos_orders | Conversion (customerId, status 2,3,16) |

### 4.2. Định nghĩa Backlog

**Backlog** = Tin cuối từ khách, chưa reply.

- Điều kiện: `panCakeData.last_sent_by.email` chứa `@facebook.com`
- Tin từ khách: `messageData.from.email` có `@facebook.com`
- Tin từ page: `messageData.from.admin_name` có giá trị

### 4.3. Chưa assign

**Chưa assign** = Backlog **và** `panCakeData.current_assign_users` rỗng.

### 4.4. Response time

**Response time** = Thời gian từ tin khách → tin page reply (phút).

- Tính từ `fb_message_items`: sort theo `insertedAt`
- Tìm cặp: tin khách (from.email @facebook.com) → tin page kế tiếp (from.admin_name có)
- Lấy diff thời gian (phút)

### 4.5. Conversion

**Conversion** = Hội thoại có `customerId` và tồn tại đơn hàng hoàn thành (status 2, 3, 16) trong `period`.

- Join: `fb_conversations.customerId` = `pc_pos_orders.customerId` (hoặc `posData.customer.id`)
- Filter: `insertedAt`/`posCreatedAt` trong khoảng from–to của period

---

## 5. Alert Zone

| Mức | Điều kiện |
|-----|-----------|
| **CRITICAL** | Backlog > 30 phút |
| **WARNING** | Backlog ≤ 30 phút; Backlog chưa assign |

Hiển thị tối đa 10 mục mỗi loại, sort theo `waitingMinutes` giảm dần.

---

## 6. Row Color (Bảng hội thoại)

| Trạng thái | Màu |
|------------|-----|
| Chờ > 15p | Đỏ |
| Chờ 5–15p | Vàng |
| Chờ < 5p | Xanh |
| Đã phản hồi | Trắng |

*(Logic màu do frontend xử lý dựa trên `status`, `waitingMinutes`.)*

---

## 7. Files Backend

| File | Mô tả |
|------|-------|
| `api/internal/api/report/dto/dto.report.inbox.go` | DTO: InboxQueryParams, InboxSummary, InboxConversationItem, v.v. |
| `api/internal/api/report/service/service.report.inbox.go` | Service: GetInboxSnapshot, loadConversations, loadResponseTimes, v.v. |
| `api/internal/api/report/handler/handler.report.dashboard.go` | Handler: HandleGetInbox |
| `api/internal/api/report/router/routes.go` | Route: GET /dashboard/inbox |

---

## 8. Lưu ý kỹ thuật

1. **Timezone:** Dùng `Asia/Ho_Chi_Minh` cho "hôm nay".
2. **UpdatedAt:** `fb_conversations` dùng `panCakeUpdatedAt` hoặc `updatedAt` (Unix seconds).
3. **FbMessageItems.InsertedAt:** Có thể từ `messageData.inserted_at` (string) nếu field `insertedAt` = 0.
4. **Customer mapping:** Conversion dùng `customerId` (Pancake) — cần trùng giữa fb_conversations và pc_pos_orders.
5. **Tab 7 realtime:** Spec khuyến nghị Tab 7 không dùng period selector global; chỉ `period` cho Conversion metric.

---

## 9. Changelog

- v1.0: Thiết kế ban đầu — API, DTO, Service, Handler, Route. Data mapping từ fb_conversations, fb_message_items, fb_pages, pc_pos_orders.
