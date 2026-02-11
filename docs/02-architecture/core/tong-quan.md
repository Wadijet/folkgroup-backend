# Tổng Quan Kiến Trúc

Tài liệu về kiến trúc tổng thể của hệ thống FolkForm Auth Backend.

## 🏗️ Kiến Trúc Tổng Quan

```
┌─────────────┐
│   Client    │ (Frontend, Mobile App, etc.)
└──────┬──────┘
       │ HTTP/HTTPS
       │
       ▼
┌─────────────────────────────────┐
│      Fiber HTTP Server          │
│  (api/cmd/server/main.go)        │
└──────┬──────────────────────────┘
       │
       ▼
┌─────────────────────────────────┐
│      Middleware Layer           │
│  - Authentication (JWT)          │
│  - Authorization (RBAC)          │
│  - CORS                          │
│  - Rate Limiting                │
│  - Logging                       │
└──────┬──────────────────────────┘
       │
       ▼
┌─────────────────────────────────┐
│      Router Layer               │
│  (api/internal/api/router/routes.go) │
└──────┬──────────────────────────┘
       │
       ▼
┌─────────────────────────────────┐
│      Handler Layer              │
│  (api/internal/api/handler/)        │
│  - Parse request                │
│  - Validate input               │
│  - Call service                 │
│  - Format response              │
└──────┬──────────────────────────┘
       │
       ▼
┌─────────────────────────────────┐
│      Service Layer              │
│  (api/internal/api/services/)        │
│  - Business logic               │
│  - Data validation              │
│  - Call repository              │
└──────┬──────────────────────────┘
       │
       ▼
┌─────────────────────────────────┐
│      Repository Layer           │
│  (MongoDB Driver)               │
│  - Database operations          │
│  - Query building               │
└──────┬──────────────────────────┘
       │
       ▼
┌─────────────────────────────────┐
│      MongoDB Database           │
│  - folkform_auth                │
│  - folkform_staging             │
│  - folkform_data                │
└─────────────────────────────────┘
```

## 📦 Các Layer Chính

### 1. HTTP Server Layer

**Vị trí:** `api/cmd/server/`

**Chức năng:**
- Khởi tạo Fiber HTTP server
- Cấu hình middleware toàn cục
- Khởi tạo logger, database, cache
- Đăng ký routes

**Files:**
- `main.go` - Entry point
- `init.go` - Khởi tạo global variables
- `init.fiber.go` - Khởi tạo Fiber app
- `init.registry.go` - Khởi tạo service registry
- `init.data.go` - Khởi tạo dữ liệu mặc định

### 2. Middleware Layer

**Vị trí:** `api/internal/api/middleware/`

**Chức năng:**
- Authentication: Verify JWT token
- Authorization: Kiểm tra quyền (RBAC)
- CORS: Xử lý cross-origin requests
- Rate Limiting: Giới hạn số request
- Logging: Ghi log requests
- Error Handling: Xử lý lỗi chung

**Files:**
- `middleware.auth.go` - Authentication & Authorization

### 3. Router Layer

**Vị trí:** `api/internal/api/router/`

**Chức năng:**
- Định nghĩa routes
- Gán middleware cho routes
- CRUD routes tự động
- Custom routes

**Files:**
- `routes.go` - Route definitions

### 4. Handler Layer

**Vị trí:** `api/internal/api/handler/`

**Chức năng:**
- Nhận HTTP request
- Parse và validate input (DTO)
- Gọi service tương ứng
- Format response chuẩn
- Xử lý lỗi HTTP

**Files:**
- `handler.base.response.go` - Response format chung
- `handler.auth.*.go` - Auth handlers
- `handler.admin.*.go` - Admin handlers
- `handler.*.go` - Các handlers khác

### 5. Service Layer

**Vị trí:** `api/internal/api/services/`

**Chức năng:**
- Business logic
- Data validation
- Gọi repository
- Xử lý nghiệp vụ phức tạp

**Files:**
- `service.auth.*.go` - Auth services
- `service.admin.*.go` - Admin services
- `service.*.go` - Các services khác

### 6. Repository Layer

**Vị trí:** `api/internal/api/models/mongodb/`

**Chức năng:**
- Định nghĩa data models
- Database operations (CRUD)
- Query building
- Index management

