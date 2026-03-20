# Phương Án Chuyển Module CRM Sang Rule Intelligence

**Ngày:** 2025-03-16  
**Tham chiếu:** [rule-intelligence.md](../02-architecture/core/rule-intelligence.md), [backend-module-map.md](../module-map/backend-module-map.md)

---

## 1. Tổng Quan

Rule Intelligence đã triển khai xong cho **domain ads**. Phương án này mở rộng sang **domain crm** — chuyển logic phân loại và signal từ code Go sang Logic Script trong Rule Engine.

**Lợi ích:**
- Logic không hardcode — dễ tune ngưỡng, thêm rule mới
- Traceability — mỗi output có rule_id, logic_version
- Đồng bộ kiến trúc với Ads

---

## 2. Logic CRM Hiện Tại (Rà Soát)

### 2.1 Derivation Rules — Phân Loại Khách (đã chuyển sang Rule Engine)

| Logic (trong LOGIC_CRM_CLASSIFICATION) | Input | Output | Ngưỡng |
|----------------------------------------|-------|--------|--------|
| valueTier | totalSpent | new \| low \| medium \| high \| top | 1M, 5M, 20M, 50M VNĐ |
| lifecycleStage | lastOrderAt | active \| cooling \| inactive \| dead | 30, 90, 180 ngày |
| journeyStage | orderCount, hasConversation, tags | visitor \| engaged \| blocked_spam \| first \| repeat \| promoter | — |
| channel | orderCountOnline, orderCountOffline | online \| offline \| omnichannel | — |
| loyaltyStage | orderCount | one_time \| repeat \| core | 2, 5 đơn |
| momentumStage | revenueLast30d, revenueLast90d, daysSince | rising \| stable \| declining \| lost | ratio 0.2, 0.5 |

**Entry points:** `GetClassificationFromCustomer(ctx, c)`, `ComputeClassificationFromMetricsOrRuleEngine`. **Callers:** `toProfileResponse`, `buildMetricsSnapshot`, Recalculate, Merge, RefreshMetrics.

### 2.2 Luồng Dữ Liệu

```
pc_pos_orders + fb_conversations
  → aggregateOrderMetricsForCustomer, aggregateConversationMetricsForCustomer
  → currentMetrics (totalSpent, orderCount, lastOrderAt, revenueLast30d, ...)
  → ComputeClassificationFromMetrics
  → valueTier, lifecycleStage, journeyStage, channel, loyaltyStage, momentumStage
```

### 2.3 Logic Chưa Có (Đề Xuất Trong Docs)

| Rule | Mô tả | from_layer | to_layer |
|------|-------|------------|----------|
| **repeat_gap_risk** | Khoảng cách mua lặp bất thường (lastOrderAt vs pattern) | crm_profile | flag |
| **trigger_follow_up** | Flag → recommendation trigger flow re-engagement | flag | flow_trigger |

---

## 3. Phương Án Triển Khai

### Phase 1: Derivation Rules — Classification (✅ Đã hoàn thành 2025-03-16)

**Mục tiêu:** Chuyển 6 hàm Compute* sang Logic Script.

| Bước | Hành động | Trạng thái |
|------|-----------|------------|
| 1 | Tạo schema `schema_crm_raw` (raw metrics từ aggregate) | ✅ |
| 2 | Tạo Logic Script `LOGIC_CRM_CLASSIFICATION` — gộp 6 hàm vào 1 script | ✅ |
| 3 | Tạo Output Contract `OUT_CRM_CLASSIFICATION` | ✅ |
| 4 | Tạo Rule Definition `RULE_CRM_CLASSIFICATION` | ✅ |
| 5 | Thêm `computeClassificationViaRuleEngine` trong crm.service | ✅ |
| 6 | Thay bằng Rule Engine — **không fallback**, nil → map rỗng | ✅ |
| 7 | Seed migration `seed_rule_crm_system.go` | ✅ |
| 8 | Xóa các hàm `Compute*` cũ (ComputeValueTier, ComputeLifecycleStage, ...) | ✅ |

**Vỏ tối thiểu:** `GetClassificationFromCustomer(ctx, c)`, `ComputeClassificationFromMetricsOrRuleEngine` — gọi Rule Engine. Callers: `toProfileResponse`, `buildMetricsSnapshot`, Recalculate, Merge, RefreshMetrics.

**Layers CRM:**
- `raw` — currentMetrics (totalSpent, orderCount, lastOrderAt, revenueLast30d, revenueLast90d, orderCountOnline, orderCountOffline, hasConversation, conversationTags)
- `crm_classification` — valueTier, lifecycleStage, journeyStage, channel, loyaltyStage, momentumStage

