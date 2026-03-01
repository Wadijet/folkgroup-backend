# Đề xuất tăng hiệu suất API Report / Dashboard

## Tổng quan

Các API report (trend, period-end-balance, dashboard customers) có thể chậm do:
1. **Nhiều truy vấn DB** cho mỗi request (đặc biệt `trend-from-crm`)
2. **Aggregation scan** trên `crm_activity_history` với lượng document lớn
3. **Đọc nhiều snapshot** từ 2000 đến hiện tại (period-end-balance-from-snapshots)
4. **Thiếu index tối ưu** cho các query pattern report

Tài liệu này đề xuất các phương án cải thiện theo mức độ ưu tiên và độ phức tạp triển khai.

---

## 1. Phân tích điểm nghẽn (bottleneck)

### 1.1 `trend-from-crm` — Chậm nhất

**Luồng hiện tại:**
- Với range 7 ngày → gọi `computeCustomerPhatSinh` **7 lần** (mỗi kỳ 1 lần)
- Mỗi lần gọi = **2 truy vấn DB**:
  - `GetLastSnapshotPerCustomerBeforeEndMs` (aggregation)
  - `GetActivitiesInPeriod` (find)
- **Tổng: 14 truy vấn DB** cho 7 ngày

**Nguyên nhân:** Thiết kế vòng lặp theo từng kỳ, mỗi kỳ query riêng.

### 1.2 `GetLastSnapshotPerCustomerBeforeEndMs`

**Query:** `ownerOrganizationId` + `activityAt <= endMs` + `metadata.metricsSnapshot` exists  
**Index hiện có:** `crm_activity_org_at_report` = (ownerOrganizationId, activityAt desc)

- Index hỗ trợ filter org + activityAt
- Filter `metadata.metricsSnapshot exists` có thể gây thêm filter sau index scan
- Với org có 44k+ customers, aggregation phải scan nhiều document

### 1.3 `GetActivitiesInPeriod`

**Query:** `ownerOrganizationId` + `activityAt` trong [startMs, endMs] + `metadata.metricsSnapshot` exists  
**Index:** Cùng `crm_activity_org_at_report` — phù hợp cho range query

### 1.4 `period-end-balance-from-snapshots`

**Luồng hiện tại:**
- Đọc snapshots từ **2000** đến `endMs` (earliestPeriodKey)
- Với daily: 25 năm × 365 ≈ **9.000+ document** nếu có đủ dữ liệu
- Thực tế org mới thường ít hơn, nhưng vẫn có thể vài trăm đến vài nghìn

### 1.5 `FindSnapshotsForTrend`

**Query:** reportKey + ownerOrganizationId + periodKey trong [from, to]  
**Index:** `report_org_period_trend` = (reportKey, ownerOrganizationId, periodKey) — đã tối ưu

---

## 2. Đề xuất phương án

### 2.1 Batch phát sinh cho `trend-from-crm` (Ưu tiên cao, tác động lớn)

**Ý tưởng:** Thay vì gọi `computeCustomerPhatSinh` N lần (N kỳ), gọi **1 lần** lấy toàn bộ activities trong range [fromMs, toMs], rồi tính phát sinh từng kỳ trong memory.

**Thay đổi:**
1. Thêm hàm `GetActivitiesInRange(ctx, ownerOrgID, startMs, endMs)` — 1 query thay vì N
2. Thêm `computePhatSinhForMultiplePeriods(ctx, actSvc, ownerOrgID, periodKeys)`:
   - Gọi `GetLastSnapshotPerCustomerBeforeEndMs` **1 lần** với `endMs = startMs của kỳ đầu - 1` (trạng thái đầu range)
   - Gọi `GetActivitiesInRange` **1 lần** cho toàn range
   - Trong memory: nhóm activities theo kỳ, tính phát sinh từng kỳ (startState → endState mỗi kỳ)

**Lợi ích:** Giảm từ 14 DB calls xuống **2 DB calls** cho 7 ngày.

**Độ phức tạp:** Trung bình — cần refactor logic `computeAllPhatSinh` để hỗ trợ multi-period.

---

### 2.2 Partial index cho `metadata.metricsSnapshot` (Ưu tiên cao, dễ triển khai)

**Ý tưởng:** Tạo partial index chỉ index document có `metadata.metricsSnapshot` — query report chỉ cần document có snapshot, giảm kích thước index và tăng tốc.

**Thay đổi:** Thêm index mới (có thể qua migration hoặc model tag nếu framework hỗ trợ):

```javascript
// MongoDB
db.crm_activity_history.createIndex(
  { ownerOrganizationId: 1, activityAt: -1 },
  { 
    name: "crm_activity_org_at_report_with_snapshot",
    partialFilterExpression: { "metadata.metricsSnapshot": { $exists: true } }
  }
)
```

**Lưu ý:** Cần kiểm tra xem `CreateIndexes` từ model có hỗ trợ partial index không. Nếu không, tạo script migration riêng.

**Lợi ích:** Index nhỏ hơn, query nhanh hơn vì chỉ scan document có snapshot.

