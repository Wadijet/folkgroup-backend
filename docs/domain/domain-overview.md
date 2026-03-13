# Domain Overview — Folkgroup Backend

**Mục đích:** Tổng quan domain logic — Auth, CRM, Meta Ads, Notification, Content, AI, v.v.

---

## Các Domain Chính

| Domain | Mô tả | Module |
|--------|-------|--------|
| **Auth** | Firebase, JWT, user, role, organization, RBAC | auth |
| **CRM** | Customers, activity history, classification, bulk jobs | crm |
| **Meta Ads** | Campaign, adset, ad, insights, activity history | meta, ads |
| **Notification** | Channels, templates, routing, delivery | notification, delivery |
| **Content** | Drafts, publications, videos | content |
| **AI** | Workflows, steps, prompts, provider profiles | ai |
| **Approval** | Propose, approve, reject, execute | approval |
| **Facebook** | Pages, posts, conversations, messages | fb |
| **Pancake** | Pages, POS proxy | pc |
| **Report** | Definitions, snapshots, dirty periods | report |

---

## Related Docs

- [Module Map](../module-map/backend-module-map.md)
- [Kiến trúc](../architecture/overview.md)
- [Business Logic](../02-architecture/business-logic/)

## Changelog

- 2025-03-13: Cập nhật cross-links
