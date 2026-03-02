# Đề xuất tối ưu CRM Pending Ingest

## 1. Phân tích hiện trạng

### 1.1 Luồng hiện tại

```
CRUD (BaseService) → EmitDataChanged → handleCrmDataChange → [Skip?] → EnqueueCrmIngest (upsert)
                                                                    ↓
Worker ← GetUnprocessedCrmIngest ← crm_pending_ingest
```

### 1.2 Các điểm gây chậm và tạo quá nhiều ingest

| Vấn đề | Chi tiết |
|--------|----------|
| **1. Skip logic không chạy cho webhook** | `FindOneAndUpdate` (order, conversation, customer webhook) **không truyền PreviousDocument** → hook luôn enqueue, không bao giờ skip |
| **2. Chỉ UpdateOne có PreviousDocument** | UpdateMany, Upsert, FindOneAndUpdate đều không có PreviousDocument |
| **3. mapsEqual chậm** | Fallback so sánh map dùng BSON marshal toàn bộ posData/panCakeData — tốn CPU |
| **4. Mỗi event = 1 DB write** | Dù có deduplicate (upsert), mỗi event vẫn gọi `coll.UpdateOne` vào crm_pending_ingest |
| **5. crm_notes không skip** | `mergeRelevantDataKey` trả "" → luôn enqueue mọi CRUD note |
| **6. Worker xử lý tuần tự** | Mỗi job gọi Merge/Ingest — nặng, chạy từ cũ đến mới (createdAt asc) |

### 1.3 Nguồn event chính

| Nguồn | Method | PreviousDocument? | Kết quả |
|-------|--------|-------------------|---------|
| Webhook order_created/updated | FindOneAndUpdate | ❌ Không | Luôn enqueue |
| Webhook conversation_updated | FindOneAndUpdate | ❌ Không | Luôn enqueue |
| Webhook customer_updated | FindOneAndUpdate | ❌ Không | Luôn enqueue |
| CRUD API (UpdateOne) | UpdateOne | ✅ Có | Có thể skip |
| upsertCustomerFromConversation | Collection().UpdateOne | ❌ Không emit | Không qua hook |

---

## 2. Đề xuất phương án tối ưu

### 2.1 [Ưu tiên cao] Thêm PreviousDocument vào FindOneAndUpdate

**Vấn đề:** Webhook dùng FindOneAndUpdate, không có PreviousDocument → skip logic không bao giờ chạy.

**Giải pháp:** BaseServiceMongoImpl.FindOneAndUpdate đã có `existing` khi document tồn tại. Truyền vào EmitDataChanged.

```go
// service.base.mongo.go - FindOneAndUpdate
var prevDoc interface{}
if isExisting {
    prevDoc = existing
}
events.EmitDataChanged(ctx, events.DataChangeEvent{
    CollectionName:    s.collection.Name(),
    Operation:         op,
    Document:          result,
    PreviousDocument:  prevDoc,  // Thêm dòng này
})
```

**Tác động:** Khi Pancake gửi webhook trùng payload (retry, duplicate) hoặc update không đổi posData/panCakeData.updated_at → skip enqueue.

**Effort:** Thấp (1 file, vài dòng).

---

### 2.2 [Ưu tiên cao] Bỏ fallback mapsEqual — chỉ dùng updated_at

**Vấn đề:** Khi không có updated_at, fallback sang `mapsEqual` (BSON marshal toàn bộ map) — chậm, tốn CPU.

**Phân tích sample data:** Tất cả pc_pos_*, fb_* đều có `updated_at` trong posData/panCakeData. Trường hợp không có rất hiếm.

**Giải pháp:** Chỉ so sánh updated_at. Nếu không có updated_at → **không skip** (enqueue để an toàn). Bỏ `mapsEqual`.

**Lợi ích:** Giảm CPU, code đơn giản hơn.

**Effort:** Thấp.

---

### 2.3 [Ưu tiên trung bình] Debounce Enqueue — tránh ghi DB liên tục

**Vấn đề:** Mỗi event gọi EnqueueCrmIngest → 1 UpdateOne. Sync 100 customers trong 2 giây = 100 writes.

