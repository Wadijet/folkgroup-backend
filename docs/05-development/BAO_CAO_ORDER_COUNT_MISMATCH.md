# Báo cáo: orderCount = 0 trong khi DB có đơn

## Hiện tượng

Khách có `journeyStage` = first/repeat/vip/inactive (đòi hỏi orderCount > 0) nhưng `orderCount` trong crm_customers = 0, trong khi query trực tiếp `pc_pos_orders` với ids của khách **tìm thấy đơn**.

## Nguyên nhân gốc

### 1. Khác biệt logic giữa các luồng cập nhật metrics

| Luồng | Dùng expandCustomerIdsForAggregation? | Nguồn ids |
|-------|--------------------------------------|-----------|
| **Recalculate** | ✅ Có | buildCustomerIdsForRecalculate + expand |
| **RefreshMetrics** | ❌ Không | `[sourceIds.Pos, sourceIds.Fb, unifiedId]` |
| **MergeFromPosCustomer** | ❌ Không | `[customerId, sourceIds.Fb, unifiedId]` |
| **MergeFromFbCustomer** | ❌ Không | `[sourceIds.Pos, fbCustomerId, unifiedId]` |

### 2. Vai trò của expandCustomerIdsForAggregation

- Khi `sourceIds.Pos` rỗng: tìm POS customer qua SĐT → thêm posId vào ids
- Khi `sourceIds.Fb` rỗng: tìm FB customer qua SĐT → thêm fbId vào ids
- Giúp match đơn/hội thoại khi khách **chưa merge** hoặc link qua SĐT chưa có trong sourceIds

### 3. Kịch bản dẫn đến orderCount = 0

1. **Khách tạo từ FB trước** (MergeFromFbCustomer): `sourceIds.Pos = ""`
2. **Đơn đến sau** với `customerId` = POS customer id (từ `posData.customer.id`)
3. **ResolveUnifiedId** tìm được crm_customer (qua merge hoặc link)
4. **RefreshMetrics** chạy với `ids = [sourceIds.Pos, sourceIds.Fb, unifiedId]`
5. Nếu tại thời điểm đó `sourceIds.Pos` vẫn rỗng (merge chưa cập nhật, hoặc race) → ids thiếu POS id → aggregate trả 0
6. Hoặc: đơn match qua **billPhoneNumber** nhưng profile không có SĐT → không match qua ids → aggregate trả 0

### 4. Migration unsetRawFields

`unsetRawFields` **không** unset `orderCount`, `totalSpent`, `lastOrderAt` — các field này vẫn được `$set` khi Merge/Refresh. Vấn đề là giá trị `$set` = 0 khi aggregate trả 0.

## Giải pháp đề xuất

### Sửa RefreshMetrics và Merge để dùng expandCustomerIdsForAggregation

Đồng nhất logic với Recalculate:

1. **RefreshMetrics** (service.crm.metrics.go):
   - Thay `ids := []string{...}` bằng `ids := buildCustomerIdsForRecalculate(&customer)` rồi `ids = s.expandCustomerIdsForAggregation(...)`

2. **MergeFromPosCustomer** (service.crm.merge.go):
   - Sau khi có ids, gọi `expandCustomerIdsForAggregation` trước khi aggregate

3. **MergeFromFbCustomer** (service.crm.merge.go):
   - Tương tự

### Chạy Recalculate để sửa dữ liệu hiện tại

- Bật `CRM_RECALC_MISMATCH_ON_START=1` khi khởi động server
- Hoặc gọi endpoint recalc mismatch
- Recalculate đã dùng expand nên sẽ cập nhật đúng orderCount

## Tóm tắt

| Nguyên nhân | Mô tả |
|-------------|-------|
| RefreshMetrics không expand ids | Thiếu POS/FB id khi chưa merge → aggregate trả 0 |
| Merge không expand ids | Cùng vấn đề khi merge từ nguồn đơn |
| Race / thứ tự event | Order ingest có thể chạy khi sourceIds chưa đầy đủ |

**Fix**: Thêm `expandCustomerIdsForAggregation` vào RefreshMetrics và Merge để đồng nhất với Recalculate.
