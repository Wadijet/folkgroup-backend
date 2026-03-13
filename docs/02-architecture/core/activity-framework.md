# Activity Framework — Implementation Backend

**Spec chuẩn:** [docs-shared/ai-context/folkform/design/activity-framework.md](../../docs-shared/ai-context/folkform/design/activity-framework.md)

Tài liệu này mô tả implementation backend: shared package, ActivityBase, collections, migration.

---

## 0. Trạng Thái Triển Khai (2025-03-13)

| Domain | Trạng thái | Ghi chú |
|--------|------------|---------|
| **CRM** | ✅ Đã triển khai | Model embed ActivityBase; LogActivity ghi actor, display, snapshot, context |
| **Ads** | ✅ Đã triển khai | Model embed ActivityBase; RecordActivityForEntity qua MetaAdsActivityHistoryService |
| **Agent** | ⏳ Chưa migrate | Chỉ sửa UnixMilli, global.MongoDB_ColNames; model vẫn cũ (ID, AgentID, ActivityType, Timestamp, Data) |

**Logic tạo activity:** CRM qua `LogActivity`; Ads qua `RecordActivityForEntity`; Agent qua `LogActivity` (model cũ).

---

## 1. Nguyên Tắc

- **ActivityBase** — khung cấu trúc chung, domain embed
- **Clean break** — bỏ cấu trúc cũ, không song song
- **Dữ liệu cũ** — Xóa + Backfill (không convert)
- **Không tiền tố** — field giữ tên spec (`activityType`, `domain`, ...)
- **Customer = Ads** — quy trình triển khai tương đương (cùng độ phức tạp)

---

## 2. Shared Package

**Vị trí:** `api/internal/common/activity/`

### 2.1 ActivityBase — Khung Chung

```go
type ActivityBase struct {
    ID                   primitive.ObjectID     `bson:"_id,omitempty"`
    ActivityType         string                 `bson:"activityType"`
    Domain               string                 `bson:"domain"`
    OwnerOrganizationID  primitive.ObjectID     `bson:"ownerOrganizationId"`
    UnifiedId            string                 `bson:"unifiedId"`
    Source               string                 `bson:"source"`
    SourceRef            map[string]interface{} `bson:"sourceRef,omitempty"`
    Actor                Actor                  `bson:"actor,omitempty"`
    ActivityAt           int64                  `bson:"activityAt"`
    Display              Display                `bson:"display,omitempty"`
    Snapshot             Snapshot               `bson:"snapshot,omitempty"`
    Context              map[string]interface{} `bson:"context,omitempty"`
    Changes              []ActivityChangeItem   `bson:"changes,omitempty"`
    Metadata             map[string]interface{} `bson:"metadata,omitempty"`
    CreatedAt            int64                  `bson:"createdAt"`
}
```

### 2.2 Types Phụ

- `Actor`, `Display`, `Snapshot`, `ActivityChangeItem` — xem `types.go`
- `ToActor()` — convert legacy actorId + actorName

### 2.3 Helpers

- `TruncateMetadata()` — giới hạn payload (TODO)

---

## 3. Domain Models — Embed Base

### 3.1 CRM (Customer Intelligence)

```go
type CrmActivityHistory struct {
    activity.ActivityBase `bson:",inline"`
}
```

**Bỏ:** `actorId`, `actorName`, `displayLabel`, `displayIcon`, `displaySubtext`, `metadata.profileSnapshot`, `metadata.metricsSnapshot`  
**Thêm:** `actor`, `display`, `snapshot`, `context`  
**Metadata:** `reason`, `status`, `clientIp`, `userAgent` (giữ trong metadata)

### 3.2 Ads (Ads Intelligence)

```go
type AdsActivityHistory struct {
    activity.ActivityBase `bson:",inline"`
    AdAccountId          string `bson:"adAccountId"`
    ObjectType           string `bson:"objectType"`
    ObjectId             string `bson:"objectId"`
}
```

**Bỏ:** `metadata.metricsSnapshot`, `metadata.snapshotChanges`, `metadata.trigger` (chuyển sang base)  
**Thêm:** `actor`, `display`, `snapshot`, `context`, `source`, `sourceRef`  
**Snapshot:** `snapshot.metrics` = currentMetrics; `snapshot.profile` = (nếu có)  
**Changes:** `changes` = snapshotChanges (ActivityChangeItem)  
**Context:** `trigger` (meta_ad_insights | pc_pos_orders | manual)  
**UnifiedId:** `adAccountId:objectType:objectId` hoặc `objectId`

### 3.3 Agent

**Thiết kế mục tiêu** (chưa triển khai):

```go
type AgentActivityLog struct {
    activity.ActivityBase `bson:",inline"`
    AgentID              primitive.ObjectID `bson:"agentId"`
    Severity             string             `bson:"severity,omitempty"`
    Message              string             `bson:"message,omitempty"`
}
```

**UnifiedId:** `agentId.Hex()`, **Domain:** `agent`

**Hiện tại:** Model vẫn dùng cấu trúc cũ (`ID`, `AgentID`, `ActivityType`, `Timestamp`, `Data`, `Message`, `Severity`). Chưa embed ActivityBase.

---

## 4. Collections

| Collection | Domain | Module |
|------------|--------|--------|
| `crm_activity_history` | customer, order, conversation, note | crm |
| `agent_activity_logs` | agent | agent |
| `ads_activity_history` | ads | meta |

Mỗi domain sở hữu collection riêng.

