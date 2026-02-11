# Workflow Chung Của Endpoint CRUD - Từ Request Đến Response

## Tổng Quan

Tài liệu này mô tả workflow chung của một endpoint CRUD từ lúc nhận request đến khi trả về response, bao gồm tất cả các logic can thiệp và các điểm có thể override.

---

## 1. Request Flow

### 1.1. HTTP Request → Fiber Router

```
POST /api/v1/ai/workflow-runs/insert-one
Headers:
  - Authorization: Bearer <JWT_TOKEN>
  - X-Active-Role-ID: <ROLE_ID>
  - Content-Type: application/json
Body: { "workflowId": "...", "rootRefId": "...", ... }
```

---

## 2. Middleware Chain (Theo Thứ Tự)

### 2.1. AuthMiddleware
**File**: `api/internal/api/middleware/middleware.auth.go`

**Mục đích**: 
- Xác thực người dùng và kiểm tra quyền truy cập endpoint
- Đảm bảo chỉ user có permission mới được gọi API

**Tác dụng**:
- Bảo mật API: Chặn request không có token hoặc token không hợp lệ
- Phân quyền: Kiểm tra user có permission cụ thể (ví dụ: `AIWorkflowRuns.Insert`)
- Cache permissions: Tối ưu performance bằng cách cache permissions trong 5 phút
- Set user context: Lưu user_id vào Fiber context để handler sử dụng

**Logic**:
1. Extract JWT token từ header `Authorization`
2. Validate và decode JWT token
3. Lấy user ID từ token
4. Check permission: `permissionPrefix + ".Insert"` (ví dụ: `"AIWorkflowRuns.Insert"`)
5. Lấy permissions của user từ cache hoặc database
6. Validate user có permission không
7. **Set vào context**: `c.Locals("user_id", userID)`

**Có thể override không?**: ❌ **KHÔNG** - Middleware chuẩn, không override

**Kết quả**: 
- ✅ Pass → Tiếp tục
- ❌ Fail → Trả về `401 Unauthorized` hoặc `403 Forbidden`

---

### 2.2. OrganizationContextMiddleware
**File**: `api/internal/api/middleware/middleware.organization_context.go`

**Mục đích**:
- Xác định context làm việc của user (role và organization)
- Tự động suy ra organization từ role để handler sử dụng

**Tác dụng**:
- Multi-tenant support: User có thể làm việc với nhiều organizations thông qua roles
- Tự động set organization: Handler không cần parse organization từ request, tự động lấy từ role context
- Fallback logic: Nếu không có header, tự động lấy role đầu tiên của user
- Data segregation: Đảm bảo data được gán đúng organization để phân quyền sau này

**Logic**:
1. Lấy `user_id` từ context (đã set bởi AuthMiddleware)
2. Lấy `X-Active-Role-ID` từ header
3. Validate user có role này không
4. Nếu không có header → Lấy role đầu tiên của user
5. Lấy role từ database → Suy ra `OwnerOrganizationID`
6. **Set vào context**: 
   - `c.Locals("active_role_id", roleID)`
   - `c.Locals("active_organization_id", orgID)`

**Có thể override không?**: ❌ **KHÔNG** - Middleware chuẩn, không override

**Kết quả**: 
- ✅ Pass → Tiếp tục
- ⚠️ Không có role → Vẫn tiếp tục (một số route không cần org context)

---

## 3. Handler Layer

### 3.1. SafeHandler Wrapper
**File**: `api/internal/api/handler/handler.base.go`

**Mục đích**:
- Bảo vệ application khỏi panic, tránh crash server
- Đảm bảo mọi error đều được xử lý và trả về response hợp lệ

**Tác dụng**:
- Error handling: Bắt mọi panic và convert thành HTTP 500 error response
- Stability: Server không bị crash khi có lỗi không mong đợi
- User experience: User luôn nhận được error response thay vì connection timeout

**Logic**:
1. Recover panic → Trả về `500 Internal Server Error`
2. Gọi handler function thực tế

**Có thể override không?**: ❌ **KHÔNG** - Wrapper chuẩn

---

### 3.2. ParseRequestBody
**File**: `api/internal/api/handler/handler.base.go`

**Mục đích**:
- Parse và validate JSON request body
- Convert JSON string thành DTO struct để xử lý

**Tác dụng**:
- Type safety: Đảm bảo data đúng format và type trước khi xử lý
- Early validation: Phát hiện lỗi format sớm, tránh xử lý data không hợp lệ
- Error handling: Trả về error message rõ ràng nếu JSON không hợp lệ

