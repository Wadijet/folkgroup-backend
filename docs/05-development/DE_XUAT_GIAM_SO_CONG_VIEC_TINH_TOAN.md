# Đề xuất: Giảm số công việc phải tính toán

## 1. Tổng quan

Hệ thống hiện có nhiều điểm tính toán lặp lại hoặc không cần thiết:
- **Report**: MarkDirty → Compute snapshot
- **CRM**: Enqueue → Merge/Ingest
- **Classification**: Refresh metrics cho khách hàng

Tài liệu này đề xuất các giải pháp để **giảm số lượng công việc** cần thực hiện, từ đó giảm tải DB và CPU.

---

## 2. Các giải pháp đề xuất

### 2.1 Deduplicate queue CRM (crm_pending_ingest)

**Vấn đề:** Cùng một document (vd: customer X) có thể được cập nhật nhiều lần trong vài giây (sync hàng loạt). Mỗi lần → 1 job mới vào queue. Worker xử lý N lần cho cùng 1 document.

**Giải pháp:** Coalesce theo business key — chỉ giữ job mới nhất cho mỗi (collection, businessId).

| Collection | Business key |
|------------|--------------|
| pc_pos_customers | customerId + ownerOrgID |
| fb_customers | customerId + ownerOrgID |
| pc_pos_orders | orderId + ownerOrgID |
| fb_conversations | conversationId + ownerOrgID |
| crm_notes | noteId + ownerOrgID |

**Cách triển khai:**
- Thay `InsertOne` bằng **UpdateOne với upsert** theo filter `(collectionName, businessKey)`
- Khi có event mới: upsert job (ghi đè document, cập nhật createdAt)
- Kết quả: 100 update cùng customer → 1 job trong queue

**Effort:** TB — cần thêm field businessKey, sửa EnqueueCrmIngest và GetUnprocessed (sort/limit).

---

### 2.2 Chỉ MarkDirty khi field ảnh hưởng period thay đổi

**Vấn đề:** Order update (vd: chỉ đổi status) vẫn trigger MarkDirty. Nhưng periodKey phụ thuộc posCreatedAt/insertedAt — các field này thường **không đổi** khi update. MarkDirty cho period cũ là thừa.

**Giải pháp:** So sánh với PreviousDocument (nếu có):
- **Insert/Upsert (tạo mới):** Luôn MarkDirty
- **Update:** Chỉ MarkDirty nếu field quyết định period (posCreatedAt, insertedAt, updatedAt, activityAt) **thay đổi**

**Cách triển khai:** Thêm `PreviousDocument` vào DataChangeEvent cho UpdateOne (BaseServiceMongoImpl đã có document cũ khi validate). Trong report hook: so sánh ts cũ vs mới; nếu bằng nhau → skip MarkDirty.

**Effort:** TB — sửa BaseServiceMongoImpl, event struct, report hook.

---

### 2.3 Cache GetDirtyPeriodKeys / report_definitions

**Vấn đề:** Mỗi event gọi `GetDirtyPeriodKeysForCollection` hoặc `GetDirtyPeriodKeysForReportKeys` → query `report_definitions`. Dữ liệu ít thay đổi.

**Giải pháp:** Cache kết quả với TTL 5–10 phút.
- Key: `(collectionName)` hoặc `(reportKeys)`
- Value: `map[reportKey]periodType`
- Invalidate: khi report_definitions thay đổi (CRUD) hoặc TTL hết

**Effort:** Thấp — thêm sync.Map + TTL.

---

### 2.4 Compute report on-demand (lazy)

**Vấn đề:** Worker Compute snapshot cho mọi dirty period, kể cả period không ai xem (vd: 2 năm trước).

**Giải pháp:** 
- **Option A:** Chỉ Compute khi có request API đọc period đó (lazy compute). MarkDirty vẫn ghi; khi GetSnapshot không có → Compute rồi trả về.
- **Option B:** Chỉ Compute periods trong range "hot" (vd: 90 ngày gần nhất). Periods cũ Compute khi có request đầu tiên.

**Ưu:** Giảm mạnh số lần Compute.
**Nhược:** Request đầu tiên cho period lạnh có thể chậm.

**Effort:** TB — sửa flow GetSnapshot, Compute.

---

### 2.5 Skip Merge/Ingest khi dữ liệu không đổi

**Vấn đề:** Webhook gửi customer_updated nhiều lần với cùng payload (retry, duplicate). MergeFromFbCustomer chạy mỗi lần.

**Giải pháp:** 
- **Checksum:** Hash(panCakeData, updatedAt) lưu trong document hoặc cache. Khi event mới: so sánh hash. Nếu trùng → skip Enqueue.
- **Hoặc:** So sánh với PreviousDocument (nếu có) — các field quan trọng không đổi → skip.

