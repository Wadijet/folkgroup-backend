# Handler/Service override: cái nào không cần thiết, có thể dùng CRUD thay thế

Tài liệu phân tích các override trong handler và service: cái nào có thể bỏ để dùng CRUD chuẩn, cái nào bắt buộc giữ.

---

## 1. Kết luận nhanh

| Loại | Có thể bỏ / dùng CRUD | Cần giữ |
|------|------------------------|---------|
| **Handler InsertOne override** | 3 handler (organization, notification channel, notification routing) | 4 handler (ai.step, ai.workflow.command, ai.workflow.run **hoặc** refactor cách gán BaseService; ai.workflow vì custom transform) |
| **Service override** | Không — toàn bộ là business logic | Tất cả |

---

## 2. Handler InsertOne override

### 2.1. BaseHandler.InsertOne đã làm gì (handler.base.crud.go)

- Parse body → DTO, validate input, transform DTO → Model.
- **Xử lý ownerOrganizationId**: từ request (validate quyền) hoặc từ context.
- Set userID vào context.
- Gọi **h.BaseService.InsertOne(ctx, model)**.

Nếu **BaseService** được gán bằng **concrete service** (ví dụ `OrganizationService`, `NotificationChannelService`), thì `h.BaseService.InsertOne` sẽ gọi đúng **service.InsertOne** (kể cả khi service có override). Khi đó **handler không cần override InsertOne** — chỉ cần dùng CRUD BaseHandler.

### 2.2. Có thể bỏ override ngay (dùng CRUD thay thế)

Các handler sau đã gán **BaseService = full concrete service**, nên `BaseHandler.InsertOne` đã gọi đúng service override. **Có thể xóa handler InsertOne override** và dùng luôn BaseHandler.InsertOne (CRUD).

| Handler | BaseService được gán | Hành động đề xuất |
|---------|----------------------|--------------------|
| **OrganizationHandler** | `handler.OrganizationService` | Xóa override `InsertOne`, dùng BaseHandler.InsertOne (CRUD). |
| **NotificationChannelHandler** | `channelService` (full) | Xóa override `InsertOne`, dùng BaseHandler.InsertOne (CRUD). |
| **NotificationRoutingHandler** | `routingService` (full) | Xóa override `InsertOne`, dùng BaseHandler.InsertOne (CRUD). |

Logic nghiệp vụ (Path/Level, uniqueness, …) vẫn nằm trong **service.InsertOne** override; handler chỉ cần gọi qua BaseHandler.

### 2.3. Cần giữ override (hoặc refactor cách gán BaseService)

**A. AIWorkflowHandler.InsertOne — bắt buộc giữ**

- Lý do: Convert nested DTO (Steps, Policy, DefaultPolicy) sang Model; struct tag `transform` không hỗ trợ nested struct/array đủ.
- Không thể thay bằng CRUD thuần; handler override là cần thiết.

**B. AIStepHandler, AIWorkflowCommandHandler, AIWorkflowRunHandler — hiện đang cần override**

- Hiện tại: `NewBaseHandler(concreteService.BaseServiceMongoImpl)` → **BaseService = BaseServiceMongoImpl** (embedded base), không phải full service.
- Do đó `h.BaseService.InsertOne` gọi **BaseServiceMongoImpl.InsertOne** (không có validation). Handler phải gọi tường minh `concreteService.InsertOne` → cần handler override.

**Đề xuất refactor (để có thể bỏ override):**

- Đổi thành **NewBaseHandler(concreteService)** (truyền full service thay vì `.BaseServiceMongoImpl`).
- Khi đó `h.BaseService.InsertOne` sẽ gọi **concreteService.InsertOne** (override có validation).
- Sau đó có thể **xóa handler InsertOne override** và dùng CRUD BaseHandler cho cả 3 handler này (giống organization / notification channel / routing).

---

## 3. Service override — không thay bằng CRUD được

Tất cả override ở **service** (InsertOne, UpdateById, DeleteOne, DeleteById, DeleteMany, FindOneAndDelete, Upsert, GetResolvedConfig, …) đều chứa **business logic** (validation, uniqueness, Path/Level, RBAC, locked key, v.v.). CRUD chuẩn vẫn gọi `service.InsertOne` / `service.DeleteOne` / …; override là nơi thực thi logic đó. **Không có service override nào “không cần thiết” để thay bằng CRUD** — cần giữ toàn bộ.

---

## 4. Checklist refactor (đã thực hiện)

- [x] **OrganizationHandler**: Xóa override `InsertOne`, dùng BaseHandler.InsertOne.
- [x] **NotificationChannelHandler**: Xóa override `InsertOne`, dùng BaseHandler.InsertOne.
- [x] **NotificationRoutingHandler**: Xóa override `InsertOne`, dùng BaseHandler.InsertOne.
- [x] **AIStepHandler, AIWorkflowCommandHandler, AIWorkflowRunHandler**: Đổi `NewBaseHandler(service.BaseServiceMongoImpl)` → `NewBaseHandler(service)`, xóa override `InsertOne`.
- [x] **AIWorkflowHandler**: Giữ override `InsertOne` (custom nested transform).

---

## 5. Tài liệu liên quan

- `docs/09-ai-context/handler-pattern-crud-vs-custom.md` — khi nào CRUD chuẩn, khi nào custom handler.
- `.cursor/rules/folkgroup-backend.mdc` — ưu tiên CRUD, hạn chế overdrive.
