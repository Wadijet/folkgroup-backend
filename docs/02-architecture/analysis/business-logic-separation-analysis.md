# Phân Tích Tách Biệt Business Logic - Handler vs Service

## Tổng Quan

Tài liệu này phân tích xem hệ thống có tuân thủ nguyên tắc **"Business logic xử lý ở Service layer"** chưa, và xác định các trường hợp cần cải thiện.

---

## 1. Nguyên Tắc

### 1.1. Handler Layer Nên Làm Gì?
- ✅ Parse và validate input (DTO validation)
- ✅ Transform DTO → Model
- ✅ Xử lý HTTP request/response
- ✅ Gọi service methods
- ❌ **KHÔNG nên**: Business logic validation, cross-collection queries, complex business rules

### 1.2. Service Layer Nên Làm Gì?
- ✅ Business logic validation
- ✅ Cross-collection validation
- ✅ Duplicate checks
- ✅ Business rules enforcement
- ✅ Database operations
- ✅ Atomic operations

---

## 2. Phân Tích Hiện Trạng

### 2.1. ✅ Đúng Nguyên Tắc - Business Logic Ở Service

#### 2.1.1. OrganizationShareService.InsertOne
**File**: `api/internal/api/services/service.organization.share.go`

**Business Logic ở Service**:
- ✅ Validate ownerOrgID không được có trong ToOrgIDs
- ✅ Duplicate check với set comparison (ToOrgIDs, PermissionNames)

**Kết luận**: ✅ **ĐÚNG** - Business logic đã ở service layer

---

#### 2.1.2. DraftContentNodeService.InsertOne
**File**: `api/internal/api/services/service.draft.content.node.go`

**Business Logic ở Service**:
- ✅ Cross-collection validation: Kiểm tra parent tồn tại trong production/draft
- ✅ Sequential level constraint validation
- ✅ Kiểm tra parent đã approve chưa

**Kết luận**: ✅ **ĐÚNG** - Business logic đã ở service layer

---

#### 2.1.3. OrganizationService.DeleteOne, RoleService.DeleteOne, UserRoleService.DeleteOne
**File**: `api/internal/api/services/service.auth.organization.go`, `service.auth.role.go`, `service.auth.user_role.go`

**Business Logic ở Service**:
- ✅ Cascade delete protection: Kiểm tra dependencies trước khi xóa
- ✅ Business rules: Administrator role phải có ít nhất 1 user

**Kết luận**: ✅ **ĐÚNG** - Business logic đã ở service layer

---

### 2.2. ⚠️ Chưa Đúng Nguyên Tắc - Business Logic Ở Handler

#### 2.2.1. AIWorkflowRunHandler.InsertOne
**File**: `api/internal/api/handler/handler.ai.workflow.run.go`

**Business Logic đang ở Handler** (dòng 115-224):
- ❌ Cross-collection validation: Validate RootRefID tồn tại trong production/draft
- ❌ Kiểm tra RootRefType đúng với type của RootRefID
- ❌ Kiểm tra RootRefID đã được commit/approve chưa
- ❌ Validate sequential level constraint

**Vấn đề**:
- Handler đang gọi `contentNodeService.FindOneById()` và `draftContentNodeService.FindOneById()` trực tiếp
- Handler đang thực hiện business logic validation thay vì service

**Nên làm**:
- Tạo method `AIWorkflowRunService.ValidateRootRef(ctx, rootRefID, rootRefType)` 
- Chuyển toàn bộ validation logic xuống service
- Handler chỉ gọi service method

**Kết luận**: ⚠️ **CHƯA ĐÚNG** - Business logic đang ở handler, nên chuyển xuống service

---

#### 2.2.2. AIWorkflowCommandHandler.InsertOne
**File**: `api/internal/api/handler/handler.ai.workflow.command.go`

**Business Logic đang ở Handler** (dòng 119-316):
- ❌ Conditional validation: WorkflowID bắt buộc khi CommandType = START_WORKFLOW
- ❌ Cross-collection validation: Validate StepID tồn tại, ParentLevel matching
- ❌ Cross-collection validation: Validate RootRefID tồn tại trong production/draft
- ❌ Kiểm tra RootRefID đã được commit/approve chưa

**Vấn đề**:
- Handler đang gọi `stepService.FindOneById()`, `contentNodeService.FindOneById()`, `draftContentNodeService.FindOneById()` trực tiếp
- Handler đang thực hiện business logic validation phức tạp

**Nên làm**:
- Tạo method `AIWorkflowCommandService.ValidateCommand(ctx, command)` 
- Chuyển toàn bộ validation logic xuống service
- Handler chỉ gọi service method

**Kết luận**: ⚠️ **CHƯA ĐÚNG** - Business logic đang ở handler, nên chuyển xuống service

---

