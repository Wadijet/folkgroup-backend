# Phương án chỉnh domain Order khớp khung Ingress → Merge/Enrich → Intelligence (CIO · Domain · AID)

**Trạng thái triển khai (2026-04-02):** Đã tách **`commerce_orders`** (canonical); `pc_pos_orders` chỉ mirror Pancake. Đồng bộ 1:1 trong `applyDatachangedSideEffects` qua `order/datachanged.SyncCommerceOrderFromPancakeDataChange` (không emit queue thứ hai). Order Intel đọc **`commerce_orders`**, fallback `pc_pos_orders` nếu thiếu bản chiếu. Backfill lịch sử: gọi `orderdatachanged.UpsertCommerceFromPancakePosOrder` theo lô (user tự triển khai sau).

**Mục đích:** Đưa module **đơn hàng (PC POS + Order Intelligence)** lên cùng **ngôn ngữ kiến trúc** với [KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md](./KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md) và **bốn lớp ID** ([unified-data-contract.md](../../docs-shared/architecture/data-contract/unified-data-contract.md)), **không bắt buộc** copy nguyên xi cơ chế queue merge như CRM nếu **một đơn = một document canonical** đủ cho nghiệp vụ hiện tại.

**Tham chiếu code:** `api/internal/api/pc/models/model.pc.pos.order.go`, `orderintel/service/*`, `orderintel/worker/worker.orderintel.intel_compute.go`, `aidecision/worker/worker.aidecision.datachanged_side_effects.go` (nhánh `pc_pos_orders`).

---

## 1. Hiện trạng (đối chiếu khung)

| Pha | Khung chung | Order hiện tại | Ghi chú |
|-----|-------------|----------------|---------|
| **A — Thô + DataChanged** | Ghi collection mirror → `EmitDataChanged` → AID điều phối | ✅ CIO / sync → `pc_pos_orders` → event `order.*` → `applyDatachangedSideEffects` → enqueue `order_intel_compute` | Khớp |
| **B — Merge / chuẩn hóa đa nguồn** | Worker domain gộp nhiều nguồn → một aggregate | ⚠️ **Chưa có** — chỉ một collection `pc_pos_orders`; intel đọc thẳng document | Chấp nhận được **khi chỉ có Pancake POS**; **chưa** sẵn sàng đa nền tảng |
| **C — Intelligence + báo AID** | Worker domain tính → emit `*_recomputed` | ✅ `RunOrderIntelComputeJob` → `order_intel_recomputed` | Khớp |

**Identity trên đơn:** Model `PcPosOrder` đã có `uid` (`ord_*`), `sourceIds`, `links` — **Pha A đã gần đúng contract** nếu mọi đường sync (CIO `domain=order`, webhook…) **luôn** điền đủ qua helper upsert (xem [HUONG_DAN_IDENTITY_LINKS.md](./HUONG_DAN_IDENTITY_LINKS.md)).

---

## 2. Khoảng trống cần xử lý (theo thứ tự ưu tiên)

1. **Đa nguồn đơn hàng (tương lai):** Không có lớp “canonical order” hoặc `order_pending_ingest` — thêm nguồn (Shopee, TikTok Shop, …) sẽ **đụng** `switch` cứng `src == pc_pos_orders` và `loadOrderForJob` chỉ đọc `PcPosOrders`.
2. **Enrich trước intel (tùy chọn):** Snapshot intel dùng `customerId` / `links.customer` nhưng **không** có bước domain rõ ràng “đảm bảo link đã resolve” trước khi tính — có thể chấp nhận hoặc bổ sung một bước nhẹ trong domain.
3. **AID:** Điều phối order intel gắn **một tên collection**; mở rộng nguồn cần sửa nhiều chỗ thay vì registry.

---

## 3. Phương án theo giai đoạn (đề xuất triển khai)

### Giai đoạn 0 — Cứng hóa Pha A + ID (ít rủi ro, nên làm trước)

- **Rà soát mọi đường ghi `pc_pos_orders`:** sau `SyncUpsert` / CRUD, bảo đảm:
  - `uid` (`ord_*`) luôn có khi tạo mới (idempotent theo `sourceIds.pos` + `ownerOrganizationId` nếu đã quy ước).
  - `sourceIds.pos` (hoặc tương đương) khớp ID ngoài.
  - `links.customer.uid` set khi đã biết khách chuẩn (`cust_*`); giữ `customerId` thô để CRM/backfill.
- **Job `order_intel_compute`:** giữ ưu tiên lookup theo `orderUid`, fallback `_id` hex — đã có; bổ sung **log/metric** khi job thiếu cả hai hoặc không tìm thấy document.
- **Tài liệu:** Một mục ngắn trong module map hoặc comment package `orderintel` trỏ [KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md](./KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md).

**Không** thêm queue merge nếu chưa có nguồn thứ hai.

---

### Giai đoạn 1 — “Pha B nhẹ”: enrich / chuẩn bị context trước khi tính snapshot (tùy nhu cầu)

