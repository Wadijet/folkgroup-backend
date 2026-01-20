# Nguyên Tắc Tách Biệt Trách Nhiệm - DTO, Model, Handler, Service

## Tổng Quan

Tài liệu này mô tả nguyên tắc tách biệt trách nhiệm (Separation of Concerns) cho từng layer trong hệ thống, phân tích hiện trạng và đề xuất cải thiện.

---

## 1. DTO (Data Transfer Object) Layer

### 1.1. DTO Nên Làm Gì?

**Mục đích**: Định nghĩa contract/interface giữa Frontend và Backend

**Trách nhiệm**:
- ✅ **Định nghĩa cấu trúc dữ liệu** nhận từ HTTP request (JSON)
- ✅ **Struct tags cho validation** (`validate:"required"`, `validate:"oneof=..."`)
- ✅ **Struct tags cho transformation** (`transform:"str_objectid"`, `transform:"string,default=value"`)
- ✅ **Helper methods** để parse/validate URL params, query params (ví dụ: `ParseHistoryID`, `ParseCTAIndex`)
- ✅ **Documentation** (comments) để Frontend biết cấu trúc cần gửi

**KHÔNG nên làm**:
- ❌ Business logic validation (ví dụ: cross-collection check)
- ❌ Database operations
- ❌ HTTP request/response handling
- ❌ Transform logic phức tạp (chỉ dùng struct tags)

---

### 1.2. Ví Dụ DTO Đúng Nguyên Tắc

**File**: `api/core/api/dto/dto.ai.workflow.run.go`

```go
// AIWorkflowRunCreateInput dữ liệu đầu vào khi tạo AI workflow run
type AIWorkflowRunCreateInput struct {
	WorkflowID       string                 `json:"workflowId" validate:"required" transform:"str_objectid"`
	RootRefID        string                 `json:"rootRefId,omitempty" transform:"str_objectid_ptr,optional"`
	RootRefType      string                 `json:"rootRefType,omitempty"`
	Status           string                 `json:"status,omitempty" transform:"string,default=pending" validate:"omitempty,oneof=pending running completed failed cancelled"`
	CurrentStepIndex int                    `json:"currentStepIndex,omitempty" transform:"int,default=0"`
	StepRunIDs       []string               `json:"stepRunIds,omitempty" transform:"str_objectid_array,default=[]"`
	Params           map[string]interface{} `json:"params,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}
```

**Phân tích**:
- ✅ Định nghĩa cấu trúc JSON input
- ✅ Struct tags cho validation (`validate:"required"`, `validate:"oneof=..."`)
- ✅ Struct tags cho transformation (`transform:"str_objectid"`, `transform:"string,default=pending"`)
- ✅ Comments mô tả rõ ràng
- ❌ Không có business logic

---

### 1.3. Ví Dụ DTO Có Helper Methods (Đúng Nguyên Tắc)

**File**: `api/core/api/dto/dto.cta.library.go`

```go
// TrackingActionParams là params từ URL khi gọi tracking action
type TrackingActionParams struct {
	Action    string
	HistoryID primitive.ObjectID
	CTAIndex  int
}

