# BÁO CÁO PHÂN TÍCH THRESHOLD LAYER — CĂN CỨ ĐIỀU CHỈNH

**Ngày tạo:** 2026-03-18 11:37  
**Database:** folkform_auth  
**Nguồn:** crm_customers.currentMetrics

---

## 1. TỔNG QUAN

| Chỉ số | Giá trị |
|--------|--------|
| Tổng crm_customers (có currentMetrics.raw) | 44551 |
| Có orderCount > 0 | 3565 |
| Có orderCount = 0 | 40986 |
| Engaged (journeyStage=engaged, orderCount=0) | 40680 |

## 2. PHÂN PHỐI TOTALSPENT (VNĐ) — Lớp 2 valueTier

| Chỉ số | Giá trị |
|--------|--------|
| Min | 0 |
| Max | 626562000 |
| P25 | 1470000 |
| P50 | 2274480 |
| P75 | 3810000 |
| P90 | 8820000 |
| P95 | 14916000 |

| Khoảng | Số khách |
|--------|----------|
| 0–1M | 151 |
| 1M–5M | 2731 |
| 5M–20M | 572 |
| 20M–50M | 85 |
| 50M+ | 26 |

## 3. PHÂN PHỐI ORDERCOUNT — Lớp 2 loyalty, Lớp 3 journey/VIP

| orderCount | Số khách | Ghi chú |
|------------|----------|--------|
| 0 | 40986 | visitor/engaged |
| 1 | 3106 | first |
| 2–7 | 452 | repeat |
| 8+ | 7 | VIP |

## 4. PHÂN PHỐI DAYSSINCELASTORDER — Lớp 2 lifecycleStage

| Chỉ số | Giá trị (ngày) |
|--------|----------------|
| Min | 0 |
| Max | 515 |
| P50 | 117 |
| P75 | 254 |
| P90 | 361 |

| Khoảng | Số khách |
|--------|----------|
| 0–30 (active) | 577 |
| 31–90 (cooling) | 969 |
| 91–180 (inactive) | 739 |
| 181+ (dead) | 1280 |

## 4b. PHÂN PHỐI AVGDAYSBETWEENORDERS — Lớp 3 Repeat repeatFrequency

| Chỉ số | Giá trị (ngày) |
|--------|----------------|
| Min | 0.0 |
| Max | 404.9 |
| P25 | 3.1 |
| P50 | 22.5 |
| P75 | 77.3 |

## 5. PHÂN PHỐI AVGORDERVALUE (First, orderCount=1) — Lớp 3 purchaseQuality

| Chỉ số | Giá trị (VNĐ) |
|--------|---------------|
| Min | 124000 |
| Max | 626562000 |
| P25 | 1469710 |
| P50 | 1940000 |
| P75 | 3031600 |

| Khoảng | Số khách |
|--------|----------|
| < 150k (entry) | 2 |
| 150k–500k (medium) | 12 |
| ≥ 500k (high_aov) | 3062 |

## 5b. PHÂN PHỐI TOTALMESSAGES — Lớp 3 Engaged engagementDepth

**Chỉ khách engaged (journeyStage=engaged, orderCount=0):**

| Chỉ số | Giá trị (số tin) |
|--------|------------------|
| Min | 0 |
| Max | 160 |
| P25 | 0 |
| P50 | 0 |
| P75 | 0 |

| Khoảng | Số khách engaged |
|--------|-------------------|
| 0 | 40627 |
| 1–3 (light) | 1 |
| 4–10 (medium) | 16 |
| 11+ (deep) | 36 |

**Toàn bộ (có totalMessages):** 44551 khách

## 5c. PHÂN PHỐI OWNEDSKUCOUNT — Lớp 3 Repeat/VIP productExpansion/productDiversity

| Khoảng | Số khách |
|--------|----------|
| 0–2 (single) | 459 |
| 3–7 (multi) | 0 |
| 8+ (full_portfolio) | 0 |

## 6. PHÂN PHỐI REV30/REV90 — Lớp 2 momentumStage

| Ratio | Số khách | momentumStage |
|-------|----------|---------------|
| > 0.5 (rising) | 558 | |
| 0.2–0.5 (stable) | 10 | |
| < 0.2 (declining/lost) | 963 | |

## 7. PHÂN BỐ HIỆN TẠI THEO CLASSIFICATION

| valueTier | Số khách | % |
|-----------|----------|---|
| new | 41137 | 92.3% |
| low | 2731 | 6.1% |
| medium | 572 | 1.3% |
| high | 85 | 0.2% |
| top | 26 | 0.1% |

| lifecycleStage | Số khách | % |
|----------------|----------|---|
| active | 578 | 1.3% |
| cooling | 970 | 2.2% |
| inactive | 737 | 1.7% |
| dead | 1280 | 2.9% |
| _unspecified | 40986 | 92.0% |

| journeyStage | Số khách | % |
|--------------|----------|---|
| visitor | 306 | 0.7% |
| engaged | 40680 | 91.3% |
| first | 3106 | 7.0% |
| repeat | 459 | 1.0% |

