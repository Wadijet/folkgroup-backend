# VIP Intelligence — Phân loại Stage VIP

> **VIP = TÀI SẢN CHIẾN LƯỢC.** VIP không phải chỉ là "khách chi nhiều tiền". VIP là nguồn dòng tiền ổn định + định giá thương hiệu + social proof + moat cạnh tranh. Với luxury brand như FolkForm, VIP là lõi.

---

## I. Mục tiêu khi đánh giá nhóm VIP

CEO cần trả lời:

1. Bao nhiêu VIP đang active?
2. Bao nhiêu VIP có nguy cơ mất?
3. VIP đang tăng hay giảm?
4. VIP đang mua nhiều hơn hay ít đi?
5. VIP có đang tiến hóa thành "core patron" không?

---

## II. Phân loại VIP theo 6 chiều

### 1️⃣ VIP Status Health — **Dùng Lớp 2**

→ Dùng trực tiếp `lifecycleStage` (Lớp 2): active, cooling, inactive, dead. **Không duplicate trong Lớp 3.**

### 2️⃣ VIP Depth (Độ sâu quan hệ)

Dựa trên orderCount (VIP ≥ 8 đơn):

| Giá trị | OrderCount | Mô tả |
|---------|------------|-------|
| silver_vip | 8–12 | Vừa đạt ngưỡng VIP |
| gold_vip | 13–25 | |
| platinum_vip | 26–40 | |
| core_patron | 40+ | Rất gắn bó — lõi khách hàng |

### 3️⃣ VIP Spend Trend

So sánh AOV gần nhất vs AOV trung bình (±15%):

| Giá trị | Ý nghĩa |
|---------|----------|
| upscaling_vip | Đang mua nhiều hơn — tích cực |
| stable_vip | Ổn định |
| downscaling_vip | Đang mua ít đi — **tín hiệu cực kỳ nguy hiểm** |

### 4️⃣ VIP Product Diversity

Dựa trên OwnedSkuCount (số SKU đã mua):

| Giá trị | SKU | Mô tả |
|---------|-----|-------|
| single_line_vip | 0–2 | Ít đa dạng |
| multi_line_vip | 3–7 | |
| full_portfolio_vip | 8+ | LTV cao nhất |

### 5️⃣ VIP Engagement Level

| Giá trị | Điều kiện | Ghi chú |
|---------|-----------|---------|
| engaged_vip | Có conversation sau đơn gần nhất | Chủ động inbox |
| silent_vip | Không có conversation | **Rất dễ mất** |
| transactional_vip | Chỉ mua, ít chat | |

### 6️⃣ VIP Risk Score

Composite từ Status Health, Spend Trend, Engagement, Recency (daysSinceLast):

| Giá trị | Ý nghĩa |
|---------|----------|
| low | An toàn |
| medium | Cần theo dõi |
| high | Có nguy cơ mất |
| critical | Rủi ro cao — ưu tiên can thiệp |

---

## III. Cấu trúc VIP metrics (nested)

```json
{
  "journeyStage": "vip",
  "lifecycleStage": "active",
  "vip": {
    "vipDepth": "silver_vip|gold_vip|platinum_vip|core_patron",
    "spendTrend": "upscaling_vip|stable_vip|downscaling_vip",
    "productDiversity": "single_line_vip|multi_line_vip|full_portfolio_vip",
    "engagementLevel": "engaged_vip|silent_vip|transactional_vip",
    "riskScore": "low|medium|high|critical"
  }
}
```

Chỉ có khi `journeyStage=vip` và `orderCount ≥ 8`. `lifecycleStage` lấy từ Lớp 2.

---

## IV. API

- **Endpoint**: `GET /dashboard/customers` (Tab 4 Customer Intelligence)
- **Filter**: `journey=vip`
- **Response**: Mỗi `CustomerItem` có `journeyStage=vip` sẽ có field `vip` chứa 5 tiêu chí Lớp 3 (vipDepth, spendTrend, productDiversity, engagementLevel, riskScore). statusHealth dùng `lifecycleStage` (Lớp 2).

---

## V. Nguồn dữ liệu (CrmCustomer)

| Field | Dùng cho |
|-------|----------|
| LifecycleStage | (Lớp 2 — dùng trực tiếp) |
| OrderCount, TotalSpend | vipDepth |
| AvgOrderValue, RevenueLast30d, OrdersLast30d | spendTrend |
| OwnedSkuCount | productDiversity |
| LastConversationAt, LastOrderAtMs | engagementLevel |
| Composite 4 chiều trên + DaysSinceLast | riskScore |
