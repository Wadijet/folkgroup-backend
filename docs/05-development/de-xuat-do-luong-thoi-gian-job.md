# Đề xuất: Đo lường thời gian thực hiện từng loại Job

## 1. Mục tiêu

- Đo thời gian thực hiện mỗi job theo từng loại
- Lưu trữ **thời gian trung bình của 1.000 job gần nhất** mỗi loại
- Tính **số job đã chạy mỗi loại trong 1 giờ gần nhất**
- **Lưu tạm in-memory** — đơn giản, không DB/Redis
- **1 API duy nhất** trả tất cả kết quả

---

## 2. Danh sách loại Job cần đo

| Worker | Job Type Key | Mô tả |
|--------|--------------|-------|
| **CrmIngestWorker** | `crm_ingest:PcPosCustomers` | Merge từ POS customer |
| | `crm_ingest:FbCustomers` | Merge từ FB customer |
| | `crm_ingest:PcPosOrders` | Ingest order touchpoint |
| | `crm_ingest:FbConvesations` | Ingest conversation touchpoint |
| | `crm_ingest:CrmNotes` | Ingest note touchpoint |
| **CrmBulkWorker** | `crm_bulk:sync` | Sync profile (từ rebuild hoặc job sync) |
| | `crm_bulk:backfill` | Backfill activity (từ rebuild hoặc job backfill) |
| | `crm_bulk:rebuild` | Rebuild CRM (backward compat, 1 job) |
| | `crm_bulk:recalculate_one` | Recalculate 1 customer |
| | `crm_bulk:recalculate_batch` | Recalculate 1 batch (từ recalculate-all) |
| **ReportDirtyWorker** | `report_dirty:customer_daily`, `report_dirty:customer_weekly`, `report_dirty:customer_monthly`, `report_dirty:customer_yearly`, `report_dirty:order_daily`, ... | Compute report period theo từng loại (customer/order) và chu kỳ (daily/weekly/monthly/yearly) |
| **ClassificationRefreshWorker** | `classification_refresh:full` | Refresh full |
| | `classification_refresh:smart` | Refresh smart |
| **CommandCleanupWorker** | `command_cleanup` | Release stuck commands |
| **AgentCommandCleanup** | `agent_command_cleanup` | Release stuck agent commands |
| **Delivery Processor** | `delivery:email`, `delivery:telegram`, `delivery:webhook` | Gửi notification qua từng kênh |

---

## 3. Phương án kỹ thuật

### 3.1. Ring Buffer (Sliding Window 1.000 mẫu)

- **Cấu trúc**: Mỗi job type có một ring buffer cố định 1.000 phần tử, mỗi phần tử lưu `(timestamp, duration)`
- **Thêm mới**: O(1) — ghi đè phần tử cũ nhất khi đầy
- **Tính trung bình**: O(1) — duy trì `sum` và `count`, cập nhật khi thêm/xóa
- **Đếm job trong 1 giờ**: O(n) — duyệt buffer, đếm phần tử có `timestamp >= now - 1h` (n ≤ 1.000)
- **Bộ nhớ**: ~16 bytes (timestamp + duration) × 1.000 × số job type ≈ vài trăm KB

### 3.2. Lưu trữ

**In-memory** — lưu tạm, mất khi restart. Không dùng DB/Redis.

### 3.3. API

```go
// RecordDuration ghi nhận thời gian thực hiện 1 job (gọi từ worker)
func RecordDuration(jobType string, duration time.Duration)

// GetAll trả về tất cả metrics — dùng cho 1 API duy nhất
func GetAll() map[string]struct {
    AvgMs         int64  // Thời gian trung bình (ms)
    MinMs         int64  // Thời gian tối thiểu (ms)
    MaxMs         int64  // Thời gian tối đa (ms)
    SampleCount   int    // Số mẫu trong 1k gần nhất
    CountLastHour int    // Số job chạy trong 1 giờ gần nhất
}
```

---

## 4. Cấu trúc code

```
api/internal/worker/
├── metrics/
│   └── metrics.go         # Ring buffer, RecordDuration, GetAll
├── crm_ingest_worker.go   # Gọi RecordDuration sau mỗi processItem
├── crm_bulk_worker.go     # Gọi RecordDuration sau mỗi processJob
├── report_dirty_worker.go # Gọi RecordDuration sau mỗi Compute
└── ...
```

### 4.1. Ví dụ tích hợp trong worker

**CrmIngestWorker** — đo từng item:

```go
for _, item := range list {
    start := time.Now()
    err := w.processItem(ctx, customerSvc, &item)
    jobType := "crm_ingest:" + item.CollectionName
    if jobType == "crm_ingest:" {
        jobType = "crm_ingest:unknown"
    }
    metrics.RecordDuration(jobType, time.Since(start))
    // ... set processed ...
}
```

**CrmBulkWorker** — đo từng job:

```go
for _, item := range list {
    start := time.Now()
    result, err := w.processJob(ctx, customerSvc, &item)
    metrics.RecordDuration("crm_bulk:"+item.JobType, time.Since(start))
    // ...
}
```

**ReportDirtyWorker** — đo mỗi Compute theo reportKey (customer_daily, order_daily, ...):

```go
for _, d := range list {
    start := time.Now()
    if err := w.reportService.Compute(...); err != nil { ... }
    metrics.RecordDuration("report_dirty:"+d.ReportKey, time.Since(start))
    // ...
}
```

---

## 5. API duy nhất

```
GET /internal/metrics/job-metrics
```

Gọi `metrics.GetAll()` và trả về:

```json
{
  "code": 200,
  "message": "Thành công",
  "data": {
    "crm_ingest:PcPosCustomers": {
      "avgMs": 45,
      "minMs": 12,
      "maxMs": 320,
      "sampleCount": 1000,
      "countLastHour": 320
    },
    "crm_ingest:FbCustomers": {
      "avgMs": 32,
      "sampleCount": 850,
      "countLastHour": 180
    },
    "crm_bulk:sync": {
      "avgMs": 12000,
      "sampleCount": 120,
      "countLastHour": 5
    },
    "report_dirty:customer_daily": {
      "avgMs": 2300,
      "minMs": 800,
      "maxMs": 12000,
      "sampleCount": 500,
      "countLastHour": 45
    }
  },
  "status": "success"
}
```

---

## 6. Đánh giá nghẽn (mở rộng sau)

Hiện tại chỉ có metrics in-memory. Để đánh giá job nào nghẽn cần thêm **backlog** (query DB). Có thể mở rộng sau: gọi `CountUnprocessed*` khi xử lý API và trả thêm `backlog`, `etaHours` vào response.

---

## 7. Tóm tắt triển khai

| Bước | Nội dung |
|------|----------|
| 1 | Tạo `api/internal/worker/metrics/metrics.go` — ring buffer, RecordDuration, GetAll |
| 2 | Tích hợp `RecordDuration` vào từng worker |
| 3 | Thêm endpoint GET `/internal/metrics/job-metrics` — gọi GetAll, trả JSON |

---

## 8. Lưu ý

- **Thread-safety**: Dùng `sync.RWMutex` cho mỗi job type bucket
- **Job type động**: Delivery dùng `delivery:email`, `delivery:sms`, ...
- **Count 1 giờ**: Ngưỡng `time.Now().Add(-1*time.Hour)`; đếm entry có `timestamp >= ngưỡng`
