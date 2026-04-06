# Hướng Dẫn Sử Dụng Identity & Links

**Phiên bản:** 1.3  
**Ngày:** 2026-03-17  
**Cập nhật:** 2026-04-07 — mục 2.1 dùng **L1-persist/L2-persist** (mirror/canonical), trỏ [KHUNG_KHUON_MODULE_INTELLIGENCE](./KHUNG_KHUON_MODULE_INTELLIGENCE.md) mục 0 để không nhầm với pipeline CIX / CRM `layer1`. **2026-04-06:** trỏ [uid-field-naming](../../docs-shared/architecture/data-contract/uid-field-naming.md); [unified-data-contract](../../docs-shared/architecture/data-contract/unified-data-contract.md) §1.7  
**Cập nhật trước:** mục 3.6, 9.1 — CRM rà soát (lookup, filter, response)  
**Mục đích:** Hướng dẫn developer cách dùng ID và link — ưu tiên cấu trúc mới (4 lớp), fallback logic cũ.

**Tham chiếu:**
- [identity-links-model](../../docs-shared/architecture/data-contract/identity-links-model.md) — Spec chuẩn
- [uid-field-naming](../../docs-shared/architecture/data-contract/uid-field-naming.md) — **Bảng tiền tố (khớp `uid.go`), `links`, camelCase, event/queue/collection, mirror/canonical** — đọc trước khi đặt tên field mới
- [LINK_SYSTEM_SPEC](../../docs-shared/ai-context/folkform/sample-data/LINK_SYSTEM_SPEC.md) — Config thực tế
- [PHUONG_AN_TRIEN_KHAI_IDENTITY_LINKS.md](./PHUONG_AN_TRIEN_KHAI_IDENTITY_LINKS.md) — Phương án triển khai

---

## 1. Tổng Quan — Ưu Tiên Mới, Fallback Cũ

| Ưu tiên | Field / Cách | Khi dùng |
|---------|--------------|----------|
| **1 (mới)** | `uid` | ID chuẩn của entity — dùng cho mọi cross-module, API contract |
| **1 (mới)** | `links.<key>.uid` | Tham chiếu entity khác khi đã resolve |
| **2 (fallback)** | `unifiedId` | Cũ — customer canonical ID; vẫn hỗ trợ lookup |
| **2 (fallback)** | `customerId`, `sessionId` | Cũ — external ID từ kênh; dùng khi chưa có uid |
| **2 (fallback)** | `customerUid` | Cũ — field rời; migration dần sang `links.customer` |

**Nguyên tắc:** Code mới ưu tiên `uid` và `links`. Code cũ tiếp tục chạy nhờ fallback.

---

## 2. Bốn Lớp Định Danh (Tóm Tắt)

| Lớp | Field | Ý nghĩa | Ví dụ |
|-----|-------|---------|-------|
| **(1) Storage** | `_id` | ObjectID MongoDB — nội bộ | `ObjectId("507f...")` |
| **(2) Canonical** | `uid` | ID chuẩn hệ thống — public | `cust_a1b2c3d4e5f6` |
| **(3) External IDs** | `sourceIds` | "Tôi là ai ở hệ ngoài" | `{pos: "uuid", fb: "page_psid"}` |
| **(4) Links** | `links` | "Tôi nối tới ai" | `{customer: {uid: "cust_xxx", status: "resolved"}}` |

### 2.1. Hai lớp document persistence: L1-persist (mirror) và L2-persist (canonical)

**Tách bạch với bảng trên:** bốn hàng §2 là *loại field*; **L1-persist / L2-persist** là *bản ghi mirror hay canonical* trong data contract — **không** phải bước “L1/L2/L3” của **pipeline rule CIX** hay trường BSON **`layer1`/`layer2`** CRM ([KHUNG_KHUON_MODULE_INTELLIGENCE.md](./KHUNG_KHUON_MODULE_INTELLIGENCE.md) mục 0).

| | **L1-persist — mirror / thô** | **L2-persist — canonical / đã merge** |
|---|------------------------|-------------------------------|
| **Vai trò** | Nguồn dữ liệu đồng bộ theo kênh; tham chiếu để tạo/cập nhật canonical | Thực thể trong hệ; tương tác API / event / module khác |
| **`uid`** | Thường không có hoặc không dùng làm ID công khai chính | **Bắt buộc theo contract** khi entity đã canonical hóa (prefix `cust_`, `ord_`, …) |
| **`sourceIds`** | Một nguồn hoặc field phẳng kiểu id nguồn | Gom đa nguồn sau merge hoặc 1:1 có `source` |
| **`links`** | Có thể có **mirror → mirror** (hoặc `externalRefs`) để giữ đúng quan hệ nguồn; merge dùng để suy ra canonical | **canonical → canonical**, ưu tiên `uid` đích đã resolve |
| **Nối mirror↔canonical** | Canonical mang `source` + `sourceRecordMongoId` → `_id` mirror (1:1) hoặc resolver từ id nguồn | — |

