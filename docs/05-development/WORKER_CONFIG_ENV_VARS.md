# Cấu hình Worker — Ngưỡng và Mức ưu tiên

Cho phép chỉnh ngưỡng CPU/RAM và mức ưu tiên từng worker qua biến môi trường (env) hoặc **API**.

## API Endpoints

| Method | Path | Mô tả | Permission |
|--------|------|-------|------------|
| `GET` | `/api/v1/system/worker-config` | Lấy cấu hình: ngưỡng, priorities, active, schedules, pool sizes, retentions, alert webhook, state | `MongoDB.Manage` |
| `PUT` | `/api/v1/system/worker-config` | Cập nhật cấu hình (tất cả field hỗ trợ runtime) | `MongoDB.Manage` |

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
    "workerPriorities": { "report_dirty_ads": 1, "report_dirty_order": 1, "report_dirty_customer": 1, "crm_pending_merge": 2, "crm_bulk": 4, ... },
    "workerActive": { "report_dirty_ads": true, "report_dirty_customer": true, "crm_bulk": true, ... },
    "workerActiveOverrides": { "crm_bulk": false },
    "workerMetadata": { "report_dirty_ads": { "module": "report", "domain": "ads", "description": "..." }, ... },
    "reportSchedules": {
      "ads": { "interval": "2m0s", "batchSize": 20 },
      "order": { "interval": "5m0s", "batchSize": 15 },
      "customer": { "interval": "10m0s", "batchSize": 10 }
    },
    "reportScheduleOverrides": { "customer": { "interval": "24h", "batchSize": 10 } },
    "workerSchedules": {
      "crm_pending_merge": { "interval": "30s", "batchSize": 50 },
      "crm_bulk": { "interval": "2m0s", "batchSize": 2 },
      "ads_execution": { "interval": "30s", "batchSize": 10 },
      "report_dirty_ads": { "interval": "2m0s", "batchSize": 20 },
      "report_dirty_order": { "interval": "5m0s", "batchSize": 15 },
      "report_dirty_customer": { "interval": "10m0s", "batchSize": 10 }
    },
    "workerScheduleOverrides": { "crm_bulk": { "interval": "5m", "batchSize": 5 } },
    "workerPoolSizes": { "notification_delivery_processor": 6, "report_dirty_ads": 6, "report_dirty_order": 6, "report_dirty_customer": 6, "ads_execution": 4 },
    "workerPoolSizeOverrides": { "report_dirty_customer": 3 },
    "workerRetentions": { "notification_agent_activity_cleanup": 1 },
    "workerRetentionOverrides": {},
    "alertWebhookURL": "",
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
    "report_dirty_customer": 2
  },
  "workerActive": {
    "crm_bulk": false,
    "report_dirty_customer": true
  },
  "reportSchedules": {
    "ads": { "interval": "15m", "batchSize": 20 },
    "customer": { "interval": "24h", "batchSize": 10 }
  },
  "reportSchedulesClear": ["order"],
  "workerSchedules": {
    "crm_pending_merge": { "interval": "1m", "batchSize": 100 },
    "crm_bulk": { "interval": "5m", "batchSize": 5 },
    "report_dirty_customer": { "interval": "24h", "batchSize": 5 }
  },
  "workerSchedulesClear": ["notification_command_cleanup"],
  "workerPoolSizes": { "report_dirty_customer": 3, "ads_execution": 2 },
  "workerPoolSizesClear": ["notification_delivery_processor"],
  "workerRetentions": { "notification_agent_activity_cleanup": 7 },
  "workerRetentionsClear": [],
  "alertWebhookURL": "https://hooks.slack.com/..."
}
```

- **thresholds**: Chỉ gửi các field cần thay đổi. Giá trị 0 = không đổi.
- **priorities**: Map worker_name → 1–5. Gửi `0` hoặc xóa key để reset về mặc định/env.
- **workerActive**: Map worker_name → true/false. Bật/tắt từng worker: `false` = tạm dừng, `true` = chạy bình thường.
- **reportSchedules**: Map domain (ads, order, customer) → { interval, batchSize }. interval: duration string ("2m", "15m", "24h"). batchSize: 0 = không đổi.
- **reportSchedulesClear**: Mảng domain cần xóa override (dùng lại env/default).
- **workerSchedules**: Map worker_name → { interval, batchSize }. interval: "30s", "2m", "24h". batchSize: 0 = không đổi. report_dirty_ads/order/customer → map tới reportSchedules (ads, order, customer).
- **workerSchedulesClear**: Mảng worker_name cần xóa override (dùng lại env/default).
- **workerPoolSizes**: Map worker_name → pool size (số goroutine song song). Workers: notification_delivery_processor, report_dirty_ads, report_dirty_order, report_dirty_customer, ads_execution.
- **workerPoolSizesClear**: Mảng worker_name cần xóa override pool size.
- **workerRetentions**: Map worker_name → retentionDays (số ngày giữ log). Ví dụ: notification_agent_activity_cleanup: 7.
- **workerRetentionsClear**: Mảng worker_name cần xóa override retention.
- **alertWebhookURL**: URL để POST khi CPU/RAM/disk quá tải. Rỗng = tắt. Env: `WORKER_ALERT_WEBHOOK_URL`.

### Redis & báo cáo (touch → MarkDirty)

Luồng **không** MarkDirty ngay trong consumer `datachanged`: ghi key Redis `ff:rt:*`, worker **`report_redis_touch_flush`** (một process) gọi `MarkDirty` theo **ba nhịp độc lập** (ads / order / customer).

| Env (config server) | Mặc định (KD) | Mô tả |
|---------------------|---------------|--------|
| `REDIS_ADDR` | (rỗng) | Có giá trị mới kết nối Redis; rỗng = không ghi touch từ CRUD. |
| `REDIS_PASSWORD` | (rỗng) | Tuỳ chọn. |
| `REDIS_DB` | `0` | Index DB Redis. |
| `REPORT_REDIS_TOUCH_TTL_SEC` | `7200` | TTL (giây) cho key touch. |
| `REPORT_REDIS_TOUCH_FLUSH_INTERVAL_ADS_SEC` | `30` | Chu kỳ flush **ads** (ads_daily) — chiến dịch cần gần realtime. |
| `REPORT_REDIS_TOUCH_FLUSH_INTERVAL_ORDER_SEC` | `120` | Chu kỳ flush **order** (đơn / doanh thu). |
| `REPORT_REDIS_TOUCH_FLUSH_INTERVAL_CUSTOMER_SEC` | `300` | Chu kỳ flush **customer** (profile/segment đổi chậm hơn). |
| `REPORT_REDIS_TOUCH_POLL_TICK_SEC` | `5` | Bước ngủ giữa các vòng **kiểm tra** trong worker (không thay thế ba interval trên). |

API `GET /system/worker-config` → `workerSchedules.report_redis_touch_flush.interval` thể hiện **poll tick** (mặc định ~5s), không phải chu kỳ từng loại — từng loại chỉnh bằng env ở bảng trên.

Worker: **`report_redis_touch_flush`** — ưu tiên **Normal (3)**; `WORKER_REPORT_REDIS_TOUCH_FLUSH_INTERVAL` override **poll tick** (duration).

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
| `WORKER_POOL_SIZE_REPORT_DIRTY_ADS` | `6` | Số goroutine song song cho Report Dirty Ads Worker |
| `WORKER_POOL_SIZE_REPORT_DIRTY_ORDER` | `6` | Số goroutine song song cho Report Dirty Order Worker |
| `WORKER_POOL_SIZE_REPORT_DIRTY_CUSTOMER` | `6` | Số goroutine song song cho Report Dirty Customer Worker |

**API override:** `workerPoolSizes` trong PUT body — map worker_name → số. Thay đổi có hiệu lực ngay.

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

### Lịch chạy (interval, batchSize) từng worker

| Env | Mặc định | Mô tả |
|-----|----------|-------|
| `WORKER_<NAME>_INTERVAL` | (xem bảng) | Khoảng thời gian giữa các lần chạy. Ví dụ: `WORKER_CRM_PENDING_MERGE_INTERVAL=1m` |
| `WORKER_<NAME>_BATCH` | (xem bảng) | Số item mỗi batch. Ví dụ: `WORKER_CRM_BULK_BATCH=5` |

API: `workerSchedules` trong PUT body — map worker_name → { interval, batchSize }. Thay đổi có hiệu lực ngay, không cần restart.

### Retention (số ngày giữ log)

| Env | Mặc định | Mô tả |
|-----|----------|-------|
| `WORKER_<NAME>_RETENTION_DAYS` | (xem bảng) | Số ngày giữ log trước khi xóa. Ví dụ: `WORKER_NOTIFICATION_AGENT_ACTIVITY_CLEANUP_RETENTION_DAYS=7` |

API: `workerRetentions` trong PUT body — map worker_name → retentionDays.

### Alert webhook

| Env | Mặc định | Mô tả |
|-----|----------|-------|
| `WORKER_ALERT_WEBHOOK_URL` | (rỗng) | URL để POST khi CPU/RAM/disk quá tải. Payload JSON: `{ timestamp, state, cpuPercent, ramPercent, diskPercent }` |

API: `alertWebhookURL` trong PUT body — string. Rỗng = tắt.

**Workers đã hỗ trợ config runtime (interval, batch):** crm_pending_merge, crm_bulk, notification_command_cleanup, ads_execution.

**Report workers (interval, batch qua reportSchedules hoặc workerSchedules):** report_dirty_ads, report_dirty_order, report_dirty_customer — mỗi worker độc lập, config riêng (priority, active, pool size). Thêm **report_redis_touch_flush** (quét Redis → MarkDirty) — interval qua `workerSchedules` hoặc env ở bảng Redis & báo cáo phía trên.

**Workers hỗ trợ config runtime (interval, retention):** notification_agent_activity_cleanup.

**Identity Backfill Worker:**
- `WORKER_IDENTITY_BACKFILL_MODE`: `uid` | `sourceIds` | `links` | `all` — mặc định `uid`. Chế độ `all` chạy cả 3 mode trong mỗi chu kỳ.

### Domain (phân loại theo nghiệp vụ)

| Domain | Workers |
|--------|---------|
| `ads` | report_dirty_ads, ads_execution, ads_auto_propose, ads_circuit_breaker, ads_daily_scheduler, ads_pancake_heartbeat, ads_counterfactual |
| `order` | report_dirty_order |
| `customer` | report_dirty_customer, crm_pending_merge, crm_bulk, crm_classification_full, crm_classification_smart |
| `notification` | notification_delivery_processor, notification_delivery_cleanup |
| `system` | report_redis_touch_flush, notification_command_cleanup, notification_agent_command_cleanup, notification_agent_activity_cleanup |

API GET trả `workerMetadata` với field `domain` — dùng để nhóm/filter workers theo domain.

### Tên Worker (format module_suffix) và mặc định

| Tên Worker | Module | Domain | Mô tả | Env override | Mặc định |
|------------|--------|--------|-------|-------------|----------|
| `report_dirty_ads` | report | ads | Tính toán lại báo cáo ads_daily khi có dirty periods | `WORKER_PRIORITY_REPORT_DIRTY_ADS` | 1 (Critical) |
| `report_dirty_order` | report | order | Tính toán lại báo cáo order_daily khi có dirty periods | `WORKER_PRIORITY_REPORT_DIRTY_ORDER` | 1 (Critical) |
| `report_dirty_customer` | report | customer | Tính toán lại báo cáo customer_daily khi có dirty periods | `WORKER_PRIORITY_REPORT_DIRTY_CUSTOMER` | 1 (Critical) |
| `report_redis_touch_flush` | report | system | Một worker, ba nhịp flush theo prefix ads/order/customer → MarkDirty | `WORKER_PRIORITY_REPORT_REDIS_TOUCH_FLUSH` | 3 (Normal) |
| `notification_delivery_processor` | notification | notification | Xử lý hàng đợi gửi thông báo (email, Telegram, SMS...) | `WORKER_PRIORITY_NOTIFICATION_DELIVERY_PROCESSOR` | 2 (High) |
| `notification_delivery_cleanup` | notification | notification | Dọn item bị kẹt trong hàng đợi delivery | `WORKER_PRIORITY_NOTIFICATION_DELIVERY_CLEANUP` | 4 (Low) |
| `notification_command_cleanup` | notification | system | Dọn command cũ hết hạn | `WORKER_PRIORITY_NOTIFICATION_COMMAND_CLEANUP` | 4 (Low) |
| `notification_agent_command_cleanup` | notification | system | Dọn agent command cũ hết hạn | `WORKER_PRIORITY_NOTIFICATION_AGENT_COMMAND_CLEANUP` | 4 (Low) |
| `notification_agent_activity_cleanup` | notification | system | Dọn agent activity log cũ | `WORKER_PRIORITY_NOTIFICATION_AGENT_ACTIVITY_CLEANUP` | 4 (Low) |
| `crm_pending_merge` | crm | customer | Queue merge **mirror→canonical** CRM (khác CIO ingest) | `WORKER_PRIORITY_CRM_PENDING_MERGE` | 2 (High) |
| `crm_bulk` | crm | customer | Xử lý bulk job cập nhật customer hàng loạt | `WORKER_PRIORITY_CRM_BULK` | 4 (Low) |
| `ads_execution` | ads | ads | Thực thi đề xuất quảng cáo đã duyệt | `WORKER_PRIORITY_ADS_EXECUTION` | 3 (Normal) |
| `ads_auto_propose` | ads | aidecision | Auto propose (aidecision/adsautop → executor.propose_requested) | `WORKER_PRIORITY_ADS_AUTO_PROPOSE` | 3 (Normal) |
| `ads_circuit_breaker` | ads | ads | Giám sát và tạm dừng account khi lỗi Meta API | `WORKER_PRIORITY_ADS_CIRCUIT_BREAKER` | 3 (Normal) |
| `ads_daily_scheduler` | ads | ads | Lên lịch mode detection và task ads hàng ngày | `WORKER_PRIORITY_ADS_DAILY_SCHEDULER` | 3 (Normal) |
| `ads_pancake_heartbeat` | ads | ads | Gửi heartbeat đến Pancake đồng bộ trạng thái | `WORKER_PRIORITY_ADS_PANCAKE_HEARTBEAT` | 3 (Normal) |
| `ads_counterfactual` | ads | ads | Đánh giá kill đã qua 4h → counterfactual outcomes | `WORKER_PRIORITY_ADS_COUNTERFACTUAL` | 4 (Low) |
| `crm_classification_full` | crm | customer | Refresh toàn bộ phân loại khách hàng (24h) | `WORKER_PRIORITY_CRM_CLASSIFICATION_FULL` | 5 (Lowest) |
| `crm_classification_smart` | crm | customer | Refresh phân loại thông minh — khách gần ngưỡng lifecycle (6h) | `WORKER_PRIORITY_CRM_CLASSIFICATION_SMART` | 5 (Lowest) |
| `identity_backfill` | identity | system | Backfill uid, sourceIds, links cho document cũ (4 lớp identity) | `WORKER_PRIORITY_IDENTITY_BACKFILL` | 5 (Lowest) |

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
WORKER_POOL_SIZE_REPORT_DIRTY_ADS=8
WORKER_POOL_SIZE_REPORT_DIRTY_CUSTOMER=4
```

