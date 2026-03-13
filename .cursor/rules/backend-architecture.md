---
description: Kiến trúc backend, module boundaries, cấu trúc code
alwaysApply: false
---

# Backend Architecture

## Layered Architecture

```
Request → Router → Middleware → Handler → Service → Repository → Database
```

## Cấu Trúc Code

```
api/
├── cmd/server/           # Entry point
├── internal/
│   ├── api/             # API layer theo module
│   │   ├── auth/        # handler, service, router, dto, models
│   │   ├── crm/
│   │   ├── meta/
│   │   ├── notification/
│   │   ├── ...          # Mỗi module có cấu trúc tương tự
│   │   ├── base/        # BaseHandler, BaseServiceMongoImpl
│   │   ├── handler/     # Shared handlers (nếu có)
│   │   ├── middleware/
│   │   ├── router/      # routes.go, CRUD config
│   │   ├── dto/
│   │   └── models/mongodb/
│   ├── database/
│   ├── global/
│   └── worker/
```

## Module Boundaries

- Mỗi module: `api/internal/api/<module>/` chứa handler, service, router, dto, models
- Base: `api/internal/api/base/` — BaseHandler, BaseServiceMongoImpl
- Models chung: `api/internal/api/models/mongodb/`
- DTO chung: `api/internal/api/dto/`

## Tham Chiếu

- [docs/module-map/backend-module-map.md](../../docs/module-map/backend-module-map.md)
- [docs/architecture/overview.md](../../docs/architecture/overview.md)
