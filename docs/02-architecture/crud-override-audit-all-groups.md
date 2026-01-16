# Rà Soát CRUD Override - Tất Cả Các Nhóm

## Tổng Quan

Kiểm tra tất cả các handlers trong hệ thống xem có CRUD methods bị override không cần thiết không.

---

## Danh Sách Handlers Có Override CRUD

### 1. AIRunHandler
- **Override**: `InsertOne`
- **Lý do trong code**: Comment nói "KHÔNG CẦN OVERRIDE (có thể dùng BaseHandler)"
- **Phân tích**: Chỉ copy logic BaseHandler, không có logic đặc biệt
- **Kết luận**: ❌ **KHÔNG CẦN THIẾT** - Có thể xóa override, dùng BaseHandler.InsertOne trực tiếp

### 2. AIStepRunHandler
- **Override**: `InsertOne`
- **Lý do trong code**: Comment nói "KHÔNG CẦN OVERRIDE (có thể dùng BaseHandler)"
- **Phân tích**: Chỉ copy logic BaseHandler, không có logic đặc biệt
- **Kết luận**: ❌ **KHÔNG CẦN THIẾT** - Có thể xóa override, dùng BaseHandler.InsertOne trực tiếp

### 3. AIStepHandler
- **Override**: `InsertOne`
- **Lý do**: Validate input/output schema với standard schema (business logic validation)
- **Phân tích**: Cần validate schema phức tạp với `models.ValidateStepSchema` - không thể dùng struct tag
- **Kết luận**: ✅ **HỢP LỆ** - Cần giữ override

### 4. AIWorkflowHandler
- **Override**: `InsertOne`
- **Lý do**: Convert nested structures (Steps, DefaultPolicy) - transform tag không hỗ trợ
- **Phân tích**: Cần convert `[]AIStepInput` → `[]AIWorkflowStepReference` và map Policy
- **Kết luận**: ✅ **HỢP LỆ** - Cần giữ override

### 5. AIWorkflowRunHandler
- **Override**: `InsertOne`
- **Lý do**: 
  - Set default values cho business logic (CurrentStepIndex = 0, StepRunIDs = [])
  - Validate RootRefID phức tạp (kiểm tra tồn tại, đúng type, đã commit/approve)
- **Phân tích**: 
  - Validate tồn tại có thể dùng `validate:"exists=..."` 
  - Logic check "đã commit/approve" vẫn cần trong handler (quá phức tạp)
- **Kết luận**: ⚠️ **CÓ THỂ ĐƠN GIẢN HÓA** - Dùng validator cho validate tồn tại, giữ logic phức tạp trong handler
- **Đề xuất**: Xem `docs/02-architecture/enhance-transform-validator-for-nested-structs.md`

### 6. AIWorkflowCommandHandler
- **Override**: `InsertOne`
- **Lý do**: 
  - Validate CommandType và StepID/WorkflowID dựa trên CommandType (cross-field validation)
  - Validate StepID và ParentLevel matching
  - Validate RootRefID phức tạp
- **Phân tích**: 
  - Validate tồn tại có thể dùng `validate:"exists=..."` cho StepID, WorkflowID, RootRefID
  - Cross-field validation (CommandType → StepID/WorkflowID) vẫn cần trong handler
- **Kết luận**: ⚠️ **CÓ THỂ ĐƠN GIẢN HÓA** - Dùng validator cho validate tồn tại, giữ cross-field validation trong handler
- **Đề xuất**: Xem `docs/02-architecture/enhance-transform-validator-for-nested-structs.md`

### 7. AIPromptTemplateHandler
- **Override**: `InsertOne`, `UpdateOne`
- **Lý do**: Map nested struct `Provider` từ DTO sang Model (Provider.ProfileID, Provider.Config)
- **Phân tích**: Transform tag hiện tại không hỗ trợ nested struct, nhưng **CÓ THỂ MỞ RỘNG** để hỗ trợ
- **Kết luận**: ⚠️ **CÓ THỂ LOẠI BỎ** - Sau khi mở rộng transform tag với `transform:"nested_struct"` và validator `validate:"exists=ai_provider_profiles"`
- **Đề xuất**: Xem `docs/02-architecture/enhance-transform-validator-for-nested-structs.md`

### 8. AIProviderProfileHandler
- **Override**: `InsertOne`, `UpdateOne`
- **Lý do**: Map nested struct `Config` từ DTO sang Model (Config.Model, Config.Temperature, etc.)
- **Phân tích**: Transform tag hiện tại không hỗ trợ nested struct, nhưng **CÓ THỂ MỞ RỘNG** để hỗ trợ
- **Kết luận**: ⚠️ **CÓ THỂ LOẠI BỎ** - Sau khi mở rộng transform tag với `transform:"nested_struct"`
- **Đề xuất**: Xem `docs/02-architecture/enhance-transform-validator-for-nested-structs.md`

