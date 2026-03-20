# Phương Án Triển Khai Identity + Links Model

**Ngày:** 2026-03-17  
**Tham chiếu:** [identity-links-model](../../docs-shared/architecture/data-contract/identity-links-model.md), [uid-field-naming](../../docs-shared/architecture/data-contract/uid-field-naming.md), [COLLECTIONS_CAN_UID_SYSTEM](../../docs-shared/ai-context/folkform/sample-data/COLLECTIONS_CAN_UID_SYSTEM.md), [LINK_SYSTEM_SPEC](../../docs-shared/ai-context/folkform/sample-data/LINK_SYSTEM_SPEC.md), [IDENTITY_COLLECTION_CONFIG](../../docs-shared/ai-context/folkform/sample-data/IDENTITY_COLLECTION_CONFIG.md)

---

## 1. Tổng Quan

Phương án gồm **2 phần**:

| Phần | Mục tiêu | Deliverables |
|------|----------|--------------|
| **Phần 1** | Đề xuất cơ chế để **doc mới** luôn có đủ 4 lớp ID | Hooks, helpers, sửa creation flow trong services |
| **Phần 2** | Tạo job rà soát và bổ sung **doc cũ** | Migration jobs chạy batch, có thể schedule |

Bốn lớp định danh/liên kết:
- **(1) Storage** — `_id` (giữ nguyên)
- **(2) Canonical** — `uid` (prefix + _id.Hex())
- **(3) External IDs** — `sourceIds`
- **(4) Links** — `links` với `uid` + `externalRefs`

**Nguyên tắc:** uid luôn = prefix + _id.Hex() — không dùng external ID để tạo uid.

---

## 2. Chiến Lược Tạo UID (Chuẩn Duy Nhất)

**uid = prefix + _id.Hex()** — áp dụng cho mọi collection.

```go
uid := utility.UIDFromObjectID(utility.UIDPrefixCustomer, doc.ID)
// cust_507f1f77bcf86cd799439011
```

- **Doc mới:** Tạo `_id` trước → `uid = UIDFromObjectID(prefix, _id)`
- **Doc cũ (migration):** Đã có `_id` → `uid = UIDFromObjectID(prefix, _id)`

**UIDFromSource** không dùng cho uid — chỉ dùng cho lookup (tìm doc theo sourceIds) hoặc logic khác.

---

## 3. Phase 0: Nền Tảng (Trước Phần 1 & 2)

Cần có trước khi triển khai:
- LinkItem model, ExternalRef
- BuildLinkResolved, BuildLinkPending
- ResolveUnifiedId hỗ trợ lookup theo uid

---

### 3.1. Models & Types (Phase 0)

Tạo shared types cho link item:

```
api/internal/api/models/identity/
  link_item.go      // LinkItem struct
  external_ref.go   // ExternalRef struct
```

```go
// LinkItem — schema chuẩn 1 link
type LinkItem struct {
    Uid          string         `json:"uid" bson:"uid"`
    ExternalRefs  []ExternalRef  `json:"externalRefs" bson:"externalRefs"`
    Role         string         `json:"role,omitempty" bson:"role,omitempty"`
    Status       string         `json:"status" bson:"status"` // resolved | pending_resolution | conflict | detached
    Confidence   float64        `json:"confidence,omitempty" bson:"confidence,omitempty"`
}

type ExternalRef struct {
    Source string `json:"source" bson:"source"` // pos, facebook, zalo, shopify
    ID     string `json:"id" bson:"id"`
}
```

### 3.2. Helper Chính: EnsureIdentity4Layers

**Ý tưởng:** Một helper duy nhất — nhét document + config vào → tự xử lý ra đủ 4 lớp ID.

```
api/internal/utility/identity/
  enricher.go    // EnsureIdentity4Layers
  config.go      // IdentityConfig
```

```go
// EnsureIdentity4Layers bổ sung đủ 4 lớp ID cho document.
// Chỉ cần collectionName — helper tra cứu nội bộ prefix, sourceIds mapping, links mapping.
func EnsureIdentity4Layers(ctx context.Context, collectionName string, doc interface{}, resolver CustomerResolver) error
```

**Registry:** Helper tra cứu theo collection name. **Collection → links bắt buộc** — xem [LINK_SYSTEM_SPEC](../../docs-shared/ai-context/folkform/sample-data/LINK_SYSTEM_SPEC.md). **Path extract** — xem [IDENTITY_COLLECTION_CONFIG](../../docs-shared/ai-context/folkform/sample-data/IDENTITY_COLLECTION_CONFIG.md).

---

### Logic Từng Lớp

*Nguyên tắc chung: **đã có rồi thì bỏ qua** — kiểm tra trước khi thực hiện, tránh query thừa.*

#### Lớp 2 (uid)

