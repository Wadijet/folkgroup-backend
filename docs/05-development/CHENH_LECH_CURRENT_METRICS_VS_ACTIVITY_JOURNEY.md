# Chẩn đoán: Chênh lệch currentMetrics vs metricsSnapshot trong hành trình khách hàng

> Ngày tạo: 2026-03-11
> Script: `go run scripts/diagnose_journey_metrics_mismatch.go [orgId] [--limit N]`

## 1. Tổng quan

**Hành trình khách hàng** hiển thị lịch sử hoạt động từ `crm_activity_history`. Mỗi activity có thể có `metadata.metricsSnapshot` (orderCount, totalSpent, conversationCount, journeyStage...). **Current metrics** lấy từ `crm_customers.currentMetrics`.

Chênh lệch xảy ra khi số liệu trong activity (metricsSnapshot) không khớp với currentMetrics hiện tại của khách.

## 2. Kết quả chẩn đoán (mẫu org 69a655f0088600c32e62f955)

| Chỉ số | Giá trị |
|--------|---------|
| Số khách có activity với metricsSnapshot | 43,204 |
| Khách có current > snapshot (bình thường — có order/conv mới) | 17,067 |
| **Khách chênh lệch bất thường** | **105** |
| — journey_mismatch (snapshot=visitor, current=engaged) | 105 |

### 2.1 Đặc điểm chênh lệch

- **Activity type:** `customer_updated` (từ recalculate)
- **Pattern:** metricsSnapshot có `journeyStage=visitor`, currentMetrics có `journeyStage=engaged`
- **OrderCount, TotalSpent, ConvCount:** CẢ HAI đều = 0 trong snapshot VÀ current
- **DB thực tế:** `convs match customer = 1 hoặc 2` — trong `fb_conversations` CÓ conversation cho khách

→ **Snapshot trong activity bị thiếu conversation** (convCount=0, visitor) trong khi DB có conv. Current metrics hiện tại có engaged — có thể từ cập nhật sau đó hoặc nguồn khác.

## 3. Nguyên nhân khả dĩ

### 3.1 GetMetricsForSnapshotAt vs Recalculate — logic khác nhau

| Bước | Recalculate | GetMetricsForSnapshotAt (dùng trong logRecalculateActivity) |
|------|-------------|------------------------------------------------------------|
| Mở rộng ids | `expandCustomerIdsForAggregation` — thêm FB/POS id tìm qua SĐT | **Không** — chỉ dùng `[Pos, Fb, UnifiedId]` |
| ConversationIds | getConversationIdsFromPosCustomers + getConversationIdsFromFbMatch(**expanded ids**) | getConversationIdsFromPosCustomers + getConversationIdsFromFbMatch(**base ids**) |
| **hasConversation fallback** | `convCount > 0 \|\| checkHasConversation(...)` | **Không** — chỉ dùng `convCount > 0` từ aggregate |
| Kết quả | Có thể = engaged nhờ checkHasConversation dù aggregate=0 | Luôn = visitor khi aggregate trả convCount=0 |

→ **Nguyên nhân chính:** Recalculate dùng `hasConv := convMetrics.ConversationCount > 0 || s.checkHasConversation(...)`. Khi aggregate không đếm được conv (filter/path BSON khác) nhưng `checkHasConversation` tìm thấy (query rộng hơn) → hasConv=true → engaged. GetMetricsForSnapshotAt **không** gọi checkHasConversation → HasConversation = (convCount > 0) = false → visitor.

### 3.2 Thứ tự thực thi

1. Recalculate: aggregate với ids mở rộng → convCount=1 → cập nhật currentMetrics (engaged)
2. logRecalculateActivity: FindOne customer (đã update), gọi GetMetricsForSnapshotAt
3. GetMetricsForSnapshotAt: aggregate với ids không mở rộng → convCount=0 → snapshot = visitor
4. Activity được ghi với snapshot sai

### 3.3 Race / activity cũ

- Activity được tạo trước khi áp dụng fix getConversationIdsFromFbMatch (2026-03-10)
- Recalc chạy batch: nhiều job song song có thể gây race

## 4. Đề xuất xử lý

### 4.1 Fix code: Đồng bộ logic GetMetricsForSnapshotAt với Recalculate

**Option A — Khuyến nghị:** Thêm `expandCustomerIdsForAggregation` vào GetMetricsForSnapshotAt khi dùng cho snapshot "hiện tại" (activityAt gần now). Hoặc tách hàm: `GetMetricsForSnapshotAtWithExpandedIds` dùng cho logRecalculateActivity.

**Option B — Đơn giản, khuyến nghị triển khai:** Truyền metrics đã tính sẵn từ Recalculate vào logRecalculateActivity thay vì gọi GetMetricsForSnapshotAt — đảm bảo snapshot = chính xác metrics vừa cập nhật.

```go
// Trong RecalculateCustomerFromAllSources:
// cm đã có từ BuildCurrentMetricsFromOrderAndConv (dòng 106)
s.logRecalculateActivity(ctx, unifiedId, ownerOrgID, now, cm)
```

### 4.2 Sửa dữ liệu hiện tại

- Chạy recalc lại cho 105 khách mismatch để tạo activity `customer_updated` mới với snapshot đúng
- Hoặc script cập nhật metadata.metricsSnapshot của activity cũ từ currentMetrics (cẩn thận — có thể làm sai timeline nếu có order/conv mới sau activity)

### 4.3 Kiểm tra tiếp

- So sánh buildConversationFilterForCustomerIds và filter trong getConversationIdsFromFbMatch với filter script chẩn đoán dùng
- Xác nhận path BSON trong fb_conversations (panCakeData.page_customer.id vs customer_id...) khớp giữa aggregate và script

## 5. Chạy chẩn đoán

```bash
# Mặc định org, limit 20 mẫu
go run scripts/diagnose_journey_metrics_mismatch.go

# Chỉ định org và số mẫu chi tiết
go run scripts/diagnose_journey_metrics_mismatch.go 69a655f0088600c32e62f955 --limit 10

# Chỉ thống kê, không in chi tiết
go run scripts/diagnose_journey_metrics_mismatch.go 69a655f0088600c32e62f955 --limit 0
```

## 6. Tham chiếu

- `docs/05-development/BAO_CAO_KIEM_TRA_MISMATCH_20260310.md` — Fix visitor/engaged, getConversationIdsFromFbMatch
- `api/internal/api/crm/service/service.crm.recalculate.go` — logRecalculateActivity, expandCustomerIdsForAggregation
- `api/internal/api/crm/service/service.crm.metrics.go` — GetMetricsForSnapshotAt
