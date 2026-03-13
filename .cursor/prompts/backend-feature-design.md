# Backend Feature Design Prompt

Khi thiết kế feature mới:

1. Tra `docs/module-map/backend-module-map.md` — module sở hữu logic
2. Ưu tiên CRUD có sẵn trước khi tạo endpoint mới
3. Kiểm tra `docs-shared/ai-context/folkform/api-context.md` — API contract
4. Handler mỏng, Service chứa business logic
5. Luôn filter theo OwnerOrganizationID
