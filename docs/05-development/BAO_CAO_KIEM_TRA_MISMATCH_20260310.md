# Báo cáo kiểm tra visitor/engaged mismatch — 2026-03-10

## 1. Kết quả chẩn đoán

| Chỉ số | Giá trị |
|--------|---------|
| Tổng engaged trong crm_customers | 38,197 |
| **Mismatch (engaged crm, visitor activity)** | **13,432** |
| — Có conversation (currentMetrics) | 0 |
| — Không có conversation | 13,432 |

**Lưu ý:** Script đã sửa để đọc `conversationCount` và `hasConversation` từ `currentMetrics` (sau migration unset top-level).

---

## 2. Phát hiện chính

### 2.1 Tất cả khách mismatch đều có conv thực tế

Chạy `diagnose_mismatch_root_cause` cho 10 mẫu:
- **FB customers:** Conv tồn tại, match qua `page_customer.id` = `sourceIds.fb`
- **POS customers:** Conv tồn tại, link qua `posData.fb_id` = `conversationId`

→ Aggregate **nên** tìm được conv, nhưng crm đang có `convCount=0`, `hasConversation=false`.

### 2.2 Nguyên nhân khả dĩ

1. **Recalc chưa chạy qua các khách này** — Hàng đợi lớn, batch nhỏ (2 khách/lần).
2. **Aggregate có bug** — Không match đúng (filter, path BSON, conversationIds).
3. **journeyStage cũ** — Được set trước khi migration `currentMetrics`; `journeyStage` vẫn `engaged` trong khi `currentMetrics` đã bị cập nhật sai (ví dụ từ RefreshMetrics).

### 2.3 Mismatch tăng (13,316 → 13,360 → 13,432)

- Recalc ghi `customer_updated` với metrics mới.
- Nếu aggregate trả 0 conv → ghi `visitor` → activity cuối = visitor.
- Nếu `journeyStage` không được cập nhật đồng bộ (race, lỗi) → vẫn `engaged` → mismatch tăng.

---

## 3. Khuyến nghị

### 3.1 Kiểm tra aggregate trực tiếp

Viết script test: với 1 unifiedId mẫu (vd: `77e5b6f3-58ad-4e4c-aab1-8c62c2608916`), gọi `aggregateConversationMetricsForCustomer` và `checkHasConversation` — xem có trả conv hay không.

### 3.2 Đảm bảo server chạy code mới

- Restart server sau deploy để ingest/RefreshMetrics dùng đúng logic (conversationIds, filter asOf).
- Kiểm tra job recalc: `crm_bulk_jobs` có `processedAt` chưa.

### 3.3 Xử lý hàng loạt

- Nếu aggregate đúng: chạy recalc cho toàn bộ org, chờ xử lý hết.
- Nếu aggregate sai: sửa bug trước rồi mới recalc.

---

## 4. Sửa đã áp dụng (2026-03-10)

### 4.1 logRecalculateActivity
- **Trước:** Dùng `BuildSnapshotForNewCustomer` với customer từ FindOne — phụ thuộc decode `currentMetrics`.
- **Sau:** Dùng `GetMetricsForSnapshotAt(cust, activityAt)` — metrics snapshot = aggregate, đồng nhất với các activity khác.

### 4.2 getConversationIdsFromFbMatch
- Query `fb_conversations` theo customerIds (filter giống aggregate), lấy `conversationId`.
- Merge vào conversationIds trước khi aggregate — thêm đường match qua `conversationId` cho FB (fallback khi path BSON khác).

### 4.3 Áp dụng tại
- Recalc, GetMetricsForSnapshotAt, RefreshMetrics, MergeFromPosCustomer, MergeFromFbCustomer.

---

## 5. File đã sửa

- `scripts/diagnose_visitor_engaged_mismatch.go` — Đọc `conversationCount` và `hasConversation` từ `currentMetrics` thay vì top-level.
- `service.crm.recalculate.go` — logRecalculateActivity, getConversationIdsFromFbMatch, appendUnique.
- `service.crm.metrics.go` — GetMetricsForSnapshotAt, RefreshMetrics.
- `service.crm.merge.go` — MergeFromPosCustomer, MergeFromFbCustomer.
