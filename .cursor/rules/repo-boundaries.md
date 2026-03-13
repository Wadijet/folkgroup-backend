---
description: Backend ownership, không sửa repo ngoài scope
alwaysApply: false
---

# Repo Boundaries — Backend

## Backend Sở Hữu

- Business logic, domain services, data access
- Validation rules, API implementation
- Integration endpoints

## Backend KHÔNG Sửa

- folkgroup-frontend (UI, Flutter)
- folkgroup-agent (automation logic)
- docs-shared (chỉ sửa khi có task rõ ràng)

## Khi Task Chạm Repo Khác

Đọc `docs-shared/system-map/system-map.md` và `docs-shared/modules/module-map.md` trước. Thứ tự sửa: docs → backend → agent → frontend.
