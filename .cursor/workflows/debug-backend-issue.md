# Workflow: Debug Backend Issue

1. **Xác định module** — Tra backend-module-map
2. **Trace request** — Router → Middleware → Handler → Service
3. **Log** — Kiểm tra logger, error wrapping
4. **DB** — Query filter, OwnerOrganizationID
5. **Docs** — Đọc business logic trong 02-architecture/business-logic/
