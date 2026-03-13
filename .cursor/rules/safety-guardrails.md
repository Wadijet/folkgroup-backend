---
description: Error handling, bảo mật, anti-patterns
alwaysApply: false
---

# Safety Guardrails

## Error Handling

- Luôn xử lý errors, không bỏ qua
- Dùng `common.ErrorCode`, `common.NewError()`
- Convert MongoDB: `common.ConvertMongoError()`, `common.ErrNotFound`
- Wrap: `fmt.Errorf("...: %w", err)`
- Không expose lỗi nội bộ ra client

## Bảo Mật

- Không log passwords, tokens, API keys
- Không cho tạo/update `IsSystem = true` từ API thông thường
- Validate system data trong service layer

## Anti-patterns (Tránh)

- ❌ `context.Background()` trong handler — dùng `c.Context()`
- ❌ Query không filter theo `OwnerOrganizationID`
- ❌ Business logic trong DTO/Handler — đặt trong Service
- ❌ Gọi DB trực tiếp từ Handler — qua Service
- ❌ `panic` trong production — trả error
- ❌ Circular dependencies
- ❌ Response không đúng format chuẩn

## Tham Chiếu

- [repo-boundaries.md](repo-boundaries.md) — Backend không sửa frontend/agent
- [docs-protocol.md](docs-protocol.md) — Khi nào đọc docs-shared
