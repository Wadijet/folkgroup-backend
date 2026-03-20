# ĐỀ XUẤT PHƯƠNG ÁN — Script Báo Cáo Phân Tích Dữ Liệu Điều Chỉnh Threshold Customer

**Ngày tạo:** 2026-03-18  
**Mục đích:** Tạo căn cứ dữ liệu để điều chỉnh threshold customer của các layer (Lớp 2, Lớp 3).

---

## 1. Bối cảnh

### 1.1. Nguồn threshold hiện tại

| Layer | Nguồn | Vị trí |
|-------|-------|--------|
| **Lớp 2 (CRM Classification)** | Rule Engine RULE_CRM_CLASSIFICATION | `seed_rule_crm_system.go` → PARAM_CRM_CLASSIFICATION |
| **Lớp 3 (First, Repeat, VIP, Inactive, Engaged)** | Hardcoded constants | `api/internal/api/report/layer3/layer3.go` |

### 1.2. Threshold Lớp 2 (PARAM_CRM_CLASSIFICATION)

| Param | Giá trị mặc định | Ý nghĩa |
|-------|------------------|----------|
| valueVip | 50.000.000 | totalSpent ≥ → valueTier=top |
| valueHigh | 20.000.000 | totalSpent ≥ → valueTier=high |
| valueMedium | 5.000.000 | totalSpent ≥ → valueTier=medium |
| valueLow | 1.000.000 | totalSpent ≥ → valueTier=low |
| lifecycleActive | 30 | daysSinceLastOrder ≤ → active |
| lifecycleCooling | 90 | daysSinceLastOrder ≤ → cooling |
| lifecycleInactive | 180 | daysSinceLastOrder ≤ → inactive |
| loyaltyCore | 5 | orderCount ≥ → loyaltyStage=core |
| loyaltyRepeat | 2 | orderCount ≥ → loyaltyStage=repeat |
| momentumRising | 0.5 | rev30/rev90 > → rising |
| momentumStableLo | 0.2 | rev30/rev90 ≥ → stable |
| momentumStableHi | 0.5 | rev30/rev90 ≤ → stable |

### 1.3. Threshold Lớp 3 (layer3.go constants)

| Constant | Giá trị | Ý nghĩa |
|----------|---------|---------|
| firstHighAOV | 500.000 | AOV ≥ → purchaseQuality=high_aov |
| firstEntryAOV | 150.000 | AOV < → purchaseQuality=entry |
| firstReorderTooEarly | 7 | days < → reorderTiming=too_early |
| firstReorderExpectedMax | 60 | days > → reorderTiming=overdue |
| repeatFreqEarlyMax | 7 | days < → repeatFrequency=early |
| repeatFreqOnTrackMax | 45 | days ≤ → on_track |
| repeatFreqDelayedMax | 90 | days ≤ → delayed |
| repeatSkuMulti | 3 | skuCount ≥ → productExpansion=multi_category |
| vipSilverMax | 12 | orderCount 8–12 → silver_vip |
| vipGoldMax | 25 | orderCount 13–25 → gold_vip |
| vipPlatinumMax | 40 | orderCount 26–40 → platinum_vip |
| vipSingleLineMax | 2 | skuCount 0–2 → single_line_vip |
| vipMultiLineMax | 7 | skuCount 3–7 → multi_line_vip |
| vipSpendTrendThreshold | 0.15 | ±15% AOV → upscaling/downscaling |

**Threshold hardcoded trong hàm (không có constant):**

| Vị trí | Giá trị | Ý nghĩa |
|--------|---------|---------|
| repeatSpendMomentum | 0.15 | lastAOV vs avgOrderValue: diff ≥15% → upscaling, ≤-15% → downscaling |
| repeatFrequency (khi có avgDays) | 0.5 / 1.5 / 2 | days < avgDays×0.5 → early; ≤avgDays×1.5 → on_track; ≤avgDays×2 → delayed |
| repeatUpgradePotential | totalSpend ≥ 2.000.000 | +1 điểm; score ≥6 high, ≤2 low |
| vipRiskScore | days > 90 / > 60 | daysSinceLastOrder > 90 → -2; > 60 → -1 |
| inactiveReactivationPotential | oc ≥ 8 / ≥ 2 | orderCount ≥ 8 → +2; ≥ 2 → +1; score ≥7 high, ≤3 low |
| repeatDepth | 2 / 3–4 / 5–7 / 8+ | R1 / R2 / R3 / R4 (theo orderCount) |

### 1.4. Chỉ tiêu liên quan tin nhắn & hội thoại (Lớp 3 Engaged, Lớp 2 journey)

