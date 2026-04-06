# Nguyên tắc luồng CRUD → DataChanged → AI Decision

Tài liệu **bắt buộc** đọc trước khi sửa luồng đồng bộ nguồn, hook, queue `decision_events_queue`, hoặc side-effect CRM / báo cáo / Ads sau thay đổi Mongo. Mục tiêu: **không phá luồng** — một cửa vào AI Decision từ thay đổi dữ liệu nguồn.

**Hai luồng vận hành (khái niệm):** **§1.1** — *data change* (persistence) vs *intelligence handoff* (sau worker domain); không nhầm `eventType` / `EventSource` giữa hai nhánh.

**Vision tổng quan:** [08 - ai-decision.md](../../docs-shared/architecture/vision/08%20-%20ai-decision.md) — **§17** tóm tắt lớp 1 / lớp 2; tài liệu **này** là bản đầy đủ + file code + checklist (**§8** = lý do không lặp so `updated_at` ở lớp 2).

---

## 1. Sơ đồ chuẩn (tóm tắt)

```
Đồng bộ ngoài / CRUD Base
    → Lớp 1: DoSyncUpsert (tuỳ domain) — giảm ghi DB theo updated_at nguồn
    → Ghi Mongo thành công
    → events.EmitDataChanged (BaseServiceMongoImpl hoặc gọi thủ công có lý do)
    → CHỈ MỘT handler: RegisterAIDecisionOnDataChanged
    → Lớp 2 hook: cổng (org, registry, không delete) → decSvc.EmitEvent
    → decision_events_queue (EventSource = "datachanged")
    → AIDecisionConsumer: applyDatachangedSideEffects (một cửa: hydrate + ingest + **Redis touch báo cáo** + ads + refresh metrics)
    → dispatchConsumerEvent (orchestrate case / domain tiếp theo)
```

### 1.1 Hai luồng event — *data change* vs *intelligence handoff*

Hai luồng **không thay thế** lớp 1 / lớp 2 (CIO vs cổng hook) ở trên; đây là **cách nhóm hành vi** trên `decision_events_queue` sau khi đã có event.

#### Luồng A — Thay đổi dữ liệu nguồn (persistence / “data change”)

**Ý nghĩa:** Có ghi **Mongo** trên collection nguồn đồng bộ → `EmitDataChanged` → (nếu qua cửa) event với **`EventSource = "datachanged"`** và `eventType` dạng `<prefix>.inserted|.updated` theo `source_sync_registry`.

**Cấu hình collection nào thực sự ghi queue:** registry đầy đủ tại `source_sync_registry.go`, nhưng **emit** có thể bị lọc — `ShouldEmitDatachangedToDecisionQueue` (`datachanged_emit_filter.go`):

- **YAML** `emit_to_decision_queue` trong `datachangedrouting` (embed hoặc `DATACHANGED_ROUTING_CONFIG`) nếu khai báo cho collection → ghi đè bước dưới.
- **Mặc định code:** map `datachangedemit.EmitPerCollection` (`api/internal/api/aidecision/datachangedemit/emit_policy.go`); alias `hooks.DatachangedEmitPerCollection` — key có trong map → `true`/`false`; collection **không** có key: non-Meta → emit; nhóm Meta Marketing → **chỉ `meta_ad_insights`** emit (cố định trong code).
- Các collection **không** thuộc nhóm Meta đó (ví dụ `pc_pos_orders`, `fb_conversations`) **không** bị chế độ Meta áp — vẫn có thể là datachanged bình thường nếu trong registry.

**Sau khi vào queue:** consumer chạy **`applyDatachangedSideEffects`** (một cửa): CRM ingest, báo cáo Redis, **giao tính lại intelligence** xuống domain (enqueue job / debounce tùy miền — Ads có debounce campaign, CRM có defer/dedupe policy, hội thoại có debounce `message.batch_ready` nếu bật env, …). **Không** coi đây là “đã có intelligence mới” — thường là **chuẩn bị** hoặc **yêu cầu** tính lại.