**Logic**:
1. Parse JSON body từ request
2. Unmarshal vào DTO struct
3. Validate JSON format

**Có thể override không?**: ❌ **KHÔNG** - Logic chuẩn

**Kết quả**:
- ✅ Pass → DTO object
- ❌ Fail → Trả về `400 Bad Request`

---

### 3.3. validateInput (Struct Tag Validation)
**File**: `api/internal/api/handler/handler.base.go`

**Mục đích**:
- Validate dữ liệu đầu vào theo business rules
- Đảm bảo data hợp lệ trước khi xử lý và lưu database

**Tác dụng**:
- Data integrity: Đảm bảo chỉ data hợp lệ mới được xử lý
- Security: Chống XSS, SQL injection, validate password strength
- Foreign key validation: Kiểm tra ObjectID tồn tại trong collection khác
- Declarative validation: Dùng struct tags, code ngắn gọn, dễ maintain
- Early error detection: Phát hiện lỗi sớm, tránh xử lý data không hợp lệ

**Logic**:
1. Dùng `github.com/go-playground/validator/v10`
2. Validate theo struct tags:
   - `validate:"required"` - Field bắt buộc
   - `validate:"oneof=value1 value2"` - Enum values
   - `validate:"omitempty,min=X,max=Y"` - Range validation
   - `validate:"exists=<collection>"` - Foreign key validation (custom validator)
   - `validate:"no_xss"` - XSS protection (custom validator)
   - `validate:"no_sql_injection"` - SQL injection protection (custom validator)
   - `validate:"strong_password"` - Password strength (custom validator)

**Có thể override không?**: ⚠️ **CÓ THỂ** - Handler có thể override để thêm custom validation

**Ví dụ override**:
- `AIWorkflowCommandHandler.InsertOne`: Conditional validation (WorkflowID bắt buộc khi CommandType = START_WORKFLOW)
- `DraftApprovalHandler.InsertOne`: Cross-field validation ("ít nhất một target")

**Kết quả**:
- ✅ Pass → Tiếp tục
- ❌ Fail → Trả về `400 Bad Request` với error details

---

### 3.4. transformCreateInputToModel
**File**: `api/internal/api/handler/handler.base.go`

**Mục đích**:
- Convert DTO (Data Transfer Object) sang Model (Database Model)
- Xử lý type conversion và default values tự động

**Tác dụng**:
- Type conversion: Tự động convert string ObjectID → primitive.ObjectID, string arrays → ObjectID arrays
- Default values: Tự động set giá trị mặc định cho fields (status="active", CurrentStepIndex=0, etc.)
- Nested struct mapping: Recursive mapping nested structs (DTO.Config → Model.Config)
- Code reduction: Giảm boilerplate code, không cần viết conversion logic thủ công
- Consistency: Đảm bảo tất cả handlers xử lý conversion giống nhau

**Logic**:
1. Dùng reflection để duyệt qua các field của DTO
2. Apply transform tags:
   - `transform:"str_objectid"` → Convert string → `primitive.ObjectID`
   - `transform:"str_objectid_ptr,optional"` → Convert string → `*primitive.ObjectID` (optional)
   - `transform:"str_objectid_array,optional"` → Convert `[]string` → `[]primitive.ObjectID`
   - `transform:"string,default=value"` → Set default value nếu empty
   - `transform:"int,default=0"` → Set default value nếu zero
   - `transform:"nested_struct"` → Recursive mapping nested struct (DTO → Model)
3. Map DTO fields → Model fields (tên field giống nhau)

**Có thể override không?**: ⚠️ **CÓ THỂ** - Handler có thể override để thêm custom transform logic

**Ví dụ override**:
- `AIWorkflowHandler.InsertOne`: Convert nested struct arrays (Steps, Policy) - transform tag không hỗ trợ arrays

**Kết quả**:
- ✅ Pass → Model object với tất cả fields đã transform
- ❌ Fail → Trả về `400 Bad Request`

---

### 3.5. OwnerOrganizationID Handling
**File**: `api/internal/api/handler/handler.base.go`

**Mục đích**:
- Tự động gán `OwnerOrganizationID` cho document để phân quyền dữ liệu (data segregation)
- Đảm bảo mọi document đều có organization owner

**Tác dụng**:
- Data segregation: Đảm bảo data được gán đúng organization để filter sau này
- Multi-tenant support: User có thể tạo data cho organization khác (nếu có quyền)
- Automatic assignment: Tự động lấy từ context nếu không có trong request, giảm boilerplate
- Authorization check: Validate user có quyền với organization nếu chỉ định trong request
- Backward compatible: Hỗ trợ cả 2 cách: chỉ định trong request hoặc tự động từ context

