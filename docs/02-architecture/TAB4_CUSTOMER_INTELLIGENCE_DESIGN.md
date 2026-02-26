# Tab 4 — Customer Intelligence — Thiết Kế Backend

> Thiết kế backend cho Tab 4 Dashboard: Đo lường chất lượng tài sản khách hàng — CEO theo dõi phân bố hạng, vòng đời và xu hướng để bảo vệ/tái kích hoạt khách giá trị.

---

## 1. Mục đích (Purpose)

**Đo lường chất lượng tài sản khách hàng** — CEO theo dõi:
- Phân bố hạng khách hàng (Tier)
- Vòng đời (Lifecycle) — Active, Cooling, Inactive, VIP inactive
- VIP inactive — khách giá trị cần tái kích hoạt

---

## 2. Layout UI (Tóm tắt)

| Row | Nội dung | Mô tả |
|-----|----------|-------|
| Row 0 | Period Selector | Day \| Week \| Month \| 60d \| 90d \| Year \| Custom |
| Row 1 | KPI Summary | 6 ô: Tổng khách, Khách mới, Repeat Rate, VIP inactive, Tổng trị giá tái KHO, Active hôm nay |
| Row 2 | Tier Distribution | Pie/Donut: New \| Silver \| Gold \| Platinum/VIP |
| Row 3 | Lifecycle | Bar: Active \| Cooling \| Inactive \| VIP inactive \| Chưa mua |
| Row 4 | Customer Table | Tên, SĐT, Tier, Total Spend, Orders, Last Order, Days Since, Lifecycle, Sale, Tags, Hành động |
| Right | VIP Inactive Panel | Top 10–15 khách VIP/Gold không mua > 90 ngày |

---

## 3. API

### 3.1. Endpoint chính

```
GET /api/v1/dashboard/customers
```

**Permission:** `Report.Read`  
**Middleware:** `OrganizationContextMiddleware`

**Query params:**

| Param | Type | Mặc định | Mô tả |
|-------|------|-----------|-------|
| from | string | — | dd-mm-yyyy (cho period=custom) |
| to | string | — | dd-mm-yyyy (cho period=custom) |
| period | string | month | `day` \| `week` \| `month` \| `60d` \| `90d` \| `year` \| `custom` |
| filter | string | all | `vip_inactive` \| `inactive` \| `cooling` \| `active` \| `tier_new` \| `tier_silver` \| `tier_gold` \| `tier_platinum` \| `all` |
| limit | int | 500 | Số dòng customer table (max 2000) |
| offset | int | 0 | Phân trang |
| sort | string | days_since_desc | `days_since_desc` \| `total_spend_desc` \| `last_order_desc` \| `name_asc` |
| vipInactiveLimit | int | 15 | Số khách VIP inactive trong panel (max 20) |
| activeDays | int | 30 | Ngưỡng Active: days ≤ 30 |
| coolingDays | int | 60 | Ngưỡng Cooling: 30 < days ≤ 60 |
| inactiveDays | int | 90 | Ngưỡng Inactive: 60 < days ≤ 90 |

**Response format (chuẩn):**

```json
{
  "code": 200,
  "message": "Thành công",
  "data": {
    "summary": {
      "totalCustomers": 0,
      "newCustomersInPeriod": 0,
      "repeatRate": 0,
      "vipInactiveCount": 0,
      "reactivationValue": 0,
      "activeTodayCount": 0
    },
    "summaryStatuses": { },
    "tierDistribution": { "new": 0, "silver": 0, "gold": 0, "platinum": 0 },
    "lifecycleDistribution": { "active": 0, "cooling": 0, "inactive": 0, "vip_inactive": 0, "never_purchased": 0 },
    "customers": [...],
    "vipInactiveCustomers": [...]
  },
  "status": "success"
}
```

---

## 4. Data Mapping

### 4.1. Nguồn dữ liệu

| Collection | Mục đích |
|------------|----------|
| pc_pos_customers | Khách hàng: name, phoneNumbers, posData (level, order_count, purchased_amount, last_order_at, tags, assigning_seller) |
| pc_pos_orders | Aggregate: first_order_period, last_order_at, purchased_amount, order_count; status 2,3,16 (hoàn thành) |

### 4.2. Tier (phân hạng khách)

