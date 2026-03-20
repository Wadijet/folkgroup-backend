# Đánh giá Layer 3 Intelligence — Logic & Nguồn dữ liệu

> Kiểm tra các tiêu chí Lớp 3 (First, Repeat, VIP, Inactive, Engaged) có chạy đúng logic và lên số liệu hay không.

**Ngày tạo:** 2025-03-17

---

## 1. Tổng quan luồng dữ liệu

```
pc_pos_orders + fb_conversations
        ↓
aggregateOrderMetricsForCustomer + aggregateConversationMetricsForCustomer
        ↓
BuildCurrentMetricsFromOrderAndConv → currentMetrics (raw, layer1, layer2, layer3)
        ↓
crm_customers.currentMetrics (lưu DB)
        ↓
ListCustomersForDashboard → toDashboardItem (GetXXFromCustomer)
        ↓
crmItemToLayer3Map → layer3.DeriveFromMap
        ↓
CustomerItem.first / repeat / vip / inactive / engaged
```

**Lưu ý:** `unsetRawFields` đã chuyển `revenueLast30d`, `ordersLast30d`, `lastConversationAt`, `totalMessages`, `conversationFromAds`... **chỉ còn trong currentMetrics.raw**. Top-level không còn các field này. Nếu `currentMetrics` = nil hoặc thiếu raw → fallback top-level = 0.

---

## 2. Bảng ánh xạ: Tiêu chí ↔ Nguồn dữ liệu ↔ Rủi ro

### 2.1. First (journeyStage=first, orderCount=1)

| Tiêu chí | Key layer3 cần | Nguồn (toDashboardItem) | Rủi ro |
|----------|-----------------|--------------------------|--------|
| purchaseQuality | avgOrderValue | AvgOrderValue = totalSpent/orderCount (tính local) | ✅ OK — luôn có |
| experienceQuality | cancelledOrderCount | GetIntFromCustomer | ⚠️ Chỉ có trong currentMetrics.raw |
| engagementAfterPurchase | lastConversationAt, lastOrderAt | GetInt64FromCustomer | ⚠️ lastConversationAt từ conv aggregate — cần link conv↔customer |
| reorderTiming | daysSinceLast (từ lastOrderAt) | LastOrderAtMs | ✅ OK |
| repeatProbability | Composite từ 4 trên | — | Phụ thuộc 4 tiêu chí trên |

### 2.2. Repeat (journeyStage=repeat, orderCount 2–7)

| Tiêu chí | Key layer3 cần | Nguồn | Rủi ro |
|----------|----------------|-------|--------|
| repeatDepth | orderCount | OrderCount | ✅ OK |
| repeatFrequency | secondLastOrderAt, lastOrderAt, daysSinceLast | SecondLastOrderAt | ⚠️ secondLastOrderAt chỉ có trong currentMetrics.raw |
| spendMomentum | avgOrderValue, revenueLast30d, ordersLast30d | GetFloat/GetInt | ⚠️ revenueLast30d, ordersLast30d chỉ trong currentMetrics |
| productExpansion | ownedSkuCount | len(OwnedSkuQuantities) | ✅ OK — từ top-level ownedSkuQuantities |
| emotionalEngagement | lastConversationAt, lastOrderAt | GetInt64FromCustomer | ⚠️ Cần conv metrics |
| upgradePotential | Composite | — | Phụ thuộc các tiêu chí trên |

### 2.3. VIP (valueTier=top, orderCount≥8)

| Tiêu chí | Key layer3 cần | Nguồn | Rủi ro |
|----------|----------------|-------|--------|
| vipDepth | orderCount | OrderCount | ✅ OK |
| spendTrend | avgOrderValue, revenueLast30d, ordersLast30d | GetFloat/GetInt | ⚠️ Chỉ trong currentMetrics |
| productDiversity | ownedSkuCount | len(OwnedSkuQuantities) | ✅ OK |
| engagementLevel | lastConversationAt, lastOrderAt | GetInt64FromCustomer | ⚠️ Cần conv |
| riskScore | Composite + lifecycleStage | — | Phụ thuộc spendTrend, engagement |

### 2.4. Inactive (lifecycleStage ∈ cooling|inactive|dead)

| Tiêu chí | Key layer3 cần | Nguồn | Rủi ro |
|----------|----------------|-------|--------|
| engagementDrop | lastConversationAt, lastOrderAt | GetInt64FromCustomer | ⚠️ Cần conv |
| reactivationPotential | valueTier, lifecycle, orderCount, engagementDrop | — | Phụ thuộc engagementDrop |

### 2.5. Engaged (journeyStage=engaged, orderCount=0)

| Tiêu chí | Key layer3 cần | Nguồn | Rủi ro |
|----------|----------------|-------|--------|
| conversationTemperature | lastConversationAt | GetInt64FromCustomer | ⚠️ Cần conv |
| engagementDepth | totalMessages | GetIntFromCustomer | ⚠️ Chỉ trong currentMetrics |
| sourceType | conversationFromAds | GetBoolFromCustomer | ⚠️ Chỉ trong currentMetrics |

---

## 3. Các vấn đề chính

### 3.1. Khách hàng thiếu currentMetrics