**Files:**
- `model.*.go` - Data models

### 7. Database Layer

**Vị trí:** `api/internal/database/`

**Chức năng:**
- Kết nối MongoDB
- Database initialization
- Connection pooling

**Files:**
- `mongo.connect.go` - MongoDB connection
- `mongo.init.go` - Database initialization

## 🔄 Request Flow

### 1. Request Đến Server

```
Client → Fiber Server → Middleware → Router → Handler
```

### 2. Xử Lý Request

```
Handler → Service → Repository → MongoDB
```

### 3. Response Trả Về

```
MongoDB → Repository → Service → Handler → Middleware → Client
```

### Ví Dụ: Đăng Nhập

```
1. Client: POST /api/v1/auth/login/firebase
   Body: { "idToken": "...", "hwid": "..." }

2. Middleware: 
   - CORS check ✓
   - Rate limiting check ✓
   - (Không cần auth cho login)

3. Router: Route đến handler.auth.user.go

4. Handler: 
   - Parse DTO từ request body
   - Validate input
   - Gọi service.LoginWithFirebase()

5. Service:
   - Verify Firebase ID token
   - Tìm hoặc tạo user trong MongoDB
   - Tạo JWT token
   - Trả về user + token

6. Handler:
   - Format response
   - Trả về JSON

7. Client: Nhận response với JWT token
```

## 🔐 Authentication & Authorization

### Authentication Flow

```
1. User đăng nhập bằng Firebase SDK
2. Firebase trả về ID Token
3. Client gửi ID Token đến backend
4. Backend verify token với Firebase
5. Backend tạo/update user trong MongoDB
6. Backend tạo JWT token
7. Client lưu JWT token
```

Xem chi tiết tại [Authentication Flow](authentication.md)

### Authorization (RBAC)

```
1. Client gửi request với JWT token
2. Middleware verify JWT token
3. Middleware lấy user info từ token
4. Middleware lấy roles và permissions của user
5. Middleware kiểm tra permission có đủ không
6. Nếu đủ → Cho phép request
7. Nếu không đủ → Trả về 403 Forbidden
```

Xem chi tiết tại [RBAC System](rbac.md)

## 🗄️ Database Schema

### Databases

- **folkform_auth**: Authentication và authorization data
- **folkform_staging**: Staging data
- **folkform_data**: Business data

### Collections Chính

- `users` - User accounts
- `roles` - User roles
- `permissions` - System permissions
- `role_permissions` - Role-Permission mapping
- `user_roles` - User-Role mapping
- `organizations` - Organization tree
- `agents` - Agent management
- `fb_pages`, `fb_posts`, `fb_conversations`, `fb_messages` - Facebook integration
- `pc_orders`, `pc_access_tokens` - Pancake integration

Xem chi tiết tại [Database Schema](database.md)

## 🔧 Utilities & Helpers

### Core Utilities

**Vị trí:** `api/internal/utility/`

- `jwt.go` - JWT token generation và verification
- `cipher.go` - Password hashing
- `firebase.go` - Firebase integration
- `cache.go` - Caching utilities
- `common.go` - Common utilities
- `format.*.go` - Format conversion utilities

### Global Variables

**Vị trí:** `api/internal/global/`

- `global.vars.go` - Global variables (config, database, etc.)
- `validator.go` - Input validation

## 📝 Logging

### Log Configuration

- **Format**: Text với timestamp và caller info
- **Output**: Stdout + File (`logs/app.log`)
- **Level**: Debug (có thể cấu hình)

### Log Levels

- `Debug`: Chi tiết debug info
- `Info`: Thông tin chung
- `Warn`: Cảnh báo
- `Error`: Lỗi

## 🚀 Performance

### Caching

- Permission cache: Cache permissions của user để tránh query database mỗi request
- TTL: Có thể cấu hình

### Database Indexing

- Unique indexes: `firebaseUid`, `email`, `phone`
- Compound indexes: Cho các query phức tạp

### Connection Pooling

- MongoDB connection pool được cấu hình tự động
- Max connections: Có thể cấu hình

## 📚 Tài Liệu Liên Quan

- [Authentication Flow](authentication.md)
- [RBAC System](rbac.md)
- [Database Schema](database.md)
- [Organization Structure](organization.md)

