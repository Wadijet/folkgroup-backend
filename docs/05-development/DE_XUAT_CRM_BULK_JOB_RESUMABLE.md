# Đề xuất: CRM Bulk Job chạy lâu + Restart mất tiến độ

**Ngày:** 2025-03-15  
**Vấn đề:** CRM bulk job (sync, backfill, rebuild, recalculate_all) chạy quá lâu; mỗi khi restart server lại chạy lại từ đầu.

---

## 1. Phân tích hiện trạng

### 1.1 Luồng xử lý hiện tại

```
API (rebuild/recalculate) → Enqueue job → crm_bulk_jobs
                                    ↓
CrmBulkWorker (poll 2 phút) → GetUnprocessed → processJob() → SetProcessed
```

- Mỗi job là **atomic**: chỉ khi `processJob()` hoàn thành mới gọi `SetProcessed`.
- Nếu server restart **giữa chừng** → job chưa có `processedAt` → `GetUnprocessed` lấy lại → **chạy lại từ đầu**.

### 1.2 Các job type và đặc điểm

| JobType | Xử lý | Có batch nội bộ? | Restart mất gì? |
|---------|-------|------------------|-----------------|
| sync | Toàn bộ pc_pos_customers + fb_customers | Cursor iterate, không checkpoint | Toàn bộ tiến độ |
| backfill | Orders, conversations, notes | Batch 1000, không lưu checkpoint | Toàn bộ tiến độ |
| rebuild | Tạo 2 job: sync + backfill | Mỗi job độc lập, checkpoint riêng | Chỉ mất job đang chạy |
| recalculate_one | 1 customer | Nhanh, ít ảnh hưởng | 1 customer |
| recalculate_all | Toàn bộ crm_customers | Worker pool, không checkpoint | Toàn bộ tiến độ |

### 1.3 Nguyên nhân gốc rễ

- **Thiết kế atomic**: Job được thiết kế "all-or-nothing" — hoặc xong hết hoặc chưa xong.
- **Không có checkpoint**: Không lưu tiến độ (offset, lastId, counts) sau mỗi batch.
- **Job quá lớn**: Một job rebuild/recalculate có thể xử lý hàng chục nghìn bản ghi trong một lần chạy.

---

## 2. Đề xuất giải pháp

### 2.1 Giải pháp A: Chia nhỏ job (Job Chunking) — **Khuyến nghị**

**Ý tưởng:** Thay vì 1 job xử lý toàn bộ, tách thành **nhiều job nhỏ** độc lập. Mỗi job xử lý 1 batch; restart chỉ ảnh hưởng job đang chạy, các job đã `SetProcessed` không bị mất.

**Cách triển khai:**

1. **API rebuild/recalculate** không tạo 1 job, mà tạo **N job** với params:
   - `sync`: `{ sources, offset, limit }` — mỗi job sync 1 batch (ví dụ 500 POS + 500 FB)
   - `backfill`: `{ types, offset, limit }` — mỗi job backfill 1 batch
   - `recalculate_all`: `{ offset, limit }` — mỗi job recalc 1 batch (ví dụ 200 customers)

2. **Thêm job types mới** (hoặc mở rộng params):
   - `sync_batch`, `backfill_batch`, `recalculate_batch` — mỗi job xử lý 1 batch
   - Hoặc giữ job type cũ, thêm `offset`/`limit` vào params

3. **Orchestrator** (tùy chọn): Một job "parent" tạo N job con; hoặc API trực tiếp tạo N job.

**Ưu điểm:**
- Ít thay đổi logic xử lý hiện có (chỉ thêm pagination/offset)
- Mỗi job nhỏ hoàn thành độc lập → restart ít mất tiến độ
- Dễ quan sát (số job đã xử lý / tổng số job)

**Nhược điểm:**
- Số lượng document trong `crm_bulk_jobs` tăng (N job thay vì 1)
- Cần tính batch size hợp lý (quá nhỏ → nhiều job; quá lớn → vẫn chạy lâu)

**Effort:** Trung bình (2–3 ngày)

---

### 2.2 Giải pháp B: Checkpoint / Progress tracking

**Ý tưởng:** Thêm trường `progress` (bson.M) vào `CrmBulkJob`. Sau mỗi batch xử lý, cập nhật progress (offset, lastId, counts). Khi restart, đọc progress và tiếp tục từ đó.

