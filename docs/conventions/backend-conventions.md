# Backend Conventions — Folkgroup Backend

**Mục đích:** Quy ước đặt tên, pattern, conventions cho backend.

---

## File Naming

| Loại | Quy ước | Ví dụ |
|------|---------|-------|
| Handler | `handler.<module>.<entity>.go` | handler.notification.trigger.go |
| Service | `service.<module>.<entity>.go` | service.customer.go |
| Model | `model.<module>.<entity>.go` | model.mongodb.user.go |
| DTO | `dto.<module>.<entity>.go` | dto.customer.create.go |

---

## Định danh, tiền tố `uid`, tên field API

- **Một nguồn chuẩn (shared):** [uid-field-naming.md](../../docs-shared/architecture/data-contract/uid-field-naming.md) — prefix `uid` (`utility/uid.go`), **tên module/package**, **file/service**, **collection** (`global.vars.go`), worker, route, env; khóa `links`, camelCase JSON; **mirror/canonical (L1-persist/L2-persist)** — phân biệt doc [KHUNG_KHUON_MODULE_INTELLIGENCE.md](../05-development/KHUNG_KHUON_MODULE_INTELLIGENCE.md) mục 0.
- **Hợp đồng tổng:** [unified-data-contract.md](../../docs-shared/architecture/data-contract/unified-data-contract.md).
- **Thực hành CRM / resolver:** [HUONG_DAN_IDENTITY_LINKS.md](../05-development/HUONG_DAN_IDENTITY_LINKS.md).

## Architecture

- **Layered:** Request → Router → Middleware → Handler → Service → Repository → DB
- **Handler:** Mỏng — parse, validate, gọi service, trả response
- **Service:** Chứa business logic
- **Organization:** Luôn filter theo `OwnerOrganizationID`

---

## Tham chiếu

- [.cursor/rules/folkgroup-backend.mdc](../../.cursor/rules/folkgroup-backend.mdc) — Cursor rules đầy đủ
- [Cấu trúc code](../05-development/cau-truc-code.md)
- [Thêm API mới](../05-development/them-api-moi.md)