**Ví dụ `pc_pos_orders` (Luồng 1 đơn POS):** cùng một lần `applyDatachangedSideEffects` — **Customer intelligence** qua `emitCrmIntelligenceRefreshAfterDatachanged` (sau ingest / trừ khi defer refresh), **Ads intelligence** qua `ProcessDataChangeForAdsProfile` khi có `ad_id`, **Order intelligence** qua `EnqueueOrderIntelligenceFromParent` (**bắt buộc tại apply**) để routing noop vẫn có job `order_intel_compute`; dispatch `order.inserted|updated` vẫn orchestrate case (upsert job idempotent). Hydrate `customerId` / `orderUid` / `conversationId` từ `posData`: `service.aidecision.source_hydrate.go`.

**Ví dụ `fb_message_items` (Luồng 1 CIX):** trong `applyDatachangedSideEffects` — **`conversationintel.EnqueueCixIntelComputeFromDatachanged`** xếp thẳng job **`cix_intel_compute`** (không qua `conversation.intelligence_requested`; package `conversationintel` chỉ còn enqueue). Worker domain poll `cix_intel_compute` → `AnalyzeSession` → ghi `cix_analysis_results` → emit **`cix_intel_recomputed`** (payload **`analysisResultId`**) → consumer gọi **`ReceiveCixPayload`** (fan-in case). Collection **`cix_analysis_results`** không emit datachanged vào decision queue (map per-collection = `false`).

**Gấp (khẩn cấp) — không chờ debounce / trì hoãn khi đủ điều kiện:** Luồng 1 **bắt buộc** có lớp “ưu tiên realtime” để không chặn SLA bởi gom batch. Hiện backend triển khai **theo từng kênh** (bổ sung miền mới thì phải nối tương tự):

| Kênh | Hành vi khi **gấp** | File / ghi chú |
|------|---------------------|----------------|
| **Ads Intelligence recompute** (debounce theo campaign) | `Urgent=true` → `nextEmitAt = now`, emit `ads.intelligence.recompute_requested` **ngay** (không chờ min interval / trailing). **Chỉ** từ **`meta_ad_insights`** (Luồng 1 nhóm Ads). Đơn POS / `fb_conversations` nếu xếp job qua `ProcessDataChangeForAdsProfile` thì **luôn** `Urgent=false` (debounce bình thường) — không coi là cùng “Luồng 1 Ads” với insight. | `meta/hooks/hooks.ads_debouncer.go`, `hooks.ads_profile.go`, `insight_urgency.go` — `IsUrgentMetaInsightDataChange` (cờ `metaData` + env `ADS_INTEL_INSIGHT_URGENT_*`). |
| **Debounce tin nhắn** (`AI_DECISION_DEBOUNCE_ENABLED=true`) | Nội dung khớp **critical pattern** → `shouldFlushImmediate`, gọi orchestrate **ngay** (không chờ window 30s). | `aidecision/service/service.aidecision.debounce.go` — `criticalPatterns`. |
| **Side-effect CRM ingest / báo cáo / refresh metrics** (trailing defer) | Mức **`UrgencyRealtime`** → `DeferWindowFor` = **0** (chạy ngay, không `ScheduleDeferredSideEffect`). Ghi đè payload: `immediateSideEffects` \| `forceImmediateSideEffects` \| `urgentSideEffects` = true → luôn Realtime. | `aidecision/eventintake/datachanged_business.go`, `datachanged_defer.go`; rule JS (khi bật) có thể chỉnh số giây — xem `side_effect_policy_rule.go` / seed rule. |

**Lưu ý:** Dedupe CRM sau queue (`eventintake` policy env) vẫn có thể áp dụng **sau** khi event đã vào queue — không thay thế “gấp” ở tầng debounce recompute Ads hay defer side-effect; khi mở rộng dedupe cần tránh làm chậm đường gấp đã cam kết.

#### Luồng B — Sau khi domain đã tính intelligence (handoff / “intelligence change”)

**Ý nghĩa:** Worker hoặc svc **miền** đã cập nhật snapshot / payload intelligence (hoặc sẵn sàng đóng gói context) rồi **ghi lại** `decision_events_queue`. **`EventSource` không phải `"datachanged"`** — phải phản ánh **ai phát event** (đọc code: `service.aidecision.emit_cix.go` — quy ước tóm tắt trong comment package).