### 9. DraftApprovalHandler
- **Override**: `InsertOne`
- **Lý do**: 
  - Validate cross-field: Phải có ít nhất một target (workflowRunID, draftNodeID, draftVideoID, hoặc draftPublicationID)
  - Set RequestedBy từ context
  - Convert nhiều optional ObjectID fields
- **Phân tích**: Validation cross-field phức tạp, không thể dùng struct tag
- **Kết luận**: ✅ **HỢP LỆ** - Cần giữ override

### 10. OrganizationHandler
- **Override**: `InsertOne`
- **Lý do**: 
  - Tính toán Path và Level dựa trên parent (logic nghiệp vụ phức tạp)
  - Query database để lấy parent organization
  - Validate Type và parent relationship
- **Phân tích**: Logic nghiệp vụ rất phức tạp, cần query database
- **Kết luận**: ✅ **HỢP LỆ** - Cần giữ override

### 11. NotificationChannelHandler
- **Override**: `InsertOne`
- **Lý do**: 
  - Validate uniqueness phức tạp (query database để check duplicate)
  - Validation khác nhau tùy theo channelType (email/telegram/webhook)
  - Cần query database nhiều lần
- **Phân tích**: Validation uniqueness rất phức tạp, không thể dùng struct tag
- **Kết luận**: ✅ **HỢP LỆ** - Cần giữ override

### 12. NotificationRoutingHandler
- **Override**: `InsertOne`
- **Lý do**: 
  - Validate uniqueness phức tạp (query database để check duplicate)
  - Validation nghiệp vụ đặc biệt (chỉ check rules active, validate eventType bắt buộc)
- **Phân tích**: Validation uniqueness phức tạp, cần query database
- **Kết luận**: ✅ **HỢP LỆ** - Cần giữ override

---

## Tổng Kết

### Override Không Cần Thiết (Cần Xóa Ngay)

1. **AIRunHandler.InsertOne** ❌
   - Chỉ copy logic BaseHandler
   - Không có logic đặc biệt
   - **Đề xuất**: Xóa override, dùng `BaseHandler.InsertOne` trực tiếp

2. **AIStepRunHandler.InsertOne** ❌
   - Chỉ copy logic BaseHandler
   - Không có logic đặc biệt
   - **Đề xuất**: Xóa override, dùng `BaseHandler.InsertOne` trực tiếp

### Override Có Thể Loại Bỏ (Sau Khi Mở Rộng Transform/Validator)

3. **AIPromptTemplateHandler.InsertOne/UpdateOne** ⚠️
   - **Hiện tại**: Map nested struct Provider.Config manual
   - **Sau khi mở rộng**: Dùng `transform:"nested_struct"` và `validate:"exists=ai_provider_profiles"`
   - **Đề xuất**: Xem `docs/02-architecture/enhance-transform-validator-for-nested-structs.md`

4. **AIProviderProfileHandler.InsertOne/UpdateOne** ⚠️
   - **Hiện tại**: Map nested struct Config manual
   - **Sau khi mở rộng**: Dùng `transform:"nested_struct"`
   - **Đề xuất**: Xem `docs/02-architecture/enhance-transform-validator-for-nested-structs.md`

### Override Có Thể Đơn Giản Hóa (Sau Khi Mở Rộng Validator)

5. **AIWorkflowRunHandler.InsertOne** ⚠️
   - **Hiện tại**: Validate RootRefID tồn tại + check đã commit/approve
   - **Sau khi mở rộng**: Dùng `validate:"exists=..."` cho validate tồn tại, giữ logic check commit/approve
   - **Giảm**: ~30 dòng code

6. **AIWorkflowCommandHandler.InsertOne** ⚠️
   - **Hiện tại**: Validate StepID, WorkflowID, RootRefID tồn tại + cross-field validation
   - **Sau khi mở rộng**: Dùng `validate:"exists=..."` cho validate tồn tại, giữ cross-field validation
   - **Giảm**: ~20 dòng code

### Override Hợp Lệ (Cần Giữ)

1. **AIStepHandler.InsertOne** ✅ - Validate schema phức tạp
2. **AIWorkflowHandler.InsertOne** ✅ - Convert nested structures (Steps, DefaultPolicy) - có thể mở rộng transform tag
3. **DraftApprovalHandler.InsertOne** ✅ - Validate cross-field
4. **OrganizationHandler.InsertOne** ✅ - Tính Path và Level
5. **NotificationChannelHandler.InsertOne** ✅ - Validate uniqueness phức tạp
6. **NotificationRoutingHandler.InsertOne** ✅ - Validate uniqueness phức tạp

