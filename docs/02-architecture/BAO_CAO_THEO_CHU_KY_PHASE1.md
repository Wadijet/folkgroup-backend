# Báo cáo theo chu kỳ — Phase 1 (một file, đơn giản)

Tài liệu **tổng hợp** đủ để triển khai Phase 1: định nghĩa trong DB, một report = một collection nguồn, snapshot + dirty + hook + API + worker. Làm đơn giản trước, chạy được rồi mở rộng sau.

**Datetime:** Thống nhất dùng **Unix timestamp (int64)**, đơn vị **giây**. Mọi trường thời gian trong `report_definitions`, `report_snapshots`, `report_dirty_periods` (createdAt, updatedAt, computedAt, markedAt, processedAt) đều lưu Unix seconds. Trường `timeField` trong dữ liệu nguồn (ví dụ insertedAt) giữ theo đúng collection — nếu nguồn lưu ms thì engine đổi sang seconds khi so sánh với period hoặc quy ước nguồn cũng dùng seconds.

---

## 1. Tóm tắt

- **Báo cáo** tính theo chu kỳ (ngày) và **lưu sẵn** (snapshot). Client gọi API trend → đọc snapshot, không tính realtime.
- **Định nghĩa** trong DB: collection `report_definitions`. Mỗi document = một báo cáo (key, name, periodType, sourceCollection, timeField, dimensions, metrics[]).
- **Khi dữ liệu nguồn đổi** → hook đánh dấu period “dirty” → worker hoặc API recompute tính lại và ghi snapshot.
- **Phase 1:** Một report = **một** collection nguồn; tất cả metrics lấy từ cùng sourceCollection. (Sau có thể mở rộng mỗi metric có sourceCollection riêng.)

---

## 2. Luồng

```
pc_pos_orders (Insert/Update) → Hook → MarkDirty(reportKey, periodKey, org)
                                        ↓
report_definitions (load) → Engine (1 aggregation) → report_snapshots (upsert)
                                        ↑
Worker/Cron đọc report_dirty_periods → Recompute từng (reportKey, periodKey, org)
```

---

## 3. Schema (Phase 1 đơn giản)

### 3.1. report_definitions

| Field | Kiểu | Mô tả |
|-------|------|--------|
| `key` | string | Unique (ví dụ `order_daily`). reportKey. |
| `name` | string | Tên báo cáo (tách riêng với chu kỳ). |
| `periodType` | string | `day` \| `week` \| `month`. Chu kỳ. |
| `periodLabel` | string | (Optional) Tên hiển thị chu kỳ, ví dụ "Theo ngày". |
| `sourceCollection` | string | Collection nguồn (Phase 1: một report một collection). |
| `timeField` | string | Field thời gian trong document nguồn — giá trị **Unix (int64)**, đơn vị **giây**. Ví dụ `insertedAt`. |
| `dimensions` | []string | Group by, ví dụ `["ownerOrganizationId"]`. |
| `metrics` | []object | Mỗi phần tử: outputKey, aggType, fieldPath, countIfExpr (nếu countIf). Phase 1 không dùng sourceCollection/timeField riêng per metric. |
| `metadata` | object | (Optional) description, category, tags. |
| `isActive` | bool | Mặc định true. |
| `createdAt`, `updatedAt` | int64 | Unix seconds. |

**Mỗi metric (Phase 1):** outputKey, aggType (sum|avg|count|countIf|min|max), fieldPath, countIfExpr (nếu aggType=countIf). Optional: metadata (label, unit).

**Index:** unique trên `key`.

### 3.2. report_snapshots

| Field | Kiểu | Mô tả |
|-------|------|--------|
| `reportKey` | string | |
| `periodKey` | string | Ví dụ "2025-02-01". |
| `periodType` | string | day \| week \| month. |
| `ownerOrganizationId` | ObjectID | |
| `dimensions` | object | (Optional) shopId, ... |
| `metrics` | object | Map outputKey → value, ví dụ { "revenue": 123, "orderCount": 5 }. |
| `computedAt` | int64 | Unix seconds. |
| `createdAt`, `updatedAt` | int64 | Unix seconds. |

**Index:** unique (reportKey, periodKey, ownerOrganizationId). Query trend: (reportKey, ownerOrganizationId, periodKey).

### 3.3. report_dirty_periods

| Field | Kiểu | Mô tả |
|-------|------|--------|
| `reportKey` | string | |
| `periodKey` | string | |
| `ownerOrganizationId` | ObjectID | |
| `markedAt` | int64 | Unix seconds. |
| `processedAt` | *int64 | Unix seconds; null = chưa xử lý. |

**Index:** (reportKey, processedAt) hoặc (processedAt) để worker lấy chưa xử lý.

---

## 4. Engine (Phase 1: một aggregation)

**Input:** reportKey, periodKey, ownerOrganizationId.

1. Load document từ `report_definitions` theo key = reportKey, isActive = true.
2. Tính time range của period: periodType = day, periodKey = "2025-02-01" → startOfDay, endOfDay (Unix seconds, timezone cố định ví dụ VN).
3. Filter: ownerOrganizationId = input, timeField ∈ [startOfDay, endOfDay] (so sánh cùng đơn vị Unix seconds).
4. Build aggregation: $match (filter) → $group (_id = null hoặc dimensions, với mỗi metric: sum/avg/count/countIf theo fieldPath/countIfExpr, output key = outputKey).
5. Chạy aggregation trên sourceCollection (lấy collection từ global theo tên).
6. Upsert vào report_snapshots: reportKey, periodKey, periodType, ownerOrganizationId, metrics (map từ kết quả), computedAt = now (Unix seconds), updatedAt = now (Unix seconds).

---

## 5. Hook