**Nguyên tắc code:** Join và payload **liên module** dùng **`uid` / `links` của canonical (L2-persist)**. Khi chỉ có id trên mirror, resolve (CRM, v.v.) rồi mới dùng như contract.

**Spec đầy đủ:** [unified-data-contract.md](../../docs-shared/architecture/data-contract/unified-data-contract.md) §1.7, [identity-links-model.md](../../docs-shared/architecture/data-contract/identity-links-model.md) §1.1.

---

## 3. Khi Nào Dùng Gì

### 3.1. Lấy ID của entity hiện tại

```go
// ✅ Ưu tiên (mới)
uid := customer.Uid  // cust_xxx

// ⚠️ Fallback (cũ) — khi doc chưa có uid (backfill chưa chạy)
if uid == "" {
    uid = utility.UIDFromObjectID(utility.UIDPrefixCustomer, customer.ID)
}
```

### 3.2. Tham chiếu customer từ entity khác (session, order, conversation)

```go
// ✅ Ưu tiên (mới) — qua links
customerUid := session.Links["customer"].Uid
if customerUid != "" {
    // Đã resolve — dùng trực tiếp
}

// ⚠️ Fallback (cũ) — field rời
if customerUid == "" {
    customerUid = session.CustomerUid
}
if customerUid == "" {
    customerUid = session.UnifiedID  // deprecated
}

// ⚠️ Fallback (cũ) — external ID, cần resolve
if customerUid == "" && session.CustomerId != "" {
    customerUid, _ = crmSvc.ResolveUnifiedId(ctx, session.CustomerId, ownerOrgID)
    // ResolveUnifiedId đã hỗ trợ lookup theo uid, sourceIds.*, unifiedId
}
```

### 3.3. Lookup customer theo external ID (pos, fb, zalo)

```go
// ✅ Ưu tiên (mới) — ResolveToUid trả về uid (cust_xxx)
// Cần inject CrmResolver hoặc dùng identity.GetDefaultResolver()
resolver := identity.GetDefaultResolver()
if resolver != nil {
    uid, ok := resolver.ResolveToUid(ctx, externalId, "facebook", ownerOrgID)
    if ok {
        // uid = cust_xxx
    }
}

// ⚠️ Fallback (cũ) — ResolveUnifiedId trả về unifiedId
// Hỗ trợ lookup: uid, sourceIds.pos, sourceIds.fb, sourceIds.zalo, sourceIds.allInboxIds, unifiedId
unifiedId, ok := crmSvc.ResolveUnifiedId(ctx, externalId, ownerOrgID)
if ok {
    // unifiedId — dùng được; nếu cần uid thì dùng ResolveToUid
}
```

### 3.4. Query document theo link (vd: orders của customer)

```go
// ✅ Ưu tiên (mới) — query theo links.customer.uid
filter := bson.M{
    "ownerOrganizationId": ownerOrgID,
    "links.customer.uid":  customerUid,
}

// ⚠️ Fallback (cũ) — khi links chưa có — CRM đã implement $or gộp cả links
// Ví dụ aggregateOrderMetricsForCustomer, buildConversationFilterForCustomerIds:
filter = bson.M{
    "ownerOrganizationId": ownerOrgID,
    "$or": []bson.M{
        {"customerId": bson.M{"$in": ids}},
        {"links.customer.uid": bson.M{"$in": ids}},  // Identity 4 lớp
        {"posData.customer_id": bson.M{"$in": ids}},
    },
}
```

### 3.5. Tạo document mới — Insert qua BaseService

Doc mới insert qua `BaseServiceMongoImpl.InsertOne` / `InsertMany` **tự động** được enrich (uid, sourceIds, links) nếu collection nằm trong registry. Không cần gọi thủ công.

**Insert trực tiếp** (không qua BaseService) — gọi helper trước:

```go
dataMap, _ := utility.ToMap(doc)
if identity.ShouldEnrich(collectionName) {
    _ = identity.EnrichIdentity4Layers(ctx, collectionName, dataMap, nil)
}
coll.InsertOne(ctx, dataMap)
```

### 3.6. Lookup customer theo uid hoặc unifiedId (API param)

API nhận param `unifiedId` — có thể là uid hoặc unifiedId thực tế. Dùng helper:

```go
// buildCustomerFilterByIdOrUid — service.crm.customer.go
filter := buildCustomerFilterByIdOrUid(idOrUid, ownerOrgID)
// → bson.M{"ownerOrganizationId": ..., "$or": [{"uid": idOrUid}, {"unifiedId": idOrUid}]}
customer, err := svc.FindOne(ctx, filter, nil)
```

---

## 4. Prefix UID Theo Entity