| Chỉ tiêu | Nguồn | Threshold hiện tại | Ý nghĩa |
|----------|-------|--------------------|---------|
| **totalMessages** | currentMetrics.raw.totalMessages | light ≤3, medium ≤10, deep >10 | Lớp 3 Engaged — engagementDepth (độ sâu tương tác) |
| **conversationCount** | currentMetrics.raw.conversationCount | — | Số hội thoại; hasConversation = count > 0 → journeyStage engaged |
| **lastConversationAt** | currentMetrics.raw.lastConversationAt | hot ≤1d, warm ≤3d, cooling ≤7d, cold >7d | Lớp 3 Engaged — conversationTemperature |
| **conversationFromAds** | currentMetrics.raw.conversationFromAds | true → ads, false → organic | Lớp 3 Engaged — sourceType |

**Lưu ý:** `totalMessages` = tổng số tin nhắn của khách (aggregate từ fb_message_items qua conversations). Dùng để phân loại khách **engaged** (chưa có đơn) theo mức độ tương tác: light (ít tin) → medium → deep (nhiều tin).

**hasConversation** (dùng trong RULE_CRM_CLASSIFICATION cho journeyStage=engaged) thường derive từ `conversationCount > 0`.

---

## 2. Mục tiêu báo cáo

Báo cáo cần trả lời:

1. **Phân phối thực tế** của dữ liệu (totalSpent, orderCount, daysSinceLastOrder, avgDaysBetweenOrders, AOV, rev30/rev90, spend momentum, ownedSkuCount, **totalMessages**, conversationCount, cancelledOrderCount…).
2. **Số lượng khách** theo từng nhóm hiện tại (valueTier, lifecycleStage, journeyStage, layer3 group).
3. **Đề xuất threshold** dựa trên percentile (P25, P50, P75, P90) để cân bằng kích thước nhóm.
4. **Mô phỏng** nếu thay đổi threshold → số khách mỗi nhóm thay đổi thế nào.

---

## 3. Phương án thiết kế script

### 3.1. Cấu trúc tổng quan

```
scripts/
├── analyze_layer_thresholds.go    # Script chính — aggregate từ crm_customers
├── reports/
│   └── BAO_CAO_THRESHOLD_LAYER_YYYYMMDD.md
```

**Chạy:** `cd api && go run ../scripts/analyze_layer_thresholds.go`

### 3.2. Nguồn dữ liệu

| Nguồn | Collection | Trường cần |
|-------|------------|------------|
| Metrics | `crm_customers` | `currentMetrics.raw`, `currentMetrics.layer1`, `currentMetrics.layer2` |
| Hoặc | `report_snapshots` (nếu có) | Aggregated metrics theo org |

**Lưu ý:** `crm_customers` có `currentMetrics` nested (raw, layer1, layer2). Cần extract:
- raw: totalSpent, orderCount, lastOrderAt, secondLastOrderAt, avgOrderValue, revenueLast30d, revenueLast90d, ordersLast30d, orderCountOnline, orderCountOffline, ownedSkuCount (hoặc ownedSkuQuantities), cancelledOrderCount, lastConversationAt, **totalMessages**, **conversationCount**, **conversationFromAds**, hasConversation, conversationTags…
- layer2: valueTier, lifecycleStage, journeyStage (để so sánh phân bố hiện tại)

### 3.3. Các section báo cáo đề xuất

#### Section 1: Tổng quan

- Tổng số crm_customers (có currentMetrics)
- Số khách có orderCount > 0 vs = 0
- Timezone, ngày chạy, database

#### Section 2: Phân phối totalSpent (Lớp 2 — valueTier)

| Chỉ số | Giá trị |
|--------|---------|
| Min, Max | |
| P25, P50, P75, P90, P95 | |
| Số khách nằm trong từng khoảng (0–1M, 1M–5M, 5M–20M, 20M–50M, 50M+) | |
| **Đề xuất threshold** | Dựa trên percentile (vd: P75 low, P50 medium, P25 high, P10 top) |

#### Section 3: Phân phối orderCount (Lớp 2 — loyalty, Lớp 3 — journey/VIP)

| orderCount | Số khách | Ghi chú |
|------------|----------|---------|
| 0 | | visitor/engaged |
| 1 | | first |
| 2–7 | | repeat |
| 8+ | | VIP (valueTier=top) |
| Biên 2, 5, 8, 12, 25, 40 | | So sánh với threshold hiện tại |

#### Section 4: Phân phối daysSinceLastOrder (Lớp 2 — lifecycleStage)

