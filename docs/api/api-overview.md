# API Overview — Folkgroup Backend

**Mục đích:** Tổng quan API surface — endpoint, method, module. Giúp developer nhìn nhanh và Cursor AI hiểu cấu trúc API.

**Canonical chi tiết (một nguồn — `docs-shared` trên workspace / junction):** `docs-shared/ai-context/folkform/api-context.md` (AI Decision live trace: **Version 4.02**). Bổ sung `opsTier`: `docs-shared/ai-context/folkform/api-context-ai-decision-ops-tier.md` (đề xuất **4.09**).

---

## Base URL

- **Local:** `http://localhost:8080/api/v1`
- **Auth:** Bearer JWT (Firebase)

---

## Modules & Endpoints (theo Router)

| Module | Prefix | Mô tả | Handler/Service |
|--------|--------|-------|-----------------|
| **auth** | `/auth` | Đăng nhập, JWT, user, role, organization | auth/ |
| **approval** | `/approval` | Propose, approve, reject, execute | approval/ |
| **decision** | `/decision` | Decision Brain — list/create/find decision cases | decision/ |
| **ads** | `/ads` | Meta Ads, action evaluation, auto propose | ads/ |
| **fb** | `/fb` | Facebook Pages, posts, conversations, messages | fb/ |
| **meta** | `/meta` | Ad-account, campaign, ad-set, ad, ad-insight, activity-history | meta/ |
| **pc** | `/pc` | Pancake Pages, POS | pc/ |
| **webhook** | `/webhook` | Webhook endpoints | webhook/ |
| **report** | `/report` | Definitions, snapshots, dirty periods; API trend/recompute/MarkDirty. **Dirty từ CRUD:** Redis touch (`ff:rt:*`) trong consumer AI Decision → worker `report_redis_touch_flush` → `report_dirty_periods` | report/ |
| **crm** | `/crm` | Customers, CRM pending ingest, bulk jobs, rebuild, recalculate | crm/ |
| **cio** | `/cio` | Ingest hub đa domain qua một endpoint (`/cio/ingest`) | cio/ |
| **notification** | `/notification` | Channels, templates, routing, trigger | notification/ |
| **cta** | `/cta` | CTA Library | cta/ |
| **delivery** | `/delivery` | Delivery send, history | delivery/ |
| **agent** | `/agent-management` | Agent configs, commands, registry, check-in | agent/ |
| **content** | `/content` | Content drafts, publications, videos | content/ |
| **ai** | `/ai` | AI workflows, steps, prompts, provider profiles | ai/ |
| **rule-intelligence** | `/rule-intelligence` | Rule Engine, definition, logic, param-set, output-contract, run, logs | ruleintel/ |
| **ai-decision** | `/ai-decision` | Queue execute (202 + eventId + **traceId**), live trace (timeline + WS), cases | aidecision/ |

---

## CRUD Pattern

Hầu hết collections dùng CRUD chuẩn qua `BaseHandler`:

- `GET /:collection` — Find (filter, pagination)
- `GET /:collection/:id` — FindOneById
- `POST /:collection` — InsertOne
- `PUT /:collection/:id` — UpdateById
- `DELETE /:collection/:id` — DeleteById

Chi tiết: [api-context.md](../../docs-shared/ai-context/folkform/api-context.md)

---

## Manual POS CRUD (L1 nhập tay)

Các endpoint này ghi trực tiếp vào nhóm collection `order_src_manual_*`:

- `GET/POST/PUT/DELETE /manual-pos/order`
- `GET/POST/PUT/DELETE /manual-pos/product`
- `GET/POST/PUT/DELETE /manual-pos/variation`
- `GET/POST/PUT/DELETE /manual-pos/category`
- `GET/POST/PUT/DELETE /manual-pos/customer`
- `GET/POST/PUT/DELETE /manual-pos/shop`
- `GET/POST/PUT/DELETE /manual-pos/warehouse`