Ví dụ (không đủ liệt kê toàn bộ):

| Vai trò | `EventSource` điển hình | Ghi chú |
|--------|-------------------------|--------|
| Điều phối nội bộ AI Decision (orchestrate / consumer) | `aidecision` | `ads.context_requested`, `customer.context_requested`, `cix.analysis_requested` (từ orchestrate), … |
| HTTP CIX đưa việc vào queue | `cix_api` | `cix.analysis_requested` từ handler (khác với chuỗi nội bộ) |
| Worker CIX (sau AnalyzeSession) | `cix_intel` | **`cix_intel_recomputed`** — payload có **`analysisResultId`** → consumer **`ReceiveCixPayload`** (thay luồng datachanged `cix_analysis_result.*` / `cix.analysis_completed`) |
| Worker / queue CRM | `crm` | `crm.intelligence.compute_requested`, `customer.context_ready`, **`crm_intel_recomputed`**, … |
| Worker Order Intelligence | `orderintel` | **`order_intel_recomputed`** (một event sau worker — thay `order.flags_emitted` / `commerce.order_completed`) |
| Worker Meta Ads Intelligence | `meta_ads_intel` | **`campaign_intel_recomputed`** (sau recompute — tên **tường minh**, không dùng `meta_campaign.updated` cho nhánh này), `ads.context_ready`, … |
| Hook debounce tin | `debounce` | `message.batch_ready` |

**Luồng B** là nơi AI Decision chạy **khung chung** (routing, case, orchestrate, CIX, propose/execute, …) và có thể **phát thêm** event “xin” context (`*.context_requested`) — trace/correlation nối dài xem **§9**.

**Ads (tóm tắt):** (1) **Luồng 1 “Ads” đúng nghĩa:** datachanged **`meta_ad_insights`** (vào queue khi qua filter L2) → có thể **urgent** → `ads.intelligence.recompute_requested` → … (2) Datachanged **đơn POS / hội thoại** có `ad_id` / `ad_ids` có thể **gọi chung** `ProcessDataChangeForAdsProfile` để xếp recompute (attribution) nhưng **không** là emit insight, **không** urgent insight — chỉ debounce campaign. Sau đó chung: worker → **`campaign_intel_recomputed`** → `ads.context_*` → propose/executor. Chi tiết: `service.aidecision.ads_pipeline.go`, `worker.aidecision.consumer_dispatch.go`.

---

## 2. Nguyên tắc Lớp 1 và Lớp 2 (tách biệt, bắt buộc)

Hai lớp trả lời **hai câu hỏi khác nhau**; trộn vai trò sẽ làm tăng ghi DB oan hoặc tăng tải queue oan.

### 2.0. Phân bổ trách nhiệm theo vision: CIO vs AI Decision

Theo [04 - cio.md](../../docs-shared/architecture/vision/04%20-%20cio.md) và [08 - ai-decision.md](../../docs-shared/architecture/vision/08%20-%20ai-decision.md) **§17**:

| Lớp | Thuộc **logic / trách nhiệm** (vision) | Gói code thực tế (backend) | Ghi chú |
|-----|----------------------------------------|----------------------------|---------|
| **Lớp 1** | **CIO (ingest / persist an toàn)** — đảm bảo khi sync từ ngoài chỉ ghi Mongo khi bản tin nguồn **mới hơn** theo `updated_at` trong blob nguồn. | `api/internal/api/base/service/sync_upsert_helper.go` — dùng chung bởi **pc**, **fb**, … | CIO **không** bắt buộc là một package tên `cio`; đây là **nghiệp vụ hub ingest** theo vision. |
| **Lớp 2** | **AI Decision (cổng vào queue)** — sau `EmitDataChanged`, **enqueue** `decision_events_queue` nếu qua cửa org + collection đăng ký; **không** lặp so `updated_at` nguồn (đã thuộc lớp 1). | `api/internal/api/aidecision/hooks/datachanged.go` | **Không** gán lớp 2 cho CIO: CIO **không consume** queue — trái ranh giới §2 CIO doc. |