| Khoảng (ngày) | Số khách | lifecycleStage tương ứng |
|---------------|----------|--------------------------|
| 0–30 | | active |
| 31–90 | | cooling |
| 91–180 | | inactive |
| 181+ | | dead |

**Đề xuất:** P75, P90 của daysSince cho từng nhóm orderCount > 0.

#### Section 4b: Phân phối avgDaysBetweenOrders (Lớp 3 Repeat — repeatFrequency)

**Chỉ áp dụng cho khách repeat** (orderCount ≥ 2, có secondLastOrderAt): `avgDays = (lastOrderAt - secondLastOrderAt) / msPerDay`.

Khi có avgDays, repeatFrequency dùng ratio: early < 0.5×avgDays, on_track ≤ 1.5×avgDays, delayed ≤ 2×avgDays, overdue > 2×avgDays. Khi không có avgDays → fallback 7/45/90 ngày.

**Đề xuất:** P25, P50, P75 của avgDaysBetweenOrders → gợi ý điều chỉnh fallback 7/45/90.

#### Section 5: Phân phối avgOrderValue (Lớp 3 — First purchaseQuality)

| Khoảng (VNĐ) | Số khách (chỉ first: orderCount=1) |
|--------------|-------------------------------------|
| < 150.000 | entry |
| 150.000–500.000 | medium |
| ≥ 500.000 | high_aov |

**cancelledOrderCount** (First experienceQuality): cancelled > 0 → risk, = 0 → smooth. Có thể thống kê tỷ lệ khách first có cancelled > 0.

**Đề xuất:** P25, P50, P75 của AOV cho khách first.

#### Section 5b: Phân phối totalMessages (Lớp 3 Engaged — engagementDepth)

**Chỉ áp dụng cho khách engaged** (journeyStage=engaged, orderCount=0).

| Khoảng (số tin nhắn) | Số khách engaged | engagementDepth tương ứng |
|---------------------|-------------------|---------------------------|
| 0 | | light (hoặc thiếu data) |
| 1–3 | | light |
| 4–10 | | medium |
| 11+ | | deep |

**Đề xuất:** P25, P50, P75 của totalMessages cho khách engaged → gợi ý điều chỉnh biên light/medium/deep (hiện 3/10).

**Phân phối conversationCount** (số hội thoại): dùng để kiểm tra hasConversation, validate journeyStage engaged.

#### Section 5c: Phân phối ownedSkuCount (Lớp 3 Repeat/VIP)

| Khoảng (số SKU) | Số khách | Ý nghĩa |
|-----------------|----------|----------|
| 0–2 | | Repeat: single_category; VIP: single_line_vip |
| 3–7 | | Repeat: multi_category; VIP: multi_line_vip |
| 8+ | | VIP: full_portfolio_vip |

**Đề xuất:** P50, P75 của ownedSkuCount cho khách repeat/VIP → gợi ý biên 3, 7.

#### Section 6: Phân phối rev30/rev90 và spend momentum (Lớp 2 — momentumStage; Lớp 3 Repeat/VIP)

**rev30/rev90** (Lớp 2 momentumStage):

| Ratio | Số khách | momentumStage |
|-------|----------|---------------|
| > 0.5 | | rising |
| 0.2–0.5 | | stable |
| < 0.2 | | declining/lost |

**Spend momentum** (Lớp 3 Repeat/VIP — lastAOV vs avgOrderValue): `diff = (rev30/ordersLast30d - avgOrderValue) / avgOrderValue`. Cần ordersLast30d > 0.

| diff | Số khách | spendMomentum / spendTrend |
|------|----------|----------------------------|
| ≥ 15% | | upscaling |
| -15% ~ +15% | | stable |
| ≤ -15% | | downscaling |

#### Section 7: Phân bố hiện tại theo classification

| valueTier | Số khách | % |
|-----------|----------|---|
| new | | |
| low | | |
| medium | | |
| high | | |
| top | | |

| lifecycleStage | Số khách | % |
|----------------|----------|---|
| active | | |
| cooling | | |
| inactive | | |
| dead | | |

| journeyStage | Số khách | % |
|--------------|----------|---|
| visitor | | |
| engaged | | |
| first | | |
| repeat | | |
| promoter | | |
| blocked_spam | | |

#### Section 8: Mô phỏng thay đổi threshold (tùy chọn)

- Nếu valueLow = P50(totalSpent) thay vì 1M → số khách low/medium thay đổi?
- Nếu lifecycleActive = 45 thay vì 30 → số khách active/cooling thay đổi?

#### Section 9: Khuyến nghị

