# Cấu Trúc Code

Tài liệu về cấu trúc và tổ chức code trong dự án.

## 📋 Tổng Quan

Dự án được tổ chức theo kiến trúc layered với các layer rõ ràng.

## 🏗️ Cấu Trúc Thư Mục

```
api/
├── cmd/server/          # Entry point
├── internal/
│   ├── api/            # API layer
│   │   ├── handler/    # HTTP handlers (shared)
│   │   ├── middleware/ # HTTP middleware
│   │   ├── router/     # Route definitions, CRUD config
│   │   ├── dto/        # Data Transfer Objects
│   │   ├── models/mongodb/  # Data models
│   │   ├── auth/       # Module auth (handler, service, router)
│   │   ├── crm/        # Module CRM
│   │   ├── meta/       # Module Meta Ads
│   │   ├── notification/
│   │   ├── ...         # Các module khác (ads, approval, fb, pc, delivery, agent, content, ai, ...)
│   ├── database/       # Database connections
│   ├── global/         # Global variables
│   ├── logger/         # Logging utilities
│   └── ...             # approval, delivery, worker, notifytrigger, ...
└── config/             # Configuration files
```

**Lưu ý:** Services nằm trong từng module (ví dụ: `api/internal/api/crm/service/`), không phải `api/core/api/services/`.

## 📝 Naming Conventions

### Files

- Handler: `handler.<module>.<entity>.go`
- Service: `service.<module>.<entity>.go`
- Model: `model.<module>.<entity>.go`
- DTO: `dto.<module>.<entity>.go`

### Functions

- Handler: `Handle<Action><Entity>`
- Service: `<Action><Entity>`
- Utility: `<Action><Entity>`

## 🔄 Flow

```
Request → Router → Middleware → Handler → Service → Repository → Database
```

## 📚 Tài Liệu Liên Quan

- [Thêm API Mới](them-api-moi.md)
- [Thêm Service Mới](them-service-moi.md)
- [Coding Standards](coding-standards.md)

