# Đề xuất tối ưu toàn bộ hoạt động tính toán

**Ngày rà soát:** 2025-03-10  
**Phạm vi:** Workers, Report Compute, CRM Ingest/Sync/Backfill/Recalculate, Ads Evaluation, Delivery.

---

## 1. Tổng quan hiện trạng

### 1.1 Bảng Workers và tham số

| Worker | Interval | BatchSize | Priority | Ghi chú |
|--------|----------|-----------|----------|---------|
| WORKER_CONTROLLER | 3s | - | - | Lấy mẫu CPU/RAM, throttle |
| DELIVERY | 5s | 10 | High (2) | Gửi notification |
| DELIVERY_CLEANUP | 1 phút | 50 | Low (4) | Goroutine trong Processor |
| COMMAND_CLEANUP | 1 phút | 300 (timeout) | Low (4) | Release stuck commands |
| AGENT_COMMAND_CLEANUP | 1 phút | 300 (timeout) | Low (4) | Release stuck agent commands |
| AGENT_ACTIVITY_CLEANUP | 1 giờ | 1 (retention) | Low (4) | Xóa activity cũ |
| REPORT_DIRTY | 2 phút | 30 | **Critical (1)** | Compute report snapshots |
| CRM_INGEST | 30s | 50 (adaptive max 100) | High (2) | Merge/Ingest từ pending |
| CRM_BULK | 2 phút | **2** | Low (4) | Sync/Backfill/Rebuild/Recalc |
| ADS_EXECUTION | 30s | 10 | Normal (3) | Execute ads actions |
| ADS_AUTO_PROPOSE | 30 phút | - | Normal (3) | Momentum + Auto propose |
| ADS_CIRCUIT_BREAKER | 10 phút | - | Normal (3) | Kiểm tra CB |
| ADS_DAILY_SCHEDULER | 1 phút | - | Normal (3) | Jobs theo giờ cố định |
| ADS_PANCAKE_HEARTBEAT | 15 phút | - | Normal (3) | Kiểm tra POS sync |
| CLASSIFICATION_FULL | 24h | 200 | **Lowest (5)** | Refresh toàn bộ phân loại |
| CLASSIFICATION_SMART | 6h | 200 | **Lowest (5)** | Refresh khách gần ngưỡng |

### 1.2 Throttle logic (controller.go)

| Trạng thái | CPU | RAM | Hành vi |
|------------|-----|-----|---------|
| **Normal** | < 40% | < 60% | Tất cả workers chạy bình thường |
| **Throttled** | ≥ 40% | ≥ 60% | Lowest skip; các worker khác interval × multiplier |
| **Paused** | ≥ 60% | ≥ 75% | **Chỉ Critical chạy**; High/Normal/Low/Lowest đều skip |

**Hậu quả:** Khi Paused, CRM_BULK, CRM_INGEST, Delivery, Ads workers đều không chạy → job tích tụ.

### 1.3 Report config (đã tối ưu sẵn)

- **Customer:** Chỉ `customer_daily` tính định kỳ; weekly/monthly/yearly on-demand.
- **Order:** Chỉ `order_daily`; weekly/monthly/yearly on-demand.
- **Ads:** `ads_daily` bật.

---

## 2. Đề xuất tối ưu theo mức ưu tiên

### 2.1 Ưu tiên cao

#### 2.1.1 Tăng CRM_BULK batchSize

**Vấn đề:** `batchSize = 2` → rebuild/recalculate toàn org rất chậm, job tích tụ.

**Đề xuất:**
- Tăng lên **5–10** (hoặc cấu hình qua env `CRM_BULK_BATCH_SIZE`).
- Mỗi job rebuild/recalculate có thể chạy lâu; batch 2 quá thấp.

**File:** `api/cmd/server/main.go` dòng 291.

```go
// Trước
worker.NewCrmBulkWorker(2*time.Minute, 2)

// Sau (đề xuất)
batchSize := 5
if v := os.Getenv("CRM_BULK_BATCH_SIZE"); v != "" {
    if n, _ := strconv.Atoi(v); n > 0 { batchSize = n }
}
worker.NewCrmBulkWorker(2*time.Minute, batchSize)
```