**Tóm tắt:** Lớp 1 = **trách nhiệm ingest (CIO)**; Lớp 2 = **trách nhiệm cổng event (AI Decision)**.

| | **Lớp 1** | **Lớp 2** |
|---|-----------|-----------|
| **Câu hỏi** | Có **cần ghi** bản ghi vào Mongo lần này không? | Sau khi đã ghi, có **cần tạo event** vào `decision_events_queue` không? |
| **Đơn vị “sự thật”** | Thời điểm / phiên bản từ **hệ thống ngoài** (`posData.updated_at`, `panCakeData.updated_at`, …). | **Không** có proxy thứ hai: mỗi lần `EmitDataChanged` hợp lệ qua cửa hook → enqueue (trừ delete / thiếu org / ngoài registry). |
| **Vị trí chạy** | Trước khi quyết định ghi: **`DoSyncUpsert`**. | Sau `EmitDataChanged`, trong hook: **`RegisterAIDecisionOnDataChanged`**. |
| **Khi “chặn”** | **Không ghi DB** → không có `EmitDataChanged` từ sync path đó. | **Không Insert queue** khi thiếu org, collection không đăng ký, hoặc **delete** (policy hiện tại). |

### 2.1. Lớp 1 — Cổng ghi Mongo (sync-upsert), trách nhiệm ingest (CIO)

**Mục tiêu:** Job đồng bộ từ API ngoài (Pancake POS, Pancake FB, …) **không ghi đè** Mongo khi bản tin đến **cũ hơn hoặc bằng** bản đã lưu (theo `updated_at` **trong** `posData` / `panCakeData`).

**Nguyên tắc:**

1. **Bắt buộc** dùng **`DoSyncUpsert`** (hoặc service bọc `SyncUpsertOne` gọi nó) cho luồng sync có payload nguồn kèm `updated_at`.
2. So sánh thời gian **ở đúng chỗ nguồn** (cùng field map với extract `posUpdatedAt` / `panCakeUpdatedAt`), không thay bằng so sánh chỉ `updatedAt` root (thường là thời điểm **đồng bộ server**).
3. Nếu incoming **không** có `updated_at` hợp lệ (`newUpdatedAt == 0`), logic có thể **vẫn ghi** (theo contract `BuildSyncUpsertFilter`) — thiết kế API sync phải rõ ràng.
4. Giảm tải **queue** sau khi đã ghi: ưu tiên **lớp 1** (bớt ghi oan) và tránh CRUD ghi trùng; hook **không** so lại `updated_at` nguồn để tránh trùng trách nhiệm với lớp 1.

**Code:** `api/internal/api/base/service/sync_upsert_helper.go` — `DoSyncUpsert`, `BuildSyncUpsertFilter`.

**Sai lầm thường gặp:** Bỏ qua `DoSyncUpsert`, dùng `Upsert` / `ReplaceOne` thẳng → mỗi lần job chạy đều ghi Mongo → `EmitDataChanged` mỗi lần → tải hệ thống tăng; **lớp 2 không cứu được** nếu mỗi lần `updated_at` nguồn vẫn đổi từ phía API.

### 2.2. Lớp 2 — Cổng emit queue (hook), trách nhiệm AI Decision

**Mục tiêu:** Sau khi Mongo **đã** có thay đổi và `EmitDataChanged` chạy, **định tuyến** vào `decision_events_queue` theo **một** handler — **không** lặp lại logic so `updated_at` trong `posData` / `panCakeData` (đã là **lớp 1**). Ranh giới **persistence → tầng quyết định**, không thuộc CIO.

**Nguyên tắc:**

1. Cửa hook: document non-nil, **không** delete, có `ownerOrganizationID`, collection trong **`source_sync_registry`** → `EmitEvent` queue.
2. **`OpDelete`:** hook **không** tạo event datachanged queue (theo thiết kế hiện tại).
3. **Lớp 2 không** giảm ghi DB và **không** thay lớp 1: nếu sync vẫn ghi thừa, xử tại **`DoSyncUpsert`** / đường CRUD.