| Tier | Điều kiện (order_count) | Mô tả |
|------|--------------------------|-------|
| new | 1 | Khách mới |
| silver | 2–4 | Silver |
| gold | 5–9 | Gold |
| platinum | 10+ | Platinum/VIP |

- **Nguồn:** `posData.level` (nếu map được) hoặc **fallback** từ `posData.order_count`, `posData.succeed_order_count`, `totalOrder`, `succeedOrderCount`.

### 4.3. Lifecycle (vòng đời)

| Lifecycle | Điều kiện | Mô tả |
|-----------|-----------|-------|
| active | days_since_last ≤ 30 | Đang hoạt động |
| cooling | 30 < days ≤ 60 | Đang nguội |
| inactive | 60 < days ≤ 90 | Không hoạt động |
| vip_inactive | (Gold hoặc Platinum) và days > 90 | VIP không mua lâu |
| never_purchased | Chưa có đơn | Chưa mua |

- **days_since_last** = TODAY − `last_order_at`
- Ngưỡng cấu hình: active_days=30, cooling_days=60, inactive_days=90.

### 4.4. VIP Inactive

**VIP Inactive** = Khách có Tier Gold hoặc Platinum **và** `days_since_last > inactive_days` (90).

- **Tổng trị giá tái KHO** = SUM(purchased_amount) của tất cả VIP inactive.
- Panel bên phải: top 10–15 theo `purchased_amount` giảm dần.

---

## 5. KPI chi tiết

| KPI | Công thức / Nguồn |
|-----|-------------------|
| Tổng khách hàng | COUNT(pc_pos_customers) |
| Khách mới (period) | Khách có đơn **đầu tiên** trong khoảng from–to (aggregate từ pc_pos_orders) |
| Repeat Rate | Số khách có ≥2 đơn / Số khách có ≥1 đơn |
| VIP inactive | Số khách Gold/Platinum với days_since_last > 90 |
| Tổng trị giá tái KHO | SUM(purchased_amount) của VIP inactive |
| Active hôm nay | Khách có đơn trong 24h qua (last_order_at trong ngày) |

---

## 6. Cột bảng Customer

| Cột | Field |
|-----|-------|
| Tên | name |
| SĐT | phoneNumbers[0] hoặc posData.phone_numbers |
| Tier | new/silver/gold/platinum |
| Total Spend | purchased_amount / totalSpent |
| Orders | order_count / succeedOrderCount |
| Last Order | last_order_at (ISO) |
| Days Since | days_since_last_order |
| Lifecycle | active/cooling/inactive/vip_inactive/never_purchased |
| Sale | posData.assigning_seller.name hoặc từ đơn gần nhất |
| Tags | posData.tags (text) |

**Row color:**
- Đỏ nhạt: VIP inactive
- Cam: Inactive
- Vàng: Cooling
- Trắng: Active

---

## 7. Thống kê theo Snapshot

**KPI Summary và phân bố (Tier, Lifecycle)** ưu tiên lấy từ `report_snapshots` (đã tính sẵn theo chu kỳ). Chi tiết xem `CUSTOMER_REPORT_SNAPSHOT_DESIGN.md`.

---

## 8. Files cần tạo/sửa

| File | Nội dung |
|------|----------|
| `report/dto/dto.report.customers.go` | DTO: CustomersQueryParams, CustomerSummary, CustomerItem, vipInactiveItem |
| `report/service/service.report.customers.go` | GetCustomersSnapshot |
| `report/service/service.report.customer.go` | ComputeCustomerReport, GetSnapshotForCustomersDashboard |
| `report/handler/handler.report.dashboard.go` | HandleGetCustomers (ưu tiên snapshot) |
| `report/router/routes.go` | Register GET /dashboard/customers |

---

## 9. Logic aggregate từ orders

Khi `pc_pos_customers` thiếu `last_order_at`, `purchased_amount`, `order_count`:

1. Aggregate từ `pc_pos_orders` theo `customerId`:
   - `first_order_at` = MIN(createdAt) — dùng cho Khách mới (period)
   - `last_order_at` = MAX(createdAt)
   - `purchased_amount` = SUM(total_amount) đơn status 2,3,16
   - `order_count` = COUNT đơn status 2,3,16

2. Merge vào customer record khi query.

---

## End of Design Document