// ParseHistoryID parse và validate historyId string sang ObjectID
func (p *TrackingActionParams) ParseHistoryID(historyIDStr string) (primitive.ObjectID, error) {
	if historyIDStr == "" {
		return primitive.NilObjectID, common.NewError(...)
	}
	historyID, err := primitive.ObjectIDFromHex(historyIDStr)
	// ...
	return historyID, nil
}
```

**Phân tích**:
- ✅ Helper methods để parse/validate URL params
- ✅ Trả về error rõ ràng
- ❌ Không có business logic

---

### 1.4. Hiện Trạng DTO Layer

**Kết luận**: ✅ **ĐÚNG NGUYÊN TẮC**

- DTO chỉ định nghĩa cấu trúc dữ liệu và struct tags
- Không có business logic trong DTO
- Helper methods chỉ để parse/validate format (không phải business logic)

---

## 2. Model (Database Model) Layer

### 2.1. Model Nên Làm Gì?

**Mục đích**: Định nghĩa cấu trúc dữ liệu lưu trong database

**Trách nhiệm**:
- ✅ **Định nghĩa cấu trúc database document** (struct fields)
- ✅ **BSON tags** cho MongoDB mapping (`bson:"fieldName"`)
- ✅ **JSON tags** cho API response (`json:"fieldName"`)
- ✅ **Index tags** cho MongoDB indexes (`index:"single:1"`, `index:"text"`)
- ✅ **Constants** cho enum values (ví dụ: `AIWorkflowRunStatusPending = "pending"`)
- ✅ **Documentation** (comments) mô tả fields và collection

**KHÔNG nên làm**:
- ❌ Business logic methods
- ❌ Validation logic
- ❌ Database operations
- ❌ HTTP handling
- ❌ Transform logic

---

### 2.2. Ví Dụ Model Đúng Nguyên Tắc

**File**: `api/core/api/models/mongodb/model.ai.workflow.run.go`

```go
// AIWorkflowRunStatus định nghĩa các trạng thái workflow run
const (
	AIWorkflowRunStatusPending   = "pending"
	AIWorkflowRunStatusRunning   = "running"
	AIWorkflowRunStatusCompleted = "completed"
	AIWorkflowRunStatusFailed    = "failed"
	AIWorkflowRunStatusCancelled = "cancelled"
)

// AIWorkflowRun đại diện cho workflow run (Module 2)
// Collection: ai_workflow_runs
type AIWorkflowRun struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`

	WorkflowID primitive.ObjectID `json:"workflowId" bson:"workflowId" index:"single:1"`

	RootRefID   *primitive.ObjectID `json:"rootRefId,omitempty" bson:"rootRefId,omitempty" index:"single:1"`
	RootRefType string              `json:"rootRefType,omitempty" bson:"rootRefType,omitempty" index:"single:1"`

	Status           string                 `json:"status" bson:"status" index:"single:1"`
	CurrentStepIndex int                    `json:"currentStepIndex" bson:"currentStepIndex"`
	StepRunIDs       []primitive.ObjectID   `json:"stepRunIds,omitempty" bson:"stepRunIds,omitempty"`

	Result      map[string]interface{} `json:"result,omitempty" bson:"result,omitempty"`
	Error       string                 `json:"error,omitempty" bson:"error,omitempty"`
	ErrorDetails map[string]interface{} `json:"errorDetails,omitempty" bson:"errorDetails,omitempty"`

	Params map[string]interface{} `json:"params,omitempty" bson:"params,omitempty"`

	StartedAt   int64 `json:"startedAt,omitempty" bson:"startedAt,omitempty" index:"single:1"`
	CompletedAt int64 `json:"completedAt,omitempty" bson:"completedAt,omitempty"`
	CreatedAt   int64 `json:"createdAt" bson:"createdAt" index:"single:1"`

	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`

	Metadata map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"`
}
```

**Phân tích**:
- ✅ Định nghĩa cấu trúc database document
- ✅ BSON tags cho MongoDB mapping
- ✅ JSON tags cho API response
- ✅ Index tags cho MongoDB indexes
- ✅ Constants cho enum values
- ✅ Comments mô tả rõ ràng
- ❌ Không có business logic methods

---

### 2.3. Hiện Trạng Model Layer

**Kết luận**: ✅ **ĐÚNG NGUYÊN TẮC**

- Model chỉ định nghĩa cấu trúc dữ liệu và tags
- Không có business logic trong Model
- Constants được định nghĩa đúng chỗ

---

## 3. Handler Layer

### 3.1. Handler Nên Làm Gì?

**Mục đích**: Xử lý HTTP request/response, điều phối giữa HTTP và Service layer

**Trách nhiệm**:
- ✅ **Parse HTTP request** (JSON body, URL params, query params)
- ✅ **Validate input format** (struct tag validation, JSON format)
- ✅ **Transform DTO → Model** (dùng transform tags)
- ✅ **Gọi Service methods** để thực hiện business logic
- ✅ **Format HTTP response** (JSON, status codes)
- ✅ **Error handling** (convert service errors → HTTP errors)
- ✅ **Authorization checks** (permission, organization access) - nhưng chỉ check, không implement logic

**KHÔNG nên làm**:
- ❌ Business logic validation (cross-collection, uniqueness, conditional)
- ❌ Database operations trực tiếp
- ❌ Gọi service khác để validate (nên gọi service của chính nó)
- ❌ Tính toán business logic (ví dụ: tính Path/Level)

---

### 3.2. Ví Dụ Handler Đúng Nguyên Tắc

**File**: `api/core/api/handler/handler.organization.share.go`

```go
// OrganizationShareHandler xử lý các request liên quan đến Organization Share
// Đã dùng CRUD chuẩn - logic nghiệp vụ (duplicate check, validation) đã được đưa vào service.InsertOne override
type OrganizationShareHandler struct {
	BaseHandler[models.OrganizationShare, dto.OrganizationShareCreateInput, dto.OrganizationShareUpdateInput]
	OrganizationShareService *services.OrganizationShareService
}