#### 2.1.2 Nâng priority CRM_BULK khi bị Paused

**Vấn đề:** CRM_BULK có PriorityLow → khi Paused hoàn toàn không chạy. User tạo job rebuild/recalculate nhưng không thấy chạy.

**Đề xuất (lựa chọn):**
- **A)** Giữ Low nhưng tăng batchSize + log rõ khi throttle (đã có).
- **B)** Nâng lên PriorityNormal để khi Throttled vẫn chạy (chậm hơn); khi Paused vẫn skip.
- **C)** Thêm cơ chế "burst": khi có job mới, cho phép 1 lần chạy ngay bất kể throttle (cần thiết kế cẩn thận).

**Khuyến nghị:** A + B — tăng batchSize và cân nhắc nâng PriorityNormal nếu CRM bulk là nghiệp vụ quan trọng.

#### 2.1.3 Report Dirty — Adaptive batch khi backlog cao

**Vấn đề:** Report Dirty batchSize cố định 30; khi backlog lớn (nhiều org, nhiều period) xử lý chậm.

**Đề xuất:** Áp dụng pattern tương tự CRM Ingest:

```go
// Trong report_dirty_worker.go, trước vòng lặp:
baseBatchSize := GetEffectiveBatchSize(w.batchSize, PriorityCritical)
batchSize := baseBatchSize
if count, err := w.reportService.CountUnprocessedDirtyPeriods(ctx); err == nil && count > int64(baseBatchSize*3) {
    adaptive := int(count / 2)
    if adaptive > 100 { adaptive = 100 }
    if adaptive > batchSize { batchSize = adaptive }
}
```

**Điều kiện:** Cần thêm `CountUnprocessedDirtyPeriods` trong ReportService (tương tự `CountUnprocessedCrmIngest`).

---

### 2.2 Ưu tiên trung bình

#### 2.2.1 Stagger Classification workers

**Vấn đề:** Cả CLASSIFICATION_FULL và CLASSIFICATION_SMART đều `time.Sleep(1*time.Minute)` sau startup → có thể trùng tải nếu chạy gần nhau.

**Đề xuất:**
- Full: sleep **5 phút** (chạy ít hơn, không cần gấp).
- Smart: giữ **1 phút** (chạy 4 lần/ngày, cần sớm hơn).

**File:** `api/internal/worker/classification_refresh_worker.go` — cần tham số hóa sleep duration qua constructor.

#### 2.2.2 ADS Daily Scheduler — Giảm tần suất check

**Vấn đề:** Worker check mỗi phút nhưng job chỉ chạy ở các mốc cố định (05:30, 06:00, 07:30, 12:30, 14:00, 14:30, 16:00, 18:00, 21–23h). 59/60 lần check không làm gì.

**Đề xuất:** Tăng interval lên **5 phút**. Các mốc giờ vẫn đủ chính xác (sai lệch tối đa 5 phút).

**File:** `api/cmd/server/main.go`:
```go
// Trước
adsworker.NewAdsDailySchedulerWorker(1*time.Minute, baseURL)

// Sau
adsworker.NewAdsDailySchedulerWorker(5*time.Minute, baseURL)
```

**Lưu ý:** Cần đảm bảo `worker.ads.daily_scheduler.go` chấp nhận interval ≥ 1 phút (hiện `interval < 30s` mới force 1 phút).

#### 2.2.3 Backfill batchSize cấu hình qua env

**Vấn đề:** `backfillBatchSize = 1000` cố định trong `service.crm.backfill.go`. Trên server RAM thấp có thể gây áp lực.

**Đề xuất:** Cho phép override qua env `CRM_BACKFILL_BATCH_SIZE` (mặc định 1000).

---

### 2.3 Ưu tiên thấp

#### 2.3.1 Metrics cho Ads workers

**Vấn đề:** Một số workers đã có `metrics.RecordDuration`; Ads workers (Auto Propose, Circuit Breaker, Daily Scheduler) chưa có.

