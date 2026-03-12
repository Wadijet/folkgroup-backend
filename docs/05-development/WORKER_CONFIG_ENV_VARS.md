# Cấu hình Worker — Ngưỡng và Mức ưu tiên

Cho phép chỉnh ngưỡng CPU/RAM và mức ưu tiên từng worker qua biến môi trường (env) hoặc **API**.

## API Endpoints

| Method | Path | Mô tả | Permission |
|--------|------|-------|------------|
| `GET` | `/api/v1/system/worker-config` | Lấy cấu hình hiện tại (ngưỡng + priorities + state) | `MongoDB.Manage` |
| `PUT` | `/api/v1/system/worker-config` | Cập nhật cấu hình (ngưỡng + priorities) | `MongoDB.Manage` |

### GET Response

```json
{
  "code": 200,
  "message": "Thành công",
  "data": {
    "thresholds": {
      "enabled": true,
      "cpuThresholdThrottle": 40,
      "cpuThresholdPause": 60,
      "cpuThresholdAlert": 95,
      "ramThresholdThrottle": 60,
      "ramThresholdPause": 75,
      "ramThresholdAlert": 95,
      ...
    },
    "priorities": { "crm_bulk": 3 },
    "workerPriorities": { "report_dirty": 1, "crm_ingest": 2, "crm_bulk": 4, ... },
    "workerActive": { "report_dirty": true, "crm_bulk": true, ... },
    "workerActiveOverrides": { "crm_bulk": false },
    "workerMetadata": {
      "report_dirty": { "module": "report", "description": "Tính toán lại báo cáo khi có dirty periods (order, customer, ads)" },
      "crm_ingest": { "module": "crm", "description": "Đồng bộ dữ liệu customer từ agent vào hệ thống" },
      ...
    },
    "state": "normal",
    "cpuPercent": 25.5,
    "ramPercent": 45.2,
    "diskPercent": 60.1
  },
  "status": "success"
}
```

### PUT Body

```json
{
  "thresholds": {
    "enabled": true,
    "cpuThresholdPause": 70,
    "ramThresholdPause": 80
  },
  "priorities": {
    "crm_bulk": 3,
    "report_dirty": 1
  },
  "workerActive": {
    "crm_bulk": false,
    "report_dirty": true
  }
}
```

- **thresholds**: Chỉ gửi các field cần thay đổi. Giá trị 0 = không đổi.
- **priorities**: Map worker_name → 1–5. Gửi `0` hoặc xóa key để reset về mặc định/env.
- **workerActive**: Map worker_name → true/false. Bật/tắt từng worker: `false` = tạm dừng, `true` = chạy bình thường.

---

## 1. Ngưỡng Throttle (CPU, RAM, Disk)

> **Lưu ý API hiện tại:**
> - **CPU**: Đủ 3 mức (throttle, pause, alert)
> - **RAM**: Đủ 3 mức (throttle, pause, alert)
> - **Disk**: Chỉ có cảnh báo (chưa có giảm tốc/tạm dừng)
>
> Nếu backend bổ sung thêm ngưỡng (vd: CPU alert, Disk throttle/pause), có thể thêm vào từng nhóm tương ứng.

| Env | Mặc định | Mô tả |
|-----|----------|-------|
| `WORKER_CPU_THROTTLE_ENABLED` | `true` | Bật/tắt throttle (`true`, `1` = bật) |
| `WORKER_CPU_THRESHOLD_THROTTLE` | `40` | % CPU để chuyển **Throttled** (interval × multiplier, Lowest skip) |
| `WORKER_CPU_THRESHOLD_PAUSE` | `60` | % CPU để **Paused** (chỉ Critical chạy) |
| `WORKER_CPU_THRESHOLD_ALERT` | `95` | % CPU để gửi cảnh báo |
| `WORKER_RAM_THRESHOLD_THROTTLE` | `60` | % RAM để Throttled |
| `WORKER_RAM_THRESHOLD_PAUSE` | `75` | % RAM để Paused |
| `WORKER_RAM_THRESHOLD_ALERT` | `95` | % RAM để gửi cảnh báo |
| `WORKER_DISK_THRESHOLD_ALERT` | `90` | % Disk để gửi cảnh báo |
| `WORKER_CPU_SAMPLE_INTERVAL` | `3` | Giây giữa các lần lấy mẫu CPU |
| `WORKER_THROTTLE_INTERVAL_MULTIPLIER` | `4` | Khi Throttled: interval × multiplier |
| `WORKER_THROTTLE_BATCH_DIVISOR` | `3` | Khi Throttled: batchSize / divisor |