---

### 2.3 Giới hạn `earliestPeriodKey` theo dữ liệu thực tế (Ưu tiên trung bình)

**Ý tưởng:** Thay vì luôn bắt đầu từ 2000, lấy **periodKey nhỏ nhất** có snapshot của org đó.

**Thay đổi:**
- Trong `GetPeriodEndBalanceFromSnapshots`: thay vì `earliestPeriodKey` cố định, query:
  ```go
  // Lấy snapshot có periodKey nhỏ nhất của org (1 query)
  firstSnapshot := FindFirstSnapshotByOrg(ctx, reportKey, ownerOrgID)
  fromStr := firstSnapshot.PeriodKey  // hoặc earliestPeriodKey nếu không có
  ```
- Hoặc cache "first period có data" per org (TTL 1h) để tránh query mỗi request.

**Lợi ích:** Giảm số document đọc từ vài nghìn xuống vài chục đến vài trăm (tùy org).

---

### 2.4 Cache kết quả report (Ưu tiên trung bình, tác động lớn)

**Ý tưởng:** Cache response của các API report theo key `(ownerOrgID, reportKey, from, to)` với TTL (vd: 5–15 phút).

**Chiến lược invalidate:**
- Khi `MarkDirty` được gọi (data thay đổi) → xóa cache cho org + reportKey + period bị dirty
- Hoặc TTL ngắn (5 phút) cho dữ liệu gần real-time

**Lưu ý:**
- Cần Redis hoặc in-memory cache
- Cần cơ chế MarkDirty gọi cache invalidation

**Lợi ích:** Request lặp lại (cùng params) trả về ngay từ cache.

---

### 2.5 Projection — Chỉ lấy field cần thiết (Ưu tiên thấp)

**Ý tưởng:** `GetActivitiesInPeriod` và `GetLastSnapshotPerCustomerBeforeEndMs` chỉ cần `unifiedId`, `activityAt`, `metadata.metricsSnapshot`. Có thể dùng projection để giảm data transfer.

**Thay đổi:**
- Find: `opts.SetProjection(bson.M{"unifiedId": 1, "activityAt": 1, "metadata.metricsSnapshot": 1})`
- Aggregation: thêm `$project` stage để chỉ giữ field cần thiết

**Lợi ích:** Giảm bandwidth, đặc biệt khi document có nhiều field lớn.

---

### 2.6 Ưu tiên dùng API từ snapshots (Khuyến nghị sử dụng)

**Ý tưởng:** API `trend-from-snapshots` và `period-end-balance-from-snapshots` **nhanh hơn** vì đọc từ `report_snapshots` (đã pre-compute). API từ CRM dùng khi cần độ chính xác real-time hoặc snapshot chưa có.

**Khuyến nghị:**
- Frontend/dashboard: **ưu tiên** `trend-from-snapshots`, `period-end-balance-from-snapshots`
- Chỉ dùng `trend-from-crm` khi: range mới chưa có snapshot, hoặc cần verify
- Đảm bảo job compute snapshot chạy đủ thường xuyên để giảm nhu cầu gọi API từ CRM

---

### 2.7 Giới hạn range tối đa (Bảo vệ)

**Ý tưởng:** Giới hạn số kỳ tối đa trong 1 request (vd: 31 ngày, 12 tháng, 5 năm) để tránh request quá nặng.

**Thay đổi:** Validate trong handler hoặc DTO: nếu `to - from` vượt ngưỡng → trả lỗi 400 với message rõ ràng.

---

## 3. Thứ tự triển khai đề xuất

| Bước | Phương án                         | Ưu tiên | Effort | Tác động |
|------|-----------------------------------|---------|--------|----------|
| 1    | Batch phát sinh trend-from-crm     | Cao     | TB     | Rất lớn  |
| 2    | Partial index metricsSnapshot     | Cao     | Thấp   | Lớn      |
| 3    | Projection cho activity query     | TB      | Thấp   | TB       |
| 4    | earliestPeriodKey động            | TB      | TB     | TB       |
| 5    | Cache report response             | TB      | Cao    | Lớn      |
| 6    | Giới hạn range                    | Thấp   | Thấp   | Bảo vệ   |

---

## 4. Đo lường hiệu suất

Trước và sau khi áp dụng, nên đo:
- **Latency p50, p95, p99** của từng API
- **Số truy vấn DB** mỗi request (qua log hoặc APM)
- **Thời gian aggregation** (MongoDB profiler)

Có thể dùng `explain()` trên aggregation pipeline để kiểm tra index usage.

---

## 5. Tài liệu tham khảo

- `api/internal/api/report/service/service.report.customers.trend.go` — GetCustomersTrendFromCrm
- `api/internal/api/report/service/service.report.customer.phatsinh.go` — computeAllPhatSinh
- `api/internal/api/crm/service/service.crm.activity.go` — GetLastSnapshotPerCustomerBeforeEndMs, GetActivitiesInPeriod
- `api/internal/api/report/service/service.report.customer.snapshot_balance.go` — GetPeriodEndBalanceFromSnapshots