**Logic**:
1. Kiểm tra model có field `OwnerOrganizationID` không (dùng reflection)
2. Nếu có `ownerOrganizationId` trong request:
   - Validate user có quyền với organization này không (`validateUserHasAccessToOrg`)
   - Nếu có quyền → Giữ nguyên giá trị từ request
3. Nếu không có trong request:
   - Lấy `active_organization_id` từ context (đã set bởi OrganizationContextMiddleware)
   - Set vào model

**Có thể override không?**: ⚠️ **CÓ THỂ** - Handler có thể override để thêm custom logic

**Ví dụ override**:
- `OrganizationHandler.InsertOne`: Tính toán Path/Level dựa trên parent organization

**Kết quả**:
- ✅ Model có `OwnerOrganizationID` đã được set

---

### 3.6. Set UserID vào Context
**File**: `api/internal/api/handler/handler.base.go`

**Mục đích**:
- Truyền user ID từ Fiber context sang Go context
- Cho phép service layer biết user đang thực hiện action

**Tác dụng**:
- Service layer access: Service có thể check admin permissions, audit logging
- Context propagation: User ID được truyền qua tất cả service calls
- Audit trail: Có thể track user nào tạo/sửa data
- Admin checks: Service có thể check user có phải admin không để bypass một số rules

**Logic**:
1. Lấy `user_id` từ `c.Locals("user_id")`
2. Set vào Go context: `services.SetUserIDToContext(ctx, userID)`
3. Service có thể dùng để check admin permissions

**Có thể override không?**: ❌ **KHÔNG** - Logic chuẩn

---

### 3.7. Custom Business Logic Validation (Handler Override)
**File**: Handler cụ thể (ví dụ: `handler.ai.workflow.run.go`)

**Mục đích**:
- Validate business rules phức tạp không thể dùng struct tags
- Đảm bảo data thỏa mãn các điều kiện nghiệp vụ đặc biệt

**Tác dụng**:
- Cross-field validation: Validate nhiều fields cùng lúc (ví dụ: "ít nhất một target")
- Cross-collection validation: Kiểm tra ObjectID tồn tại trong collection khác
- Business rules: Enforce các rules nghiệp vụ (ví dụ: RootRefID phải đã approve)
- Conditional validation: Validate dựa trên giá trị field khác (ví dụ: WorkflowID bắt buộc khi CommandType = START_WORKFLOW)
- Schema validation: Validate structure phức tạp (ví dụ: input/output schema của AI step)
- Uniqueness check: Kiểm tra duplicate với logic phức tạp (set comparison, multiple conditions)

**Logic** (tùy handler):
- Cross-field validation
- Cross-collection validation
- Business rules validation
- Conditional validation

**Có thể override không?**: ✅ **CÓ** - Đây là điểm override chính

**Ví dụ override**:
- `AIWorkflowRunHandler.InsertOne`: Validate RootRefID tồn tại trong production/draft
- `AIStepHandler.InsertOne`: Validate input/output schema với standard schema
- `NotificationRoutingHandler.InsertOne`: Validate uniqueness (eventType + ownerOrganizationId)

**Kết quả**:
- ✅ Pass → Tiếp tục
- ❌ Fail → Trả về `400 Bad Request` hoặc `409 Conflict`

---

## 4. Service Layer

### 4.1. BaseService.InsertOne
**File**: `api/internal/api/services/service.base.mongo.go`

**Mục đích**:
- Thực hiện insert document vào MongoDB
- Tự động set timestamps và generate ID

**Tác dụng**:
- Timestamp management: Tự động set CreatedAt và UpdatedAt, không cần handler quan tâm
- ID generation: Tự động generate MongoDB ObjectID nếu chưa có
- Database abstraction: Handler không cần biết chi tiết MongoDB operations
- Consistency: Đảm bảo tất cả documents đều có timestamps và ID

**Logic**:
1. Set timestamps:
   - `CreatedAt = time.Now().Unix()`
   - `UpdatedAt = time.Now().Unix()`
2. Generate `_id` nếu chưa có
3. Insert vào MongoDB collection
4. Return inserted document

**Có thể override không?**: ⚠️ **CÓ THỂ** - Service có thể override để thêm business logic

**Ví dụ override**:
- `OrganizationShareService.InsertOne`: Duplicate check với set comparison
- `DraftContentNodeService.InsertOne`: Cross-collection validation (parent phải tồn tại và đã approve)

