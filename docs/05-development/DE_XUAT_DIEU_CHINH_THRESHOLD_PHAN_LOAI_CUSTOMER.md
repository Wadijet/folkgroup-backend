# Đề Xuất: Điều Chỉnh Threshold Phân Loại Customer

> **Căn cứ:** Báo cáo [BAO_CAO_THRESHOLD_LAYER_20260318.md](../../scripts/reports/BAO_CAO_THRESHOLD_LAYER_20260318.md), [BAO_CAO_KHUNG_GIO_CAO_DIEM_20260318.md](../../scripts/reports/BAO_CAO_KHUNG_GIO_CAO_DIEM_20260318.md)  
> **Ngày:** 2026-03-18  
> **Mục đích:** Đề xuất điều chỉnh các ngưỡng (threshold) trong phân loại khách hàng dựa trên dữ liệu thực tế từ báo cáo phân tích threshold layer.

---

## 1. Tóm Tắt Dữ Liệu Từ Báo Cáo

### 1.1. Tổng Quan (BAO_CAO_THRESHOLD_LAYER 2026-03-18)

| Chỉ số | Giá trị |
|--------|---------|
| Tổng crm_customers (có currentMetrics.raw) | 44.551 |
| Có orderCount > 0 | 3.565 |
| Có orderCount = 0 | 40.986 |
| Engaged (journeyStage=engaged, orderCount=0) | 40.680 |

### 1.2. Phân Phối Chính (từ báo cáo threshold)

| Chỉ tiêu | P25 | P50 | P75 | P90 | Ghi chú |
|----------|-----|-----|-----|-----|---------|
| totalSpent (VNĐ) | 1.470.000 | 2.274.480 | 3.810.000 | 8.820.000 | Chỉ khách có đơn |
| daysSinceLastOrder | — | 117 | 254 | 361 | Ngày |
| avgOrderValue (First) | 1.469.710 | 1.940.000 | 3.031.600 | — | orderCount=1 |
| avgDaysBetweenOrders | 3,1 | 22,5 | 77,3 | — | Repeat |

**Phân bố totalSpent:** 0–1M: 151 | 1M–5M: 2.731 | 5M–20M: 572 | 20M–50M: 85 | 50M+: 26

**Phân bố First AOV:** &lt;150k (entry): 2 | 150k–500k (medium): 12 | ≥500k (high_aov): 3.062

**Engaged totalMessages:** 40.627 có 0 tin | 1–3 (light): 1 | 4–10 (medium): 16 | 11+ (deep): 36

### 1.3. Phân Tích Conversation → Chốt Đơn (báo khung giờ 09/03)

**Lưu ý:** Báo cáo 20260318 báo "Đơn link được: 0" — có thể lỗi aggregate hoặc thay đổi cấu trúc link. Dùng data từ báo cáo 20260309 làm tham chiếu:

| Chỉ số | Giá trị (báo 09/03) |
|--------|---------------------|
| Đơn có conversation_id link được | 2.150 |
| **TB giờ chăm khách → chốt đơn** | **288,5 giờ** (~12 ngày) |
| **Trung vị giờ chăm khách → chốt đơn** | **5,8 giờ** |

**Phân bố thời gian chăm khách (conv đầu → chốt đơn):**

| Khoảng | Số đơn | Tỷ lệ |
|--------|--------|-------|
| <1h | 512 | 23,8% |
| 1–4h | 487 | 22,7% |
| 4–24h | 443 | 20,6% |
| 1–3 ngày | 252 | 11,7% |
| 3–7 ngày | 144 | 6,7% |
| >7 ngày | 312 | 14,5% |

**Insight:** ~67% khách chốt đơn trong vòng 24h từ lúc bắt đầu conversation; ~12% trong 1–3 ngày; ~7% trong 3–7 ngày.

### 1.4. Conversion Rate Theo Khung Giờ (báo 18/03)

- **Cao điểm (CR 10–12%):** 10h–12h, 14h, 16h–18h, 22h (VN)
- **Trung bình (CR 7–10%):** 08h–09h, 13h, 19h–21h
- **Thấp (CR <7%):** 00h–07h, 15h, 23h

---

## 2. Threshold Hiện Tại (Code)

### 2.1. Rule Engine RULE_CRM_CLASSIFICATION (seed_rule_crm_system.go)