**Effort:** TB — cần PreviousDocument hoặc checksum logic.

---

### 2.6 Giới hạn range period khi Compute

**Vấn đề:** Worker Compute tất cả dirty periods, kể cả period rất cũ (vd: 2020).

**Giải pháp:** Chỉ lấy dirty periods có `periodKey >= earliestPeriod` (vd: 90 ngày trước). Periods cũ hơn: đánh dấu processedAt = now (coi như đã xử lý) hoặc xóa khỏi queue, Compute on-demand khi cần.

**Effort:** Thấp — thêm filter trong GetUnprocessedDirtyPeriods.

---

### 2.7 Batch MarkDirty theo (reportKey, periodKey, org)

**Vấn đề:** Đã có debounce trong report hook (queueMarkDirty). Nhưng mỗi event vẫn gọi GetDirtyPeriodKeysForCollection (query DB).

**Giải pháp:** Cache report keys per collection. Khi event: lấy từ cache, không query. Đã nêu ở 2.3.

---

### 2.8 Classification: Chỉ refresh khách có thay đổi gần đây

**Vấn đề:** Full mode duyệt tất cả khách có order. Nhiều khách không có thay đổi (order, conversation) từ lần refresh trước.

**Giải pháp:** 
- Lưu `lastRefreshedAt` hoặc `lastActivityAt` trong crm_customers
- Chỉ refresh khách có `lastOrderAt` hoặc `lastConversationAt` > lastRefreshedAt (hoặc trong N ngày gần nhất)
- Hoặc: Smart mode mở rộng — thêm vùng "có activity mới" (order/conversation trong 7 ngày)

**Effort:** TB — sửa ListCustomerIdsForClassificationRefresh.

---

### 2.9 TTL / Xóa job queue cũ

**Vấn đề:** crm_pending_ingest có thể tích tụ job cũ (lỗi, retry nhiều lần). Worker vẫn cố xử lý.

**Giải pháp:** 
- Job quá N giờ (vd: 24h) → đánh dấu processedAt = now, processError = "expired"
- Hoặc: job có retryCount > M → bỏ qua, ghi log
- Cron dọn dẹp: xóa job đã processed quá 7 ngày

**Effort:** Thấp — thêm điều kiện trong worker + cron cleanup.

---

### 2.10 Ưu tiên job theo độ "nóng"

**Vấn đề:** Job cũ (sync 2 ngày trước) và job mới (webhook vừa nhận) được xử lý cùng mức độ ưu tiên.

**Giải pháp:** Worker lấy job sort theo `createdAt desc` (mới trước) thay vì `createdAt asc`. Hoặc: 2 queue — high priority (realtime) và low priority (backfill).

**Effort:** Thấp — đổi sort trong GetUnprocessedCrmIngest.

---

## 3. Thứ tự ưu tiên triển khai

| # | Giải pháp | Tác động | Effort | Ưu tiên |
|---|-----------|----------|--------|---------|
| 1 | Cache GetDirtyPeriodKeys (2.3) | Giảm query report_definitions mỗi event | Thấp | Cao |
| 2 | Deduplicate queue CRM (2.1) | Giảm mạnh số job khi sync hàng loạt | TB | Cao |
| 3 | Giới hạn range period Compute (2.6) | Giảm Compute periods cũ | Thấp | Cao |
| 4 | TTL / Xóa job queue cũ (2.9) | Tránh xử lý job lỗi vô hạn | Thấp | TB |
| 5 | Ưu tiên job mới trước (2.10) | UX tốt hơn cho realtime | Thấp | TB |
| 6 | Chỉ MarkDirty khi field period đổi (2.2) | Giảm MarkDirty thừa | TB | TB |
| 7 | Skip Merge khi dữ liệu không đổi (2.5) | Giảm Merge trùng | TB | TB |
| 8 | Compute on-demand (2.4) | Giảm Compute không cần thiết | TB | Thấp |
| 9 | Classification chỉ khách có thay đổi (2.8) | Giảm refresh full | TB | Thấp |

---

## 4. Tóm tắt

| Nhóm | Giải pháp chính |
|------|-----------------|
| **Giảm job tạo ra** | Deduplicate queue, Skip khi không đổi, Chỉ MarkDirty khi field period đổi |
| **Giảm query** | Cache report_definitions, Cache GetDirtyPeriodKeys |
| **Giảm Compute** | Giới hạn range period, Compute on-demand |
| **Vệ sinh** | TTL job, Ưu tiên job mới |

---

## 5. Tài liệu tham khảo

- `api/internal/api/report/service/service.report.hooks.go` — Report hook
- `api/internal/api/crm/service/service.crm.pending.ingest.go` — CRM queue
- `api/internal/worker/report_dirty_worker.go` — Report worker
- `api/internal/worker/crm_ingest_worker.go` — CRM worker