**Giải pháp A — In-memory debounce (đơn giản):**
- Map `businessKey -> {updatedAt, lastEnqueueAt}` với TTL 5–10s
- Event mới: nếu cùng businessKey, cùng updated_at, và lastEnqueueAt < 5s → skip ghi DB
- Sau 5s hoặc updated_at khác → enqueue và cập nhật cache

**Giải pháp B — Batch enqueue (phức tạp hơn):**
- Hook không ghi DB ngay, đẩy vào channel
- Goroutine định kỳ (vd: mỗi 2s) batch upsert nhiều job một lúc

**Ưu tiên:** Giải pháp A — đơn giản, giảm write khi webhook gửi nhiều event trùng trong vài giây.

**Effort:** TB.

---

### 2.4 [Ưu tiên trung bình] Ưu tiên job mới trước (createdAt desc)

**Vấn đề:** Worker sort `createdAt asc` → xử lý job cũ trước. User thường quan tâm dữ liệu mới.

**Giải pháp:** Đổi sort thành `createdAt desc` trong GetUnprocessedCrmIngest.

**Lưu ý:** Job cũ vẫn được xử lý, chỉ chậm hơn. Có thể kết hợp với TTL (2.9) để bỏ qua job quá cũ.

**Effort:** Thấp.

---

### 2.5 [Ưu tiên thấp] crm_notes — so sánh updatedAt

**Vấn đề:** crm_notes không có posData/panCakeData, `mergeRelevantDataKey` trả "" → luôn enqueue.

**Giải pháp:** Thêm case cho crm_notes: so sánh `updatedAt` (field top-level của CrmNote) giữa doc và prevDoc. Nếu bằng → skip.

**Effort:** Thấp.

---

### 2.6 [Ưu tiên thấp] TTL / cleanup job cũ

**Vấn đề:** Job lỗi, retry nhiều lần tích tụ. Worker vẫn cố xử lý.

**Giải pháp:** 
- Job có `createdAt` > 24h và chưa processed → đánh dấu processedAt = now, processError = "expired"
- Cron xóa job đã processed > 7 ngày

**Effort:** Thấp.

---

## 3. Thứ tự triển khai đề xuất

| # | Phương án | Tác động | Effort | Ưu tiên |
|---|-----------|----------|--------|---------|
| 1 | Thêm PreviousDocument vào FindOneAndUpdate (2.1) | Bật skip cho webhook | Thấp | **Cao** |
| 2 | Bỏ mapsEqual, chỉ dùng updated_at (2.2) | Giảm CPU, đơn giản hóa | Thấp | **Cao** |
| 3 | Debounce Enqueue (2.3) | Giảm DB write khi sync nhanh | TB | TB |
| 4 | Sort createdAt desc (2.4) | UX tốt hơn cho realtime | Thấp | TB |
| 5 | crm_notes so sánh updatedAt (2.5) | Giảm ingest note trùng | Thấp | Thấp |
| 6 | TTL / cleanup (2.6) | Tránh backlog vô hạn | Thấp | Thấp |

---

## 4. Tóm tắt

| Nhóm | Hành động |
|------|-----------|
| **Giảm job tạo ra** | 2.1 (PreviousDocument), 2.2 (bỏ mapsEqual), 2.3 (debounce), 2.5 (crm_notes) |
| **Giảm CPU** | 2.2 (bỏ BSON marshal map) |
| **Giảm DB write** | 2.3 (debounce) |
| **Vệ sinh** | 2.4 (ưu tiên mới), 2.6 (TTL) |

---

## 5. Tài liệu tham khảo

- `docs/05-development/DE_XUAT_GIAM_SO_CONG_VIEC_TINH_TOAN.md` — Đề xuất gốc
- `api/internal/api/crm/service/service.crm.hooks.go` — Hook CRM
- `api/internal/api/base/service/service.base.mongo.go` — FindOneAndUpdate, EmitDataChanged
- `api/internal/api/webhook/handler/handler.pancake.webhook.go` — Webhook handler