| Tham số | Giá trị mặc định | Mô tả |
|---------|------------------|-------|
| valueVip | 50.000.000 | Ngưỡng top (VIP) |
| valueHigh | 20.000.000 | Ngưỡng high |
| valueMedium | 5.000.000 | Ngưỡng medium |
| valueLow | 1.000.000 | Ngưỡng low |
| lifecycleActive | 30 | Ngày không mua → active |
| lifecycleCooling | 90 | Ngày không mua → cooling |
| lifecycleInactive | 180 | Ngày không mua → inactive |
| loyaltyCore | 5 | Đơn → core |
| loyaltyRepeat | 2 | Đơn → repeat |
| momentumRising | 0,5 | rev30/rev90 > 0,5 → rising |
| momentumStableLo | 0,2 | 0,2–0,5 → stable |
| momentumStableHi | 0,5 | — |

### 2.2. Layer 3 (layer3.go)

| Hằng số | Giá trị | Mô tả |
|---------|---------|-------|
| firstHighAOV | 500.000 | AOV First → high_aov |
| firstEntryAOV | 150.000 | AOV First → entry |
| firstReorderTooEarly | 7 | Ngày → too_early |
| firstReorderExpectedMax | 60 | Ngày → within_expected |
| repeatFreqEarlyMax | 7 | Repeat frequency early |
| repeatFreqOnTrackMax | 45 | Repeat frequency on_track |
| repeatFreqDelayedMax | 90 | Repeat frequency delayed |
| repeatSkuMulti | 3 | productExpansion: sku ≥ 3 → multi_category |
| repeatSpendMomentum | ±15% | lastAOV vs avgOrderValue → upscaling/downscaling |
| repeatUpgradePotential | totalSpend ≥ 2M | +1 điểm; score 6/2 → high/low |
| vipRiskScore | days > 60 / > 90 | daysSinceLastOrder: -1 / -2 điểm |
| inactiveReactivationPotential | oc ≥ 2 / ≥ 8 | +1 / +2 điểm; score 7/3 → high/low |
| **Engaged nhiệt độ** | 1 / 3 / 7 ngày | hot / warm / cooling / cold |
| **Engaged độ sâu (totalMessages)** | 3 / 10 tin nhắn | light ≤3 / medium ≤10 / deep >10 |

---

## 3. Đề Xuất Điều Chỉnh

### 3.1. Engaged — Nhiệt Độ Hội Thoại (Conversation Temperature)

**Hiện tại:** hot ≤1d, warm ≤3d, cooling ≤7d, cold >7d

**Căn cứ báo cáo:** 67% chốt trong 24h, 12% trong 1–3 ngày, 7% trong 3–7 ngày.

| Đề xuất | Lý do |
|---------|-------|
| **Giữ nguyên** 1 / 3 / 7 ngày | Phân bố thực tế phù hợp: hot = đang nóng (≤1d), warm = còn ấm (1–3d), cooling = nguội dần (3–7d), cold = lạnh (>7d). |

**Không cần thay đổi.**

---

### 3.2. Engaged — Độ Sâu Tương Tác (Engagement Depth)

**Hiện tại:** light ≤3 msg, medium ≤10 msg, deep >10 msg

**Căn cứ báo cáo threshold:** 40.627/40.680 engaged có totalMessages=0; chỉ 53 có 1+ tin (1 light, 16 medium, 36 deep).

| Đề xuất | Lý do |
|---------|-------|
| **Giữ nguyên** 3 / 10 | Phần lớn engaged thiếu totalMessages (có thể chưa aggregate từ fb_conversations). 53 khách có tin phân bố: 1 light, 16 medium, 36 deep — biên hiện tại hợp lý. |
| **Rà soát:** Kiểm tra Recalculate/conversation_metrics có ghi totalMessages vào raw cho engaged không. | Nếu 40k engaged thực sự có 0 tin → engagementDepth=light là đúng. |

**Khuyến nghị:** Giữ nguyên; ưu tiên rà soát pipeline aggregate totalMessages.

---

### 3.3. First — Reorder Timing

**Hiện tại:** too_early <7d, within_expected 7–60d, overdue >60d

**Căn cứ báo cáo threshold:** avgDaysBetweenOrders (Repeat): P25=3,1d, P50=22,5d, P75=77,3d. Fallback 7/45/90 tương ứng early/on_track/delayed.

| Đề xuất | Lý do |
|---------|-------|
| **Giữ nguyên** 7 / 60 ngày | P25=3,1 gần 7; P50=22,5 < 45; P75=77,3 gần 90. Biên fallback 7/45/90 phù hợp với phân phối thực tế. |

---

### 3.4. Lifecycle (Active / Cooling / Inactive / Dead)

**Hiện tại:** active ≤30d, cooling ≤90d, inactive ≤180d, dead >180d