**Cấu trúc progress (ví dụ):**

```go
// progress lưu trong CrmBulkJob.Params hoặc field riêng
progress := bson.M{
    "sync": bson.M{
        "posOffset": 1500,
        "fbOffset":  800,
    },
    "backfill": bson.M{
        "ordersSkip":      2000,
        "conversationsSkip": 1500,
        "notesSkip":       100,
    },
    "recalculate": bson.M{
        "lastUnifiedId": "xxx",  // hoặc skip
        "processed":     1200,
    },
}
```

**Luồng:**
1. Worker lấy job → đọc `progress` (nếu có)
2. Xử lý batch → cập nhật `progress` (UpdateOne) → tiếp tục batch tiếp
3. Khi xong hết → `SetProcessed`

**Ưu điểm:**
- 1 job = 1 document, không tăng số lượng job
- Tiến độ được lưu liên tục

**Nhược điểm:**
- Phức tạp: mỗi job type cần logic resume khác nhau
- Cần sửa sâu SyncAllCustomers, BackfillActivity, RecalculateAllCustomers
- Race condition nếu nhiều worker cùng claim 1 job (cần claim/lock)

**Effort:** Cao (5–7 ngày)

---

### 2.3 Giải pháp C: Claim/Lock + Stale timeout

**Ý tưởng:** Job được "claim" bởi worker khi bắt đầu (`processingStartedAt`, `workerId`). `GetUnprocessed` bỏ qua job đang được claim **trừ khi** đã quá timeout (stale) — coi như worker chết, cho phép job khác claim lại.

**Thay đổi:**
- Thêm `processingStartedAt *int64`, `workerId string` vào CrmBulkJob
- Worker: trước khi xử lý → `ClaimJob(id)`; sau khi xong → `SetProcessed`
- `GetUnprocessed`: filter `processedAt == nil AND (processingStartedAt == nil OR now - processingStartedAt > staleTimeout)`
- Stale timeout: ví dụ 30 phút — job claim quá 30 phút không xong thì cho phép retry

**Ưu điểm:**
- Cho phép retry job "treo" khi worker crash
- Không giải quyết "chạy lại từ đầu" — vẫn mất tiến độ khi restart

**Nhược điểm:**
- Chỉ hỗ trợ retry, không resume từ checkpoint

**Kết luận:** Giải pháp C **bổ trợ** cho A hoặc B, không thay thế.

---

### 2.4 Giải pháp D: Graceful shutdown

**Ý tưởng:** Khi server nhận signal SIGTERM, không kill ngay mà đợi job hiện tại hoàn thành (hoặc timeout). Giảm tần suất job bị gián đoạn khi restart.

**Triển khai:**
- `main.go`: listen SIGTERM/SIGINT → cancel context → đợi worker goroutines với timeout (ví dụ 5 phút)
- Worker: check `ctx.Done()` giữa các batch (nếu có batch) để thoát sớm

**Ưu điểm:**
- Đơn giản, ít thay đổi
- Giảm mất job khi deploy/restart có kế hoạch

**Nhược điểm:**
- Không giúp khi crash, OOM, kill -9
- Job quá dài vẫn phải chờ hoặc timeout

**Kết luận:** Nên làm **bổ trợ** cho mọi môi trường.

---

### 2.5 Giải pháp E: Worker/Queue ngoài (Redis, BullMQ, Celery...)

**Ý tưởng:** Chạy CRM bulk worker trên process/container riêng, dùng queue ngoài (Redis, RabbitMQ). Server API restart không ảnh hưởng worker.

**Ưu điểm:**
- Tách biệt hoàn toàn API và worker
- Có thể scale worker độc lập

**Nhược điểm:**
- Thay đổi kiến trúc lớn
- Thêm infrastructure (Redis, queue service)

**Kết luận:** Giải pháp dài hạn, không giải quyết trực tiếp "chạy lại từ đầu" — vẫn cần A hoặc B.

---

## 3. Khuyến nghị triển khai

### Giai đoạn 1 (ngắn hạn) — 1–2 tuần