### Worker Pool (song song hóa trong batch)

Ba worker sau dùng worker pool để xử lý song song items trong mỗi batch. Pool size giảm tự động khi Throttled/Paused.

| Env | Mặc định | Mô tả |
|-----|----------|-------|
| `WORKER_POOL_SIZE_DELIVERY` | `6` | Số goroutine song song cho Delivery Processor (notification_delivery_processor) |
| `WORKER_POOL_SIZE_ADS_EXECUTION` | `4` | Số goroutine song song cho Ads Execution Worker |
| `WORKER_POOL_SIZE_REPORT_DIRTY` | `6` | Số goroutine song song cho Report Dirty Worker |

**Điều chỉnh theo CPU/RAM qua Controller:**
- **Throttled**: pool size = base / 2 (tối thiểu 1)
- **Paused**: pool size = 1 (chạy tuần tự để giảm tải)

### Trạng thái

| Trạng thái | Điều kiện | Hành vi |
|------------|-----------|---------|
| **Normal** | CPU < 40% và RAM < 60% | Tất cả workers chạy bình thường |
| **Throttled** | CPU ≥ 40% hoặc RAM ≥ 60% | Lowest skip; các worker khác interval × multiplier, batch / divisor |
| **Paused** | CPU ≥ 60% hoặc RAM ≥ 75% | **Chỉ Critical chạy**; High/Normal/Low/Lowest đều skip |

---

## 2. Mức ưu tiên Worker

| Env | Mặc định | Mô tả |
|-----|----------|-------|
| `WORKER_PRIORITY_<NAME>` | (xem bảng dưới) | Override mức ưu tiên worker. Giá trị: 1–5 |

### Giá trị Priority

| Số | Tên | Khi Throttled | Khi Paused |
|----|-----|---------------|------------|
| 1 | Critical | Chạy bình thường | Chạy (interval × 2) |
| 2 | High | interval × 2 | Skip |
| 3 | Normal | interval × 4 | Skip |
| 4 | Low | interval × 4 | Skip |
| 5 | Lowest | **Skip** | Skip |

### Active/Inactive từng worker

| Env | Mặc định | Mô tả |
|-----|----------|-------|
| `WORKER_ACTIVE_<NAME>` | `true` | Bật/tắt worker. `true` = chạy, `false` = tạm dừng (không xử lý job). |

API: `workerActive` trong PUT body — map worker_name → true/false.

### Tên Worker (format module_suffix) và mặc định

| Tên Worker | Module | Mô tả | Env override | Mặc định |
|------------|--------|-------|-------------|----------|
| `report_dirty` | report | Tính toán lại báo cáo khi có dirty periods | `WORKER_PRIORITY_REPORT_DIRTY` | 1 (Critical) |
| `notification_delivery_processor` | notification | Xử lý hàng đợi gửi thông báo (email, Telegram, SMS...) | `WORKER_PRIORITY_NOTIFICATION_DELIVERY_PROCESSOR` | 2 (High) |
| `notification_delivery_cleanup` | notification | Dọn item bị kẹt trong hàng đợi delivery | `WORKER_PRIORITY_NOTIFICATION_DELIVERY_CLEANUP` | 4 (Low) |
| `notification_command_cleanup` | notification | Dọn command cũ hết hạn | `WORKER_PRIORITY_NOTIFICATION_COMMAND_CLEANUP` | 4 (Low) |
| `notification_agent_command_cleanup` | notification | Dọn agent command cũ hết hạn | `WORKER_PRIORITY_NOTIFICATION_AGENT_COMMAND_CLEANUP` | 4 (Low) |
| `notification_agent_activity_cleanup` | notification | Dọn agent activity log cũ | `WORKER_PRIORITY_NOTIFICATION_AGENT_ACTIVITY_CLEANUP` | 4 (Low) |
| `crm_ingest` | crm | Đồng bộ dữ liệu customer từ agent vào hệ thống | `WORKER_PRIORITY_CRM_INGEST` | 2 (High) |
| `crm_bulk` | crm | Xử lý bulk job cập nhật customer hàng loạt | `WORKER_PRIORITY_CRM_BULK` | 4 (Low) |
| `ads_execution` | ads | Thực thi đề xuất quảng cáo đã duyệt | `WORKER_PRIORITY_ADS_EXECUTION` | 3 (Normal) |
| `ads_auto_propose` | ads | Tạo đề xuất quảng cáo tự động theo rule | `WORKER_PRIORITY_ADS_AUTO_PROPOSE` | 3 (Normal) |
| `ads_circuit_breaker` | ads | Giám sát và tạm dừng account khi lỗi Meta API | `WORKER_PRIORITY_ADS_CIRCUIT_BREAKER` | 3 (Normal) |
| `ads_daily_scheduler` | ads | Lên lịch mode detection và task ads hàng ngày | `WORKER_PRIORITY_ADS_DAILY_SCHEDULER` | 3 (Normal) |
| `ads_pancake_heartbeat` | ads | Gửi heartbeat đến Pancake đồng bộ trạng thái | `WORKER_PRIORITY_ADS_PANCAKE_HEARTBEAT` | 3 (Normal) |
| `crm_classification_full` | crm | Refresh toàn bộ phân loại khách hàng (24h) | `WORKER_PRIORITY_CRM_CLASSIFICATION_FULL` | 5 (Lowest) |
| `crm_classification_smart` | crm | Refresh phân loại thông minh — khách gần ngưỡng lifecycle (6h) | `WORKER_PRIORITY_CRM_CLASSIFICATION_SMART` | 5 (Lowest) |

