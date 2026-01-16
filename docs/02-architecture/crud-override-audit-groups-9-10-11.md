# Rà Soát CRUD Override - Nhóm 9, 10, 11

## Tổng Quan

Kiểm tra các handler trong nhóm 9 (Organization Share), nhóm 10 (Facebook Integration), và nhóm 11 (Webhook) xem có CRUD methods bị override không cần thiết không.

---

## Nhóm 9: Organization Share

### OrganizationShareHandler

**Cấu trúc:**
```go
type OrganizationShareHandler struct {
    BaseHandler[models.OrganizationShare, dto.OrganizationShareCreateInput, dto.OrganizationShareUpdateInput]
    OrganizationShareService *services.OrganizationShareService
}
```

**CRUD Methods Override:**
- ❌ **KHÔNG có override InsertOne**
- ❌ **KHÔNG có override UpdateOne**
- ❌ **KHÔNG có override DeleteOne**
- ❌ **KHÔNG có override FindOne**
- ❌ **KHÔNG có override FindMany**

**Custom Endpoints:**
1. ✅ **CreateShare** - `POST /api/v1/organization-shares`
   - **Lý do**: Validation nghiệp vụ phức tạp (check duplicate với set comparison, authorization check)
   - **Status**: ✅ Hợp lệ - không thể dùng CRUD chuẩn

2. ✅ **DeleteShare** - `DELETE /api/v1/organization-shares/:id`
   - **Lý do**: Authorization check phức tạp (check user là người tạo hoặc có quyền với ownerOrg)
   - **Status**: ✅ Hợp lệ - không thể dùng CRUD chuẩn

3. ✅ **ListShares** - `GET /api/v1/organization-shares?ownerOrganizationId=xxx hoặc ?toOrgId=xxx`
   - **Lý do**: Query phức tạp với $or operator, authorization check
   - **Status**: ✅ Hợp lệ - không thể dùng CRUD chuẩn

**Đánh giá:**
- ⚠️ **CÓ THỂ REFACTOR ĐỂ DÙNG CRUD CHUẨN**: Logic nghiệp vụ có thể đưa vào service layer
- ⚠️ **Custom endpoints không cần thiết**: Có thể dùng InsertOne, DeleteOne, Find nếu đưa logic vào service

**Đề xuất refactor:**
- ⚠️ **CreateShare → InsertOne**: Đưa duplicate check vào `service.InsertOne` override
- ⚠️ **DeleteShare → DeleteOne**: Đưa authorization check vào `service.DeleteById` override
- ⚠️ **ListShares → Find**: Dùng query params, đưa authorization check vào `service.Find` override

**Xem chi tiết:** `docs/02-architecture/refactor-organization-share-to-crud.md`

---

## Nhóm 10: Facebook Integration

### FbMessageHandler

**Cấu trúc:**
```go
type FbMessageHandler struct {
    BaseHandler[models.FbMessage, dto.FbMessageCreateInput, dto.FbMessageCreateInput]
    FbMessageService *services.FbMessageService
}
```

**CRUD Methods Override:**
- ❌ **KHÔNG có override InsertOne**
- ❌ **KHÔNG có override UpdateOne**
- ❌ **KHÔNG có override DeleteOne**
- ❌ **KHÔNG có override FindOne**
- ❌ **KHÔNG có override FindMany**

**Custom Endpoints:**
1. ✅ **HandleUpsertMessages** - `POST /api/v1/facebook/message/upsert-messages`
   - **Lý do**: Batch operation - tách messages[] và lưu vào 2 collections riêng
   - **Status**: ✅ Hợp lệ - không thể dùng CRUD chuẩn

**Đánh giá:**
- ✅ **KHÔNG có CRUD override không cần thiết**
- ✅ Custom endpoint có lý do hợp lệ

---

### FbMessageItemHandler

**Cấu trúc:**
```go
type FbMessageItemHandler struct {
    BaseHandler[models.FbMessageItem, dto.FbMessageItemCreateInput, dto.FbMessageItemUpdateInput]
    FbMessageItemService *services.FbMessageItemService
}
```

**CRUD Methods Override:**
- ❌ **KHÔNG có override InsertOne**
- ❌ **KHÔNG có override UpdateOne**
- ❌ **KHÔNG có override DeleteOne**
- ❌ **KHÔNG có override FindOne**
- ❌ **KHÔNG có override FindMany**

**Custom Endpoints:**
1. ✅ **HandleFindByConversationId** - `GET /api/v1/facebook/message-items/by-conversation/:conversationId`
   - **Lý do**: Query convenience - tìm bằng external ID (conversationId), có phân trang
   - **Status**: ✅ Hợp lệ - không thể dùng CRUD chuẩn

