# Cơ Cấu Module — AID Trung Tâm & Queue Theo Miền

**Mục đích:** Chốt cách tổ chức module trong monorepo backend theo hướng **event-driven**: một bus điều phối thuộc AI Decision (`decision_events_queue`), mỗi miền nghiệp vụ giữ **queue/job và worker riêng** khi cần. Tài liệu dùng khi thêm module, thêm luồng datachanged, hoặc refactor đăng ký consumer.

**Phạm vi:** `api/internal/api/*`, `api/internal/worker/*`, liên quan `events.OnDataChanged` và consumer AID.

**Liên quan:** [backend-module-map.md](backend-module-map.md), [NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION.md](../05-development/NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION.md), [KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md](../05-development/KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md).

---

## 1. Nguyên tắc chốt (ba lớp trách nhiệm)

1. **Miền nghiệp vụ** (`api/internal/api/<module>/`) — sở hữu model, service/CRUD, **queue và worker của chính miền** (collection job, batch, retry), và **phát event handoff** sau khi worker xong (payload theo data contract).
2. **Điều phối (AID)** — package `aidecision` sở hữu **`decision_events_queue`**, đăng ký **`RegisterAIDecisionOnDataChanged`**, chạy **`applyDatachangedSideEffects`** (một cửa side-effect sau datachanged) và **`dispatchConsumerEvent`**; **không** là nơi chứa logic merge L1→L2 hay tính intel nặng của từng miền.
3. **Nền tảng** — `base`, `events`, `global`, `database`, `models/mongodb`, `dto`, `middleware`, `router` — không gắn một bounded context nghiệp vụ cụ thể.

**Worker thực thi nặng** nằm tại `api/internal/worker/*.go` nhưng trong tài liệu và review PR cần ghi rõ **module sở hữu** (owner) tương ứng để không lạc ranh giới.

---

## 2. Sơ đồ quan hệ (tóm tắt)

```
Ingress (B) → ghi L1 Mongo → EmitDataChanged
    → hook AID (C) → decision_events_queue [EventSource=datachanged]
        → consumer: applyDatachangedSideEffects → enqueue queue miền (D)
        → consumer: dispatchConsumerEvent → handler theo eventType
Domain worker (D) → persist L2 / intel / side-effect
    → emit handoff → decision_events_queue [EventSource ≠ datachanged]
        → AID (C) → orchestrate / propose / execute (E)
```

---

## 3. Nhóm A — Nền tảng & hợp đồng chung

| Vai trò | Package | Ghi chú |
|--------|---------|---------|
| CRUD/sync chung, EmitDataChanged | `base/` | `DoSyncUpsert`, `BaseServiceMongoImpl` |
| Bus in-process, OnDataChanged | `events/` | Handler toàn cục; không business |
| Route tổng, CRUD config | `router/` | Mount từng module HTTP |
| Model/DTO dùng chéo | `models/mongodb/`, `dto/` | Theo quy ước đặt tên file |
| Khởi tạo wiring | `initsvc/` | Tuỳ triển khai server |

---

## 4. Nhóm B — Ingress & kênh ngoài (P0–P1, đẩy vào L1)

| Module | Router (nếu có) | Trách nhiệm trong cơ cấu |
|--------|------------------|---------------------------|
| **cio** | `cio/router/routes.go` | Hub đa kênh, điều phối ingest có kiểm soát |
| **pc** | `pc/router/routes.go` | Pancake POS/Pages → L1 |
| **fb** | `fb/router/routes.go` | Facebook → L1 |
| **meta** | `meta/router/routes.go` | Meta Ads entity / insight → L1; tiền đề pipeline Ads |
| **webhook** | `webhook/router/routes.go` | HTTP ngoài → thường chuyển tiếp sync/ingress |

**Chốt:** Nhóm B **ghi Mongo (L1)** và bảo đảm **`EmitDataChanged`** đúng sau persist; không mở nhánh intel nặng bypass AID trừ khi đã có quy ước và policy rõ (bulk, admin, v.v.).

---

## 5. Nhóm C — Trung tâm điều phối (AID)