// NewOrganizationShareHandler tạo mới OrganizationShareHandler
func NewOrganizationShareHandler() (*OrganizationShareHandler, error) {
	shareService, err := services.NewOrganizationShareService()
	// ...
	baseHandler := NewBaseHandler[models.OrganizationShare, dto.OrganizationShareCreateInput, dto.OrganizationShareUpdateInput](shareService)
	handler := &OrganizationShareHandler{
		BaseHandler:              *baseHandler,
		OrganizationShareService: shareService,
	}
	return handler, nil
}

// KHÔNG CẦN OVERRIDE InsertOne - Dùng BaseHandler.InsertOne trực tiếp
// Business logic (duplicate check, validation) đã được xử lý ở OrganizationShareService.InsertOne
```

**Phân tích**:
- ✅ Handler chỉ định nghĩa struct và constructor
- ✅ Gọi BaseHandler cho CRUD operations
- ✅ Business logic đã được chuyển xuống Service layer
- ❌ Không có business logic trong Handler

---

### 3.3. Ví Dụ Handler Chưa Đúng Nguyên Tắc

**File**: `api/core/api/handler/handler.ai.workflow.run.go`

```go
func (h *AIWorkflowRunHandler) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// ... parse, transform ...

		// ❌ BUSINESS LOGIC Ở HANDLER - NÊN CHUYỂN XUỐNG SERVICE
		if model.RootRefID != nil && model.RootRefType != "" {
			// Gọi service khác để validate
			contentNodeService, err := services.NewContentNodeService()
			rootProduction, err := contentNodeService.FindOneById(ctx, *model.RootRefID)
			// ... validate logic ...
		}

		// Gọi BaseService.InsertOne
		data, err := h.BaseService.InsertOne(ctx, *model)
		// ...
	})
}
```

**Vấn đề**:
- ❌ Handler đang gọi service khác (`contentNodeService`, `draftContentNodeService`) để validate
- ❌ Handler đang thực hiện business logic validation (cross-collection check)
- ❌ Nên tạo method `AIWorkflowRunService.ValidateRootRef()` và chuyển logic xuống service

**Nên làm**:
```go
func (h *AIWorkflowRunHandler) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// ... parse, transform ...

		// ✅ Gọi service method để validate
		if err := h.AIWorkflowRunService.ValidateRootRef(ctx, model.RootRefID, model.RootRefType); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Gọi service.InsertOne (service sẽ tự validate trước khi insert)
		data, err := h.AIWorkflowRunService.InsertOne(ctx, *model)
		// ...
	})
}
```

---

### 3.4. Hiện Trạng Handler Layer

**Kết luận**: ⚠️ **CHƯA HOÀN TOÀN ĐÚNG NGUYÊN TẮC**

**Đúng nguyên tắc** (3 handlers):
- `OrganizationShareHandler` - Business logic đã ở service
- `AIProviderProfileHandler` - Dùng BaseHandler trực tiếp
- `AIPromptTemplateHandler` - Dùng BaseHandler trực tiếp

**Chưa đúng nguyên tắc** (7 handlers):
- `AIWorkflowRunHandler` - Cross-collection validation ở handler
- `AIWorkflowCommandHandler` - Conditional + cross-collection validation ở handler
- `NotificationRoutingHandler` - Uniqueness check ở handler
- `NotificationChannelHandler` - Uniqueness check ở handler
- `AIStepHandler` - Schema validation ở handler
- `OrganizationHandler` - Tính toán Path/Level ở handler
- `DraftApprovalHandler` - Cross-field validation ở handler

**Tỷ lệ tuân thủ**: ~30% (3/10 handlers)

---

## 4. Service Layer

### 4.1. Service Nên Làm Gì?

**Mục đích**: Thực hiện business logic và database operations

**Trách nhiệm**:
- ✅ **Business logic validation** (cross-collection, uniqueness, conditional, schema)
- ✅ **Database operations** (CRUD: Insert, Update, Delete, Find)
- ✅ **Cross-collection queries** (validate foreign keys, check dependencies)
- ✅ **Atomic operations** (transactions, atomic updates)
- ✅ **Business rules enforcement** (duplicate check, cascade delete protection)
- ✅ **Data transformations** phức tạp (không thể dùng transform tags)
- ✅ **Reusable business methods** (có thể gọi từ handler, background job, CLI)

**KHÔNG nên làm**:
- ❌ HTTP request/response handling
- ❌ Parse JSON/URL params
- ❌ Format HTTP responses
- ❌ Fiber context handling

---

### 4.2. Ví Dụ Service Đúng Nguyên Tắc

**File**: `api/core/api/services/service.organization.share.go`

```go
// InsertOne override để thêm duplicate check và validation
func (s *OrganizationShareService) InsertOne(ctx context.Context, data models.OrganizationShare) (models.OrganizationShare, error) {
	// ✅ 1. Business logic validation: ownerOrgID không được có trong ToOrgIDs
	for _, toOrgID := range data.ToOrgIDs {
		if toOrgID == data.OwnerOrganizationID {
			return data, common.NewError(...)
		}
	}

	// ✅ 2. Business logic: Check duplicate với set comparison
	existingShares, err := s.Find(ctx, bson.M{
		"ownerOrganizationId": data.OwnerOrganizationID,
	}, nil)
	// ... set comparison logic ...

	// ✅ 3. Gọi InsertOne của base service
	return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}