**Code:** `api/internal/api/aidecision/hooks/datachanged.go`. File `events/datachanged_merge.go` giữ **`MergeRelevantDataKey`**, **`ExtractUpdatedAtFromDoc`**, **`TimestampFromMap`** phục vụ **CRM pending ingest** (delta), không dùng để skip enqueue ở hook.

**Sai lầm thường gặp:** Dùng lớp 2 để “chặn ghi DB” — **sai**; kỳ vọng hook giảm queue khi đã bỏ lọc `updated_at` — cần **siết lớp 1** và đường ghi Mongo.

### 2.3. Thứ tự vận hành (góc nhìn một request sync)

1. **Lớp 1** quyết định có **ghi Mongo** hay skip.  
2. Nếu có ghi và code đi qua **`EmitDataChanged`**:  
3. **Lớp 2** (hook) quyết định có **`EmitEvent` → `decision_events_queue`** hay không (cửa org/registry/delete, **không** so lại `updated_at` nguồn).  
4. Consumer **`applyDatachangedSideEffects`** quyết định ingest / report / ads / refresh metrics (một cửa).

### 2.4. Policy sau queue (không nhầm với lớp 2)

**`eventintake.EvaluateDatachangedSideEffects`** (vd. dedupe CRM theo `AI_DECISION_EVENTINTAKE_CRM_DEDUPE_SEC`) chạy **sau** khi event **đã** vào queue — là **lớp điều tiết side-effect**, không thay thế định nghĩa lớp 1 / lớp 2 ở trên.

---

## 3. Quy tắc bắt buộc — Hook & `OnDataChanged`

1. **Chỉ một đăng ký** `events.OnDataChanged` trong toàn app — tại `api/cmd/server/init.registry.go` → `aidecisionhooks.RegisterAIDecisionOnDataChanged`.
2. **Cấm** thêm `OnDataChanged(...)` trong module khác để gọi CRM, Report, Ads, hoặc ghi queue khác song song.
3. Handler hook **chỉ** được:
   - Lọc điều kiện (org, registry collection, delete).
   - Gọi `AIDecisionService.EmitEvent` → **InsertOne** vào `decision_events_queue`.
4. **Cấm** trong hook: `EnqueueCrmPendingMerge`, `RefreshMetrics`, ghi **MarkDirty** báo cáo / gọi Redis touch trực tiếp, `ProcessDataChangeForAdsProfile`, gọi trực tiếp worker domain khác. (Sau queue, báo cáo chỉ qua `RecordReportTouchFromDataChange` trong `applyDatachangedSideEffects`.)

**File:** `api/internal/api/aidecision/hooks/datachanged.go`  
**Registry collection → prefix event:** `api/internal/api/aidecision/hooks/source_sync_registry.go`

---

## 4. Quy tắc bắt buộc — Consumer AI Decision (sau queue)

1. Mọi side-effect từ **`eventSource == "datachanged"`** (ingest CRM pending, **touch báo cáo trên Redis** `ff:rt:*`, Ads profile debounce, xếp `crm.intelligence.compute_requested`) **chỉ** chạy trong **`applyDatachangedSideEffects`** — `api/internal/api/aidecision/worker/worker.aidecision.datachanged_side_effects.go`. **MarkDirty** vào `report_dirty_periods` **không** gọi tại đây — do worker **`report_redis_touch_flush`** (`FlushReportTouchesFromRedis`) sau khi quét Redis.
2. **Cấm** gọi lại các side-effect đó từ `OrchestrateConversationSourceEvent` / `OrchestrateOrderSourceEvent` hoặc handler dispatch trùng ý nghĩa (đã gom về một cửa).
3. Thứ tự xử lý event trong consumer: `processEvent` → nếu datachanged thì **`applyDatachangedSideEffects(ctx, svc, evt)`** → routing rule → **`dispatchConsumerEvent`**.

**Policy dedupe CRM (tùy env):** `api/internal/api/aidecision/eventintake/policy.go`

---

