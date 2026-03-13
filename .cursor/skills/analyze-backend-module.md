# Skill: Analyze Backend Module

**Mục đích:** Phân tích module backend — services, handlers, dependencies.

**Cách dùng:** Khi cần hiểu module trước khi sửa.

**Bước:**
1. Tra `docs/module-map/backend-module-map.md` — vị trí module
2. Quét `api/internal/api/<module>/` — handler, service, router
3. Xác định dependencies (auth, meta, crm, ...)
4. Đọc docs liên quan trong bảng Module Map