```

**Phân tích**:
- ✅ Business logic validation (ownerOrgID validation)
- ✅ Business logic (duplicate check với set comparison)
- ✅ Database operations (Find, InsertOne)
- ✅ Reusable method (có thể gọi từ handler, background job)
- ❌ Không có HTTP handling

---

### 4.3. Ví Dụ Service Đúng Nguyên Tắc (Cross-Collection Validation)

**File**: `api/core/api/services/service.draft.content.node.go`

```go
// InsertOne override để validate sequential level constraint
func (s *DraftContentNodeService) InsertOne(ctx context.Context, data models.DraftContentNode) (models.DraftContentNode, error) {
	// ✅ Business logic: Cross-collection validation
	if data.ParentID != nil {
		// Thử tìm parent trong production trước
		parentProduction, err := s.contentNodeService.FindOneById(ctx, *data.ParentID)
		// ... validate logic ...
	}

	// ✅ Business logic: Sequential level constraint validation
	if err := utility.ValidateSequentialLevelConstraint(...); err != nil {
		return data, err
	}

	// ✅ Gọi InsertOne của base service
	return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}
```

**Phân tích**:
- ✅ Cross-collection validation (check parent tồn tại)
- ✅ Business rules enforcement (sequential level constraint)
- ✅ Database operations (FindOneById, InsertOne)
- ❌ Không có HTTP handling

---

### 4.4. Hiện Trạng Service Layer

**Kết luận**: ✅ **ĐÚNG NGUYÊN TẮC** (cho các service đã implement)

**Đúng nguyên tắc** (3 services):
- `OrganizationShareService.InsertOne` - Business logic ở service
- `DraftContentNodeService.InsertOne` - Cross-collection validation ở service
- `OrganizationService.DeleteOne`, `RoleService.DeleteOne`, `UserRoleService.DeleteOne` - Cascade delete protection ở service

**Cần bổ sung** (7 services cần tạo methods):
- `AIWorkflowRunService.ValidateRootRef()` - Chuyển validation từ handler
- `AIWorkflowCommandService.ValidateCommand()` - Chuyển validation từ handler
- `NotificationRoutingService.ValidateUniqueness()` - Chuyển uniqueness check từ handler
- `NotificationChannelService.ValidateUniqueness()` - Chuyển uniqueness check từ handler
- `AIStepService.ValidateSchema()` - Chuyển schema validation từ handler
- `OrganizationService.CalculatePathAndLevel()` - Chuyển tính toán từ handler
- `DraftApprovalService.ValidateTargets()` - Chuyển cross-field validation từ handler

---

## 5. Tổng Kết - Nguyên Tắc Tách Biệt Trách Nhiệm

### 5.1. Bảng So Sánh

| Layer | Nên Làm | KHÔNG Nên Làm | Tỷ Lệ Tuân Thủ |
|-------|---------|---------------|----------------|
| **DTO** | Định nghĩa cấu trúc, struct tags, helper parse methods | Business logic, database operations | ✅ **100%** |
| **Model** | Định nghĩa database schema, BSON/JSON tags, constants | Business logic, validation | ✅ **100%** |
| **Handler** | Parse HTTP, validate format, transform DTO→Model, gọi service, format response | Business logic validation, database operations trực tiếp | ⚠️ **~30%** |
| **Service** | Business logic, database operations, cross-collection validation | HTTP handling, parse JSON/URL | ✅ **~30%** (cần bổ sung methods) |

---

### 5.2. Flow Dữ Liệu Đúng Nguyên Tắc

```
HTTP Request (JSON)
    ↓