**Kết quả**:
- ✅ Pass → Inserted document
- ❌ Fail → Database error

---

## 5. Response Layer

### 5.1. HandleResponse
**File**: `api/internal/api/handler/handler.base.go`

**Mục đích**:
- Format response thành JSON chuẩn
- Đảm bảo tất cả responses có format nhất quán

**Tác dụng**:
- Response consistency: Tất cả endpoints trả về cùng format JSON
- Error handling: Format error response với code, message, status rõ ràng
- HTTP status codes: Tự động set status code phù hợp (200, 400, 401, 403, 500, etc.)
- Client-friendly: Response format dễ parse và xử lý ở client side

**Logic**:
1. Nếu có error:
   - Format error response: `{"code": ..., "message": ..., "status": "error"}`
   - Set HTTP status code từ error
2. Nếu không có error:
   - Format success response: `{"code": 200, "data": ..., "status": "success"}`
   - Set HTTP status code `200 OK`
3. Return JSON response

**Có thể override không?**: ❌ **KHÔNG** - Logic chuẩn

---

## 6. Tổng Kết - Các Điểm Có Thể Override

### 6.1. Handler Layer Overrides

| Điểm Override | Mục Đích | Ví Dụ |
|---------------|----------|-------|
| `validateInput` | Thêm custom validation logic | Conditional validation, cross-field validation |
| `transformCreateInputToModel` | Custom transform logic | Nested struct arrays, complex conversions |
| `InsertOne` (toàn bộ) | Thêm business logic validation | Cross-collection validation, schema validation, uniqueness check |
| `UpdateOne` (toàn bộ) | Custom update logic | Partial updates với business rules |
| `DeleteOne` (toàn bộ) | Cascade delete protection | Check dependencies trước khi xóa |
| `Find/FindOne` | Custom query logic | Complex filters, aggregations |

---

### 6.2. Service Layer Overrides

| Điểm Override | Mục Đích | Ví Dụ |
|---------------|----------|-------|
| `InsertOne` | Business logic trước khi insert | Duplicate check, cross-collection validation |
| `UpdateOne` | Business logic trước khi update | Validation rules, atomic operations |
| `DeleteOne/DeleteById` | Cascade delete protection | Check dependencies, business rules |
| `FindOne` | Custom query logic | Wrapper methods, convenience methods |

---

## 7. Flow Diagram

```
HTTP Request
    ↓
[AuthMiddleware] → Validate JWT, Check Permission
    ↓
[OrganizationContextMiddleware] → Set active_role_id, active_organization_id
    ↓
[Handler.SafeHandler] → Recover panic
    ↓
[Handler.ParseRequestBody] → Parse JSON → DTO
    ↓
[Handler.validateInput] → Struct tag validation
    ↓
[Handler.transformCreateInputToModel] → Transform DTO → Model
    ↓
[Handler: OwnerOrganizationID Handling] → Set ownerOrganizationId
    ↓
[Handler: Set UserID to Context] → services.SetUserIDToContext
    ↓
[Handler: Custom Business Logic] → ⚠️ OVERRIDE POINT
    ↓
[Service.InsertOne] → Set timestamps, Insert to MongoDB
    ↓
[Service: Custom Business Logic] → ⚠️ OVERRIDE POINT
    ↓
[Handler.HandleResponse] → Format JSON response
    ↓
HTTP Response
```

---

## 8. Lưu Ý

1. **Middleware**: Không thể override, logic chuẩn cho tất cả endpoints
2. **BaseHandler**: Cung cấp logic chung, handler có thể override các method cụ thể
3. **BaseService**: Cung cấp logic chung, service có thể override các method cụ thể
4. **Transform Tags**: Tự động xử lý type conversion và default values, giảm nhu cầu override
5. **Validation Tags**: Tự động xử lý validation, chỉ cần override cho logic phức tạp
6. **OwnerOrganizationID**: Tự động xử lý từ context, chỉ cần override cho logic đặc biệt

---

## 9. Best Practices

1. **Ưu tiên dùng struct tags** (transform, validate) thay vì override
2. **Chỉ override khi cần**:
   - Cross-field/cross-collection validation
   - Business logic phức tạp
   - Cascade delete protection
   - Nested struct arrays (transform tag không hỗ trợ)
3. **Đảm bảo logic cơ bản**: Khi override, vẫn phải gọi BaseHandler/BaseService để đảm bảo logic chuẩn (timestamps, ownerOrganizationId, etc.)
4. **Comment rõ lý do override**: Mỗi override phải có comment giải thích tại sao không thể dùng logic chuẩn