- Bảng tóm tắt threshold hiện tại vs đề xuất
- Ghi chú: cần xem xét theo từng org (nếu multi-tenant) hoặc theo ngành hàng

---

## 4. Công nghệ & Cách triển khai

### 4.1. MongoDB Aggregation

Dùng `$group`, `$bucket`, `$percentile` (MongoDB 7.0+) hoặc `$group` + sort + manual percentile.

**Ví dụ:** Phân phối totalSpent

```javascript
db.crm_customers.aggregate([
  { $match: { "currentMetrics.raw.totalSpent": { $exists: true } } },
  { $group: {
      _id: null,
      min: { $min: "$currentMetrics.raw.totalSpent" },
      max: { $max: "$currentMetrics.raw.totalSpent" },
      avg: { $avg: "$currentMetrics.raw.totalSpent" },
      count: { $sum: 1 }
  }}
])
```

Percentile: cần `$sort` + `$group` với `$push` rồi tính ở Go, hoặc dùng `$setWindowFields` (Mongo 5.0+).

### 4.2. Go script structure

```go
// analyze_layer_thresholds.go
// 1. Kết nối MongoDB (config từ env)
// 2. Aggregate từ crm_customers
// 3. Tính percentile (P25, P50, P75, P90) từ slice đã sort
// 4. Build markdown report
// 5. Ghi file scripts/reports/BAO_CAO_THRESHOLD_LAYER_YYYYMMDD.md
```

### 4.3. Filter theo org (tùy chọn)

- Có thể thêm flag `--org-id` để phân tích theo 1 org cụ thể.
- Mặc định: toàn bộ crm_customers (hoặc org có nhiều dữ liệu nhất).

---

## 5. Lộ trình triển khai

| Bước | Nội dung | Ước lượng |
|------|----------|-----------|
| 1 | Script cơ bản: aggregate totalSpent, orderCount, daysSinceLastOrder từ crm_customers | 1–2 ngày |
| 2 | Tính percentile, phân bucket, xuất Section 1–4, 4b (avgDaysBetweenOrders) | 1 ngày |
| 3 | Section 5–7: AOV, totalMessages (engaged), ownedSkuCount, rev30/rev90, spend momentum, phân bố classification | 1 ngày |
| 4 | Section 8–9: mô phỏng threshold, khuyến nghị | 0.5 ngày |
| 5 | Tùy chọn: filter theo org, chạy định kỳ | 0.5 ngày |

---

## 6. Tham chiếu

- [seed_rule_crm_system.go](../../api/internal/api/ruleintel/migration/seed_rule_crm_system.go) — Params Lớp 2
- [layer3.go](../../api/internal/api/report/layer3/layer3.go) — Constants Lớp 3
- [LAYER3_LOGIC_AUDIT.md](./LAYER3_LOGIC_AUDIT.md) — Rà soát logic Layer 3
- [report_data_linkage.go](../../scripts/report_data_linkage.go) — Mẫu script báo cáo
- [analyze_ads_peak_hours.go](../../scripts/analyze_ads_peak_hours.go) — Mẫu aggregate + markdown

---

## 7. Output mẫu (phần đầu)

```markdown
# BÁO CÁO PHÂN TÍCH THRESHOLD LAYER — CĂN CỨ ĐIỀU CHỈNH

**Ngày tạo:** 2026-03-18 14:00  
**Database:** folkform_auth  
**Nguồn:** crm_customers.currentMetrics

---

## 1. Tổng quan

| Chỉ số | Giá trị |
|--------|---------|
| Tổng crm_customers | 42841 |
| Có currentMetrics | 42841 |
| Có orderCount > 0 | 15234 |
| Có orderCount = 0 | 27607 |

## 2. Phân phối totalSpent (VNĐ)

| Chỉ số | Giá trị |
|--------|---------|
| Min | 50.000 |
| Max | 125.000.000 |
| P25 | 450.000 |
| P50 | 1.200.000 |
| P75 | 3.800.000 |
| P90 | 12.000.000 |
| P95 | 25.000.000 |

### Đề xuất valueTier (dựa trên percentile)

| Tier | Threshold hiện tại | Đề xuất (P-based) | Số khách (hiện) | Số khách (đề xuất) |
|------|---------------------|-------------------|-----------------|---------------------|
| low | 1.000.000 | P50 ≈ 1.200.000 | 8.234 | 7.891 |
| medium | 5.000.000 | P75 ≈ 3.800.000 | 4.112 | 5.234 |
| ...
```

---

*Tài liệu này là đề xuất phương án. Khi triển khai cần review lại với team và điều chỉnh theo nhu cầu thực tế.*