| Thành phần | Đường dẫn chính | Chốt |
|------------|-----------------|------|
| HTTP API, trace, execute | `aidecision/router`, `handler`, `service` | API surface AID |
| Cổng datachanged → queue | `aidecision/hooks/` | `RegisterAIDecisionOnDataChanged`, `source_sync_registry`, lọc emit |
| Policy defer / intake | `aidecision/eventintake/` | Defer side-effect, dedupe, rule |
| Consumer | `aidecision/worker/` | `processEvent`, `applyDatachangedSideEffects`, `dispatchConsumerEvent` |
| Adapter CRM ↔ bus | `aidecision/crmqueue/` | Event type / payload gắn luồng CRM trên queue AID |

**Chốt:** **`decision_events_queue` thuộc nhóm C.** Đây là **bus điều phối**, không thay cho **queue job nặng** của miền (merge, intel compute, …).

---

## 6. Nhóm D — Miền nghiệp vụ & queue/worker riêng

Mỗi dòng là một **bounded context**; mở rộng feature ưu tiên giữ logic trong package này và chỉ **enqueue / emit** sang AID hoặc sang worker khác.

| Module | Vai trò | Queue / worker điển hình (tham chiếu code) |
|--------|---------|---------------------------------------------|
| **crm** | Khách L2, merge L1→L2, bulk, intel CRM | `crm_pending_merge`, `crm_intel_compute`, … |
| **order** | Đơn, đồng bộ canonical commerce | Datachanged qua `order/datachanged`; phối hợp orderintel |
| **orderintel** | Intelligence đơn | `order_intel_compute` |
| **meta** | Ads profile, enqueue intel Meta | `ads_intel_compute`, debounce campaign, … |
| **ads** | API/rule phía Ads | Phối hợp pipeline với meta |
| **conversationintel** | Intel hội thoại (CIX) | `cix_intel_compute`, package `conversationintel/datachanged` |
| **cix** | API / lớp CIX theo thiết kế | Phối hợp CIO → CIX → AID |
| **conversation** | Mirror / messaging (đang hình thành) | Khi ổn định: datachanged + queue tuỳ thiết kế |
| **report** | Snapshot, dirty Redis | `report_redis_touch` / worker flush |
| **notification** | Kênh, template, trigger | Worker cleanup / command theo thiết kế |

**Skeleton khuyến nghị trong mỗi module D (khi phát sinh luồng mới):**

- `handler/`, `service/`, `router/` (nếu có HTTP)
- `models/` hoặc dùng `models/mongodb/`
- **`datachanged/`** — chỉ **enqueue job miền** hoặc **gọi service mỏng**; không đóng vai trò orchestrator toàn hệ

---

## 7. Nhóm E — Thực thi, học, rule

| Module | Vai trò |
|--------|---------|
| **executor** | Approval gate + execution |
| **approval** | Engine phê duyệt (nếu tách khỏi executor) |
| **delivery** | Gửi / thực thi kênh |
| **learning** | Learning cases |
| **ruleintel** | Rule engine, run, trace |
| **agent** | Agent registry, check-in |

**Chốt:** Nhóm E **tiêu thụ** kết quả upstream (case, propose); không đảm nhiệm ghi L1 ingress.

---

## 8. Nhóm F — Nội dung & AI generic

| Module | Vai trò |
|--------|---------|
| **content** | Draft, publish, media |
| **ai** | Workflow AI generic |
| **cta** | CTA library |

---

## 9. Gói đặc biệt cần nêu rõ

| Package | Ghi chú |
|---------|---------|
| **`decision/`** | Tồn tại song song `aidecision/` — khi sửa luồng quyết định, ưu tiên **`aidecision`**; tránh nhân đôi “trung tâm”. **Phase 6** (mục 12): dọn ranh giới và deprecate dần legacy trong `decision/`. |
| **`auth`** | Xác thực / org — không thuộc pipeline ingress→intel nhưng mọi request đi qua. |

---

## 10. Bảng `EventSource` (chốt khởi đầu — mở rộng có changelog)