---

## Phân Loại Theo Lý Do Override

### 1. Map Nested Struct (Transform tag hiện tại không hỗ trợ - CÓ THỂ MỞ RỘNG)
- AIPromptTemplateHandler (Provider.Config) ⚠️ - Có thể loại bỏ sau khi mở rộng
- AIProviderProfileHandler (Config) ⚠️ - Có thể loại bỏ sau khi mở rộng
- AIWorkflowHandler (Steps, DefaultPolicy) ⚠️ - Có thể đơn giản hóa sau khi mở rộng

### 2. Validate Business Logic Phức Tạp
- AIStepHandler (Validate schema)
- AIWorkflowCommandHandler (Validate CommandType, StepID, RootRefID)
- DraftApprovalHandler (Validate cross-field)
- NotificationChannelHandler (Validate uniqueness)
- NotificationRoutingHandler (Validate uniqueness)

### 3. Logic Nghiệp Vụ Phức Tạp (Query Database)
- AIWorkflowRunHandler (Validate RootRefID tồn tại có thể dùng validator, nhưng check commit/approve vẫn cần handler)
- OrganizationHandler (Tính Path và Level dựa trên parent)

### 4. Không Cần Thiết (Chỉ Copy Logic BaseHandler)
- AIRunHandler.InsertOne ❌ - Xóa ngay
- AIStepRunHandler.InsertOne ❌ - Xóa ngay

### 5. Có Thể Loại Bỏ (Sau Khi Mở Rộng Transform/Validator)
- AIPromptTemplateHandler.InsertOne/UpdateOne ⚠️ - Sau khi có `transform:"nested_struct"`
- AIProviderProfileHandler.InsertOne/UpdateOne ⚠️ - Sau khi có `transform:"nested_struct"`

### 6. Có Thể Đơn Giản Hóa (Sau Khi Mở Rộng Validator)
- AIWorkflowRunHandler.InsertOne ⚠️ - Dùng `validate:"exists=..."` cho validate tồn tại
- AIWorkflowCommandHandler.InsertOne ⚠️ - Dùng `validate:"exists=..."` cho validate tồn tại

---

## Đề Xuất Refactor

### Bước 1: Xóa Override Không Cần Thiết

#### 1.1. Xóa AIRunHandler.InsertOne

**File**: `api/core/api/handler/handler.ai.run.go`

**Thay đổi**:
- Xóa method `InsertOne` override
- Dùng `BaseHandler.InsertOne` trực tiếp trong routes

**Lợi ích**: Giảm ~60 dòng code, đơn giản hóa handler

#### 1.2. Xóa AIStepRunHandler.InsertOne

**File**: `api/core/api/handler/handler.ai.step.run.go`

**Thay đổi**:
- Xóa method `InsertOne` override
- Dùng `BaseHandler.InsertOne` trực tiếp trong routes

**Lợi ích**: Giảm ~60 dòng code, đơn giản hóa handler

---

## Kết Luận

**Tổng số handlers có override CRUD**: 12 handlers

### Phân Loại

1. **Override không cần thiết (xóa ngay)**: 2 handlers
   - AIRunHandler.InsertOne ❌
   - AIStepRunHandler.InsertOne ❌

2. **Override có thể loại bỏ (sau khi mở rộng)**: 2 handlers
   - AIPromptTemplateHandler.InsertOne/UpdateOne ⚠️
   - AIProviderProfileHandler.InsertOne/UpdateOne ⚠️

3. **Override có thể đơn giản hóa (sau khi mở rộng)**: 2 handlers
   - AIWorkflowRunHandler.InsertOne ⚠️
   - AIWorkflowCommandHandler.InsertOne ⚠️

4. **Override hợp lệ (cần giữ)**: 6 handlers
   - AIStepHandler.InsertOne ✅
   - AIWorkflowHandler.InsertOne ✅
   - DraftApprovalHandler.InsertOne ✅
   - OrganizationHandler.InsertOne ✅
   - NotificationChannelHandler.InsertOne ✅
   - NotificationRoutingHandler.InsertOne ✅

### Đề Xuất

1. **Ngay lập tức**: Xóa 2 override không cần thiết (AIRunHandler, AIStepRunHandler)
2. **Sau khi mở rộng transform/validator**: 
   - Loại bỏ 2 override (AIPromptTemplateHandler, AIProviderProfileHandler) - **Giảm ~180 dòng code**
   - Đơn giản hóa 2 override (AIWorkflowRunHandler, AIWorkflowCommandHandler) - **Giảm ~50 dòng code**

**Tổng giảm code**: ~230 dòng code sau khi mở rộng transform/validator

**Xem chi tiết**: `docs/02-architecture/enhance-transform-validator-for-nested-structs.md`