| Prefix | Entity | Collection |
|--------|--------|------------|
| `cust_` | Customer | crm_customers, pc_pos_customers, fb_customers |
| `ord_` | Order | pc_pos_orders |
| `conv_` | Conversation | fb_conversations |
| `sess_` | Session | cio_sessions |
| `evt_` | Event | cio_events |
| `plan_` | Touchpoint Plan | cio_touchpoint_plans |
| `dec_` | Routing Decision | cio_routing_decisions |
| `exe_` | Plan Execution | cio_plan_executions |
| `note_` | Note | crm_notes |
| `act_` | Activity | crm_activity_history |

---

## 5. Link Keys Chuẩn

| Key trong `links` | Collection đích | Dùng khi |
|-------------------|-----------------|----------|
| `customer` | crm_customers | Session, event, order, conversation, note, plan, execution |
| `session` | cio_sessions | Event |
| `order` | pc_pos_orders | (nếu cần) |

---

## 6. Source Keys (sourceIds)

| Key | Nguồn | Ví dụ |
|-----|-------|-------|
| `pos` | Pancake POS | UUID từ pc_pos_customers |
| `facebook` | Facebook / Pancake | customerId từ fb_customers |
| `zalo` | Zalo | Zalo user ID |

---

## 7. Link Status

| Status | Ý nghĩa |
|--------|---------|
| `resolved` | Đã có `uid` — entity đích đã sync |
| `pending_resolution` | Chưa có `uid` — chỉ có externalRefs |
| `conflict` | Nhiều candidate — chưa chốt |
| `detached` | Link không còn hợp lệ |

---

## 8. Fallback Logic — Tóm Tắt

Khi đọc/tham chiếu customer từ entity khác:

1. **Ưu tiên:** `links.customer.uid` hoặc `customerUid`
2. **Fallback:** `unifiedId` (deprecated)
3. **Fallback:** `customerId` + `ResolveUnifiedId` → uid

Khi query theo customer:

1. **Ưu tiên:** `links.customer.uid` = customerUid
2. **Fallback:** `customerId` ∈ list, `posData.customer_id` ∈ list (theo từng collection)

Khi cần ID entity hiện tại:

1. **Ưu tiên:** `uid`
2. **Fallback:** `UIDFromObjectID(prefix, _id)` nếu uid rỗng

---

## 9. Code Tham Chiếu

| Chức năng | Vị trí |
|-----------|--------|
| Resolve external → uid | `api/internal/api/crm/service/service.crm.resolve.go` — ResolveUnifiedId |
| Resolver interface | `api/internal/utility/identity/enricher.go` — Resolver, SetDefaultResolver |
| Enrich doc mới | `api/internal/utility/identity/enricher.go` — EnrichIdentity4Layers |
| Registry collection | `api/internal/utility/identity/registry.go` — GetConfig, ShouldEnrich |
| UID helper | `api/internal/utility/uid.go` — UIDFromObjectID, UIDPrefix* |
| LinkItem, ExternalRef | `api/internal/utility/identity/link_item.go` |
| Backfill doc cũ | `api/internal/worker/identity_backfill_worker.go` |

### 9.1. CRM Module — Đã Rà Soát (2026-03-17)

| Chức năng | Vị trí | Mô tả |
|-----------|--------|-------|
| Lookup customer (uid hoặc unifiedId) | `service.crm.customer.go` — `buildCustomerFilterByIdOrUid` | Filter `$or: {uid, unifiedId}` cho GetProfile, RefreshMetrics, GetFullProfile |
| Danh sách ID để aggregate | `service.crm.recalculate.go` — `buildCustomerIdsForRecalculate` | Thêm `c.Uid` ưu tiên đầu |
| Query orders theo customer | `service.crm.metrics.go` — `aggregateOrderMetricsForCustomer` | Thêm `links.customer.uid` vào `$or` |
| Query orders (backfill) | `service.crm.recalculate.go` — `backfillOrderActivitiesForCustomer` | Thêm `links.customer.uid` |
| Query conversations | `service.crm.conversation_metrics.go` — `buildConversationFilterForCustomerIds` | Thêm `links.customer.uid` |
| Aggregate conv metrics | `service.crm.metrics.go` — `aggregateConversationMetricsForCustomer` | Thêm `links.customer.uid` |
| findByPosId / findByFbId / findByZaloId | `service.crm.merge.go` | Thêm `uid` vào `$or` |
| Response profile | `dto.crm.customer.go` — `CrmCustomerProfileResponse` | Thêm field `Uid` |
| Dashboard CustomerID | `service.crm.dashboard.go` — `toDashboardItem` | Ưu tiên `c.Uid`, fallback `c.UnifiedId` |

---

## 10. Checklist Khi Viết Code Mới

- [ ] Dùng `uid` thay vì `_id` khi expose ra API/contract
- [ ] Dùng `links.<key>.uid` khi tham chiếu entity khác (nếu đã có)
- [ ] Fallback `customerUid` / `unifiedId` / `customerId` khi doc cũ
- [ ] Query ưu tiên `links.customer.uid`, fallback `customerId` theo collection
- [ ] Insert qua BaseService để tự enrich; nếu insert trực tiếp thì gọi `EnrichIdentity4Layers`
