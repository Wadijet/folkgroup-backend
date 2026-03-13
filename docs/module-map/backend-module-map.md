# Bản Đồ Module Backend — Folkgroup Backend

**Mục đích:** Map các module backend với code thực tế và tài liệu. Dùng khi implement feature, debug, hoặc tìm hiểu logic.

**Canonical:** Tài liệu local backend (`docs/`). Module map workspace-level: `docs-shared/modules/module-map.md`.

---

## Các Module Chính (theo Router)

| Module | Router | Mô tả | Docs chính |
|--------|--------|-------|------------|
| **auth** | `auth/router/routes.go` | Đăng nhập, JWT, user, role, organization | [api/api-overview](../api/api-overview.md), [02-architecture/core/tong-quan](../02-architecture/core/tong-quan.md) |
| **approval** | `approval/router/routes.go` | Approval workflow (propose, approve, reject, execute) | [02-architecture/core/tong-quan](../02-architecture/core/tong-quan.md) |
| **decision** | `decision/router/routes.go` | Decision Brain — learning memory, decision cases | [02-architecture/core/decision-brain](../02-architecture/core/decision-brain.md) |
| **ads** | `ads/router/routes.go` | Meta Ads, action evaluation, auto propose | [docs-shared/ai-context/folkform/design/ads-intelligence/](../../docs-shared/ai-context/folkform/design/ads-intelligence/) |
| **fb** | `fb/router/routes.go` | Facebook Pages, posts, conversations, messages | [api/api-overview](../api/api-overview.md) |
| **meta** | `meta/router/routes.go` | Meta Ads (ad-account, campaign, ad-set, ad, ad-insight, activity-history) | [api/api-overview](../api/api-overview.md) |
| **pc** | `pc/router/routes.go` | Pancake (Pages, POS) | [api/api-overview](../api/api-overview.md) |
| **webhook** | `webhook/router/routes.go` | Webhook endpoints | — |
| **report** | `report/router/routes.go` | Report definitions, snapshots, dirty periods | — |
| **crm** | `crm/router/routes.go` | Customers, CRM pending ingest, bulk jobs, rebuild, recalculate | [docs-shared/ai-context/folkform/design/CRM_MODULE_DESIGN.md](../../docs-shared/ai-context/folkform/design/CRM_MODULE_DESIGN.md) |
| **notification** | `notification/router/routes.go` | Channels, templates, routing, trigger | [docs-shared/ai-context/folkform/notification-system.md](../../docs-shared/ai-context/folkform/notification-system.md) |
| **cta** | `cta/router/routes.go` | CTA Library | — |
| **delivery** | `delivery/router/routes.go` | Delivery send, history | — |
| **agent** | `agent/router/routes.go` | Agent configs, commands, registry, check-in | [api/api-overview](../api/api-overview.md) |
| **content** | `content/router/routes.go` | Content drafts, publications, videos | [docs-shared/ai-context/folkform/design/](../../docs-shared/ai-context/folkform/design/) |
| **ai** | `ai/router/routes.go` | AI workflows, steps, prompts, provider profiles | — |

---

## Cấu Trúc Code Thực Tế

```
api/
├── cmd/server/           # Entry point, init
├── internal/
│   ├── api/             # API layer (handler, service, router theo module)
│   │   ├── auth/
│   │   ├── ads/
│   │   ├── approval/
│   │   ├── decision/
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
| Vision, concept | `docs-shared/architecture/ai-commerce-os-overview.md` |
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

- 2025-03-13: Sửa broken links (03-api, 02-architecture/systems không tồn tại) → trỏ api-overview, docs-shared