Ghi chú: sau khi ghi L1 manual, backend tiếp tục pipeline datachanged để chiếu đơn sang `order_core_records` (L2) và kích hoạt side-effects liên quan.

---

## CIO Ingest Hub

Endpoint chung:

- `POST /cio/ingest`

Domain hỗ trợ manual:

- `manual_order`
- `manual_pos_product`
- `manual_pos_variation`
- `manual_pos_category`
- `manual_pos_customer`
- `manual_pos_shop`
- `manual_pos_warehouse`

Alias ngắn:

- `m_order`, `m_pos_product`, `m_pos_variation`, `m_pos_category`, `m_pos_customer`, `m_pos_shop`, `m_pos_warehouse`

---

## CRM Bulk Jobs

| Method | Path | Mô tả |
|--------|------|-------|
| POST | `/customers/rebuild` | Tạo 2 job: sync + backfill (có checkpoint) |
| POST | `/customers/recalculate-all` | Tạo N job batch (batchSize mặc định 200) |
| POST | `/customers/:unifiedId/recalculate` | Tạo 1 job recalculate_one |
| GET | `/crm-bulk-jobs` | Danh sách job queue |
| GET/PUT | `/crm-bulk-jobs/:id` | Chi tiết, cập nhật job |

Chi tiết: [crm-bulk-jobs.md](crm-bulk-jobs.md)

---

## System Endpoints

| Method | Path | Mô tả |
|--------|------|-------|
| GET | `/system/health` | Kiểm tra tình trạng API và database |
| GET | `/internal/metrics/job-metrics` | Metrics thời gian thực hiện từng loại job (avgMs, countLastHour) |
| GET | `/system/worker-config` | Cấu hình worker (ngưỡng, schedules, pool, retention, state) |
| PUT | `/system/worker-config` | Cập nhật cấu hình worker (runtime, không cần restart) |

Chi tiết: [WORKER_CONFIG_ENV_VARS.md](../05-development/WORKER_CONFIG_ENV_VARS.md)

---

## Response Format

```json
{
  "code": 200,
  "message": "Thành công",
  "data": { ... },
  "status": "success"
}
```

---

## Related Docs

- [Module Map](../module-map/backend-module-map.md)
- [API Context (chi tiết)](../../docs-shared/ai-context/folkform/api-context.md)
- [Kiến trúc](../architecture/overview.md)

## Rule Intelligence Endpoints

| Method | Path | Mô tả |
|--------|------|-------|
| POST | `/rule-intelligence/run` | Chạy rule với context (rule_id, domain, entity_ref, layers, params_override) |
| GET | `/rule-intelligence/logs/:traceId` | Xem rule execution log theo trace_id — link từ proposal "Xem log tạo đề xuất" |
| CRUD | `/rule-intelligence/definition` | Rule definitions |
| CRUD | `/rule-intelligence/logic` | Logic scripts |
| CRUD | `/rule-intelligence/param-set` | Parameter sets |
| CRUD | `/rule-intelligence/output-contract` | Output contracts |

Chi tiết: [02-architecture/core/rule-intelligence](../02-architecture/core/rule-intelligence.md)

---

## Learning Endpoints (Learning cases + Rule suggestions)

| Method | Path | Mô tả |
|--------|------|-------|
| GET | `/learning/cases` | List learning cases (filter: `domain`, `caseType`, `goalCode`, `result`, `targetType`, `targetId`, `sourceRefId`, **`decisionCaseId`**, **`traceId`** (map `executionTraceId`), **`correlationId`**, **`aidecisionProposeEventId`**) |
| GET | `/learning/cases/:id` | Find by ID |
| POST | `/learning/cases` | Create learning case |
| GET | `/learning/rule-suggestions` | List rule suggestions (Phase 3 — filter: domain, goalCode, status) |
| PATCH | `/learning/rule-suggestions/:id` | Cập nhật status (reviewed, applied, dismissed). :id = suggestionId |