---

## 5. Dữ Liệu Cũ — Xóa + Backfill

**Phương án:** Xóa toàn bộ activity cũ, backfill từ nguồn gốc. Không convert.

Quy trình **tương đương** cho Customer và Ads.

### 5.1 Customer (crm_activity_history)

| Bước | Công việc |
|------|------------|
| 1 | Backup collection (tùy chọn, để audit) |
| 2 | Xóa toàn bộ `crm_activity_history` |
| 3 | Chạy `BackfillActivity(ownerOrgId, limit, types)` |
| 4 | Types: `["order", "conversation", "note"]` — rỗng = tất cả |

**Nguồn backfill:**
- `pc_pos_orders` → order_created, order_completed, order_cancelled
- `fb_conversations` → conversation_started
- `crm_notes` → note_added, note_updated

**Lưu ý:** Activity từ merge (customer_created, customer_updated) sẽ tạo mới khi có event tiếp theo. Có thể cần backfill theo org.

### 5.2 Ads (ads_activity_history)

| Bước | Công việc |
|------|------------|
| 1 | Backup collection (tùy chọn) |
| 2 | Xóa toàn bộ `ads_activity_history` |
| 3 | Duyệt campaign/adset/ad có `currentMetrics` |
| 4 | Gọi `RecordActivityForEntity` với old=nil, current=currentMetrics, trigger="backfill" |

**Nguồn backfill:**
- Campaign/AdSet/Ad có `currentMetrics` trong meta_ads_*
- Script duyệt theo ownerOrganizationId, adAccountId

**Lưu ý:** Mỗi entity chỉ tạo 1 activity (snapshot hiện tại). Không có lịch sử thay đổi trước đó.

### 5.3 Thứ Tự Chạy

1. Deploy code mới (model + service đã dùng ActivityBase)
2. Dừng ghi activity tạm (hoặc chấp nhận mất activity trong lúc xóa)
3. Xóa `crm_activity_history` → Backfill Customer
4. Xóa `ads_activity_history` → Backfill Ads
5. Tạo index mới (snapshot.profile, snapshot.metrics)

### 5.4 Rủi Ro

| Rủi ro | Cách xử lý |
|--------|------------|
| Mất activity không có nguồn | Chấp nhận; hoặc backup trước để audit |
| Backfill lâu | Chạy theo batch (limit per org), có thể chạy nền |
| Report thiếu dữ liệu tạm | Chạy backfill xong mới tính report |

---

## 6. Cập Nhật Code Đọc

Quy trình **tương đương** cho Customer và Ads.

### 6.1 Customer (CRM)

| File | Thay đổi |
|------|----------|
| `service.crm.activity` | LogActivity ghi `actor`, `display`, `snapshot`, `context` |
| `service.crm.fullprofile` | Đọc `display.label`, `actor.id` thay vì displayLabel, actorId |
| `service.report.*` | Query `snapshot.metrics` thay vì `metadata.metricsSnapshot` |
| `service.report.hooks` | `GetPeriodTimestamp` đọc `snapshot`, `activityAt` |
| `service.crm.ingest`, `merge`, `recalculate` | Ghi snapshot vào `snapshot.profile`, `snapshot.metrics` |
| DTO `CrmActivitySummary` | Map từ `display`, `actor` |

### 6.2 Ads

| File | Thay đổi |
|------|----------|
| `service.meta.evaluation` | RecordActivityForEntity ghi `snapshot.metrics`, `changes`, `context.trigger` |
| `service.ads.auto_propose` | GetChsFromYesterday đọc `snapshot.metrics` thay vì `metadata.metricsSnapshot` |
| `MetaAdsActivityHistoryService` | Dùng qua service, không InsertOne trực tiếp |
| `model.meta.ads_activity_history` | Embed ActivityBase, bỏ Metadata.metricsSnapshot/snapshotChanges |

---

## 7. Sửa Lỗi Đã Rà Soát

| # | Task | Trạng thái |
|---|------|------------|
| 1 | Agent: `Unix()` → `UnixMilli()` | ✅ |
| 2 | Agent: `global.MongoDB_ColNames.AgentActivityLogs` thay hardcode | ✅ |
| 3 | Ads: `RecordActivityForEntity` qua `MetaAdsActivityHistoryService` | ✅ |
| 4 | Report hooks: chuẩn hóa timestamp ms | ✅ |

---

## 8. Thứ Tự Triển Khai

| Phase | Customer (CRM) | Ads (Meta) |
|-------|----------------|------------|
| **1** | — | Thêm `ActivityBase` vào `common/activity` ✅ |
| **2** | Model embed ActivityBase, bỏ field cũ ✅ | Model embed ActivityBase, bỏ Metadata.* ✅ |
| **3** | Cập nhật CrmActivityService (LogActivity, ...) ✅ | Cập nhật RecordActivityForEntity qua service ✅ |
| **4** | Cập nhật report, fullprofile, ingest, DTO ✅ | Cập nhật GetChsFromYesterday, auto_propose ✅ |
| **5** | Xóa + Backfill `crm_activity_history` | Xóa + Backfill `ads_activity_history` |
| **6** | Index mới (snapshot.profile, snapshot.metrics) | Index mới (snapshot.metrics) |

**Dữ liệu cũ:** Xóa + Backfill (phương án 2). Không convert.

**Agent:** Đã sửa UnixMilli, global.MongoDB_ColNames. Model chưa migrate sang ActivityBase.
