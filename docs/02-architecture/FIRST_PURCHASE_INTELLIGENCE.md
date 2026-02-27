# First Purchase Intelligence — Phân loại Stage Mua Lần Đầu

> Nhóm First = Pipeline chưa maximize LTV. Tối ưu First → Repeat tăng LTV 2–3x mà không cần thêm ads.

---

## I. Mục tiêu

1. Khách mua lần đầu có tiềm năng thành repeat không?
2. Bao lâu thì họ quay lại?
3. Ai có nguy cơ chỉ mua 1 lần rồi biến mất?
4. Phải làm gì để họ quay lại?

---

## II. Cấu trúc First metrics (nested)

```json
{
  "journeyStage": "first",
  "first": {
    "purchaseQuality": "high_aov|entry|medium|discount|gift_buyer",
    "experienceQuality": "smooth|risk|complaint",
    "engagementAfterPurchase": "post_purchase_engaged|silent|negative",
    "reorderTiming": "within_expected|overdue|too_early",
    "repeatProbability": "high|medium|low"
  }
}
```

Chỉ có khi `journeyStage=first` (orderCount=1).

---

## III. Logic Phase 1 (đã triển khai)

### 1️⃣ Purchase Quality

| Giá trị | Điều kiện | Nguồn |
|--------|-----------|-------|
| high_aov | AvgOrderValue >= 500.000 VND | CrmCustomer.AvgOrderValue |
| entry | AvgOrderValue < 150.000 VND | Idem |
| medium | 150k–500k | Idem |
| discount | Cần first order totalDiscount | Phase 2 |
| gift_buyer | Cần detect từ note/product | Phase 2 |

### 2️⃣ Experience Quality

| Giá trị | Điều kiện | Nguồn |
|---------|-----------|-------|
| smooth | Không có đơn hủy | CancelledOrderCount = 0 |
| risk | Có đơn hủy/trả | CancelledOrderCount > 0 |
| complaint | Cần activity/note | Phase 2 |

### 3️⃣ Engagement After Purchase

| Giá trị | Điều kiện | Nguồn |
|---------|-----------|-------|
| post_purchase_engaged | LastConversationAt > LastOrderAt | CrmCustomer |
| silent | Không có conversation sau mua | Idem |

### 4️⃣ Reorder Timing

| Giá trị | Điều kiện |
|---------|-----------|
| too_early | daysSinceLast < 7 |
| within_expected | 7 ≤ days ≤ 60 |
| overdue | days > 60 |

### 5️⃣ Repeat Probability

Score từ 4 chiều trên:

- **high**: score ≥ 5
- **low**: score ≤ 1
- **medium**: còn lại

---

## IV. API

`GET /api/v1/dashboard/customers` với `journey=first` — mỗi `CustomerItem` có `first` khi `journeyStage=first`.

---

## V. Phase 2 (cần bổ sung data)

- **discount / gift_buyer**: Query first order từ pc_pos_orders (totalDiscount, note)
- **complaint**: Activity/note có từ khóa phàn nàn
- **avg_days_between_orders**: Aggregation theo category để tinh chỉnh reorder timing