Chi tiết: [02-architecture/core/learning-engine](../02-architecture/core/learning-engine.md)

---

## AI Decision (event-driven)

| Method | Path | Mô tả |
|--------|------|--------|
| POST | `/ai-decision/execute` | Enqueue event **`aidecision.execute_requested`**. **HTTP 202**, `data.eventId`, **`data.traceId`** (sinh sẵn nếu body không có `traceId`) — worker `AIDecisionConsumer` gọi `ExecuteWithCase` (không trả quyết định đồng bộ). |
| GET | `/ai-decision/e2e-reference-catalog` | **Catalog pha/bước E2E** (G1–G6) cho UI: `data.stages` (kỹ thuật + `userSummaryVi`), `data.steps` (`descriptionTechnicalVi` + `descriptionUserVi`), `data.queueMilestones` (mốc consumer — neo **G2**, `labelVi` + `userLabelVi`), `data.livePhaseMap`, `data.schemaVersion`. Quyền: `MetaAdAccount.Read` + org. Chi tiết: [bang-pha-buoc-event-e2e §3.1](../flows/bang-pha-buoc-event-e2e.md#31-api-catalog-e2e-json-cho-frontend). |
| GET | `/ai-decision/traces/:traceId/timeline` | **Replay** các sự kiện live đã buffer (JSON `data.events`). Quyền: `MetaAdAccount.Read` + org. |
| GET | `/ai-decision/traces/:traceId/live` | **WebSocket** — gửi replay như timeline, sau đó stream tiếp; mỗi message = một JSON (`phase`, `summary`, `step`, …). Quyền: `MetaAdAccount.Read` + org. |
| GET | `/ai-decision/org-live/timeline` | Replay buffer live **theo org** (mọi trace). |
| GET | `/ai-decision/org-live/metrics` | Snapshot **trung tâm chỉ huy** (`schemaVersion` **2**): nhóm **`meta`**, **`queue.depth`**, **`intake`**, **`publishCounters`** (lũy kế phase/sourceKind), **`realtime.gaugeByPhase`**, **`consumer`**, **`workers`**, `hasRecentConsumerActivity`, `alerts`. Quyền: `MetaAdAccount.Read` + org. Chi tiết: [THIET_KE v1.11](../05-development/THIET_KE_TRUNG_TAM_CHI_HUY_AI_DECISION.md) (Publish 4.5; timeline 4.6; Mongo org-live 4.7). |
| GET | `/ai-decision/org-live` | **WebSocket** — replay org timeline + stream sự kiện; định kỳ gửi thêm message `type: "aggregate"` (cùng payload như GET metrics) cho UI real-time. |
| POST | `/ai-decision/events` | Ingest event vào `decision_events_queue`. Body có thể gửi **`pipelineStage`** (tuỳ chọn); nếu bỏ trống, backend gán **`external_ingest`**. Response `data`: `eventId`, `status`, **`opsTier`**, **`opsTierLabelVi`** — phân loại vận hành theo `eventType` (cùng logic feed live; chi tiết mục **4.4** trong [THIET_KE_TRUNG_TAM_CHI_HUY_AI_DECISION.md](../05-development/THIET_KE_TRUNG_TAM_CHI_HUY_AI_DECISION.md)). Bảng giá trị `pipelineStage` và Mongo field: [co-cau-module-aid-va-domain-queue.md](../module-map/co-cau-module-aid-va-domain-queue.md) mục 11. |
| POST | `/ai-decision/cases/:decisionCaseId/close` | Đóng decision case runtime |

**`pipelineStage` (Mongo `decision_events_queue.pipelineStage`):** giai đoạn trong khung tổng (khác `eventSource` = kênh phát, `eventType` = loại nghiệp vụ). Định nghĩa trong code: `api/internal/api/aidecision/eventtypes/pipeline_stage.go` — tài liệu bảng giá trị: [co-cau-module-aid-va-domain-queue.md](../module-map/co-cau-module-aid-va-domain-queue.md) mục 11.

**Canonical (docs-shared — một nguồn):** [api-context.md](../../docs-shared/ai-context/folkform/api-context.md) — **Version 4.02** (live trace). **`opsTier`:** [api-context-ai-decision-ops-tier.md](../../docs-shared/ai-context/folkform/api-context-ai-decision-ops-tier.md) (đề xuất **4.09**). **Vision / envelope live:** [08 - ai-decision](../../docs-shared/architecture/vision/08%20-%20ai-decision.md); bổ sung tier: [16-ai-decision-ops-tier.md](../../docs-shared/architecture/vision/16-ai-decision-ops-tier.md).

**Env (live & command center):** `AI_DECISION_LIVE_ENABLED` — mặc định bật; `=0` tắt ring/WebSocket/replay; **phễu + gauge phase trace vẫn cập nhật** qua cùng hook `Publish` (chỉ nhánh metrics). `AI_DECISION_METRICS_RECONCILE_SEC` — chu kỳ đồng bộ độ sâu queue Mongo → RAM (mặc định 300). `AI_DECISION_WS_AGGREGATE_SEC` — chu kỳ message `aggregate` trên WS `org-live` (mặc định 3). `AI_DECISION_METRICS_CHANGE_LOG` — `=1` bật log chi tiết mỗi lần đếm metrics (mặc định tắt). **`AI_DECISION_LIVE_ORG_PERSIST`** — chỉ khi `=1` mới ghi replay org-live ra Mongo collection **`decision_org_live_events`** (mặc định tắt; restart chỉ còn ring RAM). **Metrics command center** (lũy kế, gauge, consumer): **RAM theo process** — xem [THIET_KE_TRUNG_TAM_CHI_HUY_AI_DECISION.md](../05-development/THIET_KE_TRUNG_TAM_CHI_HUY_AI_DECISION.md).

---

## Executor — đề xuất (Propose) qua AI Decision

| Method | Path | Mô tả |
|--------|------|--------|
| POST | `/executor/actions/propose` | Body: `domain`, `actionType`, `reason`, `payload`, … — **enqueue** `executor.propose_requested` (payload có `domain`). **HTTP 202** + `data.eventId`. Consumer gọi `approval.Propose`; **không** trả `action_pending` đồng bộ. |
| POST | `/ads/actions/propose` | Cùng nguyên tắc — `EmitAdsProposeRequest` → `executor.propose_requested` (`domain=ads`). **HTTP 202** + `eventId`. |

Chi tiết vision: [08 - ai-decision.md §8.1](../../docs-shared/architecture/vision/08%20-%20ai-decision.md).

---

## Changelog

- 2026-04-09: AI Decision — **GET `/ai-decision/e2e-reference-catalog`** — JSON catalog G1–G6 + bước chi tiết + milestone consumer + map `live phase` → E2E; doc [bang-pha-buoc-event-e2e §3.1](../flows/bang-pha-buoc-event-e2e.md#31-api-catalog-e2e-json-cho-frontend).
- 2026-04-07: AI Decision — field **`pipelineStage`** trên `decision_events_queue` + body tùy chọn `POST /ai-decision/events`; hằng số `eventtypes/pipeline_stage.go`; doc [co-cau-module-aid-va-domain-queue.md](../module-map/co-cau-module-aid-va-domain-queue.md) mục 11; [NGUYEN_TAC](../05-development/NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION.md) sau sơ đồ mục 1. Cùng ngày: [THIET_KE v1.11](../05-development/THIET_KE_TRUNG_TAM_CHI_HUY_AI_DECISION.md) mục 4.7 collection `decision_org_live_events`.
- 2026-03-25: AI Decision — **`opsTier` / `opsTierLabelVi`** trên `DecisionLiveEvent` (feed/timeline) + response `POST /ai-decision/events`; package `eventopstier`; org-live persist Mongo `decision_org_live_events` + env `AI_DECISION_LIVE_ORG_PERSIST` (mặc định tắt); [THIET_KE](../05-development/THIET_KE_TRUNG_TAM_CHI_HUY_AI_DECISION.md) mục 4.4.
- 2026-03-25: AI Decision — **command center schema 2** (`GET org-live/metrics` + WS `aggregate`): JSON nhóm `meta` / `queue` / `intake` / `publishCounters` / `realtime` — [THIET_KE v1.7](../05-development/THIET_KE_TRUNG_TAM_CHI_HUY_AI_DECISION.md); api-context [4.08](../../docs-shared/ai-context/folkform/api-context.md#version-408); vision [08 §16.1](../../docs-shared/architecture/vision/08%20-%20ai-decision.md) v7.7.1.
- 2026-03-25: AI Decision — **command center v1.4**: response thêm `gaugeByPhase`, `consumer`; phân biệt lũy kế vs gauge; metrics toàn RAM (per process); cập nhật [THIET_KE_TRUNG_TAM_CHI_HUY_AI_DECISION.md](../05-development/THIET_KE_TRUNG_TAM_CHI_HUY_AI_DECISION.md), env `AI_DECISION_METRICS_CHANGE_LOG`.
- 2026-03-25: AI Decision — **command center**: `GET /ai-decision/org-live/metrics`, WS **`/org-live`** message `type: "aggregate"`; reconcile queue Mongo → RAM; env `AI_DECISION_METRICS_RECONCILE_SEC`, `AI_DECISION_WS_AGGREGATE_SEC`; doc [THIET_KE_TRUNG_TAM_CHI_HUY_AI_DECISION.md](../05-development/THIET_KE_TRUNG_TAM_CHI_HUY_AI_DECISION.md).
- 2026-03-25: Learning — **`GET /learning/cases`** thêm filter trace E2E (`decisionCaseId`, `traceId`, `correlationId`, `aidecisionProposeEventId`); consumer **`executor.propose_requested`** merge envelope queue vào payload propose; doc [learning-engine §7–9](../02-architecture/core/learning-engine.md); vision [08 §18](../../docs-shared/architecture/vision/08%20-%20ai-decision.md)
- 2026-03-24: Executor — **POST /executor/actions/propose** và **POST /ads/actions/propose** chỉ enqueue **`executor.propose_requested`** (202 + `eventId`); vision [08 §6 / §8.1](../../docs-shared/architecture/vision/08%20-%20ai-decision.md)
- 2026-03-23: AI Decision — **live trace**: `traceId` trong response `POST /execute`; **GET /traces/:traceId/timeline** + WebSocket **/traces/:traceId/live**; `AI_DECISION_LIVE_ENABLED`; vision [08 §16](../../docs-shared/architecture/vision/08%20-%20ai-decision.md)
- 2026-03-23: AI Decision — **POST /ai-decision/execute** chỉ còn **202 + eventId** (queue `aidecision.execute_requested`); cập nhật `api-context.md` v4.01
- 2026-03-19: Phase 3 Learning — thêm GET/PATCH `/learning/rule-suggestions` (gợi ý điều chỉnh rule từ failure rate)
- 2025-03-17: Thêm GET `/rule-intelligence/logs/:traceId` — xem rule execution log; trace_id truyền vào proposal payload
- 2025-03-15: Thêm Rule Intelligence (run, definition, logic, param-set, output-contract)
- 2025-03-15: Thêm System Endpoints (health, job-metrics, worker-config)
- 2025-03-15: Thêm tài liệu CRM Bulk Jobs (rebuild, recalculate-all, recalculate-one)
- 2025-03-13: Thêm module Decision Brain
- 2025-03-13: Cập nhật cross-links
