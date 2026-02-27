# Repeat Intelligence — Phân loại Stage Mua Lại

> **Repeat = TÀI SẢN ĐANG SINH LỜI.** Nhóm quyết định: doanh thu ổn định, dòng tiền bền vững, định giá công ty. Luxury brand sống bằng nhóm này.

---

## I. Mục tiêu khi đánh giá nhóm Repeat

Trả lời 5 câu hỏi:

1. Nhóm repeat đang tăng hay giảm?
2. Bao nhiêu repeat đang tiến hóa thành VIP?
3. Bao nhiêu repeat đang suy giảm?
4. Bao nhiêu repeat có nguy cơ thành inactive?
5. Làm sao tăng AOV và tần suất mua?

---

## II. Phân loại nhóm Repeat theo 6 chiều

### 1️⃣ Repeat Depth (Độ sâu mua lại)

Dựa vào `orderCount`:

| Giá trị | Điều kiện | Ghi chú |
|--------|-----------|---------|
| R1 | 2 đơn | Nhóm nguy hiểm nhất — chưa chắc bền |
| R2 | 3–4 đơn | |
| R3 | 5–7 đơn | |
| R4 | 8+ đơn | Core — đã gần VIP |

### 2️⃣ Repeat Frequency (Nhịp độ mua)

So sánh `avg_days_between_orders` vs `days_since_last_order`:

| Giá trị | Mô tả | Rủi ro |
|---------|-------|--------|
| early | Mua sớm hơn kỳ vọng | Thấp |
| on_track | Đúng nhịp | Thấp |
| delayed | Trễ hơn | Cao |
| overdue | Quá trễ | Cao — rủi ro churn |

- Dùng `LastOrderAtMs` và `SecondLastOrderAt` để tính avg days giữa 2 đơn gần nhất.
- Fallback: ngưỡng cố định (early < 7 ngày, on_track ≤ 45 ngày, delayed ≤ 90 ngày, overdue > 90 ngày).

### 3️⃣ Spend Momentum

So sánh AOV lần gần nhất vs AOV trung bình:

| Giá trị | Ý nghĩa |
|---------|---------|
| upscaling | Đang mua cao hơn — tích cực |
| stable | Ổn định |
| downscaling | Đang mua thấp dần — chuẩn bị churn |

- Nguồn: `RevenueLast30d / OrdersLast30d` vs `AvgOrderValue`. Ngưỡng ±15%.

### 4️⃣ Product Expansion

| Giá trị | Điều kiện | Ghi chú |
|---------|-----------|---------|
| single_category | OwnedSkuCount < 3 | Ít đa dạng SKU |
| multi_category | OwnedSkuCount ≥ 3 | LTV cao hơn |

- Dùng số SKU đã mua làm proxy (chưa có category).

### 5️⃣ Emotional Engagement

| Giá trị | Điều kiện | Nguồn |
|---------|-----------|-------|
| engaged_repeat | Có conversation sau đơn gần nhất | LastConversationAt > LastOrderAtMs |
| silent_repeat | Không có conversation | LastConversationAt = 0 |
| transactional_repeat | Chỉ mua, ít chat | Có conversation trước mua |

### 6️⃣ Upgrade Potential

Đo khả năng trở thành VIP — composite từ 5 chiều trên + TotalSpend:

| Giá trị | Ý nghĩa |
|---------|---------|
| high | Score ≥ 6 |
| medium | 2 < Score < 6 |
| low | Score ≤ 2 |

---

## III. Cấu trúc Repeat metrics (nested)

```json
{
  "journeyStage": "repeat",
  "repeat": {
    "repeatDepth": "R1|R2|R3|R4",
    "repeatFrequency": "on_track|early|delayed|overdue",
    "spendMomentum": "upscaling|stable|downscaling",
    "productExpansion": "single_category|multi_category",
    "emotionalEngagement": "engaged_repeat|silent_repeat|transactional_repeat",
    "upgradePotential": "high|medium|low"
  }
}
```

Chỉ có khi `journeyStage=repeat` (orderCount 2–7, chưa VIP).

---

## IV. API

- **Endpoint**: `GET /dashboard/customers` (Tab 4 Customer Intelligence)
- **Filter**: `journey=repeat`
- **Response**: Mỗi `CustomerItem` có `journeyStage=repeat` sẽ có field `repeat` chứa 6 chiều trên.

---

## V. Nguồn dữ liệu (CrmCustomer)

| Field | Dùng cho |
|-------|----------|
| OrderCount | repeatDepth |
| LastOrderAtMs, SecondLastOrderAt, DaysSinceLast | repeatFrequency |
| AvgOrderValue, RevenueLast30d, OrdersLast30d | spendMomentum |
| OwnedSkuQuantities (len) | productExpansion |
| LastConversationAt | emotionalEngagement |
| TotalSpend + 5 chiều trên | upgradePotential |
