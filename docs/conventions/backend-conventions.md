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