## 5. CRUD & `EmitDataChanged`

1. **Ưu tiên** mọi thao tác ghi qua **`BaseServiceMongoImpl`** để tự **`EmitDataChanged`** (Insert / UpdateOne / UpdateById / Upsert / UpdateMany khi doc đổi thật, …).
2. **Nếu bắt buộc** ghi trực tiếp `collection.UpdateOne` / `ReplaceOne` / …:
   - Phải **`events.EmitDataChanged`** sau khi ghi thành công (nên kèm `PreviousDocument` khi update — CRM ingest / audit; hook lớp 2 **không** dùng để so `updated_at` nguồn).
3. **Biệt lệ đã biết:**
   - **`OpDelete`:** hook **không** tạo event queue (theo `datachanged.go`).
   - Collection **không** có trong `source_syncPrefixesMap`: có `EmitDataChanged` nhưng **không** vào AI Decision queue.
   - Document thiếu **`ownerOrganizationID`:** hook bỏ qua emit.

---

## 6. Checklist trước khi merge (đừng phá luồng)

- [ ] Có thêm `OnDataChanged` mới không? → **Chỉ được phép nếu đồng thời gỡ/ gộp vào hook AI Decision và được review kiến trúc.**
- [ ] Có gọi `EnqueueCrmPendingMerge` / `RefreshMetrics` / **MarkDirty báo cáo** / Ads **trực tiếp** sau thay đổi collection nguồn (fb_*, pc_pos_*, crm_*, meta_*) không? → **Chuyển vào `applyDatachangedSideEffects` (báo cáo = chỉ `RecordReportTouchFromDataChange` nếu Redis bật) hoặc enqueue qua `decision_events_queue` với contract rõ ràng.**
- [ ] Collection mới cần vào luồng AI Decision? → Thêm vào **`source_sync_registry.go`** + xác nhận model có **`ownerOrganizationID`**; nếu CRM ingest cần mốc nguồn nested, cập nhật **`MergeRelevantDataKey`** / **`ExtractUpdatedAtFromDoc`** trong `events/datachanged_merge.go`.
- [ ] Job sync ngoài mới? → Dùng **`DoSyncUpsert`** (lớp 1), không bỏ qua so `updated_at` nguồn.

---

## 7. File tham chiếu nhanh

| Nội dung | File |
|----------|------|
| Emit queue | `api/internal/api/aidecision/service/service.aidecision.event_queue.go` |
| Phát DataChanged từ CRUD | `api/internal/api/base/service/service.base.mongo.go` |
| Đăng ký hook | `api/cmd/server/init.registry.go` |
| Dispatch event_type → handler | `api/internal/api/aidecision/worker/worker.aidecision.consumer_dispatch.go` |
| Lọc collection → emit datachanged (allowlist / Meta insight-only) | `api/internal/api/aidecision/hooks/datachanged_emit_filter.go` |
| Quy ước `EventSource` (chuỗi xin/trả context) | `api/internal/api/aidecision/service/service.aidecision.emit_cix.go` |
| Ads: `campaign_intel_recomputed` + emit sau recompute | `api/internal/api/meta/service/service.meta.ads_intel_emit.go` |
| Trace envelope + debounce + case | `hooks/datachanged.go`, `service.aidecision.debounce.go`, `service.aidecision.case.go` — **§9** |
| Mốc thời gian nguồn (CRM delta) | `api/internal/api/events/datachanged_merge.go` |
| CRM ingest từ consumer | `api/internal/api/aidecision/crmingest/crmingest.go` |
| Luồng 1 đơn POS → 3 intel (apply) | `api/internal/api/aidecision/worker/worker.aidecision.datachanged_side_effects.go` |
| Redis client (touch báo cáo) | `api/internal/redisclient/client.go` |
| Ghi touch + flush → MarkDirty | `api/internal/api/report/service/service.report.redis_touch.go` |
| Worker flush Redis → dirty periods | `api/internal/worker/report_redis_touch_worker.go` |

---

## 8. Vì sao lớp 2 không so lại `updated_at` nguồn

