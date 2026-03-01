# Đề xuất: Hệ thống quản lý Worker theo tải CPU

## 1. Mục tiêu

Khi CPU quá tải (vd: sync Pancake hàng loạt, nhiều request API), workers phải:
- **Tạm dừng** hoặc **chậm lại** để ưu tiên tài nguyên cho request chính
- **Tự động tiếp tục** khi CPU giảm xuống mức bình thường

---

## 2. Kiến trúc đề xuất

### 2.1 Thành phần

```
┌─────────────────────────────────────────────────────────────────┐
│                    WorkerController (singleton)                    │
│  - Lấy mẫu CPU mỗi N giây (vd: 10s)                              │
│  - Tính trạng thái: Normal | Throttled | Paused                   │
│  - Expose: ShouldRun(workerName) bool                             │
│  - Expose: GetEffectiveInterval(baseInterval) time.Duration       │
└─────────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        ▼                     ▼                     ▼
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│ CrmIngest     │   │ ReportDirty    │   │ Classification │
│ Worker        │   │ Worker         │   │ Worker         │
│               │   │               │   │               │
│ Trước tick:   │   │ Trước tick:   │   │ Trước tick:   │
│ if !ShouldRun │   │ if !ShouldRun │   │ if !ShouldRun │
│   skip        │   │   skip        │   │   skip        │
└───────────────┘   └───────────────┘   └───────────────┘
```

### 2.2 Trạng thái CPU

| Trạng thái | Điều kiện (CPU%) | Hành động |
|------------|------------------|------------|
| **Normal** | CPU < 70% | Workers chạy bình thường |
| **Throttled** | 70% ≤ CPU < 90% | Tăng interval (vd: x3), giảm batchSize (vd: /2) |
| **Paused** | CPU ≥ 90% | Bỏ qua chu kỳ, đợi đến lần sample tiếp theo |

### 2.3 Cấu hình (env hoặc config)

| Key | Mặc định | Mô tả |
|-----|----------|-------|
| `WORKER_CPU_THROTTLE_ENABLED` | `true` | Bật/tắt throttle |
| `WORKER_CPU_THRESHOLD_THROTTLE` | `70` | Ngưỡng % CPU để chuyển Throttled |
| `WORKER_CPU_THRESHOLD_PAUSE` | `90` | Ngưỡng % CPU để Paused |
| `WORKER_CPU_SAMPLE_INTERVAL` | `10` | Giây giữa các lần lấy mẫu CPU |
| `WORKER_THROTTLE_INTERVAL_MULTIPLIER` | `3` | Khi Throttled: interval *= multiplier |
| `WORKER_THROTTLE_BATCH_DIVISOR` | `2` | Khi Throttled: batchSize /= divisor |

---

## 3. Đo lường CPU trong Go

### 3.1 Thư viện đề xuất: `github.com/shirou/gopsutil/v3/cpu`

```go
import "github.com/shirou/gopsutil/v3/cpu"

// Lấy CPU % (trung bình tất cả core) trong 1 giây
percent, err := cpu.Percent(time.Second, false)
// percent[0] = 45.2 (ví dụ)
```

- Cross-platform (Windows, Linux, macOS)
- Nhẹ, ít dependency

### 3.2 Thay thế không dùng thư viện ngoài

- **Windows**: `wmic cpu get loadpercentage` (chậm, cần parse)
- **Linux**: đọc `/proc/stat` (phức tạp, cần tính delta)
- **Khuyến nghị**: Dùng `gopsutil` để đơn giản và chính xác

---

## 4. Tích hợp vào Workers hiện tại

### 4.1 Cách 1: Check trước mỗi chu kỳ (ít xâm lấn)

Mỗi worker thêm 1 dòng trước khi xử lý:

```go
case <-ticker.C:
    if worker.ShouldThrottle("crm_ingest") {
        continue // Bỏ qua chu kỳ này
    }
    // ... logic xử lý như cũ
```

### 4.2 Cách 2: Interval động (linh hoạt hơn)

Thay vì `ticker` cố định, dùng `time.Sleep` với interval lấy từ controller:

```go
for {
    select {
    case <-ctx.Done():
        return
    default:
        interval := worker.GetEffectiveInterval(w.interval)
        time.Sleep(interval)
        if worker.ShouldThrottle("crm_ingest") {
            continue
        }
        // ... xử lý
    }
}
```

### 4.3 Cách 3: Worker interface chuẩn (refactor nhiều)

Định nghĩa interface `ThrottledWorker`:

```go
type ThrottledWorker interface {
    Name() string
    RunOnce(ctx context.Context) (processed int, err error)
}
```

Controller chạy tất cả workers trong vòng lặp chung, kiểm soát thứ tự và throttle tập trung.

---

## 5. Thứ tự triển khai đề xuất

| Bước | Nội dung | Effort |
|------|----------|--------|
| 1 | Thêm `gopsutil` dependency | Thấp |
| 2 | Tạo `WorkerController` (sample CPU, trạng thái Normal/Throttled/Paused) | TB |
| 3 | Thêm `ShouldThrottle(workerName)` và `GetEffectiveInterval` | Thấp |
| 4 | Tích hợp vào CrmIngestWorker, ReportDirtyWorker (cách 1) | Thấp |
| 5 | Tích hợp vào ClassificationRefreshWorker, AgentCommandCleanupWorker | Thấp |
| 6 | Thêm config/env, log khi chuyển trạng thái | Thấp |

---

## 6. Lưu ý

- **Độ trễ chấp nhận được**: Khi Throttled/Paused, queue (crm_pending_ingest, report_dirty_periods) sẽ tích tụ. Worker sẽ xử lý dần khi CPU hạ. Cần đảm bảo queue có giới hạn hoặc cảnh báo khi quá lớn.
- **CPU của process hay hệ thống?**: Nên dùng CPU **của toàn hệ thống** (gopsutil mặc định) vì nhiều process chia sẻ. Nếu cần chỉ CPU của process Go, có thể dùng `cpu.Percent` với option phù hợp (gopsutil có hỗ trợ per-process trên một số OS).
- **Memory**: Có thể mở rộng tương tự với ngưỡng RAM (vd: Paused khi RAM > 85%).

---

## 7. Tài liệu tham khảo

- `api/internal/worker/` — Các worker hiện tại
- `api/cmd/server/main.go` — Nơi khởi động workers
- [gopsutil cpu](https://github.com/shirou/gopsutil#cpu-usage)