### Debug mismatch — chỉ chạy crm_bulk (recalc)

Để xác định lỗi visitor/engaged mismatch, tạm dừng tất cả job khác (report, crm_pending_merge, notification, ads):

1. Thêm vào `development.env` (hoặc env tương ứng):
   - `WORKER_CPU_THRESHOLD_THROTTLE=1` — luôn Throttled (CPU ≥ 1%) → workers priority 5 skip
   - `WORKER_PRIORITY_*` = 5 cho tất cả trừ `crm_bulk` (giữ = 1)

2. Restart server.

3. Chỉ `crm_bulk` (recalc) chạy; report, crm_pending_merge, ... sẽ skip.

4. **Lưu ý:** Webhook gọi API trực tiếp (Pancake, Meta...) vẫn có thể gọi `IngestOrderTouchpoint`. Nếu cần tắt hoàn toàn ingest, cần disable webhook ở nguồn.

**Xóa các dòng này** khi debug xong để khôi phục chạy bình thường.

---

## 3. Job ưu tiên (bypass throttle)

Job được đánh dấu ưu tiên sẽ **bắt buộc chạy** mà không bị throttle (CPU/RAM cao vẫn xử lý).

### CrmBulkJob

- **isPriority** (bool): Thêm vào body khi gọi API rebuild, recalculate-all, recalculate-one.
- **batchSize** (int): Body recalculate-all — số khách mỗi batch (mặc định 200). Rebuild tạo 2 job (sync + backfill).
- Sort: job ưu tiên lấy trước khi GetUnprocessed.
- Khi batch có ít nhất 1 job có `isPriority=true`, worker bỏ qua ShouldThrottle.