**Param Set:** Ngưỡng value (1M, 5M, 20M, 50M), lifecycle (30, 90, 180), loyalty (2, 5), momentum (0.2, 0.5) — cho phép tune theo org.

---

### Phase 2: Interpretation Rules — Signals (Ưu tiên trung bình)

**Mục tiêu:** Thêm flag/signal từ classification.

| Rule | Input | Output | Ghi chú |
|------|-------|--------|---------|
| **repeat_gap_risk** | lastOrderAt, orderCount, avgGapDays (nếu có) | flag | Khách repeat, gap > X ngày so với trung bình |
| **vip_at_risk** | valueTier=top, lifecycleStage=cooling/inactive | flag | VIP đang nguội |
| **new_repeat_candidate** | journeyStage=first, momentumStage=rising | flag | Khách mới có tiềm năng repeat |

*Lưu ý:* repeat_gap_risk cần định nghĩa rõ "bất thường" — có thể cần thêm avgGapDays từ cohort hoặc config.

---

### Phase 3: Execution Rules — Flow Trigger (Ưu tiên thấp)

**Mục tiêu:** Flag → recommendation (trigger flow).

| Rule | Input | Output | Ghi chú |
|------|-------|--------|---------|
| **trigger_follow_up** | flag repeat_gap_risk | recommendation { flowId, payload } | Module CRM nhận → gọi notifytrigger/flow |
| **trigger_vip_winback** | flag vip_at_risk | recommendation | Tương tự |

**Thực thi:** Module CRM nhận output → trigger flow re-engagement. Rule Engine chỉ trả output, không gọi API.

---

## 4. Cấu Trúc Kỹ Thuật

### 4.1 Domain & Layers

| Field | Giá trị CRM |
|-------|-------------|
| domain | `crm` |
| from_layer | `raw`, `crm_classification`, `flag` |
| to_layer | `crm_classification`, `flag`, `flow_trigger` |

### 4.2 Input Context (layers)

```json
{
  "raw": {
    "totalSpent": 15000000,
    "orderCount": 5,
    "lastOrderAt": 1700000000000,
    "revenueLast30d": 3000000,
    "revenueLast90d": 8000000,
    "orderCountOnline": 4,
    "orderCountOffline": 1,
    "hasConversation": true,
    "conversationTags": []
  }
}
```

### 4.3 Output Contract (crm_classification)

```json
{
  "valueTier": "high",
  "lifecycleStage": "active",
  "journeyStage": "repeat",
  "channel": "omnichannel",
  "loyaltyStage": "core",
  "momentumStage": "rising"
}
```

---

## 5. Thứ Tự Thực Hiện Đề Xuất

| Thứ tự | Phase | Công việc | Ước lượng |
|--------|-------|-----------|-----------|
| 1 | Phase 1 | Seed rule CRM (schema, logic, output, param, definition) | 1–2 ngày |
| 2 | Phase 1 | computeClassificationViaRuleEngine + tích hợp Recalculate/Refresh | 0.5 ngày |
| 3 | Phase 1 | Bỏ fallback, xóa hàm Compute* cũ (sau khi validate) | 0.5 ngày |
| 4 | Phase 2 | repeat_gap_risk, vip_at_risk (nếu có spec) | 1 ngày |
| 5 | Phase 3 | trigger_follow_up (khi flow engine sẵn sàng) | 0.5 ngày |

---

## 6. Rủi Ro & Giảm Thiểu

| Rủi ro | Giảm thiểu |
|--------|-------------|
| Logic script khác Go | So sánh output Rule Engine vs Compute* trên sample data — regression test |
| Ngưỡng hardcode trong script | Đưa vào Param Set, đọc từ params |
| Performance | Script đơn giản, timeout 100ms đủ |

---

## 7. Tài Liệu Tham Chiếu

- [rule-intelligence-overview.md](../../../docs/ai-context/folkform/design/rule-intelligence/rule-intelligence-overview.md) — tổng quan dùng chung (docs-shared)
- [rule-intelligence.md](../02-architecture/core/rule-intelligence.md) — kiến trúc Rule Intelligence (backend)
- [CUSTOMER_CLASSIFICATION_SYSTEM_DESIGN](../../../docs/ai-context/folkform/design/CUSTOMER_CLASSIFICATION_SYSTEM_DESIGN.md) — design phân loại 2 lớp (docs-shared)
- [seed_rule_crm_system.go](../../api/internal/api/ruleintel/migration/seed_rule_crm_system.go) — seed CRM

---

## Changelog

- 2025-03-16: Tạo phương án ban đầu
- 2025-03-16: Phase 1 hoàn thành — seed RULE_CRM_CLASSIFICATION, vỏ `GetClassificationFromCustomer` / `ComputeClassificationFromMetricsOrRuleEngine`, xóa các hàm Compute* cũ, không fallback