#### 2.2.3. NotificationRoutingHandler.InsertOne
**File**: `api/internal/api/handler/handler.notification.routing.go`

**Business Logic đang ở Handler** (dòng 121-166):
- ❌ Uniqueness check: Kiểm tra đã có rule cho eventType + ownerOrganizationId chưa
- ❌ Uniqueness check: Kiểm tra đã có rule cho domain + ownerOrganizationId chưa

**Vấn đề**:
- Handler đang gọi `routingService.FindOne()` để check duplicate
- Handler đang thực hiện uniqueness validation

**Nên làm**:
- Tạo method `NotificationRoutingService.ValidateUniqueness(ctx, rule)` 
- Chuyển uniqueness check xuống service
- Handler chỉ gọi service method

**Kết luận**: ⚠️ **CHƯA ĐÚNG** - Business logic đang ở handler, nên chuyển xuống service

---

#### 2.2.4. NotificationChannelHandler.InsertOne
**File**: `api/internal/api/handler/handler.notification.channel.go`

**Business Logic đang ở Handler**:
- ❌ Uniqueness check: Kiểm tra đã có channel với cùng name + channelType + ownerOrganizationId chưa
- ❌ Uniqueness check: Kiểm tra duplicate recipients (email), webhookUrl, botToken

**Vấn đề**:
- Handler đang gọi `channelService.FindOne()` nhiều lần để check duplicate
- Handler đang thực hiện uniqueness validation phức tạp

**Nên làm**:
- Tạo method `NotificationChannelService.ValidateUniqueness(ctx, channel)` 
- Chuyển uniqueness check xuống service
- Handler chỉ gọi service method

**Kết luận**: ⚠️ **CHƯA ĐÚNG** - Business logic đang ở handler, nên chuyển xuống service

---

#### 2.2.5. AIStepHandler.InsertOne
**File**: `api/internal/api/handler/handler.ai.step.go`

**Business Logic đang ở Handler** (dòng 79-89):
- ❌ Schema validation: Validate input/output schema với standard schema

**Vấn đề**:
- Handler đang gọi `models.ValidateStepSchema()` trực tiếp
- Đây là business logic validation

**Nên làm**:
- Tạo method `AIStepService.ValidateSchema(ctx, stepType, inputSchema, outputSchema)` 
- Chuyển schema validation xuống service
- Handler chỉ gọi service method

**Kết luận**: ⚠️ **CHƯA ĐÚNG** - Business logic đang ở handler, nên chuyển xuống service

---

#### 2.2.6. OrganizationHandler.InsertOne
**File**: `api/internal/api/handler/handler.auth.organization.go`

**Business Logic đang ở Handler** (dòng 110-165):
- ❌ Tính toán Path dựa trên parent.Path + "/" + code
- ❌ Tính toán Level dựa trên Type và parent.Level
- ❌ Query parent organization từ database

**Vấn đề**:
- Handler đang gọi `organizationService.FindOneById()` để lấy parent
- Handler đang thực hiện business logic tính toán

**Nên làm**:
- Tạo method `OrganizationService.CalculatePathAndLevel(ctx, org, parentID)` 
- Chuyển logic tính toán xuống service
- Handler chỉ gọi service method

**Kết luận**: ⚠️ **CHƯA ĐÚNG** - Business logic đang ở handler, nên chuyển xuống service

---

#### 2.2.7. DraftApprovalHandler.InsertOne
**File**: `api/internal/api/handler/handler.content.draft.approval.go`

**Business Logic đang ở Handler** (dòng 91-101):
- ❌ Cross-field validation: "ít nhất một target" (workflowRunID, draftNodeID, draftVideoID, hoặc draftPublicationID)

**Vấn đề**:
- Handler đang thực hiện cross-field validation
- Đây là business logic validation

**Nên làm**:
- Tạo method `DraftApprovalService.ValidateTargets(ctx, approval)` 
- Chuyển validation xuống service
- Handler chỉ gọi service method

**Kết luận**: ⚠️ **CHƯA ĐÚNG** - Business logic đang ở handler, nên chuyển xuống service

---

## 3. Tổng Kết

### 3.1. Đúng Nguyên Tắc (3 services)

| Service | Business Logic | Kết Luận |
|---------|----------------|----------|
| OrganizationShareService | Duplicate check, validation | ✅ ĐÚNG |
| DraftContentNodeService | Cross-collection validation, sequential level constraint | ✅ ĐÚNG |
| OrganizationService, RoleService, UserRoleService | Cascade delete protection | ✅ ĐÚNG |

---

### 3.2. Chưa Đúng Nguyên Tắc (7 handlers)

