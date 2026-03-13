# Ads Metrics Pipeline — Raw → Layer1 → Layer2 → Layer3 → Flag → Action

**Mục đích:** Tài liệu mô tả pipeline biến đổi dữ liệu Meta Ads từ raw đến action, trích xuất từ codebase.

**Liên quan:** [Rule Intelligence](rule-intelligence.md), [FolkForm AI Agent Master Rules v4.1](../../../docs-shared/ai-context/folkform/design/ads-intelligence/FolkForm%20AI%20Agent%20Master%20Rules%20v4.1.md)

---

## Tổng Quan Pipeline

```
Raw (meta, pancake.pos, pancake.conversation)
  → computeLayer1
  → Layer 1 (metrics cơ bản)
  → computeLayer2
  → Layer 2 (scores 0–100)
  → computeLayer3
  → Layer 3 (CHS, healthState, portfolioCell)
  → EvaluateFlags (flag_evaluator)
  → Flag (alertFlags[])
  → EvaluateForKill / EvaluateForDecrease / EvaluateForIncrease
  → Action (PAUSE, DECREASE, INCREASE, RESUME, SET_BUDGET)
```

**Vị trí code:** `api/internal/api/meta/service/service.meta.evaluation.go`, `api/internal/api/ads/rules/`, `api/internal/api/ads/config/metadata.go`

---

## 1. Raw — Dữ Liệu Gốc

Dữ liệu chưa biến đổi, lấy từ 3 nguồn theo window thời gian.

### 1.1 Cấu Trúc

```json
{
  "7d": {
    "meta": { "spend", "impressions", "clicks", "mess", "inlineLinkClicks", "frequency", "cpm", "ctr", "cpc", ... },
    "pancake": {
      "pos": { "orders", "revenue" },
      "conversation": { "mess", "orders" }
    },
    "window": { "dateStart", "dateStop" },
    "metaCreatedAt": 1234567890000
  },
  "2h": { "orders", "mess", "revenue" },
  "1h": { "orders", "mess", "revenue" },
  "30p": { "orders", "mess", "revenue" }
}
```

### 1.2 Nguồn Dữ Liệu

| Window | Nguồn Meta | Nguồn Pancake | Collection |
|--------|------------|---------------|------------|
| 7d | meta_ad_insights | pc_pos_orders, fb_conversations | meta_ad_insights, pc_pos_orders, fb_conversations |
| 2h | — | pc_pos_orders, fb_conversations | Conv_Rate_now (Momentum Tracker) |
| 1h | — | pc_pos_orders, fb_conversations | HB-3 Divergence |
| 30p | — | fb_conversations | MQS, Msg_Rate early warning |

### 1.3 Fields raw.meta (7d)

| Field | Mô tả | Đơn vị |
|-------|-------|--------|
| spend | Chi phí quảng cáo | VND |
| impressions | Số lần hiển thị | số |
| clicks | Số click | số |
| mess | messaging_conversation_started | số |
| inlineLinkClicks | Click link (tính msg_rate) | số |
| frequency | Số lần TB mỗi user thấy quảng cáo | số |
| cpm | Cost per 1000 impressions | VND |
| ctr | Click-through rate | % |
| cpc | Cost per click | VND |

### 1.4 Fields raw.pancake.pos (7d)

| Field | Mô tả | Đơn vị |
|-------|-------|--------|
| orders | Số đơn từ Pancake | số |
| revenue | Doanh thu đơn hàng | VND |

---

## 2. Layer 1 — Metrics Cơ Bản

**Hàm:** `computeLayer1(raw)` — `service.meta.evaluation.go`

Biến đổi raw thành metrics nghiệp vụ theo FolkForm v4.1.

### 2.1 Output Layer 1

| Field | Công thức | Nguồn | Đơn vị |
|-------|-----------|-------|--------|
| lifecycle | metaCreatedAt → NEW/WARMING/CALIBRATED/MATURE | raw.7d | — |
| msgRate_7d | mess / inlineLinkClicks | raw.7d | tỷ lệ |
| msgRate_30p | mess_30p / (clicks_7d/336) | raw.30p, raw.7d | tỷ lệ |
| mess_30p | mess trong 30p | raw.30p | số |
| cpaMess_7d | spend / mess | raw.7d | VND |
| cpaPurchase_7d | spend / orders | raw.7d | VND |
| convRate_7d | orders / mess | raw.7d | tỷ lệ 0–1 |
| convRate_2h | orders_2h / mess_2h | raw.2h | tỷ lệ |
| convRate_1h | orders_1h / mess_1h | raw.1h | tỷ lệ |
| roas_7d | revenue / spend | raw.7d | số |
| mqs_7d | mess × convRate_7d × timeFactor | raw.7d, raw.30p | số |
| spendPct_7d | spend / dailyBudget | raw.7d | tỷ lệ |
| runtimeMinutes | (now - metaCreatedAt) / 60000 | raw.7d | phút |