**Nguyên tắc:** So sánh **`updated_at` trong `posData` / `panCakeData`** để **giảm ghi Mongo** là trách nhiệm **lớp 1** (`DoSyncUpsert`). Lớp 2 **không** lặp lại cùng một proxy (prev vs after trong hook) để skip enqueue — tránh **hai nguồn sự thật** và logic trùng phải đồng bộ khi sửa một bên.

**Hệ quả:** Mỗi lần Base (hoặc đường tương đương) **đã ghi** và phát **`EmitDataChanged`**, nếu qua cửa org + registry và không phải delete → **luôn** có thể vào `decision_events_queue`. Nếu queue dày vì ghi “kỹ thuật” không qua lớp 1, xử lý bằng **siết đường ghi** / **lớp 1**, không bằng hook so `updated_at` lần nữa.

**`events/datachanged_merge.go`:** Vẫn dùng **`MergeRelevantDataKey`** + **`ExtractUpdatedAtFromDoc`** cho **CRM pending ingest** (delta), không dùng để chặn emit ở hook.

**Thứ tự hook hiện tại (tóm tắt):** nil doc → delete → thiếu org → ngoài registry → **`emitUnifiedSourceDataChanged`**.

---

## 9. TraceId / CorrelationId — nối luồng queue → case → CIX → execute (backend)

**Mục tiêu:** Một **cặp** `traceId` (prefix `trace_`) + `correlationId` (prefix `corr_`) trên envelope **`decision_events_queue`** đi xuyên orchestrate, debounce (nếu bật), **decision_cases_runtime**, CIX và **`aidecision.execute_requested`**, trừ các job vận hành cố ý tách trace.

| Bước | Hành vi | File / ghi chú |
|------|---------|------------------|
| **Lớp 2 enqueue** | Mỗi event từ **`emitUnifiedSourceDataChanged`** sinh **`traceId`** + **`correlationId`** mới trước `EmitEvent`. | `api/internal/api/aidecision/hooks/datachanged.go` |
| **Consumer** | Sau lease, nếu envelope thiếu → **`ensureDecisionEventTraceIDs`** bù (bản ghi queue cũ / emit thủ công). Chạy **trước** metrics + Decision Live + `processEvent`. | `api/internal/api/aidecision/worker/worker.aidecision.consumer.go` |
| **Event con** | Orchestrate copy **`evt.TraceID` / `evt.CorrelationID`** xuống `cix.analysis_requested`, `customer.context_requested`, … | `worker.aidecision.orchestrate.go`, … |
| **Debounce** (`AI_DECISION_DEBOUNCE_ENABLED=true`) | **`decision_debounce_state`**: lần **insert** đầu (`$setOnInsert`) lưu `traceId` / `correlationId` từ event đang gom; **`message.batch_ready`** emit lại cùng cặp — không đứt chuỗi so với datachanged. | `service.aidecision.debounce.go`, model `DebounceState` |
| **Decision case** | **`decision_cases_runtime`**: field **`traceId`**, **`correlationId`** — ghi khi **tạo case mới**; merge/reopen chỉ **`$set` khi field đang trống** (giữ neo gốc). | `service.aidecision.case.go`, model `DecisionCase` |
| **CIX** | **`cix_analysis_results.traceId`**: trace **`rule_execution_logs`** của bước **RULE_CIX_ACTIONS** (UUID). **`pipelineRuleTraceIds`**: thứ tự trace các bước rule pipeline (L1 → … → Actions). Fan-in case: event **`cix_intel_recomputed`** + **`analysisResultId`** → **`ReceiveCixPayload`**. | `service.cix.analysis.go`, `intelrecomputed/emit.go`, `worker.aidecision.consumer.go` |
| **Execute** | **`TryExecuteIfReady`**: `traceId` ưu tiên payload CIX, fallback **`case.traceId`**; **`correlationId`** từ **`case.correlationId`**. | `service.aidecision.cix.go` |
| **Propose từ queue** | **`MergeQueueEnvelopeIntoProposePayload`** — bơm `traceId` / `correlationId` / `aidecisionProposeEventId` vào payload nếu thiếu. | `service.aidecision.propose_trace.go` |

