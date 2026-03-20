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

### Khởi tạo Handler (thứ tự init)

- **Tạo handler trong router `Register()`**, gọi factory `NewXxxHandler()` khi đăng ký route — không dùng `init()`.
- Lý do: Handler package được import trước `main()`, nên `init()` chạy trước `InitRegistry()`. Services dùng `RegistryCollections` (RuleDefinitionService, RuleEngineService, ...) cần collection đã đăng ký → gọi trong `init()` gây panic.
- Luồng đúng: `main()` → `InitRegistry()` → `InitFiberApp()` → `router.Register()` → `NewXxxHandler()` tạo service.

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

## CRUD Override Pattern (Handler & Service)

**Nguyên tắc:** Override xử lý logic riêng rồi **gọi hàm CRUD chuẩn** để áp dụng tất cả các điểm hook.

### Handler override

- Parse/validate/transform đặc thù → gọi `h.BaseService.InsertOne(ctx, model)` (hoặc UpdateOne, DeleteOne tương ứng)
- Không gọi `collection.InsertOne` hoặc service method khác bỏ qua BaseService

### Service override

- Validation / logic nghiệp vụ trước → gọi `s.BaseServiceMongoImpl.InsertOne(ctx, data)` (hoặc DeleteOne, UpdateOne tương ứng)
- Không gọi `s.collection.InsertOne` trực tiếp — sẽ bỏ qua: `validateSystemDataInsert`, `applyInsertDefaultsToModel`, `identity.EnrichIdentity4Layers`, `events.EmitDataChanged`

### Ví dụ đúng

```go
// Service override: validate → gọi base
func (s *DraftContentNodeService) InsertOne(ctx context.Context, data DraftContentNode) (DraftContentNode, error) {
    if err := utility.ValidateSequentialLevelConstraint(...); err != nil {
        return data, err
    }
    return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}
```

## Tham Chiếu

- [docs/api/api-overview.md](../../docs/api/api-overview.md)
- [docs/05-development/them-api-moi.md](../../docs/05-development/them-api-moi.md)