**Đề xuất:** Thêm `metrics.RecordDuration` cho từng job trong Ads workers để theo dõi latency.

#### 2.3.2 Timezone cấu hình cho Ads Daily Scheduler

**Vấn đề:** `Asia/Ho_Chi_Minh` hardcode trong `worker.ads.daily_scheduler.go:63`.

**Đề xuất:** Lấy timezone từ config/env nếu cần multi-region.

#### 2.3.3 Command/Agent Cleanup — Xem xét PriorityLowest

**Hiện trạng:** Cả hai dùng `PriorityLow`. Khi Throttled vẫn chạy (interval × 4).

**Đề xuất:** Nếu cleanup không gấp, có thể đổi sang `PriorityLowest` để khi Throttled bị skip, dành tài nguyên cho Report/CRM Ingest/Delivery. Cần cân nhắc: cleanup quá chậm có thể để commands stuck lâu hơn.

---

## 3. Tối ưu logic tính toán (không phải worker)

### 3.1 Report Compute

- **Đã tối ưu:** Chỉ daily tính định kỳ; weekly/monthly/yearly on-demand.
- **Kiểm tra thêm:** Aggregation pipeline trong `Compute` — có thể thêm index hoặc tối ưu stage nếu query chậm.

### 3.2 Classification Refresh — RefreshMetrics

- **Đã tối ưu:** Smart mode chỉ refresh khách gần ngưỡng lifecycle.
- **Kiểm tra:** `aggregateOrderMetricsForCustomer`, `aggregateConversationMetricsForCustomer` — đảm bảo có index phù hợp (ownerOrganizationId, createdAt, customerId...).

### 3.3 Ads Evaluation

- **RunAutoPropose** + **EvaluateAlertFlagsWithConfig** chạy mỗi 30 phút.
- Kiểm tra: có N+1 query không; có thể batch evaluation theo ad account.

---

## 4. Checklist triển khai

| # | Đề xuất | Effort | Impact | Ghi chú |
|---|---------|--------|--------|---------|
| 1 | CRM_BULK batchSize 5–10 + env | Thấp | Cao | Dễ triển khai |
| 2 | CRM_BULK PriorityNormal (tùy chọn) | Thấp | Trung bình | Cân nhắc nghiệp vụ |
| 3 | Report Dirty adaptive batch | Trung bình | Cao | Cần thêm CountUnprocessedDirtyPeriods |
| 4 | Stagger Classification sleep | Thấp | Thấp | Giảm spike tải |
| 5 | ADS Daily Scheduler 5 phút | Thấp | Thấp | Giảm overhead |
| 6 | Backfill batchSize env | Thấp | Thấp | Linh hoạt cho server nhỏ |
| 7 | Metrics Ads workers | Thấp | Trung bình | Observability |
| 8 | Timezone config Ads | Thấp | Thấp | Multi-region |

---

## 5. Sơ đồ phụ thuộc (tóm tắt)

```
[Data: pc_pos_orders, fb_conversations, crm_notes, meta_ads...]
    │
    ├─► MarkDirty ──► REPORT_DIRTY (2p, batch 30) ──► Compute ──► Snapshots
    │
    └─► EnqueueCrmIngest ──► CRM_INGEST (30s, adaptive) ──► Merge/Ingest
               │
               └─► MarkDirty (customer) ──► REPORT_DIRTY

[API: rebuild/recalculate] ──► crm_bulk_jobs ──► CRM_BULK (2p, batch 2) ──► Sync/Backfill/Rebuild/Recalc

[meta_ads.currentMetrics] ──► ADS_AUTO_PROPOSE (30p) ──► action_pending_approval
                                                              │
                                                              └─► ADS_EXECUTION (30s) ──► Meta API
```

---

## 6. Tài liệu tham khảo

- `api/cmd/server/main.go` — Đăng ký workers
- `api/internal/worker/controller.go` — Throttle logic
- `api/internal/api/report/service/service.report.config.go` — Report keys
- `docs/05-development/de-xuat-do-luong-thoi-gian-job.md` — Đo lường job