| Handler | Business Logic ở Handler | Nên Chuyển Xuống Service |
|---------|-------------------------|---------------------------|
| AIWorkflowRunHandler | Cross-collection validation (RootRefID) | ⚠️ **CẦN CHUYỂN** |
| AIWorkflowCommandHandler | Conditional validation, cross-collection validation | ⚠️ **CẦN CHUYỂN** |
| NotificationRoutingHandler | Uniqueness check | ⚠️ **CẦN CHUYỂN** |
| NotificationChannelHandler | Uniqueness check phức tạp | ⚠️ **CẦN CHUYỂN** |
| AIStepHandler | Schema validation | ⚠️ **CẦN CHUYỂN** |
| OrganizationHandler | Tính toán Path/Level | ⚠️ **CẦN CHUYỂN** |
| DraftApprovalHandler | Cross-field validation | ⚠️ **CẦN CHUYỂN** |

---

## 4. Đề Xuất Cải Thiện

### 4.1. Pattern Chuyển Business Logic Từ Handler → Service

**Trước (Handler làm business logic)**:
```go
func (h *AIWorkflowRunHandler) InsertOne(c fiber.Ctx) error {
    // ... parse, transform ...
    
    // ❌ Business logic ở handler
    if model.RootRefID != nil {
        contentNodeService, _ := services.NewContentNodeService()
        rootProduction, err := contentNodeService.FindOneById(ctx, *model.RootRefID)
        // ... validate logic ...
    }
    
    data, err := h.BaseService.InsertOne(ctx, *model)
    // ...
}
```

**Sau (Service làm business logic)**:
```go
// Handler
func (h *AIWorkflowRunHandler) InsertOne(c fiber.Ctx) error {
    // ... parse, transform ...
    
    // ✅ Gọi service để validate
    if err := h.AIWorkflowRunService.ValidateRootRef(ctx, model.RootRefID, model.RootRefType); err != nil {
        h.HandleResponse(c, nil, err)
        return nil
    }
    
    data, err := h.AIWorkflowRunService.InsertOne(ctx, *model)
    // ...
}

// Service
func (s *AIWorkflowRunService) ValidateRootRef(ctx context.Context, rootRefID *primitive.ObjectID, rootRefType string) error {
    // ✅ Business logic ở service
    if rootRefID == nil || rootRefType == "" {
        return nil
    }
    
    contentNodeService, _ := services.NewContentNodeService()
    // ... validate logic ...
    return nil
}

func (s *AIWorkflowRunService) InsertOne(ctx context.Context, data models.AIWorkflowRun) (models.AIWorkflowRun, error) {
    // Validate trước khi insert
    if err := s.ValidateRootRef(ctx, data.RootRefID, data.RootRefType); err != nil {
        return data, err
    }
    
    // Gọi base service
    return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}
```

---

### 4.2. Lợi Ích Khi Chuyển Business Logic Xuống Service

1. **Reusability**: Service method có thể được gọi từ nhiều nơi (handler, background job, CLI, etc.)
2. **Testability**: Dễ test business logic độc lập, không cần mock HTTP request
3. **Separation of Concerns**: Handler chỉ lo HTTP, Service lo business logic
4. **Consistency**: Đảm bảo business logic được enforce ở mọi nơi gọi service
5. **Maintainability**: Dễ maintain và refactor business logic

---

## 5. Kết Luận

### 5.1. Hiện Trạng

- ✅ **Đúng nguyên tắc**: 3 services (OrganizationShare, DraftContentNode, Delete operations)
- ⚠️ **Chưa đúng nguyên tắc**: 7 handlers có business logic đang ở handler layer

### 5.2. Tỷ Lệ Tuân Thủ

- **Tỷ lệ tuân thủ**: ~30% (3/10 cases)
- **Cần cải thiện**: 7 handlers cần chuyển business logic xuống service

### 5.3. Hành Động Đề Xuất

1. **Ưu tiên cao**: 
   - AIWorkflowRunHandler, AIWorkflowCommandHandler (logic phức tạp, nhiều cross-collection queries)
   - NotificationRoutingHandler, NotificationChannelHandler (uniqueness check)

2. **Ưu tiên trung bình**:
   - AIStepHandler (schema validation)
   - OrganizationHandler (tính toán Path/Level)
   - DraftApprovalHandler (cross-field validation)

3. **Pattern áp dụng**:
   - Tạo service method `ValidateXxx()` hoặc `ValidateBeforeInsert()`
   - Chuyển validation logic từ handler → service
   - Handler chỉ gọi service method

---

## 6. Lưu Ý

1. **Handler vẫn có thể validate input format** (struct tags, JSON parsing) - đây không phải business logic
2. **Business logic** = Logic nghiệp vụ phức tạp (cross-collection, conditional, uniqueness, calculations)
3. **Service method có thể được gọi từ nhiều nơi** (handler, background job, CLI) → Đảm bảo consistency
4. **Khi chuyển xuống service**, vẫn giữ validation ở handler nếu cần format error message cho HTTP response
