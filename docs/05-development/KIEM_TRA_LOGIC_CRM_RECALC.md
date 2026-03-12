# Kiểm tra logic CRM Recalc (trước khi chạy recalc)

> Ngày: 2026-03-10

## 1. ConversationIds từ posData.fb_id (link POS → conv)

| Vị trí | Trạng thái | Ghi chú |
|--------|------------|---------|
| `getConversationIdsFromPosCustomers` | ✅ | Query `pc_pos_customers` theo `customerId` $in posIds, lấy `posData.fb_id` = conversationId |
| `buildConversationFilterForCustomerIds` | ✅ | Thêm `{"conversationId": cid}` vào $or khi có conversationIds |
| `aggregateConversationMetricsForCustomer` | ✅ | Nhận conversationIds, truyền vào matchFilter |
| `checkHasConversation` | ✅ | Nhận conversationIds, thêm vào $or |
| `fetchConversations` (fullprofile) | ✅ | Nhận convIds, truyền vào filter |
| `fetchLatestConversationForCustomer` | ✅ | Nhận conversationIds |
| `backfillConversationActivitiesForCustomer` | ✅ | Nhận conversationIds |

**Callers truyền convIds đúng:**
- Recalculate: `[]string{customer.SourceIds.Pos}` → getConversationIdsFromPosCustomers
- RefreshMetrics: `[]string{customer.SourceIds.Pos}`
- GetMetricsForSnapshotAt: `[]string{c.SourceIds.Pos}`
- MergeFromPosCustomer: `[]string{sourceIds.Pos}`
- MergeFromFbCustomer: `[]string{sourceIds.Pos}` (có thể có pos khi merge fb_id/phone)
- Fullprofile: chỉ gọi khi `SourceIds.Pos != ""`

---

## 2. Ngày tháng nguồn (panCakeData/posData) vs đồng bộ (root)

| Thành phần | Field dùng | Nguồn | Trạng thái |
|------------|------------|-------|------------|
| **convExistedAt** (filter asOf) | panCakeData.inserted_at | Nguồn | ✅ |
| **convUpdatedAt** (lastConversationAt) | panCakeData.updated_at → panCakeUpdatedAt | Nguồn ưu tiên | ✅ |
| **orderDate** (order metrics) | insertedAt, posCreatedAt, posData.inserted_at | insertedAt extract từ posData | ✅ |
| **getConversationTimestamp** (ingest) | panCakeData.inserted_at, updated_at | Nguồn | ✅ |
| **getSourceCustomerTimestamp** (merge) | posData/panCakeData inserted_at, created_at | Nguồn | ✅ |

---

## 3. Filter asOf (conv đã tồn tại tại activityAt)

| Logic | Trạng thái |
|-------|------------|
| convExistedAt = convInsertedAtMs (chỉ từ panCakeData.inserted_at) | ✅ |
| Không fallback 0/updated_at khi thiếu inserted_at | ✅ |
| $or: convExistedAt <= asOf HOẶC convExistedAt null (bao gồm conv không xác định được) | ✅ |
| Parse string "2026-03-03T04:03:22.263935" → split by ".", $toDate phần đầu | ✅ |

---

## 4. Luồng Recalculate

| Bước | Trạng thái |
|------|------------|
| 1. FindOne crm_customer | ✅ |
| 2. rebuildProfileFromAllSources | ✅ |
| 3. buildCustomerIdsForRecalculate (unifiedId, pos, fb) | ✅ |
| 4. expandCustomerIdsForAggregation (thêm FB/POS qua phone) | ✅ |
| 5. getConversationIdsFromPosCustomers([]string{SourceIds.Pos}) | ✅ |
| 6. aggregateOrderMetricsForCustomer(ids, asOf=0) | ✅ |
| 7. aggregateConversationMetricsForCustomer(ids, convIds, asOf=0) | ✅ |
| 8. checkHasConversation(ids, convIds) làm fallback hasConv | ✅ |
| 9. BuildCurrentMetricsFromOrderAndConv | ✅ |
| 10. Update crm_customers | ✅ |
| 11. logRecalculateActivity (customer_updated) | ✅ |
| 12. backfillConversationActivitiesForCustomer(ids, convIds) | ✅ |

---

## 5. Luồng Ingest (order/conversation) → GetMetricsForSnapshotAt

| Bước | Trạng thái |
|------|------------|
| activityAt từ getOrderTimestamp/getConversationTimestamp (posData/panCakeData) | ✅ |
| GetMetricsForSnapshotAt(c, activityAt) | ✅ |
| aggregateConversationMetricsForCustomer(ids, convIds, activityAt) | ✅ |
| convIds từ getConversationIdsFromPosCustomers | ✅ |

---

## 6. Merge flow

| Merge | convIds | Trạng thái |
|-------|---------|------------|
| MergeFromPosCustomer | []string{sourceIds.Pos} | ✅ |
| MergeFromFbCustomer | []string{sourceIds.Pos} (có thể có khi merge fb_id) | ✅ |
| activityAt từ getSourceCustomerTimestamp(posData/panCakeData) | ✅ |

---

## 7. RefreshMetrics

| Thành phần | Trạng thái |
|------------|------------|
| convIds từ getConversationIdsFromPosCustomers | ✅ |
| aggregateConversationMetricsForCustomer(ids, convIds, 0) | ✅ |
| checkHasConversation(ids, convIds) | ✅ |

---

## 8. Các điểm cần lưu ý

1. **pc_pos_customers.customerId**: Phải khớp với crm_customers.sourceIds.pos (Pancake UUID).
2. **posData.fb_id**: Format pageId_psid = fb_conversations.conversationId.
3. **Order orderDate**: insertedAt/posCreatedAt được extract từ posData.inserted_at khi sync — nếu doc cũ chưa flatten thì dùng posData.inserted_at (có thể string, cần verify).
4. **Conv thiếu inserted_at**: Được bao gồm qua $or convExistedAt null — không loại nhầm.

---

## 9. Sửa 2026-03-10 (mismatch tăng khi recalc)

| Sửa | Mô tả |
|-----|-------|
| **logRecalculateActivity** | Dùng `GetMetricsForSnapshotAt(cust, activityAt)` thay vì `BuildSnapshotForNewCustomer` — đảm bảo metrics snapshot = aggregate, tránh lệch do decode struct |
| **getConversationIdsFromFbMatch** | Query `fb_conversations` theo customerIds, lấy `conversationId` — thêm conversationIds cho FB (fallback khi path BSON khác) |
| **Recalc, GetMetricsForSnapshotAt, RefreshMetrics, Merge** | Gọi `getConversationIdsFromFbMatch` và merge vào conversationIds trước khi aggregate |

---

## Kết luận

**Logic đã chuẩn.** Có thể chạy recalc.