[Handler] Parse JSON → DTO
    ↓
[Handler] Validate format (struct tags)
    ↓
[Handler] Transform DTO → Model (transform tags)
    ↓
[Handler] Gọi Service method
    ↓
[Service] Business logic validation
    ↓
[Service] Database operations
    ↓
[Service] Return Model
    ↓
[Handler] Format Model → JSON Response
    ↓
HTTP Response (JSON)
```

---

### 5.3. Ví Dụ Flow Đúng Nguyên Tắc

**Insert Organization Share**:

1. **Handler**: Parse JSON → `OrganizationShareCreateInput` (DTO)
2. **Handler**: Validate format (`validate:"required"`, `transform:"str_objectid"`)
3. **Handler**: Transform DTO → `OrganizationShare` (Model)
4. **Handler**: Gọi `OrganizationShareService.InsertOne(ctx, model)`
5. **Service**: Business logic validation (ownerOrgID không được có trong ToOrgIDs)
6. **Service**: Business logic (duplicate check với set comparison)
7. **Service**: Database operation (`BaseServiceMongoImpl.InsertOne`)
8. **Service**: Return `OrganizationShare` (Model)
9. **Handler**: Format Model → JSON Response
10. **Handler**: Return HTTP Response

---

### 5.4. Ví Dụ Flow Chưa Đúng Nguyên Tắc

**Insert AI Workflow Run** (hiện trạng):

1. **Handler**: Parse JSON → `AIWorkflowRunCreateInput` (DTO)
2. **Handler**: Validate format
3. **Handler**: Transform DTO → `AIWorkflowRun` (Model)
4. **Handler**: ❌ **Business logic ở Handler** - Gọi `contentNodeService.FindOneById()` để validate RootRefID
5. **Handler**: ❌ **Business logic ở Handler** - Validate RootRefType, check approve status
6. **Handler**: Gọi `BaseService.InsertOne(ctx, model)`
7. **Service**: Database operation (không có business logic validation)
8. **Handler**: Format Model → JSON Response

**Vấn đề**: Business logic đang ở Handler thay vì Service

**Nên làm**:
1. **Handler**: Parse, validate, transform
2. **Handler**: Gọi `AIWorkflowRunService.InsertOne(ctx, model)`
3. **Service**: ✅ **Business logic ở Service** - `ValidateRootRef()` method
4. **Service**: Database operation
5. **Handler**: Format response

---

## 6. Đề Xuất Cải Thiện

### 6.1. Pattern Chuyển Business Logic Từ Handler → Service

**Bước 1**: Tạo service method để validate
```go
// Service
func (s *AIWorkflowRunService) ValidateRootRef(ctx context.Context, rootRefID *primitive.ObjectID, rootRefType string) error {
	// Business logic validation
	// ...
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

**Bước 2**: Handler chỉ gọi service method
```go
// Handler
func (h *AIWorkflowRunHandler) InsertOne(c fiber.Ctx) error {
	// ... parse, transform ...
	
	// Gọi service (service sẽ tự validate)
	data, err := h.AIWorkflowRunService.InsertOne(ctx, *model)
	h.HandleResponse(c, data, err)
	return nil
}
```

---

### 6.2. Lợi Ích Khi Tuân Thủ Nguyên Tắc

1. **Reusability**: Service methods có thể gọi từ nhiều nơi (handler, background job, CLI)
2. **Testability**: Dễ test business logic độc lập, không cần mock HTTP
3. **Separation of Concerns**: Mỗi layer có trách nhiệm rõ ràng
4. **Maintainability**: Dễ maintain và refactor
5. **Consistency**: Đảm bảo business logic được enforce ở mọi nơi

---

## 7. Kết Luận

### 7.1. Hiện Trạng

- ✅ **DTO Layer**: 100% tuân thủ nguyên tắc
- ✅ **Model Layer**: 100% tuân thủ nguyên tắc
- ⚠️ **Handler Layer**: ~30% tuân thủ (7/10 handlers cần cải thiện)
- ⚠️ **Service Layer**: ~30% tuân thủ (cần bổ sung 7 service methods)

### 7.2. Hành Động Đề Xuất

1. **Ưu tiên cao**: Chuyển business logic từ Handler → Service cho:
   - `AIWorkflowRunHandler` → `AIWorkflowRunService.ValidateRootRef()`
   - `AIWorkflowCommandHandler` → `AIWorkflowCommandService.ValidateCommand()`
   - `NotificationRoutingHandler` → `NotificationRoutingService.ValidateUniqueness()`
   - `NotificationChannelHandler` → `NotificationChannelService.ValidateUniqueness()`

2. **Ưu tiên trung bình**:
   - `AIStepHandler` → `AIStepService.ValidateSchema()`
   - `OrganizationHandler` → `OrganizationService.CalculatePathAndLevel()`
   - `DraftApprovalHandler` → `DraftApprovalService.ValidateTargets()`

3. **Pattern áp dụng**:
   - Tạo service method `ValidateXxx()` hoặc `ValidateBeforeInsert()`
   - Chuyển validation logic từ handler → service
   - Handler chỉ gọi service method

---

## 8. Lưu Ý

1. **Handler vẫn có thể validate input format** (struct tags, JSON parsing) - đây không phải business logic
2. **Business logic** = Logic nghiệp vụ phức tạp (cross-collection, conditional, uniqueness, calculations)
3. **Service methods có thể được gọi từ nhiều nơi** (handler, background job, CLI) → Đảm bảo consistency
4. **Khi chuyển xuống service**, vẫn giữ validation format ở handler nếu cần format error message cho HTTP response