| # | Giải pháp | Mô tả |
|---|-----------|-------|
| 1 | **Job Chunking (A)** | Chia rebuild/recalculate_all thành nhiều job batch. Ưu tiên `recalculate_all` vì dễ tách (theo offset/limit). |
| 2 | **Graceful shutdown (D)** | Đợi worker hoàn thành hoặc timeout khi SIGTERM. |
| 3 | **Claim + Stale (C)** | Thêm claim/lock để tránh 2 worker cùng xử lý 1 job; stale timeout cho job treo. |

### Giai đoạn 2 (trung hạn) — 2–4 tuần

| # | Giải pháp | Mô tả |
|---|-----------|-------|
| 4 | **Sync/Backfill chunking** | Mở rộng chunking cho sync và backfill (phức tạp hơn recalculate vì cần sort + skip). |
| 5 | **Checkpoint (B)** | Nếu chunking chưa đủ, cân nhắc thêm progress cho job dài (rebuild toàn org). |

---

## 4. Thiết kế chi tiết: Job Chunking cho RecalculateAll

### 4.1 API thay đổi

**Hiện tại:** `POST /customers/recalculate-all` → 1 job `recalculate_all` với `params: { limit? }`

**Đề xuất:** 
- Giữ API, thêm `params.batchSize` (mặc định 200)
- Service `EnqueueRecalculateAll`:
  1. Đếm số customer: `CountDocuments(ownerOrgID)`
  2. Tạo N job `recalculate_batch` với `params: { offset, limit }`
  3. N = ceil(total / batchSize)

### 4.2 Job type mới

```go
CrmBulkJobRecalculateBatch = "recalculate_batch"
// Params: { "offset": 0, "limit": 200 }
```

### 4.3 Worker xử lý

```go
case crmmodels.CrmBulkJobRecalculateBatch:
    offset := parseInt(params, "offset", 0)
    limit := parseInt(params, "limit", 200)
    if limit <= 0 { limit = 200 }
    result, err := svc.RecalculateCustomersBatch(ctx, job.OwnerOrganizationID, offset, limit, poolSize)
```

### 4.4 Service mới

```go
// RecalculateCustomersBatch tính toán lại 1 batch khách (offset, limit).
func (s *CrmCustomerService) RecalculateCustomersBatch(ctx context.Context, ownerOrgID primitive.ObjectID, offset, limit int, poolSize int) (*crmdto.CrmRecalculateAllResult, error)
```

- Dùng `Find().SetSkip(offset).SetLimit(limit)` thay vì load toàn bộ
- Logic còn lại giống `RecalculateAllCustomers`

---

## 5. Checklist triển khai (đã hoàn thành 2025-03-15)

- [x] Thêm `CrmBulkJobRecalculateBatch` vào model
- [x] Implement `RecalculateCustomersBatch` trong CrmCustomerService
- [x] Sửa `HandleRecalculateAllCustomers`: gọi `EnqueueRecalculateAllBatches` (tạo N job), body dùng `batchSize` thay `limit`
- [x] Worker: thêm case `recalculate_batch` gọi `RecalculateCustomersBatch`
- [x] Thêm field `Progress` vào CrmBulkJob + `UpdateProgress` trong CrmBulkJobService
- [x] Checkpoint cho sync: `SyncAllCustomersBatch`, `SyncAllCustomers(progress, onProgress)`, lưu posSkip/fbSkip
- [x] Checkpoint cho rebuild: `RebuildCrm(progress, onProgress)`, sync + backfill đều lưu progress
- [x] Checkpoint cho backfill: `BackfillActivity(progress, onProgress)`, lưu ordersSkip, conversationsSkip, notesSkip
- [x] Checkpoint cho recalculate_batch: lưu `processed` sau mỗi sub-batch 50 khách
- [ ] Graceful shutdown trong main.go
- [ ] (Tùy chọn) Claim + stale timeout cho job

---

## 6. Tài liệu tham khảo

- `api/internal/worker/crm_bulk_worker.go` — CrmBulkWorker
- `api/internal/api/crm/service/service.crm.bulk.job.go` — CrmBulkJobService
- `api/internal/api/crm/service/service.crm.recalculate.go` — RecalculateAllCustomers
- `api/internal/api/crm/service/service.crm.sync.go` — SyncAllCustomers
- `api/internal/api/crm/service/service.crm.backfill.go` — BackfillActivity
- `docs/05-development/DE_XUAT_TOI_UU_HOAT_DONG_TINH_TOAN.md` — Tối ưu workers
