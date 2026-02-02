# Bảng Quy Tắc Thiết Kế Hệ Thống – AI Context

**Nguồn quy tắc đầy đủ (một nơi duy nhất):** [`.cursor/rules/folkgroup-backend.mdc`](../../.cursor/rules/folkgroup-backend.mdc)

Cursor tự áp dụng rule đó cho mọi chat/agent trong project (`alwaysApply: true`). File này chỉ là **mục lục** — trỏ tới rule và bảng "khi cần gì đọc tài liệu nào".

---

## Khi cần gì → đọc tài liệu nào

| Khi cần | Đọc |
|---------|-----|
| Tổng quan kiến trúc | `docs/02-architecture/core/tong-quan.md`, `02-architecture/README.md` |
| Auth, RBAC, Firebase | `02-architecture/core/authentication.md`, `rbac.md`, `firebase-auth-voi-database.md` |
| Thêm/sửa API, CRUD | `docs/05-development/them-api-moi.md`, `02-architecture/analysis/endpoint-workflow-general.md` |
| Handler: CRUD vs custom route | `docs/09-ai-context/handler-pattern-crud-vs-custom.md` |
| Plan / trạng thái config tổ chức | `docs/09-ai-context/organization-config-plan.md` |
| Phân quyền theo org | `02-architecture/business-logic/co-che-quan-ly-ownerorganizationid.md`, `organization-data-authorization.md` |
| Tách layer DTO/Model/Handler/Service | `02-architecture/refactoring/layer-separation-principles.md` |
| Notification, routing rules | `02-architecture/systems/notification-processing-rules.md`, `notification-domain-severity.md` |
| Coding style, cấu trúc thư mục | `05-development/coding-standards.md`, `cau-truc-code.md` |
| Test | `docs/06-testing/`, `05-development/` (CHANGELOG/proposal nếu có) |

---

**Cập nhật:** 2025-01-30