- **Vị trí:** Trong PcPosOrderService (hoặc service collection nguồn), sau InsertOne/UpdateOne thành công.
- **Logic:** Từ document lấy posCreatedAt (hoặc insertedAt, createdAt), ownerOrganizationId. Suy ra periodKey (ngày, timezone cố định). Query `report_definitions` có sourceCollection = "pc_pos_orders" → danh sách key. Với mỗi key gọi MarkDirty(ctx, reportKey, periodKey, ownerOrganizationId).
- **MarkDirty:** Insert vào report_dirty_periods (reportKey, periodKey, ownerOrganizationId, markedAt = now Unix seconds, processedAt = null). Có thể upsert để tránh trùng.
- **Không** gọi engine trong request.

---

## 6. API

- **GET** `/api/reports/:reportKey/trend`  
  Query: from, to (date), ownerOrganizationId (hoặc từ context). Query report_snapshots: reportKey, ownerOrganizationId, periodKey ∈ [from, to], sort periodKey. Trả về danh sách snapshot. Format chuẩn: code, message, data (array), status.

- **POST** `/api/reports/:reportKey/recompute`  
  Body: from, to, ownerOrganizationId. Với mỗi ngày trong [from, to] gọi engine (reportKey, periodKey, ownerOrganizationId) → upsert snapshot. Trả về số period đã xử lý. Phase 1 có thể giới hạn range (ví dụ tối đa 31 ngày).

---

## 7. Worker / Cron

- Định kỳ (ví dụ mỗi 5 phút): đọc report_dirty_periods có processedAt = null (giới hạn N bản ghi).
- Với mỗi bản ghi: gọi engine (reportKey, periodKey, ownerOrganizationId) → sau khi ghi snapshot set processedAt = now Unix seconds (hoặc xóa bản ghi).

---

## 8. Gói công việc Phase 1 (thứ tự)

| Bước | Nội dung |
|------|----------|
| 1 | Model ReportDefinition, ReportSnapshot, ReportDirtyPeriod. Đăng ký 3 collection + index (key unique, snapshot unique, dirty index). |
| 2 | Service report: LoadDefinition(reportKey), MarkDirty(reportKey, periodKey, ownerOrganizationId). Helper: GetReportKeysByCollection(collectionName) — query report_definitions có sourceCollection = collectionName. |
| 3 | Package report engine: Compute(ctx, reportKey, periodKey, ownerOrganizationId) — load definition, build aggregation, chạy, upsert snapshot. Phase 1 chỉ một sourceCollection. |
| 4 | Hook: Trong PcPosOrderService sau InsertOne/UpdateOne → lấy posCreatedAt (fallback insertedAt), ownerOrganizationId → periodKey → GetReportKeysByCollection("pc_pos_orders") → MarkDirty từng key. |
| 5 | Handler + route: GET trend, POST recompute. RBAC: Report.Read, Report.Recompute. |
| 6 | Worker/cron (optional Phase 1): đọc dirty → Compute → đánh dấu processed. |
| 7 | Seed: Insert document order_daily vào report_definitions (xem mục 9). |

---

## 9. Seed document order_daily (Phase 1)

Chèn (upsert) một document vào `report_definitions`:

```json
{
  "key": "order_daily",
  "name": "Báo cáo đơn hàng chu kỳ ngày",
  "periodType": "day",
  "periodLabel": "Theo ngày",
  "sourceCollection": "pc_pos_orders",
  "timeField": "posCreatedAt",
  "timeFieldUnit": "millisecond",
  "dimensions": ["ownerOrganizationId"],
  "metrics": [
    { "outputKey": "orderCount", "aggType": "count", "fieldPath": "_id" },
    { "outputKey": "totalAmount", "aggType": "sum", "fieldPath": "posData.total_price_after_sub_discount" }
  ],
  "metadata": {
    "description": "Số lượng đơn và tổng số tiền theo ngày, phân theo nguồn (posData.tags). Nhiều tag/đơn thì chia đều.",
    "tagDimension": {
      "fieldPath": "posData.tags",
      "nameField": "name",
      "splitMode": "equal"
    },
    "totalAmountField": "posData.total_price_after_sub_discount",
    "knownTags": ["Nguồn.Store-Sài Gòn", "Nguồn.Store-Hà Nội", "Nguồn.Web-Zalo", "Nguồn.Web-Shopify", "Nguồn.Bán lại", "Nguồn.Bán sỉ", "Nguồn.Bán mới"]
  },
  "isActive": true,
  "createdAt": <Unix seconds>,
  "updatedAt": <Unix seconds>
}
```

**Lưu ý:** Hook MarkDirty dùng `posCreatedAt` từ document để suy periodKey.

---

## 10. Lưu ý nhanh

- **Datetime:** Thống nhất Unix (int64), đơn vị **giây** trong toàn bộ report_definitions, report_snapshots, report_dirty_periods. Dữ liệu nguồn (timeField) nếu đang lưu ms thì convert sang seconds khi build filter hoặc quy ước nguồn chuyển sang seconds.
- **Timezone:** Cắt ngày theo timezone cố định (ví dụ Asia/Ho_Chi_Minh). Config hoặc constant.
- **Đa tổ chức:** Luôn filter và snapshot theo ownerOrganizationId. API lấy org từ context.
- **Response:** Chuẩn dự án: code, message, data, status (success/error). Message Tiếng Việt.
- **Permission:** Report.Read (xem trend), Report.Recompute (chạy lại). Gắn middleware cho route `/api/reports/*`.

---

Sau khi Phase 1 chạy ổn có thể mở rộng: mỗi metric có sourceCollection/timeField riêng, metadata đầy đủ, invalidationCollections, CRUD API definition.
