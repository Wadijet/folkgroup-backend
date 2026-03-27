# Nguyên tắc luồng CRUD → DataChanged → AI Decision

Tài liệu **bắt buộc** đọc trước khi sửa luồng đồng bộ nguồn, hook, queue `decision_events_queue`, hoặc side-effect CRM / báo cáo / Ads sau thay đổi Mongo. Mục tiêu: **không phá luồng** — một cửa vào AI Decision từ thay đổi dữ liệu nguồn.

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
4. **Cấm** trong hook: `EnqueueCrmIngest`, `RefreshMetrics`, ghi **MarkDirty** báo cáo / gọi Redis touch trực tiếp, `ProcessDataChangeForAdsProfile`, gọi trực tiếp worker domain khác. (Sau queue, báo cáo chỉ qua `RecordReportTouchFromDataChange` trong `applyDatachangedSideEffects`.)

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
- [ ] Có gọi `EnqueueCrmIngest` / `RefreshMetrics` / **MarkDirty báo cáo** / Ads **trực tiếp** sau thay đổi collection nguồn (fb_*, pc_pos_*, crm_*, meta_*) không? → **Chuyển vào `applyDatachangedSideEffects` (báo cáo = chỉ `RecordReportTouchFromDataChange` nếu Redis bật) hoặc enqueue qua `decision_events_queue` với contract rõ ràng.**
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
| Trace envelope + debounce + case | `hooks/datachanged.go`, `service.aidecision.debounce.go`, `service.aidecision.case.go` — **§9** |
| Mốc thời gian nguồn (CRM delta) | `api/internal/api/events/datachanged_merge.go` |
| CRM ingest từ consumer | `api/internal/api/aidecision/crmingest/crmingest.go` |
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
| **CIX** | **`cix_analysis_results.traceId`**: trace **`rule_execution_logs`** của bước **RULE_CIX_ACTIONS** (UUID). **`pipelineRuleTraceIds`**: thứ tự trace các bước rule pipeline (L1 → … → Actions). | `service.cix.analysis.go`, model CIX |
| **Execute** | **`TryExecuteIfReady`**: `traceId` ưu tiên payload CIX, fallback **`case.traceId`**; **`correlationId`** từ **`case.correlationId`**. | `service.aidecision.cix.go` |
| **Propose từ queue** | **`MergeQueueEnvelopeIntoProposePayload`** — bơm `traceId` / `correlationId` / `aidecisionProposeEventId` vào payload nếu thiếu. | `service.aidecision.propose_trace.go` |

**Phạm vi không thuộc mục này:** OpenTelemetry; **`X-Request-ID`** (log HTTP) tách với `traceId` domain; replay **GET/WS theo trace** vẫn chủ yếu **ring RAM** (xem vision **08 §19** / **api-context 4.09**). Job kiểu **`ads.intelligence.recompute_requested`** có thể không mang trace CRUD — consumer vẫn bù ID cho vận hành.

**Hợp đồng dữ liệu:** [unified-data-contract.md](../../docs-shared/architecture/data-contract/unified-data-contract.md) §2.5–2.5b. **Vision:** [08 - ai-decision.md](../../docs-shared/architecture/vision/08%20-%20ai-decision.md) §19.

---

## Changelog tài liệu

- **2026-03-26 (trace / correlation E2E):** **§9** — hook datachanged sinh cặp trace; consumer bù; debounce `decision_debounce_state` + `message.batch_ready`; case runtime; CIX `pipelineRuleTraceIds`; trỏ **unified-data-contract** §2.5b, vision **08 §19**, **api-context 4.09**.
- **2026-03-24 (báo cáo qua Redis touch):** §1, §3, §4, §6 checklist, §7 — bỏ MarkDirty tức thì từ `datachanged`; `RecordReportTouchFromDataChange` + worker `report_redis_touch_flush`; cấu hình `REDIS_ADDR`, TTL, interval flush — xem `WORKER_CONFIG_ENV_VARS.md`.
- **2026-03-24 (bỏ lọc merge-relevant lớp 2):** §1–§3, §7, **§8** — lớp 2 chỉ cổng enqueue; không `IsMergeRelevantDataUnchanged`; `datachanged_merge` cho CRM delta.
- **2026-03-24 (lớp 2 sâu):** (đã thay bằng mục trên) trước đây mô tả merge-relevant tại hook — **không còn áp dụng**.
- **2026-03-24 (bổ sung 2):** §2.0 — phân bổ **CIO vs AI Decision** (lớp 1 = ingest/CIO, lớp 2 = cổng queue/AI Decision); tiêu đề §2.1 / §2.2.
- **2026-03-24 (bổ sung):** Mở rộng §2 — định nghĩa lớp 1 / lớp 2, merge-relevant, thứ tự vận hành, sai lầm thường gặp, phân biệt `eventintake`.
- **2026-03-24:** Khởi tạo — cố định nguyên tắc lớp 1/2, một `OnDataChanged`, một cửa `applyDatachangedSideEffects`, checklist cấm/giữ.