### 2.2 Lifecycle (theo metaCreatedAt)

| Giai đoạn | Điều kiện | Mô tả |
|-----------|-----------|-------|
| NEW | < 7 ngày | Camp mới, chưa đủ data |
| WARMING | 7–14 ngày | Giai đoạn 1 |
| CALIBRATED | 14–30 ngày | Giai đoạn 2, đủ data adaptive |
| MATURE | 30+ ngày | Giai đoạn 3 |

### 2.3 MQS (Mess Quality Score)

`mqs_7d = mess × convRate_7d × timeFactor`

**Time Factor (Asia/Ho_Chi_Minh):**

| Khung giờ | Time Factor |
|-----------|-------------|
| 07:00–11:59 | × 1.2 |
| 12:00–16:59 | × 1.0 |
| 17:00–19:59 | × 0.8 |
| 20:00–22:29 | × 0.5 |
| Khác | × 1.0 |

---

## 3. Layer 2 — Scores Trung Gian

**Hàm:** `computeLayer2(raw, layer1)` — `service.meta.evaluation.go`

5 trục điểm 0–100, đầu vào cho CHS.

### 3.1 Output Layer 2

| Field | Công thức | Nguồn |
|-------|-----------|-------|
| efficiency | scoreFromRoas(roas_7d) | layer1 |
| demandQuality | scoreFromRate(msgRate_7d, convRate_7d) | layer1 |
| auctionPressure | scoreFromCpmCtr(cpm, ctr) | raw.meta |
| saturation | scoreFromFrequency(frequency) | raw.meta |
| momentum | 50 (TODO: cần trend data) | — |

### 3.2 Công Thức Score

| Hàm | Ngưỡng | Điểm |
|-----|--------|------|
| scoreFromRoas | roas ≥ 3 | 100 |
| | roas ≥ 2 | 80 |
| | roas ≥ 1 | 60 |
| | roas ≥ 0.5 | 40 |
| | < 0.5 | 20 |
| scoreFromRate | (msgRate×50 + convRate×50)/2, cap 100 | 0–100 |
| scoreFromCpmCtr | cpm < 50k, ctr > 1% | 80 |
| | cpm < 100k, ctr > 0.5% | 60 |
| | khác | 40 |
| scoreFromFrequency | freq ≤ 2 | 80 |
| | freq ≤ 4 | 60 |
| | > 4 | 40 |

---

## 4. Layer 3 — Trạng Thái Tổng Hợp

**Hàm:** `computeLayer3(layer1, layer2)` — `service.meta.evaluation.go`

### 4.1 Output Layer 3

| Field | Công thức | Mô tả |
|-------|-----------|-------|
| chs | (eff + demand + auction + sat + mom) / 5 | Campaign Health Score 0–100 |
| healthState | từ CHS | strong / healthy / warning / critical |
| performanceTier | từ roas_7d | high / medium / low |
| portfolioCell | derivePortfolioCell(lifecycle, performanceTier) | test / potential / scale / maintain / fix / recover |
| stage | "stable" | — |
| diagnoses | [] | Chuẩn đoán động |

### 4.2 healthState (từ CHS)

| CHS | healthState |
|-----|-------------|
| ≥ 80 | strong |
| ≥ 60 | healthy |
| ≥ 40 | warning |
| < 40 | critical |

### 4.3 performanceTier (từ roas_7d)

| ROAS | performanceTier |
|------|-----------------|
| ≥ 3 | high |
| ≥ 1.5 | medium |
| < 1.5 | low |

### 4.4 portfolioCell (lifecycle × performanceTier)

| Lifecycle | high | medium | low |
|-----------|------|--------|-----|
| NEW | test | test | test |
| WARMING | potential | test | test |
| CALIBRATED | scale | maintain | fix |
| MATURE | scale | maintain | recover |

---

## 5. Flag — Cờ Cảnh Báo

**Hàm:** `EvaluateFlags(ctx, FactsContext, cfg, campCtx)` — `ads/rules/flag_evaluator.go`

Điều kiện: so sánh fact với threshold (config) → set flag khi match.

### 5.1 FactsContext (đầu vào)

Chứa: raw.meta, raw.pancake.pos, layer1, layer3, InTrimWindow, CurrentMode, Diagnoses.

### 5.2 Nhóm Flag (theo metadata)

| Nhóm | Ví dụ flags |
|------|-------------|
| metric | cpa_mess_high, cpa_purchase_high, conv_rate_low, ctr_critical, msg_rate_low, cpm_low, cpm_high, frequency_high |
| chs | chs_critical, chs_warning |
| stop_loss | sl_a, sl_a_decrease, sl_b, sl_c, sl_d, sl_e |
| kill_off | ko_a, ko_b, ko_c |
| mess_trap | mess_trap_suspect |
| trim | trim_eligible, trim_eligible_decrease |
| exception | safety_net, conv_rate_strong |
| increase | increase_eligible |
| portfolio | portfolio_attention |
| morning_on | mo_eligible |
| noon_cut | noon_cut_eligible |