**Phạm vi không thuộc mục này:** OpenTelemetry; **`X-Request-ID`** (log HTTP) tách với `traceId` domain; replay **GET/WS theo trace** vẫn chủ yếu **ring RAM** (xem vision **08 §19** / **api-context 4.09**). Job kiểu **`ads.intelligence.recompute_requested`** có thể không mang trace CRUD — consumer vẫn bù ID cho vận hành.

**Hợp đồng dữ liệu:** [unified-data-contract.md](../../docs-shared/architecture/data-contract/unified-data-contract.md) §2.5–2.5b. **Vision:** [08 - ai-decision.md](../../docs-shared/architecture/vision/08%20-%20ai-decision.md) §19.

---

## Changelog tài liệu

- **2026-03-31 (CIX / intel handoff thống nhất):** **§1.1** — `fb_message_items` chỉ enqueue `cix_intel_compute` (bỏ `conversation.intelligence_requested`); Luồng B — `cix_intel_recomputed` + `analysisResultId` → `ReceiveCixPayload`; bỏ consumer `cix_analysis_result.*` / `cix.analysis_completed`; Order → `order_intel_recomputed`; lọc emit datachanged = map `DatachangedEmitPerCollection` + Meta insight-only (bỏ mô tả env allowlist cũ).
- **2026-03-31 (pc_pos_orders → 3 intel):** **§1.1** — chuỗi Order + Ads + Customer intel từ `applyDatachangedSideEffects`; Order intel enqueue tại apply (không phụ thuộc routing noop). Hydrate `conversationId` từ `posData`.
- **2026-03-31 (làm rõ Ads Luồng 1):** **§1.1** — **urgent** bỏ debounce recompute Ads **chỉ** từ `meta_ad_insights`; POS / hội thoại không thuộc Luồng 1 Ads (vẫn có thể xếp job attribution, luôn debounce). Comment `hooks.ads_profile.go`.
- **2026-03-31 (hai luồng + gấp):** **§1.1** — phân biệt Luồng A vs B; cấu hình `datachanged_emit_filter`; Ads `campaign_intel_recomputed`; **bảng “gấp”** (Ads urgent insight, critical pattern debounce tin, Realtime / payload ghi đè side-effect) + bảng file **§7**.
- **2026-03-26 (trace / correlation E2E):** **§9** — hook datachanged sinh cặp trace; consumer bù; debounce `decision_debounce_state` + `message.batch_ready`; case runtime; CIX `pipelineRuleTraceIds`; trỏ **unified-data-contract** §2.5b, vision **08 §19**, **api-context 4.09**.
- **2026-03-24 (báo cáo qua Redis touch):** §1, §3, §4, §6 checklist, §7 — bỏ MarkDirty tức thì từ `datachanged`; `RecordReportTouchFromDataChange` + worker `report_redis_touch_flush`; cấu hình `REDIS_ADDR`, TTL, interval flush — xem `WORKER_CONFIG_ENV_VARS.md`.
- **2026-03-24 (bỏ lọc merge-relevant lớp 2):** §1–§3, §7, **§8** — lớp 2 chỉ cổng enqueue; không `IsMergeRelevantDataUnchanged`; `datachanged_merge` cho CRM delta.
- **2026-03-24 (lớp 2 sâu):** (đã thay bằng mục trên) trước đây mô tả merge-relevant tại hook — **không còn áp dụng**.
- **2026-03-24 (bổ sung 2):** §2.0 — phân bổ **CIO vs AI Decision** (lớp 1 = ingest/CIO, lớp 2 = cổng queue/AI Decision); tiêu đề §2.1 / §2.2.
- **2026-03-24 (bổ sung):** Mở rộng §2 — định nghĩa lớp 1 / lớp 2, merge-relevant, thứ tự vận hành, sai lầm thường gặp, phân biệt `eventintake`.
- **2026-03-24:** Khởi tạo — cố định nguyên tắc lớp 1/2, một `OnDataChanged`, một cửa `applyDatachangedSideEffects`, checklist cấm/giữ.