2. ✅ **HandleFindOneByMessageId** - `GET /api/v1/facebook/message-items/by-message/:messageId`
   - **Lý do**: Query convenience - tìm bằng external ID (messageId)
   - **Status**: ✅ Hợp lệ - không thể dùng CRUD chuẩn

**Đánh giá:**
- ✅ **KHÔNG có CRUD override không cần thiết**
- ✅ Tất cả custom endpoints đều có lý do hợp lệ
- ⚠️ **Có thể đơn giản hóa validation**: Có thể dùng `ParseRequestParams` để validate conversationId và messageId

**Đề xuất cải thiện:**
- ⚠️ **HandleFindByConversationId**: Có thể đơn giản hóa validation conversationId với `ParseRequestParams`
- ⚠️ **HandleFindOneByMessageId**: Có thể đơn giản hóa validation messageId với `ParseRequestParams`

---

### FbConversationHandler

**Cấu trúc:**
```go
type FbConversationHandler struct {
    BaseHandler[models.FbConversation, dto.FbConversationCreateInput, dto.FbConversationCreateInput]
    FbConversationService *services.FbConversationService
}
```

**CRUD Methods Override:**
- ❌ **KHÔNG có override InsertOne**
- ❌ **KHÔNG có override UpdateOne**
- ❌ **KHÔNG có override DeleteOne**
- ❌ **KHÔNG có override FindOne**
- ❌ **KHÔNG có override FindMany**

**Custom Endpoints:**
1. ✅ **HandleFindAllSortByApiUpdate** - `GET /api/v1/facebook/conversations/sort-by-api-update`
   - **Lý do**: Query đặc biệt - sort theo apiUpdate timestamp, có phân trang
   - **Status**: ✅ Hợp lệ - không thể dùng CRUD chuẩn

**Đánh giá:**
- ✅ **KHÔNG có CRUD override không cần thiết**
- ✅ Custom endpoint có lý do hợp lệ

---

### FbPostHandler

**Cấu trúc:**
```go
type FbPostHandler struct {
    BaseHandler[models.FbPost, dto.FbPostCreateInput, dto.FbPostCreateInput]
    FbPostService *services.FbPostService
}
```

**CRUD Methods Override:**
- ❌ **KHÔNG có override InsertOne**
- ❌ **KHÔNG có override UpdateOne**
- ❌ **KHÔNG có override DeleteOne**
- ❌ **KHÔNG có override FindOne**
- ❌ **KHÔNG có override FindMany**

**Custom Endpoints:**
1. ✅ **HandleFindOneByPostID** - `GET /api/v1/facebook/posts/by-post-id/:postId`
   - **Lý do**: Query convenience - tìm bằng external ID (Facebook Post ID)
   - **Status**: ✅ Hợp lệ - không thể dùng CRUD chuẩn

**Đánh giá:**
- ✅ **KHÔNG có CRUD override không cần thiết**
- ✅ Custom endpoint có lý do hợp lệ
- ⚠️ **Có thể đơn giản hóa validation**: Có thể dùng `ParseRequestParams` để validate postId

**Đề xuất cải thiện:**
- ⚠️ **HandleFindOneByPostID**: Có thể đơn giản hóa validation postId với `ParseRequestParams`

---

### FbPageHandler

**Cấu trúc:**
```go
type FbPageHandler struct {
    BaseHandler[models.FbPage, dto.FbPageCreateInput, dto.FbPageCreateInput]
    FbPageService *services.FbPageService
}
```

**CRUD Methods Override:**
- ❌ **KHÔNG có override InsertOne**
- ❌ **KHÔNG có override UpdateOne**
- ❌ **KHÔNG có override DeleteOne**
- ❌ **KHÔNG có override FindOne**
- ❌ **KHÔNG có override FindMany**

**Custom Endpoints:**
1. ✅ **HandleFindOneByPageID** - `GET /api/v1/facebook/pages/by-page-id/:pageId`
   - **Lý do**: Query convenience - tìm bằng external ID (Facebook Page ID)
   - **Status**: ✅ Hợp lệ - không thể dùng CRUD chuẩn

2. ✅ **HandleUpdateToken** - `PUT /api/v1/facebook/pages/:id/token`
   - **Lý do**: Business logic - update Facebook page token
   - **Status**: ✅ Hợp lệ - không thể dùng CRUD chuẩn

**Đánh giá:**
- ✅ **KHÔNG có CRUD override không cần thiết**
- ✅ Tất cả custom endpoints đều có lý do hợp lệ
- ⚠️ **Có thể đơn giản hóa validation**: Có thể dùng `ParseRequestParams` để validate pageId và id

**Đề xuất cải thiện:**
- ⚠️ **HandleFindOneByPageID**: Có thể đơn giản hóa validation pageId với `ParseRequestParams`
- ⚠️ **HandleUpdateToken**: Có thể đơn giản hóa validation id với `ParseRequestParams`

