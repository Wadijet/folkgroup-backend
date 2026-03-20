# API Overview — Folkgroup Backend

**Mục đích:** Tổng quan API surface — endpoint, method, module. Giúp developer nhìn nhanh và Cursor AI hiểu cấu trúc API.

**Canonical chi tiết:** `docs-shared/ai-context/folkform/api-context.md`

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
| **report** | `/report` | Report definitions, snapshots, dirty periods | report/ |
| **crm** | `/crm` | Customers, CRM pending ingest, bulk jobs, rebuild, recalculate | crm/ |
| **notification** | `/notification` | Channels, templates, routing, trigger | notification/ |
| **cta** | `/cta` | CTA Library | cta/ |
| **delivery** | `/delivery` | Delivery send, history | delivery/ |
| **agent** | `/agent-management` | Agent configs, commands, registry, check-in | agent/ |
| **content** | `/content` | Content drafts, publications, videos | content/ |
| **ai** | `/ai` | AI workflows, steps, prompts, provider profiles | ai/ |
| **rule-intelligence** | `/rule-intelligence` | Rule Engine, definition, logic, param-set, output-contract, run, logs | ruleintel/ |

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
| GET | `/learning/cases` | List learning cases (filter: domain, caseType, goalCode, result, targetType, targetId) |
| GET | `/learning/cases/:id` | Find by ID |
| POST | `/learning/cases` | Create learning case |
| GET | `/learning/rule-suggestions` | List rule suggestions (Phase 3 — filter: domain, goalCode, status) |
| PATCH | `/learning/rule-suggestions/:id` | Cập nhật status (reviewed, applied, dismissed). :id = suggestionId |

Chi tiết: [02-architecture/core/learning-engine](../02-architecture/core/learning-engine.md)

---

## Changelog

- 2026-03-19: Phase 3 Learning — thêm GET/PATCH `/learning/rule-suggestions` (gợi ý điều chỉnh rule từ failure rate)
- 2025-03-17: Thêm GET `/rule-intelligence/logs/:traceId` — xem rule execution log; trace_id truyền vào proposal payload
- 2025-03-15: Thêm Rule Intelligence (run, definition, logic, param-set, output-contract)
- 2025-03-15: Thêm System Endpoints (health, job-metrics, worker-config)
- 2025-03-15: Thêm tài liệu CRM Bulk Jobs (rebuild, recalculate-all, recalculate-one)
- 2025-03-13: Thêm module Decision Brain
- 2025-03-13: Cập nhật cross-links