- **Kiểm tra:** Nếu `doc.uid` đã có và đúng format → bỏ qua
- **Lấy:** `prefix` + `_id` của document đó
- **Công thức:** `uid = prefix + doc._id.Hex()`
- **Nguồn:** prefix tra cứu theo collection name; `_id` từ chính document

#### Lớp 3 (sourceIds)

- **Kiểm tra:** Nếu `doc.sourceIds` đã có đủ theo config → bỏ qua
- **Lấy:** sourceId — thường nằm trong **payload data** (vd: `PanCakeData`, `PosData`)
- **Ví dụ:** `PosData.id` → `sourceIds.pos`, `PanCakeData.id` → `sourceIds.facebook`
- **Nguồn:** path theo collection — xem [IDENTITY_COLLECTION_CONFIG](../../docs-shared/ai-context/folkform/sample-data/IDENTITY_COLLECTION_CONFIG.md)

#### Lớp 4 (links)

- **Kiểm tra:** Nếu `doc.links` đã có đủ link cần thiết (uid + externalRefs) → bỏ qua
- **Bước 1:** Với mỗi nguồn, tìm **externalRefs** trong **payload data** của document (vd: `customerId`, `PosData.customer.id`, `PanCakeData.customer_id`)
- **Bước 2:** Tìm externalRefs tương ứng trong **collection có liên quan** (vd: crm_customers, cio_sessions)
- **Bước 3:** Nếu tìm thấy → cập nhật thêm **uid** vào link (status = resolved); nếu không → link với externalRefs, status = pending_resolution
- **Bước 4:** Tìm các document **có thể có liên kết tới document này** (vd: doc khác có link pending trỏ tới externalRef của doc hiện tại) → cập nhật externalRefs của chúng để trỏ tới uid của document này (nếu match)
- **Path, field cụ thể:** xem tài liệu về link trong [sample-data](../../docs-shared/ai-context/folkform/sample-data/)

**Cách dùng:**
```go
doc := &CioSession{ID: primitive.NewObjectID(), CustomerId: fbCustomerId, ...}
err := identity.EnsureIdentity4Layers(ctx, "cio_sessions", doc, crmResolver)
// doc giờ có Uid, Links.customer
```

### 3.3. Helper Phụ (utility/links.go)

```go
// BuildLinkResolved(uid string, externalRefs []ExternalRef) LinkItem
// BuildLinkPending(source, id string) LinkItem
```

### 3.4. UID Helper (utility/uid.go)

Đã có `UIDFromObjectID(prefix, id)` — tạo uid = prefix + _id.Hex().

### 3.5. ResolveUnifiedId Mở Rộng

Đảm bảo `ResolveUnifiedId` (CRM) hỗ trợ lookup theo:
- `uid`
- `sourceIds.pos`, `sourceIds.fb`, `sourceIds.zalo`, `sourceIds.FbByPage`, `sourceIds.ZaloByPage`

*(Đã có — kiểm tra coverage.)*

---

# PHẦN 1: Cơ Chế Cho Doc Mới

*Phần 1 cần Phase 0 (LinkItem model, BuildLinkResolved, BuildLinkPending) làm nền tảng trước.*

Đề xuất các cơ chế để mọi document insert mới luôn có đủ 4 lớp ID.

## 4. Cơ Chế Cho Doc Mới — Dùng EnsureIdentity4Layers

Gọi helper trước khi insert — **chỉ cần collection name**:

```go
doc := &CioSession{ID: primitive.NewObjectID(), CustomerId: fbCustomerId, ...}
_ = identity.EnsureIdentity4Layers(ctx, "cio_sessions", doc, crmResolver)
coll.InsertOne(ctx, doc)
```

Helper tra cứu theo collection name → prefix, cách lấy sourceIds/links từ doc → tự xử lý đủ 4 lớp.

---

# PHẦN 2: Job Rà Soát & Bổ Sung Doc Cũ

Tạo job chạy để rà soát và bổ sung tất cả document cũ thiếu lớp 2, 3, 4.

## 8. Job 1: Bổ Sung uid (Lớp 2)

**Mục:** Tìm doc thiếu `uid` → set `uid = UIDFromObjectID(prefix, _id)`.

| Collection | Prefix | Filter |
|------------|--------|--------|
| crm_customers | cust_ | `{ uid: "" }` hoặc `{ uid: { $exists: false } }` |
| pc_pos_customers | cust_ | tương tự |
| fb_customers | cust_ | tương tự |
| pc_pos_orders | ord_ | tương tự |
| cio_sessions | sess_ | tương tự |
| cio_events | evt_ | tương tự |
| cio_touchpoint_plans | plan_ | tương tự |
| cio_plan_executions | exe_ | tương tự |
| crm_activity_history | act_ | tương tự |
| crm_notes | note_ | tương tự |

