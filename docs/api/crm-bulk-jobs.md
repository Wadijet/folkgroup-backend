# API CRM Bulk Jobs

**Mục đích:** Tài liệu API cho CRM bulk jobs — rebuild, recalculate, sync, backfill. Worker xử lý queue `crm_bulk_jobs`.

**Cập nhật:** 2025-03-15

---

## Tổng quan

| Endpoint | Method | Mô tả |
|----------|--------|-------|
| `/customers/rebuild` | POST | Tạo job sync + backfill (2 job riêng) |
| `/customers/recalculate-all` | POST | Tạo N job recalculate_batch |
| `/customers/:unifiedId/recalculate` | POST | Tạo 1 job recalculate_one |
| `/crm-bulk-jobs` | GET | Danh sách job (CRUD read) |
| `/crm-bulk-jobs/:id` | GET, PUT | Chi tiết job, cập nhật (retry, isPriority) |

---

## POST /customers/rebuild

Đồng bộ profile (sync) và activity (backfill) cho CRM. **Tạo 2 job riêng**: sync chạy trước, backfill chạy sau.

### Request

**Query hoặc Body:**

| Tham số | Kiểu | Mô tả |
|---------|------|-------|
| `ownerOrganizationId` | string | ID tổ chức (bắt buộc nếu không có active org) |
| `sources` | string | Danh sách nguồn, phân cách dấu phẩy. Rỗng = tất cả |
| `limit` | int | Giới hạn mỗi loại (0 = không giới hạn) |
| `isPriority` | bool | Job ưu tiên: chạy ngay, không bị throttle |

**Giá trị `sources`:**

- `pos`, `fb` → sync profile từ pc_pos_customers, fb_customers
- `order`, `conversation`, `note` → backfill activity từ orders, conversations, notes
- Rỗng → chạy tất cả (sync + backfill)

### Response (202 Accepted)

```json
{
  "code": 202,
  "message": "Các job rebuild (sync + backfill) đã được đưa vào queue",
  "data": {
    "jobIds": ["674abc...", "674abd..."],
    "status": "queued"
  },
  "status": "success"
}
```

- `jobIds`: Mảng ID job (1–2 job: sync, backfill). Thứ tự: sync trước, backfill sau.
- Khi chỉ cần sync hoặc chỉ backfill: `jobIds` có 1 phần tử.

### Checkpoint

- Mỗi job có checkpoint riêng (`progress` trong document).
- Restart server → job tiếp tục từ vị trí đã lưu.

---

## POST /customers/recalculate-all

Tính toán lại tất cả khách hàng của org. **Tạo N job batch** (mỗi batch ~200 khách) thay vì 1 job lớn.

### Request

**Body:**

| Tham số | Kiểu | Mô tả |
|---------|------|-------|
| `ownerOrganizationId` | string | ID tổ chức (bắt buộc nếu không có active org) |
| `batchSize` | int | Số khách mỗi batch (mặc định 200) |
| `isPriority` | bool | Job ưu tiên |

### Response (202 Accepted)

```json
{
  "code": 202,
  "message": "Các job tính toán lại tất cả khách đã được đưa vào queue, worker sẽ xử lý từng batch",
  "data": {
    "jobIds": ["674abc...", "674abd...", "674abe..."],
    "totalBatches": 3,
    "status": "queued"
  },
  "status": "success"
}
```

- `jobIds`: Mảng ID job (mỗi job = 1 batch).
- `totalBatches`: Số job đã tạo.

### Checkpoint

- Mỗi job batch có checkpoint (`progress.processed`).
- Restart → chỉ mất tối đa 1 batch đang chạy.

---

## POST /customers/:unifiedId/recalculate

Tính toán lại 1 khách hàng.

### Request

**Body:**

| Tham số | Kiểu | Mô tả |
|---------|------|-------|
| `isPriority` | bool | Job ưu tiên |

### Response (202 Accepted)

```json
{
  "code": 202,
  "message": "Job tính toán lại khách hàng đã được đưa vào queue",
  "data": {
    "jobId": "674abc...",
    "status": "queued"
  },
  "status": "success"
}
```

---

## CRUD crm-bulk-jobs

- **GET** `/crm-bulk-jobs` — Danh sách job (filter, pagination)
- **GET** `/crm-bulk-jobs/:id` — Chi tiết job
- **PUT** `/crm-bulk-jobs/:id` — Cập nhật (retry, isPriority). Không insert/delete qua API.

### Cấu trúc CrmBulkJob

| Field | Kiểu | Mô tả |
|-------|------|-------|
| `jobType` | string | `sync`, `backfill`, `rebuild`, `recalculate_one`, `recalculate_all`, `recalculate_batch` |
| `ownerOrganizationId` | ObjectID | Tổ chức sở hữu |
| `params` | object | Tham số: sources, types, limit, offset, unifiedId... |
| `isPriority` | bool | Job ưu tiên |
| `createdAt` | int64 | Thời gian tạo |
| `processedAt` | int64? | Thời gian xử lý xong (null = chưa xử lý) |
| `processError` | string | Lỗi nếu có |
| `result` | object | Kết quả khi thành công |
| `progress` | object | Tiến độ checkpoint + hiển thị (xem bảng dưới) |

### Cấu trúc progress (chi tiết)

| Field | Mô tả |
|-------|-------|
| `posSkip`, `fbSkip` | Số bản ghi đã xử lý từ POS, Facebook (sync) |
| `ordersSkip`, `conversationsSkip`, `notesSkip` | Số bản ghi đã xử lý (backfill) |
| `currentSource` | Nguồn đang xử lý: `pos`, `fb`, `order`, `conversation`, `note`, `done` |
| `nextSources` | Mảng nguồn tiếp theo (vd: `["fb"]` khi đang làm pos) |
| `percentBySource` | Map `{ pos: 45, fb: 0, order: 30, ... }` — % tiến độ từng nguồn |
| `totals` | Map tổng số bản ghi mỗi nguồn (để tính %) |
| `phase` | `sync` hoặc `backfill` (chỉ rebuild) |
| `sync`, `backfill` | Progress lồng nhau (chỉ rebuild) |

---

## Thứ tự xử lý job

Worker lấy job theo: `isPriority` desc, `createdAt` asc.

- Rebuild: sync job tạo trước → backfill job tạo sau (cách 1s) → đúng thứ tự.
- Recalculate-all: N job batch độc lập, xử lý song song theo batchSize worker.

---

## Tài liệu liên quan

- [DE_XUAT_CRM_BULK_JOB_RESUMABLE.md](../05-development/DE_XUAT_CRM_BULK_JOB_RESUMABLE.md) — Đề xuất chunking, checkpoint
- [WORKER_CONFIG_ENV_VARS.md](../05-development/WORKER_CONFIG_ENV_VARS.md) — Cấu hình worker