Giá trị dùng trên envelope queue; **không** tự thêm string tùy tiện ngoài bảng khi phát event mới (bổ sung qua PR + cập nhật bảng này).

**Hằng số Go (một nguồn):** `api/internal/api/aidecision/eventtypes/eventsources.go` — ưu tiên dùng `eventtypes.EventSource*` trong code thay vì literal.

**Đăng ký consumer theo `eventType`:** `api/internal/api/aidecision/consumerreg` — `Register` / `Lookup`; `worker` (`consumer_register_*.go` + `worker.aidecision.consumer_dispatch`) và blank import theo miền (vd. `orderintel/aidecisionsubscribe`).

**Side-effect sau `datachanged`:** `api/internal/api/aidecision/datachangedsidefx` — `ApplyContext`, `Register`, `Run`. Contributor CRM merge / báo cáo / Meta / CIX / Order intel / defer CRM refresh đăng ký từ từng miền; **`ApplyContext` không chứa `*AIDecisionService`** để tránh vòng import với `aidecision/service`.

| `EventSource` | Ý nghĩa |
|-----------------|--------|
| `datachanged` | Đã persist; payload tối giản; consumer hydrate từ Mongo |
| `aidecision` | Điều phối nội bộ (orchestrate, context_requested, execute_requested, …) |
| `cix_api` | HTTP CIX đưa yêu cầu vào queue |
| `crm` | Handoff / job CRM (intel compute, context_ready, …) |
| `crm_merge_queue` | Sau merge L1→L2 — payload gắn job merge (vd. `pendingMergeJobId`) |
| `crm_intel` | Worker CRM intel phát (vd. `crm_intel_recomputed`) |
| `meta_ads_intel` | Worker intel Meta / emit campaign sau recompute |
| `meta_api` | API / batch Meta (vd. `ads.intelligence.recalculate_all_requested`) |
| `meta_hooks` | Hook nguồn Meta (recompute không full) |
| `order_intel` | Worker order intelligence phát (vd. `order_intel_recomputed`) |
| `cix_intel` | Worker CIX phát (vd. `cix_intel_recomputed`) |
| `debounce` | Gom batch tin nhắn → `message.batch_ready` |
| `bulk` / `admin` | (Tuỳ chọn) ingress vận hành — policy khác `live` |

**Quy tắc đặt tên:** `EventSource` = **kênh hoặc đơn vị phát**; `eventType` = **nghiệp vụ**. Tránh lấy tên collection làm `EventSource`.

---

## 11. Checklist khi thêm module hoặc luồng mới

- [ ] Module thuộc nhóm nào (B–F)?
- [ ] Có cần **queue/worker riêng** không? Nếu có: collection job, idempotency, owner worker file trong `internal/worker/`.
- [ ] Collection L1 có trong **source sync registry** / emit filter không?
- [ ] Side-effect sau datachanged: đăng ký qua **`datachangedsidefx.Register`** trong package miền (`*/datachanged/sidefx_register.go`); worker chỉ build **`ApplyContext`** + **`Run`** (`applyDatachangedSideEffects`).
- [ ] Handoff sau worker: **`EventSource`** và **`eventType`** đã có trong bảng mục 10?
- [ ] Đã đọc [NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION.md](../05-development/NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION.md) nếu chạm CRUD / datachanged?
- [ ] Queue miền thứ hai trở đi: có dùng lại **debounce/coalesce chung** (Phase 4) thay vì copy logic?
- [ ] Vận hành: metric `no_handler`, DLQ/retry job miền, bulk/outbox (Phase 5) đã được xem xét?

---

## 12. Lộ trình chuẩn hóa — Phase 4–6 (tiếp theo)

Các phase 1–3 (consumerreg, `datachangedsidefx`, `datachangedrouting` + YAML, `datachangedemit`, tách đăng ký consumer) đã ghi trong changelog. Dưới đây là **các bước kế tiếp** (chưa triển khai hết — dùng làm north star khi thêm queue miền và vận hành).

### Phase 4 — Util debounce / coalesce dùng chung