---

## Nhóm 11: Webhook

### PancakeWebhookHandler

**Cấu trúc:**
```go
type PancakeWebhookHandler struct {
    pcOrderService        *services.PcOrderService
    fbConversationService *services.FbConversationService
    fbMessageService      *services.FbMessageService
    fbCustomerService     *services.FbCustomerService
    webhookLogService     *services.WebhookLogService
}
```

**CRUD Methods Override:**
- ❌ **KHÔNG có BaseHandler** - không kế thừa từ BaseHandler
- ❌ **KHÔNG có CRUD methods** - đây là webhook handler, không phải CRUD handler

**Custom Endpoints:**
1. ✅ **HandlePancakeWebhook** - `POST /api/v1/webhooks/pancake`
   - **Lý do**: Webhook endpoint - nhận webhook từ Pancake, verify signature, process payload
   - **Status**: ✅ Hợp lệ - không thể dùng CRUD chuẩn

**Đánh giá:**
- ✅ **KHÔNG có CRUD override** (vì không có BaseHandler)
- ✅ Custom endpoint có lý do hợp lệ

---

### PancakePosWebhookHandler

**Cấu trúc:**
```go
type PancakePosWebhookHandler struct {
    pcPosOrderService    *services.PcPosOrderService
    pcPosProductService  *services.PcPosProductService
    pcPosCustomerService *services.PcPosCustomerService
    webhookLogService    *services.WebhookLogService
}
```

**CRUD Methods Override:**
- ❌ **KHÔNG có BaseHandler** - không kế thừa từ BaseHandler
- ❌ **KHÔNG có CRUD methods** - đây là webhook handler, không phải CRUD handler

**Custom Endpoints:**
1. ✅ **HandlePancakePosWebhook** - `POST /api/v1/webhooks/pancake-pos`
   - **Lý do**: Webhook endpoint - nhận webhook từ Pancake POS, verify signature
   - **Status**: ✅ Hợp lệ - không thể dùng CRUD chuẩn

**Đánh giá:**
- ✅ **KHÔNG có CRUD override** (vì không có BaseHandler)
- ✅ Custom endpoint có lý do hợp lệ

---

## Tổng Kết

### Kết Quả Kiểm Tra

#### Nhóm 9: Organization Share
- ⚠️ **CÓ THỂ REFACTOR ĐỂ DÙNG CRUD CHUẨN**
- ⚠️ **Custom endpoints không cần thiết** - Logic có thể đưa vào service layer
- 📝 **Đề xuất refactor**: Xem `docs/02-architecture/refactor-organization-share-to-crud.md`

#### Nhóm 10: Facebook Integration
- ✅ **KHÔNG có CRUD override không cần thiết**
- ✅ Tất cả custom endpoints đều có lý do hợp lệ
- ⚠️ **Có thể đơn giản hóa validation** (4 endpoints)

#### Nhóm 11: Webhook
- ✅ **KHÔNG có CRUD override** (vì không có BaseHandler)
- ✅ Tất cả custom endpoints đều có lý do hợp lệ

### Kết Luận

**Nhóm 9 (Organization Share):**
- ⚠️ **CÓ THỂ REFACTOR** - Custom endpoints có thể thay thế bằng CRUD chuẩn nếu đưa logic vào service layer
- 📝 Xem đề xuất refactor: `docs/02-architecture/refactor-organization-share-to-crud.md`

**Nhóm 10, 11:**
- ✅ **KHÔNG có CRUD override không cần thiết**
- ✅ Tất cả custom endpoints đều có lý do hợp lệ (query convenience, batch operations, webhooks)

### Đề Xuất Cải Thiện (Không Bắt Buộc)

Có thể đơn giản hóa validation trong các custom endpoints bằng cách:
1. **Organization Share** (3 endpoints):
   - CreateShare: Dùng `transform` tag cho ObjectID validation
   - DeleteShare: Dùng `ParseRequestParams` cho ID validation
   - ListShares: Dùng `ParseQueryParams` cho query params validation

2. **Facebook Integration** (4 endpoints):
   - HandleFindByConversationId: Dùng `ParseRequestParams` cho conversationId
   - HandleFindOneByMessageId: Dùng `ParseRequestParams` cho messageId
   - HandleFindOneByPostID: Dùng `ParseRequestParams` cho postId
   - HandleFindOneByPageID: Dùng `ParseRequestParams` cho pageId
   - HandleUpdateToken: Dùng `ParseRequestParams` cho id

**Lưu ý:** Đây là cải thiện tùy chọn, không phải vấn đề cần sửa ngay. Các endpoint hiện tại đều hoạt động đúng và có lý do tồn tại hợp lệ.