### DeliveryQueueItem

- **priority** (int): 1=critical, 2=high — item có priority 1 hoặc 2 được xử lý ngay không bị throttle.
- Sort: FindPending đã sort theo priority asc (ưu tiên trước).

---

## 3b. AI Decision — log định tuyến datachanged (quan sát, không đổi hành vi)

| Biến | Giá trị | Mô tả |
|------|---------|--------|
| `AI_DECISION_DATACHANGED_ROUTING_LOG` | `1` / `true` / `yes` | Log **Info** mỗi lần consumer áp side-effect datachanged: `routingConfigVersion`, `routingRuleId`, pipeline mirror + policy `Allow*` (mặc định chỉ **Debug** để giảm spam). |
| — | (không set) | Chỉ log **Debug** — cần bật level debug cho logger consumer. |
| `DATACHANGED_ROUTING_CONFIG` | Đường dẫn file YAML | Thay **toàn bộ** YAML định tuyến (không merge với embed). Nếu đọc/parse lỗi → dùng bản **embed** trong binary. Schema: `config_version`, `collection_overrides` — xem `api/config/datachanged_routing.example.yaml` và `api/internal/api/aidecision/datachangedrouting/routing.default.yaml`. |

Code: `api/internal/api/aidecision/datachangedrouting/` — `Version` trong `version.go` tăng khi đổi bảng định tuyến; struct chung `routecontract.Decision` (tránh vòng import với `datachangedsidefx`).

---

## 4. File tham chiếu

- `api/internal/worker/controller.go` — Throttle logic, `GetEffectivePoolSize`, alert webhook, đọc ngưỡng từ env
- `api/internal/worker/config.go` — `GetPriority`, `GetPoolSize`, đọc override từ env
- `api/internal/worker/schedule.go` — `GetEffectiveWorkerSchedule`, worker schedules (interval, batch)
- `api/internal/worker/retention.go` — `GetEffectiveWorkerRetention`, retention days
