# Rà soát Logic Layer 3 — So sánh Code vs Design

**Ngày:** 2025-03-17

---

## 1. Các lỗi đã sửa

### 1.1 Repeat overlap với VIP (ĐÃ SỬA)

| Vấn đề | Design | Code cũ | Sửa |
|--------|--------|---------|-----|
| Điều kiện Repeat | orderCount 2–7, chưa VIP | `orderCount >= 2` | Thêm `orderCount <= 7 \|\| valueTier != "top"` |

Design doc: *"Chỉ có khi journeyStage=repeat (orderCount 2–7, chưa VIP)"*. Code cũ derive Repeat cho mọi orderCount ≥ 2, dẫn đến khách VIP (8+ đơn) vẫn có thể có cả Repeat và Vip khi journeyStage chưa sync.

---

## 2. Logic đúng (đã xác minh)

| Tiêu chí | Code | Design | Ghi chú |
|----------|------|--------|---------|
| purchaseQuality | high_aov ≥ 500k, entry < 150k | ✓ | Biên 150000 → medium |
| experienceQuality | cancelled > 0 → risk | ✓ | |
| engagementAfterPurchase | lastConv > lastOrder → post_purchase_engaged | ✓ | |
| reorderTiming | too_early < 7, within 7–60, overdue > 60 | ✓ | |
| repeatProbability | score ≥ 5 high, ≤ 1 low | ✓ | |
| repeatDepth | R1=2, R2=3–4, R3=5–7, R4=8+ | ✓ | |
| repeatFrequency | avgDays từ last−second, fallback 7/45/90 | ✓ | |
| spendMomentum | lastAOV vs avgVal ±15% | ✓ | |
| productExpansion | sku ≥ 3 → multi | ✓ | |
| emotionalEngagement | lastConv>lastOrder→engaged, =0→silent | ✓ | |
| upgradePotential | score ≥ 6 high, ≤ 2 low | ✓ | |
| vipDepth | 8–12, 13–25, 26–40, 40+ | ✓ | |
| vipSpendTrend | ±15% | ✓ | |
| vipProductDiversity | 0–2, 3–7, 8+ | ✓ | |
| vipEngagement | Giống emotionalEngagement | ✓ | |
| vipRiskScore | Composite lifecycle+spend+engagement+days | ✓ | |
| engagementDrop | had/no/dropped | ✓ | |
| reactivationPotential | Composite value+lifecycle+oc+engagement | ✓ | |
| conversationTemperature | hot≤1, warm≤3, cooling≤7 | ✓ | |
| engagementDepth | light≤3, medium≤10 | ✓ | |
| sourceType | fromAds → ads | ✓ | |

---

## 3. Các vấn đề cần lưu ý (chưa sửa)

### 3.1 Timestamp format (convMs < 1e12)

Code giả định `lastConversationAt` có thể là Unix **giây** (10 chữ số) thay vì **ms** (13 chữ số). Nếu < 1e12 thì nhân 1000. Logic hợp lý, nhưng nếu nguồn dùng format khác có thể sai.

### 3.2 repeatFrequency — truncation int64

```go
if days <= int64(avgDays*1.5) { return "on_track" }
```

`int64(avgDays*1.5)` cắt phần thập phân. Ví dụ avgDays=30.7 → 46.05 → int64=46. Biên có thể lệch nhẹ, chấp nhận được.

### 3.3 daysSinceLast < 0

Khi `lastOrderAt <= 0` hoặc invalid, `daysSinceLast = -1`. Code trả:
- firstReorderTiming: `within_expected`
- repeatFrequency: `on_track`

Thiết kế không định nghĩa. Hiện coi là fallback an toàn.

### 3.4 Engaged Temperature — thiếu backlog

Design gốc: *"Hot: days ≤ 1 && (backlog \|\| lastFromCustomer trong 24h)"*. Code chỉ dùng `days ≤ 1`. Bỏ qua backlog/lastFromCustomer — đơn giản hơn, có thể kém chính xác với hội thoại đang chờ phản hồi.

### 3.5 First — thiếu "negative" và "complaint"

Design có `engagementAfterPurchase: negative`, `experienceQuality: complaint`. Code chưa có. Phase 2.

### 3.6 First — thiếu discount, gift_buyer

Design có `purchaseQuality: discount|gift_buyer`. Code chưa có. Phase 2.

---

## 4. Edge cases đã xử lý đúng

| Trường hợp | Xử lý |
|------------|-------|
| ord30=0 hoặc avgVal=0 (spendMomentum) | return "stable" trước khi chia |
| lastConv=0 | silent / no_engagement |
| lastOrderAt=0 (First) | firstEngagement: lastOrder > 0 mới post_purchase |
| valueTier rỗng/unknown (vipRiskScore) | default +2 (an toàn) |
| valueTier low/new (reactivationPotential) | score += 0 |
| ownedSkuQuantities rỗng | ownedSkuCount = 0 → single_category |

---

## 5. Tóm tắt

- **Đã sửa:** 1 lỗi (Repeat overlap VIP).
- **Logic chính:** Khớp design.
- **Chưa triển khai:** negative, complaint, discount, gift_buyer (Phase 2).
- **Cần theo dõi:** Format timestamp, Engaged backlog.
