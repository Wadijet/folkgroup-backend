# Đề xuất: Tần suất chạy khác nhau theo domain báo cáo (ads / order / customer)

**Ngày:** 2025-03-15  
**Trạng thái:** ✅ Đã triển khai — ReportDirtyWorker, ReportScheduleConfig, API reportSchedules.

**Bối cảnh:** Hiện chỉ còn 3 loại daily (ads_daily, order_daily, customer_daily). Chu kỳ tuần/tháng/năm đã tắt. ReportDirtyWorker xử lý cả 3 cùng interval (2 phút) → tràn CPU khi queue customer_daily lớn.

**Ý tưởng:** Mỗi domain có tần suất riêng theo mức độ cần thiết của nhu cầu nghiệp vụ.

---

## 1. Phân tích nhu cầu theo domain

| Domain   | Nhu cầu cập nhật | Tải CPU | Ghi chú |
|----------|-------------------|---------|---------|
| **ads**  | Cao — team ads cần xem spend/performance sớm để tối ưu campaign | Thấp (aggregate từ Meta) | Có thể chạy thường xuyên |
| **order**| Trung bình — báo cáo doanh thu, đơn hàng | Trung bình | Cân bằng |
| **customer** | Thấp hơn — CRM, phân khúc, LTV thường xem theo ngày | **Cao** (aggregate snapshot, compute phát sinh) | Chạy thưa hơn vừa giảm CPU vừa đủ kịp |

---

## 2. Đề xuất tần suất theo domain

| reportKey     | Interval (poll) | Batch/cycle | Lý do |
|---------------|-----------------|-------------|-------|
| **ads_daily** | 2–3 phút        | 20          | Cần cập nhật nhanh, nhẹ CPU |
| **order_daily** | 5 phút       | 15          | Nhu cầu trung bình |
| **customer_daily** | 10 phút   | 10          | Nặng nhất, chạy thưa để giảm tải CPU |

**Thứ tự ưu tiên:** ads > order > customer (theo độ cấp thiết + tải CPU).

---

## 3. Cấu trúc triển khai

### 3.1 Một worker, 3 ticker (mỗi domain một ticker)

```
ReportDirtyWorker
├── ticker ads (2 phút)     → GetUnprocessedDirtyPeriodsByReportKeys(limit=20, ["ads_daily"])
├── ticker order (5 phút)   → GetUnprocessedDirtyPeriodsByReportKeys(limit=15, ["order_daily"])
└── ticker customer (10 phút) → GetUnprocessedDirtyPeriodsByReportKeys(limit=10, ["customer_daily"])
```

### 3.2 Config đề xuất

```go
type ReportDomainConfig struct {
    ReportKeys []string
    Interval   time.Duration
    BatchSize  int
}

var reportDomainTiers = []ReportDomainConfig{
    {ReportKeys: []string{"ads_daily"},     Interval: 2 * time.Minute, BatchSize: 20},
    {ReportKeys: []string{"order_daily"},   Interval: 5 * time.Minute, BatchSize: 15},
    {ReportKeys: []string{"customer_daily"}, Interval: 10 * time.Minute, BatchSize: 10},
}
```

### 3.3 Thay đổi cần thiết

1. **ReportService:** Thêm `GetUnprocessedDirtyPeriodsByReportKeys(ctx, limit, reportKeys []string)`
2. **ReportDirtyWorker:** Thay 1 ticker bằng 3 goroutine, mỗi goroutine 1 ticker theo config
3. **main.go:** Truyền config thay vì interval/batch cố định

---

## 4. Cấu hình (env + API runtime)

### 4.1 Env (khi khởi động)

| Env | Mặc định | Mô tả |
|-----|----------|-------|
| `REPORT_ADS_INTERVAL` | `2m` | Duration ads_daily — hỗ trợ: `2m`, `15m`, `1h`, `24h` |
| `REPORT_ADS_BATCH` | 20 | Batch ads_daily |
| `REPORT_ORDER_INTERVAL` | `5m` | Duration order_daily |
| `REPORT_ORDER_BATCH` | 15 | Batch order_daily |
| `REPORT_CUSTOMER_INTERVAL` | `10m` | Duration customer_daily — ví dụ `24h` = chạy 1 lần/ngày |
| `REPORT_CUSTOMER_BATCH` | 10 | Batch customer_daily |

### 4.2 API runtime (PUT /api/v1/system/worker-config)

Thay đổi lịch chạy từ server mà không cần restart:

```json
{
  "reportSchedules": {
    "customer": { "interval": "24h", "batchSize": 10 },
    "ads": { "interval": "15m", "batchSize": 20 }
  },
  "reportSchedulesClear": ["order"]
}
```

- **reportSchedules**: Override interval/batch cho domain (ads, order, customer). interval: "2m", "15m", "24h". batchSize: 0 = không đổi.
- **reportSchedulesClear**: Xóa override cho domain, dùng lại env/default.

---

## 5. Kết quả mong đợi

- **ads_daily:** Cập nhật nhanh (2 phút), phù hợp nhu cầu team ads
- **order_daily:** Cân bằng (5 phút)
- **customer_daily:** Chạy thưa (10 phút), batch nhỏ → giảm tràn CPU, vẫn đủ cho CRM
- **Tổng thể:** Tải CPU phân tán, không dồn 3 domain cùng lúc
