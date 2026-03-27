# Bản Đồ Module Backend — Folkgroup Backend

**Mục đích:** Map các module backend với code thực tế và tài liệu. Dùng khi implement feature, debug, hoặc tìm hiểu logic.

**Canonical:** Tài liệu local backend (`docs/`). Module map workspace-level: `docs-shared/modules/module-map.md`.

---

## Các Module Chính (theo Router)

| Module | Router | Mô tả | Docs chính |
|--------|--------|-------|------------|
| **auth** | `auth/router/routes.go` | Đăng nhập, JWT, user, role, organization | [api/api-overview](../api/api-overview.md), [02-architecture/core/tong-quan](../02-architecture/core/tong-quan.md) |
| **executor** | `executor/router/routes.go` | Executor — Approval Gate + Execution (actions, send, execute, history) | [02-architecture/core/tong-quan](../02-architecture/core/tong-quan.md) |
| **ai-decision** | `aidecision/router/routes.go` | AI Decision — queue `decision_events_queue`, event **`aidecision.execute_requested`**; **POST /execute → 202** + `eventId` + **`traceId`**; **GET /traces/:traceId/timeline** (replay), **GET /traces/:traceId/live** (WebSocket); worker → `ExecuteWithCase`; package `decisionlive`. **Luồng CRUD→hook→queue + trace/correlation:** [NGUYEN_TAC §9](../05-development/NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION.md) | [api-overview](../api/api-overview.md), [api-context 4.09](../../docs-shared/ai-context/folkform/api-context.md#version-409), [vision 08 §19](../../docs-shared/architecture/vision/08%20-%20ai-decision.md), [unified-data-contract §2.5b](../../docs-shared/architecture/data-contract/unified-data-contract.md#contract-25b-trace-queue) |
| **learning** | `learning/router/routes.go` | Learning engine — bộ nhớ học tập (learning cases) | [02-architecture/core/learning-engine](../02-architecture/core/learning-engine.md) |
| **ads** | `ads/router/routes.go` | Meta Ads, action evaluation, auto propose | [docs-shared/ai-context/folkform/design/ads-intelligence/](../../docs-shared/ai-context/folkform/design/ads-intelligence/) |
| **fb** | `fb/router/routes.go` | Facebook Pages, posts, conversations, messages | [api/api-overview](../api/api-overview.md) |
| **meta** | `meta/router/routes.go` | Meta Ads (ad-account, campaign, ad-set, ad, ad-insight, activity-history) | [api/api-overview](../api/api-overview.md) |
| **pc** | `pc/router/routes.go` | Pancake (Pages, POS) | [api/api-overview](../api/api-overview.md) |
| **webhook** | `webhook/router/routes.go` | Webhook endpoints | — |
| **report** | `report/router/routes.go` | Definitions, snapshots, dirty; dirty từ datachanged qua Redis + `report_redis_touch_flush` | `service.report.redis_touch.go`, `internal/redisclient`, `worker/report_redis_touch_worker.go` |
| **crm** | `crm/router/routes.go` | Customers, CRM pending ingest, bulk jobs, rebuild, recalculate | [docs-shared/ai-context/folkform/design/CRM_MODULE_DESIGN.md](../../docs-shared/ai-context/folkform/design/CRM_MODULE_DESIGN.md) |
| **notification** | `notification/router/routes.go` | Channels, templates, routing, trigger | [docs-shared/ai-context/folkform/notification-system.md](../../docs-shared/ai-context/folkform/notification-system.md) |
| **cta** | `cta/router/routes.go` | CTA Library | — |
| **delivery** | (nội bộ executor) | Handler send/execute dùng bởi executor | — |
| **agent** | `agent/router/routes.go` | Agent configs, commands, registry, check-in | [api/api-overview](../api/api-overview.md) |
| **content** | `content/router/routes.go` | Content drafts, publications, videos | [docs-shared/ai-context/folkform/design/](../../docs-shared/ai-context/folkform/design/) |
| **ai** | `ai/router/routes.go` | AI workflows, steps, prompts, provider profiles | — |
| **ruleintel** | `ruleintel/router/routes.go` | Rule Intelligence — Rule Engine, run, logs (trace_id), definition, logic, param-set, output-contract | [02-architecture/core/rule-intelligence](../02-architecture/core/rule-intelligence.md) |
| **cio** | `cio/router/routes.go` | Customer Interaction Orchestrator — hub điều phối đa kênh, routing AI vs Human | [05-development/THIET_KE_MODULE_CIO](../05-development/THIET_KE_MODULE_CIO.md) |
| **cix** | `cix/router/routes.go` | Contextual Conversation Intelligence — Raw→L1→L2→L3→Flag→Action, CIO→CIX→Decision→Executor | [PHUONG_AN_TRIEN_KHAI_CIX](../05-development/PHUONG_AN_TRIEN_KHAI_CIX.md) |

---

## Cấu Trúc Code Thực Tế

```
api/
├── cmd/server/           # Entry point, init
├── internal/
│   ├── api/             # API layer (handler, service, router theo module)
│   │   ├── auth/
│   │   ├── ads/
│   │   ├── aidecision/
│   │   ├── learning/
│   │   ├── executor/
│   │   ├── crm/
│   │   ├── cta/
│   │   ├── delivery/
│   │   ├── fb/
│   │   ├── meta/
│   │   ├── notification/
│   │   ├── pc/
│   │   ├── report/
│   │   ├── webhook/
│   │   ├── agent/
│   │   ├── content/
│   │   ├── ai/
│   │   ├── ruleintel/
│   │   ├── cio/
│   │   ├── cix/
│   │   ├── handler/    # Shared handlers
│   │   ├── middleware/
│   │   ├── router/     # routes.go, CRUD config
│   │   ├── dto/
│   │   └── models/mongodb/
│   ├── approval/       # Approval engine
│   ├── delivery/      # Delivery logic
│   ├── database/
│   ├── global/
│   ├── logger/
│   ├── notifytrigger/
│   ├── registry/
│   ├── systemalert/
│   └── worker/
```

---

## Khi Nào Đọc docs-shared

| Tình huống | Đọc |
|------------|-----|
| Vision, concept | `docs-shared/architecture/vision/00 - ai-commerce-os-platform-l1.md` |
| API contract, endpoint spec | `docs-shared/ai-context/folkform/api-context.md` |
| Module design cross-repo | `docs-shared/ai-context/folkform/design/` |
| System map, repo boundary | `docs-shared/system-map/system-map.md` |
| Module ownership | `docs-shared/modules/module-map.md` |
| Doc canonical | `docs-shared/doc-ownership.md` |

---

## Related Docs

- [Kiến trúc tổng quan](../architecture/overview.md)
- [Cấu trúc code](../05-development/cau-truc-code.md)
- [API Overview](../api/api-overview.md)
- [docs-shared README](../../docs-shared/README.md) (khi junction đã thiết lập)

## Changelog

- 2026-03-26: **Trace / correlation E2E** — NGUYEN_TAC **§9**; docs-shared **unified-data-contract v1.1 §2.5b**, vision **08 v7.7.2 §19**, **api-context 4.09**; cột ai-decision trỏ các mục trên.
- 2026-03-24: **Nguyên tắc luồng datachanged** — tài liệu [NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION.md](../05-development/NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION.md) (một `OnDataChanged`, `applyDatachangedSideEffects` một cửa).
- 2026-03-23: AI Decision — **live trace**: `traceId` khi enqueue; **GET /ai-decision/traces/:traceId/timeline** + WebSocket **/live** (`MetaAdAccount.Read`); xem [vision 08 §16](../../docs-shared/architecture/vision/08%20-%20ai-decision.md)
- 2026-03-23: AI Decision — **POST /ai-decision/execute** chỉ enqueue (**202**); event `aidecision.execute_requested`
- 2026-03-19: Đổi tên module — decision→ai-decision+learning, approval+delivery→executor
- 2026-03-18: Cập nhật decision (AI Decision Engine), cix (luồng đã khép vòng)
- 2025-03-13: Sửa broken links (03-api, 02-architecture/systems không tồn tại) → trỏ api-overview, docs-shared