- **Đã có (in-process, trailing):** `api/internal/queuedebounce` — `Table[K]` (chỉ deadline) và `MetaTable[K,M]` (deadline + gộp metadata qua `MergeMeta`). Đang dùng trong `aidecision/eventintake`: `datachanged_defer` (năm kênh defer) và `crm_intel_after_ingest_defer` (org+unifiedId + trace).
- Mục tiêu tiếp: mở rộng cho miền khác / **persist** (Redis, Mongo TTL) khi cần chạy nhiều replica — API util giữ trung lập domain.
- Tiêu chí mở rộng: thêm miền thứ ba trở lên dùng cùng contract; env mới (nếu có) ghi trong [WORKER_CONFIG_ENV_VARS.md](../05-development/WORKER_CONFIG_ENV_VARS.md).

### Phase 5 — Vận hành

1. **Quan sát:** metric / log có cấu trúc cho **`no_handler`** trên consumer AID (event type chưa đăng ký) — hỗ trợ phát hiện lệch deploy hoặc thiếu `Register`.
2. **Độ tin cậy job miền:** **DLQ** (hoặc collection dead-letter) + **retry** có backoff / idempotency rõ cho worker queue từng miền (không gộp vào một ô duy nhất nếu policy khác nhau).
3. **Bulk / ingress lớn:** sau khi ổn định live path — **outbox** hoặc **reconcile** (ghi intent → worker xử lý, hoặc job quét bù) để tránh mất sự kiện và tránh spike trực tiếp lên consumer AID.

### Phase 6 — Dọn `decision/` so với `aidecision/`

- Rà soát import, API HTTP, và luồng cũ trong `api/internal/api/decision/` so với `aidecision/`.
- Chốt: route/handler nào **deprecated**, cái nào **proxy** sang AID, cái nào **xóa** sau migration.
- Mục tiêu cuối: một **trung tâm quyết định** rõ (`aidecision`), `decision/` chỉ còn shim tạm hoặc được gỡ hẳn — cập nhật [backend-module-map.md](backend-module-map.md) và checklist mục 11 khi hoàn tất từng bước.

---

## Changelog

- **2026-04-06 (Phase 4 util):** Thêm `api/internal/queuedebounce`; refactor `eventintake` defer side-effect + CRM intel sau ingest sang `Table` / `MetaTable`.
- **2026-04-06 (roadmap Phase 4–6):** Thêm mục 12 — util debounce/coalesce chung; vận hành (metric `no_handler`, DLQ/retry miền, outbox/reconcile bulk); dọn `decision/` vs `aidecision/`. Cập nhật mục 9–10 và checklist mục 11.
- **2026-04-06 (datachangedrouting YAML):** `routing.default.yaml` embed + env `DATACHANGED_ROUTING_CONFIG`; `collection_overrides` ghi đè pipeline + contributor đọc `routecontract.Decision` trên `ApplyContext.Route`; ví dụ `api/config/datachanged_routing.example.yaml`.
- **2026-04-06 (datachangedrouting):** Package `datachangedrouting` (`Resolve` + `LogApplied` + YAML overrides), `v1-2026-04-06`; emit queue: `datachangedemit.DefaultShouldEmitToDecisionQueue` trong `resolveBase` + YAML; hook L2: `ShouldEmit` = YAML hoặc default; env `AI_DECISION_DATACHANGED_ROUTING_LOG` (xem WORKER_CONFIG_ENV_VARS).
- **2026-04-06 (datachangedsidefx):** Registry side-effect sau datachanged — package `datachangedsidefx`, `sidefx_register.go` theo miền (CRM merge, report, meta ads profile, CIX, order intel) + defer CRM refresh trong `crm_refresh_register.go`; worker `applyDatachangedSideEffects` chỉ tính policy/defer và `Run`.
- **2026-04-06 (bổ sung):** Đồng bộ code — `eventtypes/eventsources.go`, package `consumerreg`, cập nhật bảng `EventSource` + `meta_hooks` / `meta_api` / `cix_api` / `crm_intel` / `order_intel`.
- **2026-04-06:** Khởi tạo — chốt nhóm module A–F, quan hệ AID vs queue miền, bảng `EventSource` khởi đầu, checklist.
