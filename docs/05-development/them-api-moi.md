# Thêm API Mới

Hướng dẫn thêm API endpoint mới vào hệ thống.

## 📋 Tổng Quan

Tài liệu này hướng dẫn cách thêm một API endpoint mới từ đầu đến cuối.

## 🚀 Các Bước

### 1. Tạo Model (Nếu Cần)

**File:** `api/internal/api/models/mongodb/model.<module>.<entity>.go`

```go
package mongodb

import (
    "go.mongodb.org/mongo-driver/bson/primitive"
    "time"
)

type Entity struct {
    ID        primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
    Name      string             `bson:"name" json:"name"`
    CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
    UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
}
```

### 2. Tạo DTO

**File:** `api/internal/api/dto/dto.<module>.<entity>.go`

```go
package dto

type EntityCreateInput struct {
    Name string `json:"name" validate:"required"`
}
```

### 3. Tạo Service

**File:** `api/internal/api/services/service.<module>.<entity>.go`

```go
package services

import (
    "meta_commerce/internal/api/models/mongodb"
    // ...
)

type EntityService struct {
    *BaseService[mongodb.Entity]
}

func NewEntityService() (*EntityService, error) {
    // Implementation
}
```

### 4. Tạo Handler

**File:** `api/internal/api/handler/handler.<module>.<entity>.go`

```go
package handler

import (
    "meta_commerce/internal/api/dto"
    models "meta_commerce/internal/api/models/mongodb"
    "meta_commerce/internal/api/services"
)

type EntityHandler struct {
    *BaseHandler[models.Entity, dto.EntityCreateInput, dto.EntityUpdateInput]
    entityService *services.EntityService
}

func NewEntityHandler() (*EntityHandler, error) {
    // Implementation
}

func (h *EntityHandler) HandleCustomAction(c fiber.Ctx) error {
    // Custom handler logic
}
```

### 5. Đăng Ký Route

**File:** `api/internal/api/router/routes.go`

```go
// Trong hàm register<Module>Routes
entityHandler, err := handler.NewEntityHandler()
if err != nil {
    return fmt.Errorf("failed to create entity handler: %v", err)
}

// CRUD routes
r.registerCRUDRoutes(router, "/entity", entityHandler, entityConfig, "Entity")

// Custom routes
router.Get("/entity/custom", middleware.AuthMiddleware("Entity.Read"), entityHandler.HandleCustomAction)
```

## 📝 Lưu Ý

- Tuân thủ naming conventions
- Thêm validation cho input
- Xử lý lỗi đúng cách
- Thêm permission checks
- Viết test cases

## 📚 Tài Liệu Liên Quan

- [Cấu Trúc Code](cau-truc-code.md)
- [Thêm Service Mới](them-service-moi.md)
- [Coding Standards](coding-standards.md)