### 5.3 Dynamic Flags

`diagnosis_<value>` — mỗi phần tử trong `layer3.diagnoses[]` tạo 1 cờ.

### 5.4 Operators

GREATER_THAN, LESS_THAN, GREATER_THAN_OR_EQUAL, LESS_THAN_OR_EQUAL, EQUAL, NOT_EQUAL, IN, NOT_IN

---

## 6. Action — Hành Động Đề Xuất

**Hàm:** `EvaluateForKill`, `EvaluateForDecrease`, `EvaluateForIncrease` — `ads/rules/engine.go`

Map flag → action (PAUSE, DECREASE, INCREASE, RESUME, SET_BUDGET).

### 6.1 Action Types (constants.go)

| ActionType | Mô tả |
|------------|-------|
| KILL | Tạm dừng (status=PAUSED) |
| PAUSE | Tạm dừng |
| RESUME | Bật lại |
| DECREASE | Giảm budget theo % |
| INCREASE | Tăng budget theo % |
| SET_BUDGET | Đặt budget cố định |
| CIRCUIT_BREAK_PAUSE | PAUSE toàn account (Circuit Breaker) |

### 6.2 Flag → Action (mặc định)

| Flag | Action | Value | Ghi chú |
|------|--------|-------|---------|
| sl_a, sl_b, sl_c, sl_d, sl_e | PAUSE | — | Stop Loss |
| chs_critical | PAUSE | — | CHS Kill |
| ko_a, ko_b, ko_c | PAUSE | — | Kill Off |
| trim_eligible | PAUSE | — | Trim Kill |
| sl_a_decrease | DECREASE | 20% | MQS ≥ 2 |
| mess_trap_suspect | DECREASE | 30% | AutoApprove |
| trim_eligible_decrease | DECREASE | 30% | AutoApprove |
| chs_warning + cpa_mess_high | DECREASE | 15% | Compound |
| increase_eligible | INCREASE | 30% | Camp tốt |
| safety_net | INCREASE | 35% | Bảo vệ camp |

### 6.3 Exception Flags

- **safety_net**, **conv_rate_strong** → bỏ qua kill (không PAUSE dù có flag kill)
- **conv_rate_strong** → bỏ qua decrease

---

## 7. Luồng Thực Tế Trong Code

### 7.1 Cập nhật currentMetrics (Ad)

```
updateRawAndLayersForAd()
  → fetch raw 7d (meta + pancake.pos + pancake.conversation)
  → fetch raw 2h, 1h, 30p
  → computeLayer1(raw)
  → computeLayer2(raw, layer1)
  → computeLayer3(layer1, layer2)
  → current = { raw, layer1, layer2, layer3, alertFlags: [], actions: [] }
```

**Lưu ý:** 13 rules CHỈ apply cho **campaign** — Ad không có alertFlags. Campaign rollup từ Ad, rồi gọi EvaluateFlags.

### 7.2 Đánh giá Flag (Campaign)

```
BuildFactsContext(raw, layer1, layer2, layer3)
  → EvaluateFlags(ctx, FactsContext, cfg, campCtx)
  → alertFlags[] = ["sl_a", "cpa_mess_high", ...]
```

### 7.3 Đề xuất Action (Auto Propose)

```
EvaluateForKill(flags, cfg) → action PAUSE
EvaluateForDecrease(flags, cfg) → action DECREASE
EvaluateForIncrease(flags, cfg) → action INCREASE
  → Tạo proposal (approval workflow)
  → ExecuteAdsAction (gọi Meta API)
```

---

## 8. Vị Trí Code

| Thành phần | File |
|------------|------|
| computeLayer1/2/3, raw fetch | `api/internal/api/meta/service/service.meta.evaluation.go` |
| BuildFactsContext, EvaluateFlags, EvaluateCondition | `api/internal/api/ads/rules/flag_evaluator.go` |
| EvaluateForKill/Decrease/Increase | `api/internal/api/ads/rules/engine.go` |
| FlagDefinitions, ThresholdDefinitions, ActionRuleSpecs | `api/internal/api/ads/config/metadata.go` |
| Action types | `api/internal/api/ads/constants.go` |
| Executor (gọi Meta API) | `api/internal/api/ads/service/service.ads.executor.go` |

---

## 9. Tham Chiếu

- [Rule Intelligence](rule-intelligence.md) — Kiến trúc Rule Engine, migration từ logic hiện tại
- [FolkForm AI Agent Master Rules v4.1](../../../docs-shared/ai-context/folkform/design/ads-intelligence/FolkForm%20AI%20Agent%20Master%20Rules%20v4.1.md) — Business rules, KPI target, CHS, MQS