**Logic:** `scripts/migrate_identity_uid.go` hoặc worker `identity_uid_backfill` — batch 500–1000, có thể resume.

---

## 9. Job 2: Bổ Sung sourceIds (Lớp 3)

**Mục:** Điền sourceIds từ field hiện có cho doc thiếu.

| Collection | Nguồn điền |
|------------|------------|
| crm_customers | Đã có — kiểm tra format, rà soát doc thiếu |
| pc_pos_customers | PosData.id → sourceIds.pos |
| fb_customers | PanCakeData.id → sourceIds.facebook |
| pc_pos_orders | orderId → sourceIds.pos |

**Logic:** `scripts/migrate_identity_sourceids.go` — batch, có thể resume.

---

## 10. Job 3: Bổ Sung links (Lớp 4)

**Mục:** Build links từ customerId/unifiedId → resolve → link.

| Collection | Link cần | Nguồn |
|------------|----------|-------|
| cio_sessions | customer | customerId/unifiedId → resolve → uid |
| cio_events | customer, session | customerId, sessionId |
| pc_pos_orders | customer | customerId → resolve |
| crm_activity_history | customer | unifiedId |
| crm_notes | customer | customerId |

**Logic:** `scripts/migrate_identity_links.go` — query doc có customerId/unifiedId nhưng chưa có links. Resolve → $set links.

---

## 11. Job 4: Reconciliation (pending_resolution)

**Mục:** Xử lý `links.*.status = pending_resolution` — thử resolve lại.

**Logic:** Query `{ "links.customer.status": "pending_resolution" }` → ResolveBySource(externalRef) → nếu match: $set uid, status = resolved.

**Schedule:** Schedule mỗi 5–15 phút hoặc chạy thủ công.

---

## 12. Thống Nhất Job

Có thể gộp thành 1 worker `identity_backfill` với 4 mode (uid, sourceIds, links, reconciliation) hoặc 4 job riêng. Đăng ký trong `worker/controller.go`.

---

## 13. Phụ Lục: API & Contract

- Trả `uid` thay vì `_id` trong response.
- Giữ `unifiedId` trong response tạm (alias) để backward compat.

## 16. Rà Soát CRM (Đã Hoàn Thành 2026-03-17)

Đã cập nhật code CRM để hỗ trợ uid và links:

| Thay đổi | File |
|----------|------|
| Lookup customer: `buildCustomerFilterByIdOrUid` | service.crm.customer.go |
| GetProfile, RefreshMetrics, GetFullProfile dùng filter uid/unifiedId | service.crm.customer.go, metrics.go, fullprofile.go |
| buildCustomerIdsForRecalculate thêm c.Uid | service.crm.recalculate.go |
| findByPosId/FbId/ZaloId thêm uid vào $or | service.crm.merge.go |
| aggregateOrderMetricsForCustomer thêm links.customer.uid | service.crm.metrics.go |
| buildConversationFilterForCustomerIds thêm links.customer.uid | service.crm.conversation_metrics.go |
| CrmCustomerProfileResponse thêm field Uid | dto.crm.customer.go |
| toDashboardItem ưu tiên c.Uid cho CustomerID | service.crm.dashboard.go |

Chi tiết: [HUONG_DAN_IDENTITY_LINKS.md](./HUONG_DAN_IDENTITY_LINKS.md) mục 9.1.

---

## 14. Rủi Ro & Mitigation

| Rủi ro | Mitigation |
|--------|------------|
| Job làm chậm DB | Batch nhỏ (500–1000), throttle, chạy off-peak |
| Client cũ dùng unifiedId | Giữ alias 3–6 tháng, deprecation notice |
| Link pending quá lâu | Reconciliation job + alert khi pending > 24h |
| Conflict không auto-resolve | status=conflict, human review queue |

---

## 15. Checklist Trước Khi Bắt Đầu

- [ ] Đọc [identity-links-model](../../docs-shared/architecture/data-contract/identity-links-model.md)
- [ ] Đọc [COLLECTIONS_CAN_UID_SYSTEM](../../docs-shared/ai-context/folkform/sample-data/COLLECTIONS_CAN_UID_SYSTEM.md)
- [ ] Đọc [LINK_SYSTEM_SPEC](../../docs-shared/ai-context/folkform/sample-data/LINK_SYSTEM_SPEC.md) — collection phải link đến collection nào, qua key nào
- [ ] Đọc [IDENTITY_COLLECTION_CONFIG](../../docs-shared/ai-context/folkform/sample-data/IDENTITY_COLLECTION_CONFIG.md) — path extract sourceIds/links
- [ ] Backup DB trước chạy job
- [ ] Có staging env để test job
- [ ] Thống nhất source keys: `pos`, `facebook`, `zalo`, `shopify` (lowercase, không dấu)
