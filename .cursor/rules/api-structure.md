---
description: API conventions, Handler pattern, Response format
alwaysApply: false
---

# API Structure

## Naming Conventions

| Loại | Quy ước | Ví dụ |
|------|---------|-------|
| Handler file | `handler.<module>.<entity>.go` | handler.notification.trigger.go |
| Service file | `service.<module>.<entity>.go` | service.customer.go |
| Model file | `model.<module>.<entity>.go` | model.mongodb.user.go |
| DTO file | `dto.<module>.<entity>.go` | dto.customer.create.go |
| Router | `api/internal/api/<module>/router/routes.go` | — |

## Handler Pattern

- Kế thừa `BaseHandler[T, CreateInput, UpdateInput]` khi có thể
- Dùng `SafeHandlerWrapper` để xử lý errors
- Response: `c.Status(code).JSON(fiber.Map{...})`

## Response Format (bắt buộc)

**Success:**
```go
c.Status(common.StatusOK).JSON(fiber.Map{
    "code": common.StatusOK,
    "message": "Thông báo thành công",
    "data": result,
    "status": "success",
})
```

**Error:**
```go
c.Status(common.StatusBadRequest).JSON(fiber.Map{
    "code": common.ErrCodeValidationFormat.Code,
    "message": "Thông báo lỗi bằng Tiếng Việt",
    "status": "error",
})
```

- Luôn dùng `fiber.Map`, `c.Status()` trước `JSON()`
- Không trả struct trực tiếp hoặc format khác

## Ưu Tiên CRUD

- Ưu tiên CRUD từ `BaseHandler` trước khi tạo endpoint mới
- Hỏi: "Có thể dùng CRUD với filter/query không?"
- Endpoint mới cần lý do rõ ràng

## Tham Chiếu

- [docs/api/api-overview.md](../../docs/api/api-overview.md)
- [docs/05-development/them-api-moi.md](../../docs/05-development/them-api-moi.md)