**Căn cứ báo cáo threshold:** daysSinceLastOrder P50=117, P75=254, P90=361. Phân bố: active 577 | cooling 969 | inactive 739 | dead 1.280.

| Đề xuất | Lý do |
|---------|-------|
| **Giữ nguyên** 30 / 90 / 180 | Phân bố cân đối; P50=117 nằm trong cooling, P75=254 trong inactive. Biên hiện tại phù hợp. |

---

### 3.5. Value Tier (totalSpent)

**Hiện tại:** low 1M, medium 5M, high 20M, top 50M

**Căn cứ báo cáo threshold:** P25=1,47M, P50=2,27M, P75=3,81M, P90=8,82M. Phân bố: 1M–5M có 2.731 khách (77% khách có đơn).

| Đề xuất | Lý do |
|---------|-------|
| **Tùy chọn:** valueLow = 1,5M (P25), valueMedium = 3,8M (P75) | Cân bằng nhóm low/medium theo percentile. Hiện 2.731 low (1M–5M) — nếu nâng low lên 1,5M sẽ chuyển ~151 (0–1M) sang new. |
| **Giữ nguyên** nếu muốn ổn định | 1M/5M/20M/50M đã dùng lâu, thay đổi ảnh hưởng filter/dashboard. |

**Khuyến nghị:** Giữ nguyên; nếu điều chỉnh → thử valueLow=1,5M trước.

---

### 3.6. First — Purchase Quality (AOV)

**Hiện tại:** high_aov ≥500k, entry <150k, medium 150k–500k

**Căn cứ báo cáo threshold:** First AOV: 2 entry, 12 medium, 3.062 high_aov. P25=1,47M, P50=1,94M, P75=3,03M.

| Đề xuất | Lý do |
|---------|-------|
| **Giữ nguyên** 150k / 500k | 98,5% First đã ở high_aov (≥500k). Biên 150k/500k phù hợp (chỉ 14 khách entry+medium). |

---

## 4. Tổng Hợp Đề Xuất

| Hạng mục | Điều chỉnh | Ưu tiên |
|----------|-----------|---------|
| Engaged nhiệt độ | Giữ nguyên 1/3/7 ngày | — |
| Engaged độ sâu | Giữ nguyên 3/10; rà soát pipeline totalMessages | Trung bình |
| First reorder timing | Giữ nguyên 7/60 ngày | — |
| Lifecycle | Giữ nguyên 30/90/180 ngày | — |
| Value tier | Tùy chọn: valueLow=1,5M (P25); giữ nguyên nếu ổn định | Thấp |
| First AOV | Giữ nguyên 150k/500k | — |

---

## 5. Bước Tiếp Theo

1. ~~**Engaged depth:** Chạy script phân tích totalMessages~~ → **Đã có báo cáo.** Rà soát pipeline Recalculate/conversation_metrics có ghi totalMessages vào raw cho engaged.
2. ~~**Repeat/First reorder, Lifecycle, Value/AOV, ownedSkuCount, avgDaysBetweenOrders**~~ → **Đã có báo cáo threshold.** Đề xuất: giữ nguyên hầu hết.
3. **Nếu điều chỉnh valueLow:** Cập nhật PARAM_CRM_CLASSIFICATION (ruleintel DB) valueLow=1.500.000 → chạy Recalculate.
4. **Định kỳ:** Chạy `go run ../scripts/analyze_layer_thresholds.go` mỗi quý → cập nhật đề xuất.

---

## 6. Tham Chiếu

- [BAO_CAO_THRESHOLD_LAYER_20260318.md](../../scripts/reports/BAO_CAO_THRESHOLD_LAYER_20260318.md) — báo cáo phân tích threshold (căn cứ chính)
- [BAO_CAO_KHUNG_GIO_CAO_DIEM_20260318.md](../../scripts/reports/BAO_CAO_KHUNG_GIO_CAO_DIEM_20260318.md) — báo cáo khung giờ
- [BAO_CAO_KHUNG_GIO_CAO_DIEM_20260309.md](../../scripts/reports/BAO_CAO_KHUNG_GIO_CAO_DIEM_20260309.md) — có data "Conversation trước chốt đơn"
- [FIRST_PURCHASE_INTELLIGENCE.md](../02-architecture/FIRST_PURCHASE_INTELLIGENCE.md)
- [ENGAGED_INTELLIGENCE_LAYER_EVALUATION.md](../02-architecture/ENGAGED_INTELLIGENCE_LAYER_EVALUATION.md)
- `api/internal/api/report/layer3/layer3.go`
- `api/internal/api/ruleintel/migration/seed_rule_crm_system.go`
