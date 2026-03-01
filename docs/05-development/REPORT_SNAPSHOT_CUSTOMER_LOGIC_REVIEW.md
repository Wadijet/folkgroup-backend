# Báo cáo kiểm tra logic tính toán Report Snapshot Customer

**Ngày kiểm tra:** 2025-02-28

## 1. Tổng quan luồng dữ liệu

```
crm_activity_history (metadata.metricsSnapshot)
    → GetLastSnapshotPerCustomerBeforeEndMs(endMs)
    → computeCustomerMetricsFromActivityHistory()
    → upsertSnapshot() → report_snapshots
    → GetSnapshotForCustomersDashboard() → API response
```

---

## 2. Các vấn đề phát hiện

### 2.1. [BUG] `getCustomerStateMapForPeriod` — sai endMs cho chu kỳ weekly — **ĐÃ SỬA**

**Vị trí:** `service.report.customers.trend.go` — `getCustomerStateMapForPeriod`

**Mô tả:** Khi `periodKey` có độ dài 10 (format `YYYY-MM-DD`), hàm luôn coi là **ngày** và dùng `endSec = cuối ngày đó`. Nhưng với report **weekly**, `periodKey` là **thứ Hai** của tuần (vd: `2025-02-24`), nên cần `endMs = cuối Chủ nhật` (vd: `2025-03-02 23:59:59.999`).

**Đã sửa:** Thêm tham số `periodType` vào `getCustomerStateMapForPeriod`, `GetTransitionMatrix`, `GetGroupChanges`. API thêm query `periodType` (day|week|month|year). Khi `periodType == "week"` và periodKey YYYY-MM-DD, dùng `endSec = cuối Chủ nhật`.

---

### 2.2. [MINOR] `reactivationValue` bị cắt phần thập phân — **ĐÃ SỬA**

**Vị trí:** `service.report.customer.go` dòng 312

**Đã sửa:** Dùng `int64(math.Round(reactivationValue))` thay vì `int64(reactivationValue)` để làm tròn trước khi cast.

---

### 2.3. [DESIGN] CeoGroup distribution — một khách có thể thuộc nhiều nhóm

**Vị trí:** `service.report.customer.go` dòng 201–225

**Mô tả:** Mỗi khách có thể được đếm vào nhiều CeoGroup:
- `vip_active` (vip + active)
- `vip_inactive` (vip + inactive/dead)
- `rising` (momentumStage == "rising")
- `new` (journeyStage == "first" hoặc valueTier == "new")
- `one_time` (loyaltyStage == "one_time")
- `dead` (lifecycleStage == "dead")

→ Tổng `ceoDist` có thể **lớn hơn** `totalCustomers`.

**Đánh giá:** Có vẻ là thiết kế cố ý (mỗi widget đếm riêng). CeoGroupLTV dùng `computeCeoGroupForLTV` (mutually exclusive) nên tổng LTV vẫn đúng. Nên ghi rõ trong comment/doc.

---

### 2.4. [EDGE] Khách không có activity với metricsSnapshot — bị loại khỏi report

**Vị trí:** `GetLastSnapshotPerCustomerBeforeEndMs` — chỉ lấy khách có `metadata.metricsSnapshot` trong `crm_activity_history`.

**Mô tả:** Khách mới chỉ có trong `crm_customers` nhưng chưa có activity (order_placed, customer_created, v.v.) sẽ **không** xuất hiện trong report snapshot.

**Đánh giá:** Hợp lý nếu thiết kế là “chỉ đếm khách đã có activity”. Cần đảm bảo mọi khách quan trọng đều được ghi activity (vd: `customer_created`) khi tạo.

---

### 2.5. [EDGE] `newInPeriod` — gần đúng, không phải chính xác tuyệt đối

**Vị trí:** `service.report.customer.go` dòng 235–244

**Logic hiện tại:** `orderCount == 1` và `lastOrderAt` trong `[startSec, endSec]`.

**Mô tả:** Định nghĩa “khách mới trong kỳ” = đơn đầu nằm trong kỳ. Với snapshot cuối kỳ:
- `orderCount == 1` → khách có đúng 1 đơn tại cuối kỳ
- `lastOrderAt` trong period → đơn đó nằm trong kỳ

→ Logic đúng cho trường hợp thông thường. Trường hợp biên: khách có 2 đơn trong kỳ (đơn 1 và 2) thì tại cuối kỳ `orderCount == 2`, không bị đếm là new — đúng.

---

## 3. Các phần logic đúng

| Thành phần | Trạng thái |
|------------|------------|
| `GetLastSnapshotPerCustomerBeforeEndMs` | Đúng — lấy snapshot cuối trước endMs |
| `ComputeCustomerReport` — startSec/endSec theo periodType | Đúng |
| Xử lý `lastOrderAt` ms vs seconds | Đúng — kiểm tra `> 1e12` rồi chia 1000 |
| `computeCeoGroupForLTV` — mutually exclusive | Đúng |
| Mapping `_unspecified` → `unspecified` cho channel/loyalty/momentum | Đúng |
| `GetStrFromNestedMetrics` — đọc từ raw/layer1/layer2 | Đúng |
| `layer3.DeriveFromNested` — dùng endMs cho daysSinceLast | Đúng |
| `paramsToTrendRange` — reportKey, fromStr, toStr | Đúng |
| `metricAt`, `metricAtDist`, `metricAtDistFloat` | Đúng |

---

### 2.6. [FIXED] Report tự gán visitor/new/never_purchased khi metricsSnapshot rỗng — **ĐÃ SỬA**

**Vị trí:** `service.report.customer.go`, `service.report.customers.trend.go`, `handler.report.dashboard.go`

**Mô tả:** Report phải đọc từ `metricsSnapshot` (currentMetrics của customer), không được tự gán ý nghĩa nghiệp vụ khi trường rỗng. Trước đây khi `journeyStage`/`valueTier`/`lifecycleStage` rỗng, code gán `"visitor"`/`"new"`/`"never_purchased"` — sai.

**Đã sửa:** Khi rỗng dùng `"_unspecified"` thay vì tự gán. Thêm `unspecified` vào ValueDistribution, JourneyDistribution, LifecycleDistribution và LTV tương ứng.

---

### 2.7. [BUG] computeAllPhatSinh — đếm mỗi lần chuyển trạng thái thay vì chuyển đổi ròng — **ĐÃ SỬA**

**Vị trí:** `service.report.customer.phatsinh.go` — `computeAllPhatSinh`

**Mô tả:** Logic cũ duyệt từng activity trong kỳ và đếm mỗi lần classification thay đổi. Khách có nhiều activity (vd: nhiều cuộc hội thoại FB) bị đếm nhiều lần → số phát sinh nhảy vọt (vd: engaged.sourceType.ads in: 4148).

**Đã sửa:** Chỉ đếm chuyển đổi ròng cuối cùng mỗi khách: so sánh trạng thái đầu kỳ (GetLastSnapshotPerCustomerBeforeEndMs tại startMs) với trạng thái cuối kỳ (activity cuối trong kỳ). Mỗi khách chỉ đếm 1 lần.

---

## 4. Khuyến nghị

1. **Ưu tiên cao:** Sửa bug `getCustomerStateMapForPeriod` cho chu kỳ weekly.
2. **Ưu tiên thấp:** Làm tròn `reactivationValue` trước khi cast sang int64.
3. **Documentation:** Ghi rõ CeoGroup distribution có thể overlap; CeoGroupLTV thì mutually exclusive.
