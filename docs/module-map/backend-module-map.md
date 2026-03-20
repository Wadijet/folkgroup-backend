# Bản Đồ Module Backend — Folkgroup Backend

**Mục đích:** Map các module backend với code thực tế và tài liệu. Dùng khi implement feature, debug, hoặc tìm hiểu logic.

**Canonical:** Tài liệu local backend (`docs/`). Module map workspace-level: `docs-shared/modules/module-map.md`.

---

## Các Module Chính (theo Router)

| Module | Router | Mô tả | Docs chính |
|--------|--------|-------|------------|
| **auth** | `auth/router/routes.go` | Đăng nhập, JWT, user, role, organization | [api/api-overview](../api/api-overview.md), [02-architecture/core/tong-quan](../02-architecture/core/tong-quan.md) |
| **executor** | `executor/router/routes.go` | Executor — Approval Gate + Execution (actions, send, execute, history) | [02-architecture/core/tong-quan](../02-architecture/core/tong-quan.md) |
| **ai-decision** | `aidecision/router/routes.go` | AI Decision — tầng ra quyết định, Execute, ReceiveCixPayload | [PHUONG_AN_TRIEN_KHAI_AI_DECISION_VA_LEARNING](../05-development/PHUONG_AN_TRIEN_KHAI_AI_DECISION_VA_LEARNING.md) |
| **learning** | `learning/router/routes.go` | Learning engine — bộ nhớ học tập (learning cases) | [02-architecture/core/learning-engine](../02-architecture/core/learning-engine.md) |
| **ads** | `ads/router/routes.go` | Meta Ads, action evaluation, auto propose | [docs-shared/ai-context/folkform/design/ads-intelligence/](../../docs-shared/ai-context/folkform/design/ads-intelligence/) |
| **fb** | `fb/router/routes.go` | Facebook Pages, posts, conversations, messages | [api/api-overview](../api/api-overview.md) |
| **meta** | `meta/router/routes.go` | Meta Ads (ad-account, campaign, ad-set, ad, ad-insight, activity-history) | [api/api-overview](../api/api-overview.md) |
| **pc** | `pc/router/routes.go` | Pancake (Pages, POS) | [api/api-overview](../api/api-overview.md) |
| **webhook** | `webhook/router/routes.go` | Webhook endpoints | — |
| **report** | `report/router/routes.go` | Report definitions, snapshots, dirty periods | — |
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
| Vision, concept | `docs-shared/architecture/vision/ai-commerce-os-platform-l1.md` |
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

- 2026-03-19: Đổi tên module — decision→ai-decision+learning, approval+delivery→executor
- 2026-03-18: Cập nhật decision (AI Decision Engine), cix (luồng đã khép vòng)
- 2025-03-13: Sửa broken links (03-api, 02-architecture/systems không tồn tại) → trỏ api-overview, docs-shared
