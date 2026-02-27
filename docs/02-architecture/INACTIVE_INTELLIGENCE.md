# Inactive Intelligence — Phân loại Nhóm Tài Sản Đang Chết

> **Inactive = TÀI SẢN ĐANG CHẾT (nhưng vẫn còn giá trị).** Inactive không phải "bỏ đi". Inactive là tài sản đã từng trust brand — chi phí reactivation rẻ hơn rất nhiều so với acquire mới. Luxury brand giỏi không phải ở acquire, mà ở reactivate.

---

## I. Định nghĩa rõ Inactive

**Inactive ≠ Dead.** Phải tách:

| Bucket | Ngày | Chiến lược |
|--------|------|------------|
| Cooling | 30–90 ngày | Bắt đầu nguội — can thiệp sớm |
| Inactive | 90–180 ngày | Đang chết — ưu tiên reactivation |
| Dead | >180 ngày | Đã mất — khó cứu, có thể win-back campaign |

Chiến lược khác nhau hoàn toàn.

---

## II. Mục tiêu khi phân tích nhóm Inactive

CEO cần trả lời:

1. Bao nhiêu tài sản đang chết?
2. Bao nhiêu trong số đó là VIP / High?
3. Bao nhiêu có khả năng cứu được?
4. Reactivation có hiệu quả không?
5. Vì sao họ rời đi?

---

## III. Phân loại nhóm Inactive theo 6 chiều

### 1️⃣–4️⃣ Value Tier, Inactive Duration, Previous Behavior, Spend Momentum — **Dùng Lớp 2**

→ Dùng trực tiếp `valueTier`, `lifecycleStage`, `loyaltyStage`, `momentumStage` (Lớp 2). **Không duplicate trong Lớp 3.**

### Engagement Drop (tiêu chí Lớp 3)

Trước khi inactive: Chat giảm? Sale không follow-up?

| Giá trị | Điều kiện |
|---------|-----------|
| had_post_engagement | Có conversation sau đơn gần nhất |
| no_engagement | Không có conversation |
| dropped_engagement | Có chat trước mua nhưng không sau — sale không follow-up |

### Reactivation Potential Score (tiêu chí Lớp 3)

Composite: Value tier + Recency + Order count + Engagement history.

| Giá trị | Ý nghĩa |
|---------|---------|
| high | Tiềm năng cứu cao — ưu tiên reactivation |
| medium | Có thể thử |
| low | Khó cứu |

---

## IV. Cấu trúc Inactive metrics (nested)

```json
{
  "lifecycleStage": "inactive",
  "valueTier": "vip",
  "loyaltyStage": "core",
  "momentumStage": "declining",
  "inactive": {
    "engagementDrop": "had_post_engagement|no_engagement|dropped_engagement",
    "reactivationPotential": "high|medium|low"
  }
}
```

Chỉ có khi `lifecycleStage` ∈ `cooling`, `inactive`, `dead` và `orderCount ≥ 1`. `valueTier`, `lifecycleStage`, `loyaltyStage`, `momentumStage` lấy từ Lớp 2.

---

## V. API

- **Endpoint**: `GET /dashboard/customers` (Tab 4 Customer Intelligence)
- **Filter**: `lifecycle=cooling,inactive,dead`
- **Response**: Mỗi `CustomerItem` có lifecycle cooling/inactive/dead sẽ có field `inactive` chứa 2 tiêu chí Lớp 3 (engagementDrop, reactivationPotential). valueTier, lifecycleStage, loyaltyStage, momentumStage dùng từ Lớp 2.

---

## VI. Nguồn dữ liệu (CrmCustomer)

| Field | Dùng cho |
|-------|----------|
| ValueTier, LifecycleStage, LoyaltyStage, MomentumStage | (Lớp 2 — dùng trực tiếp) |
| LastConversationAt, LastOrderAtMs | engagementDrop |
| Composite (L2 + engagementDrop) | reactivationPotential |