### Ví dụ override

```bash
# Nâng crm_bulk lên Normal (3) — chạy khi Throttled, ít bị skip hơn
WORKER_PRIORITY_CRM_BULK=3

# Hạ ngưỡng Pause — Paused sớm hơn khi CPU cao
WORKER_CPU_THRESHOLD_PAUSE=50

# Tăng ngưỡng CPU — chịu tải cao hơn trước khi Throttle
WORKER_CPU_THRESHOLD_THROTTLE=60
WORKER_CPU_THRESHOLD_PAUSE=80

# Worker pool — tăng song song hóa (Delivery, Ads, Report Dirty)
WORKER_POOL_SIZE_DELIVERY=8
WORKER_POOL_SIZE_ADS_EXECUTION=6
WORKER_POOL_SIZE_REPORT_DIRTY=8
```

### Debug mismatch — chỉ chạy crm_bulk (recalc)

Để xác định lỗi visitor/engaged mismatch, tạm dừng tất cả job khác (report, crm_ingest, notification, ads):

1. Thêm vào `development.env` (hoặc env tương ứng):
   - `WORKER_CPU_THRESHOLD_THROTTLE=1` — luôn Throttled (CPU ≥ 1%) → workers priority 5 skip
   - `WORKER_PRIORITY_*` = 5 cho tất cả trừ `crm_bulk` (giữ = 1)

2. Restart server.

3. Chỉ `crm_bulk` (recalc) chạy; report, crm_ingest, ... sẽ skip.

4. **Lưu ý:** Webhook gọi API trực tiếp (Pancake, Meta...) vẫn có thể gọi `IngestOrderTouchpoint`. Nếu cần tắt hoàn toàn ingest, cần disable webhook ở nguồn.

**Xóa các dòng này** khi debug xong để khôi phục chạy bình thường.

---

## 3. Job ưu tiên (bypass throttle)

Job được đánh dấu ưu tiên sẽ **bắt buộc chạy** mà không bị throttle (CPU/RAM cao vẫn xử lý).

### CrmBulkJob

- **isPriority** (bool): Thêm vào body khi gọi API rebuild/recalculate-all/recalculate-one.
- Sort: job ưu tiên lấy trước khi GetUnprocessed.
- Khi batch có ít nhất 1 job có `isPriority=true`, worker bỏ qua ShouldThrottle.

### DeliveryQueueItem

- **priority** (int): 1=critical, 2=high — item có priority 1 hoặc 2 được xử lý ngay không bị throttle.
- Sort: FindPending đã sort theo priority asc (ưu tiên trước).

---

## 4. File tham chiếu

- `api/internal/worker/controller.go` — Throttle logic, `GetEffectivePoolSize`, đọc ngưỡng từ env
- `api/internal/worker/config.go` — `GetPriority`, `GetPoolSize`, đọc override từ env