- **Nguyên nhân:** Document cũ chưa qua RefreshMetrics/Recalculate/Merge; hoặc backfill thiếu bước cập nhật currentMetrics.
- **Hậu quả:** GetFloatFromCustomer("revenueLast30d"), GetIntFromCustomer("ordersLast30d"), GetInt64FromCustomer("lastConversationAt")... fallback top-level = 0 (vì unsetRawFields đã xóa).
- **Cách kiểm tra:** Query `db.crm_customers.find({ currentMetrics: { $exists: false } }).count()` hoặc `currentMetrics.raw: { $exists: false }`.

### 3.2. Link conversation ↔ customer chưa đúng

- `aggregateConversationMetricsForCustomer` match theo: customerId, panCakeData.customer_id, panCakeData.customer.id, conversationIds (từ pc_pos_customers.posData.fb_id).
- Nếu khách POS chưa link fb_id → không match conv → lastConversationAt, totalMessages, conversationFromAds = 0.
- **Cách kiểm tra:** So sánh số khách có `lastConversationAt > 0` trong currentMetrics vs số conv có trong fb_conversations.

### 3.3. avgOrderValue vs totalSpent/orderCount

- Dashboard dùng `AvgOrderValue = totalSpent / orderCount` (tính local trong toDashboardItem).
- totalSpent, orderCount lấy từ GetTotalSpentFromCustomer, GetOrderCountFromCustomer — ưu tiên currentMetrics, fallback top-level.
- **Kết luận:** avgOrderValue ổn nếu totalSpent, orderCount đúng.

### 3.4. secondLastOrderAt

- Chỉ có trong currentMetrics.raw (từ aggregateOrderMetricsForCustomer).
- Nếu currentMetrics nil → repeatFrequency fallback ngưỡng cố định (early < 7, on_track ≤ 45, delayed ≤ 90, overdue > 90).

---

## 4. Script chẩn đoán MongoDB

Chạy trong mongosh hoặc Compass:

```javascript
// 1. Số khách thiếu currentMetrics
db.crm_customers.countDocuments({ 
  ownerOrganizationId: ObjectId("YOUR_ORG_ID"),
  $or: [
    { currentMetrics: { $exists: false } },
    { "currentMetrics.raw": { $exists: false } }
  ]
});

// 2. Số khách có currentMetrics.raw đầy đủ
db.crm_customers.countDocuments({ 
  ownerOrganizationId: ObjectId("YOUR_ORG_ID"),
  "currentMetrics.raw": { $exists: true },
  "currentMetrics.raw.revenueLast30d": { $exists: true }
});

// 3. Sample khách First có firstLayer3
db.crm_customers.aggregate([
  { $match: { ownerOrganizationId: ObjectId("YOUR_ORG_ID"), journeyStage: "first", orderCount: 1 } },
  { $limit: 5 },
  { $project: { 
    unifiedId: 1, 
    totalSpent: 1, orderCount: 1, 
    "currentMetrics.raw.avgOrderValue": 1,
    "currentMetrics.raw.cancelledOrderCount": 1,
    "currentMetrics.raw.lastConversationAt": 1,
    "currentMetrics.raw.lastOrderAt": 1,
    "currentMetrics.layer3.first": 1
  }}
]);

// 4. Sample khách Repeat — kiểm tra secondLastOrderAt, revenueLast30d
db.crm_customers.aggregate([
  { $match: { ownerOrganizationId: ObjectId("YOUR_ORG_ID"), journeyStage: "repeat" } },
  { $limit: 5 },
  { $project: { 
    unifiedId: 1, orderCount: 1,
    "currentMetrics.raw.secondLastOrderAt": 1,
    "currentMetrics.raw.revenueLast30d": 1,
    "currentMetrics.raw.ordersLast30d": 1,
    "currentMetrics.layer3.repeat": 1
  }}
]);

// 5. Số khách Engaged có totalMessages > 0
db.crm_customers.countDocuments({ 
  ownerOrganizationId: ObjectId("YOUR_ORG_ID"),
  journeyStage: "engaged",
  $or: [
    { "currentMetrics.raw.totalMessages": { $gt: 0 } },
    { totalMessages: { $gt: 0 } }
  ]
});
```

---

## 5. Khuyến nghị sửa

| Ưu tiên | Hành động |
|---------|-----------|
| **Cao** | Chạy Recalculate toàn bộ khách (hoặc batch) để đảm bảo currentMetrics đầy đủ |
| **Cao** | Kiểm tra link POS↔FB (sourceIds.fb, posData.fb_id) để conv metrics match đúng |
| **Trung bình** | Bổ sung denormalize revenueLast30d, ordersLast30d, lastConversationAt lên top-level khi RefreshMetrics (để fallback khi currentMetrics thiếu) — hoặc đảm bảo mọi khách đều có currentMetrics |
| **Thấp** | Log cảnh báo khi layer3 derive ra giá trị mặc định (vd: spendMomentum=stable do ord30=0) |

---

## 6. Tham chiếu code

| Thành phần | File |
|------------|------|
| Logic Layer 3 | `api/internal/api/report/layer3/layer3.go` |
| crmItemToLayer3Map | `api/internal/api/report/handler/handler.report.dashboard.go` |
| toDashboardItem | `api/internal/api/crm/service/service.crm.dashboard.go` |
| GetXXFromCustomer | `api/internal/api/crm/service/service.crm.snapshot.go` |
| BuildCurrentMetricsFromOrderAndConv | `api/internal/api/crm/service/service.crm.snapshot.go` |
| aggregateOrderMetricsForCustomer | `api/internal/api/crm/service/service.crm.metrics.go` |
| aggregateConversationMetricsForCustomer | `api/internal/api/crm/service/service.crm.conversation_metrics.go` |
