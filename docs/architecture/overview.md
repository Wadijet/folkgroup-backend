# Tổng Quan Kiến Trúc Backend

**Mục đích:** Entry point kiến trúc backend. Mô tả layers, flow request, và cấu trúc code.

**Canonical chi tiết:** [02-architecture/core/tong-quan.md](../02-architecture/core/tong-quan.md)

---

## Kiến Trúc Tổng Quan

```
Client → Fiber Server → Middleware → Router → Handler → Service → Repository → MongoDB
```

- **Handler:** Mỏng — parse, validate, gọi service, trả response
- **Service:** Chứa business logic
- **Organization:** Luôn filter theo `OwnerOrganizationID`

---

## Tham Chiếu

- [Kiến trúc chi tiết](../02-architecture/core/tong-quan.md)
- [Module Map](../module-map/backend-module-map.md)
- [Domain Overview](../domain/domain-overview.md)
- [API Overview](../api/api-overview.md)
