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
| **crm** | `/crm` | Customers, CRM pending ingest, bulk jobs, rebuild | crm/ |
| **notification** | `/notification` | Channels, templates, routing, trigger | notification/ |
| **cta** | `/cta` | CTA Library | cta/ |
| **delivery** | `/delivery` | Delivery send, history | delivery/ |
| **agent** | `/agent-management` | Agent configs, commands, registry, check-in | agent/ |
| **content** | `/content` | Content drafts, publications, videos | content/ |
| **ai** | `/ai` | AI workflows, steps, prompts, provider profiles | ai/ |

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

## Decision Brain Endpoints

| Method | Path | Mô tả |
|--------|------|-------|
| GET | `/decision/cases` | List decision cases (filter: domain, caseType, goalCode, result, targetType, targetId) |
| GET | `/decision/cases/:id` | Find by ID |
| POST | `/decision/cases` | Create decision case |

Chi tiết: [02-architecture/core/decision-brain](../02-architecture/core/decision-brain.md)

---

## Changelog

- 2025-03-13: Thêm module Decision Brain
- 2025-03-13: Cập nhật cross-links