**Mục tiêu:** Không nhất thiết tách collection mới; cải thiện **chất lượng input** cho `ComputeSnapshot` (customer canonical, conversation id thống nhất).

**Hướng A — Trong cùng worker intel (đơn giản):**

- Trong `RunOrderIntelComputeJob`, trước `ComputeSnapshot`, gọi hàm domain kiểu `EnrichOrderForIntel(ctx, order)`:
  - Nếu thiếu `links.customer.uid` nhưng có `customerId` → gọi CRM resolver / `ResolveUnifiedId` → **chỉ đọc** để bổ sung struct tạm hoặc patch nhẹ document (cân nhắc: patch DB có thể gây thêm `datachanged` — nên ưu tiên **enrich in-memory** cho snapshot trừ khi product muốn persist link).

**Hướng B — Queue riêng `order_pending_enrich` (khi enrich nặng hoặc cần retry):**

- Giống tinh thần `crm_pending_ingest` nhưng **chỉ** chuẩn hóa một document đơn (không merge nhiều nguồn).
- Sau enrich thành công → emit event yêu cầu intel (hoặc xếp thẳng `order_intel_compute`).

**Khuyến nghị:** Bắt đầu **Hướng A (in-memory)**; chỉ tách queue khi đo được latency / retry cần thiết.

---

### Giai đoạn 2 — Đa nguồn đơn hàng (khi có yêu cầu sản phẩm)

**Mục tiêu:** Một **đơn canonical** trong hệ (một `ord_*` / một hàng trong `pc_pos_orders` hoặc collection mới `commerce_orders`).

**Lựa chọn kiến trúc:**

| Cách | Ý tưởng | Ưu | Nhược |
|------|---------|-----|--------|
| **2a. Giữ `pc_pos_orders`, thêm collection mirror theo nguồn** | `shopee_orders`, … → worker `order_pending_ingest` → upsert/merge vào `pc_pos_orders` (trường `source`, `sourceIds`) | Intel giữ nguyên `loadOrderForJob` | Model `PcPosOrder` có thể cần field `source` / generalize tên |
| **2b. Collection canonical mới** | Mọi nguồn → `commerce_orders`; `pc_pos_orders` chỉ mirror POS hoặc deprecate dần | Tách rõ POS vs chuẩn | Migration + đổi consumer/report |

**Luồng khớp khung:**

1. Pha A: mỗi nguồn ghi collection mirror + `EmitDataChanged`.
2. Pha B: `IngestFromDataChange` (package `order` hoặc `order/datachanged`) enqueue `order_pending_ingest` → worker `ApplyOrderIngestFromDocument` switch theo `collectionName` → ghi **một** bản canonical.
3. Sau ingest: `NotifyOrderIntelIfNeeded` → event/debounce → `order_intel_compute` (tương tự CRM `recompute_requested`).
4. Pha C: không đổi — vẫn `RunOrderIntelComputeJob` đọc **chỉ** canonical.

**AID:** Mở rộng `applyDatachangedSideEffects` bằng **danh sách collection** hoặc **registry** (“collection → enqueue order ingest / order intel”) thay vì so sánh chuỗi một collection.

---

### Giai đoạn 3 — Config-driven routing (song song roadmap CRM)

- File / DB config: `enabledOrderSources`, `collection → handlerKey`.
- Domain đăng ký handler merge/enrich; **logic nghiệp vụ** vẫn trong `pc/service` hoặc `order/` — không nhét rule Shopee vào AID.

---

## 4. Việc **không** nên làm sớm

- **Không** chuyển `ComputeSnapshot` vào consumer AID — đã thống nhất tính tại worker domain.
- **Không** tạo `order_pending_ingest` trùng lặp với CRM: merge **khách** vẫn là `crm_pending_ingest`; queue order chỉ cho **materialize đơn canonical** khi thật sự đa nguồn.
- **Không** leak `_id` Mongo ra payload công khai; giữ `orderUid` / `uid` trong event (đúng [mục 2.1 khung](./KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md)).

---

## 5. Checklist nhanh sau khi chỉnh

- [ ] Mọi ingest path có `uid` + `sourceIds` đúng nguồn?
- [ ] `order_intel_compute` tìm được đơn theo `orderUid` sau CIO sync?
- [ ] `order_intel_recomputed` payload có field join được với CRM/CIX (customer / conversation) theo contract?
- [ ] Khi thêm collection nguồn mới: đã cập nhật `source_sync_registry` + side-effect + (sau này) registry ingest?

---

## 6. Liên kết

- [KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md](./KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md)
- [NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION.md](./NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION.md)
- [HUONG_DAN_IDENTITY_LINKS.md](./HUONG_DAN_IDENTITY_LINKS.md)

---

*Phiên bản 1.0 — 2026-04-02. Cập nhật khi chốt scope đa nguồn đơn hoặc khi đổi tên collection canonical.*
