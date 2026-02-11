# Phân Tích & Đề Xuất: Bổ Sung Field Sở Hữu Dữ Liệu Theo Tổ Chức

## 📋 Tổng Quan

Tài liệu này phân tích thực trạng hiện tại về việc quản lý sở hữu dữ liệu theo tổ chức (organization ownership) và đề xuất phương án bổ sung field `organizationId` cho các model còn thiếu.

## 🔍 Thực Trạng Hiện Tại

### ✅ Các Model ĐÃ CÓ `organizationId`

1. **Role** (`model.auth.role.go`)
   - Field: `OrganizationID primitive.ObjectID` (bắt buộc)
   - Mục đích: Role thuộc về một Organization cụ thể
   - Index: `single:1,compound:role_org_name_unique`

2. **NotificationChannel** (`model.notification.channel.go`)
   - Field: `OrganizationID primitive.ObjectID` (bắt buộc)
   - Mục đích: Channel thuộc về một Team/Organization
   - Index: `single:1`

3. **NotificationQueueItem** (`model.notification.queue.go`)
   - Field: `OrganizationID primitive.ObjectID` (bắt buộc)
   - Mục đích: Queue item thuộc về một Organization
   - Index: `single:1`

4. **NotificationHistory** (`model.notification.history.go`)
   - Field: `OrganizationID primitive.ObjectID` (bắt buộc)
   - Mục đích: Lịch sử notification thuộc về một Organization
   - Index: `single:1`

5. **NotificationTemplate** (`model.notification.template.go`)
   - Field: `OrganizationID *primitive.ObjectID` (nullable)
   - Mục đích: Template có thể global (null) hoặc thuộc Organization
   - Index: `single:1`

6. **NotificationSender** (`model.notification.sender.go`)
   - Field: `OrganizationID *primitive.ObjectID` (nullable)
   - Mục đích: Sender có thể global (null) hoặc thuộc Organization
   - Index: `single:1`

7. **NotificationRouting** (`model.notification.routing.go`)
   - Field: `OrganizationIDs []primitive.ObjectID` (array)
   - Mục đích: Routing rule áp dụng cho nhiều Teams/Organizations

8. **AuthLog** (`model.auth.log.go`)
   - Field: `OrganizationID primitive.ObjectID` (optional)
   - Mục đích: Log hoạt động có thể gắn với Organization

## 📊 Phân Loại Collections

### ✅ Collections KHÔNG CẦN `organizationId` (System/Global)

1. **Users** (`model.auth.user.go`)
   - Lý do: User là global, có thể thuộc nhiều organizations qua UserRoles
   - Phân quyền: Qua UserRoles → Role → OrganizationID

2. **Permissions** (`model.auth.permission.go`)
   - Lý do: System-wide, không thuộc organization cụ thể
   - Phân quyền: Qua RolePermissions → Role → OrganizationID

3. **Organizations** (`model.auth.organization.go`)
   - Lý do: Chính nó là organization, không cần field organizationId

4. **UserRoles** (`model.auth.user_role.go`)
   - Lý do: Mapping table, đã có organizationId gián tiếp qua Role
   - Phân quyền: UserRole → Role → OrganizationID

5. **RolePermissions** (`model.auth.role_permission.go`)
   - Lý do: Mapping table, đã có organizationId gián tiếp qua Role
   - Phân quyền: RolePermission → Role → OrganizationID

6. ~~**AccessTokens** (`model.pc.access_token.go`)~~ - **CẦN THÊM organizationId**
   - ~~Lý do: Global hoặc user-specific, không cần organizationId~~
   - **Cập nhật**: Cần phân quyền theo organization → Cần thêm `OrganizationID`

### ✅ Collections ĐÃ CÓ `organizationId`

1. **Role** - Đã có
2. **NotificationChannel** - Đã có
3. **NotificationQueueItem** - Đã có
4. **NotificationHistory** - Đã có
5. **NotificationTemplate** - Đã có (nullable)
6. **NotificationSender** - Đã có (nullable)
7. **NotificationRouting** - Đã có (array)
8. **AuthLog** - Đã có (optional)

### ❌ Collections CẦN THÊM `organizationId` (Business Data)

#### 1. Business Data Models (Dữ liệu nghiệp vụ) - CẦN

**Customer** (`model.customer.go`)
- Thiếu field sở hữu
- Ảnh hưởng: Không thể phân biệt customer thuộc organization nào
- Mức độ: **CAO** - Dữ liệu quan trọng, cần multi-tenant

**FbCustomer** (`model.fb.customer.go`)
- Thiếu field sở hữu
- Ảnh hưởng: Không thể phân biệt Facebook customer thuộc organization nào
- Mức độ: **CAO** - Dữ liệu quan trọng

**PcPosCustomer** (`model.pc.pos.customer.go`)
- Thiếu field sở hữu
- Ảnh hưởng: Không thể phân biệt POS customer thuộc organization nào
- Mức độ: **CAO** - Dữ liệu quan trọng

**PcPosOrder** (`model.pc.pos.order.go`)
- Thiếu field sở hữu
- Ảnh hưởng: Không thể phân biệt đơn hàng thuộc organization nào
- Mức độ: **CAO** - Dữ liệu quan trọng

**PcPosShop** (`model.pc.pos.shop.go`)
- Thiếu field sở hữu
- Ảnh hưởng: Không thể phân biệt shop thuộc organization nào
- Mức độ: **CAO** - Dữ liệu quan trọng

**PcPosProduct** (`model.pc.pos.product.go`)
- Thiếu field sở hữu
- Ảnh hưởng: Không thể phân biệt sản phẩm thuộc organization nào
- Mức độ: **CAO** - Dữ liệu quan trọng

**PcPosWarehouse** (`model.pc.pos.warehouse.go`)
- Thiếu field sở hữu
- Ảnh hưởng: Không thể phân biệt kho hàng thuộc organization nào
- Mức độ: **CAO** - Dữ liệu quan trọng

#### 2. Facebook Data Models

**FbPage** (`model.fb.page.go`)
- Thiếu field sở hữu
- Ảnh hưởng: Không thể phân biệt Facebook page thuộc organization nào
- Mức độ: **CAO** - Mỗi organization có thể có nhiều pages

**FbPost** (`model.fb.post.go`)
- Thiếu field sở hữu
- Ảnh hưởng: Không thể phân biệt post thuộc organization nào
- Mức độ: **CAO** - Dữ liệu quan trọng

**FbConversation** (`model.fb.conversation.go`)
- Thiếu field sở hữu
- Ảnh hưởng: Không thể phân biệt conversation thuộc organization nào
- Mức độ: **CAO** - Dữ liệu quan trọng

**FbMessage** (`model.fb.message.go`)
- Thiếu field sở hữu
- Ảnh hưởng: Không thể phân biệt message thuộc organization nào
- Mức độ: **CAO** - Dữ liệu quan trọng

#### 3. System Models

**Agent** (`model.auth.agent.go`)
- Thiếu field sở hữu
- Ảnh hưởng: Không thể phân biệt agent thuộc organization nào
- Mức độ: **TRUNG BÌNH** - Agent có thể được chia sẻ hoặc riêng biệt

**User** (`model.auth.user.go`)
- Thiếu field sở hữu trực tiếp
- Hiện tại: User thuộc Organization thông qua UserRole → Role → OrganizationID
- Mức độ: **THẤP** - Đã có cơ chế gián tiếp, nhưng có thể cần field trực tiếp cho primary organization

#### 4. Collections Có Thể Lấy Qua Relationship (Tùy Chọn)

**PcPosCategory** (`model.pc.pos.category.go`)
- Có `ShopId` → Có thể lấy `organizationId` từ `PcPosShop`
- **Khuyến nghị**: Thêm `organizationId` trực tiếp để tối ưu query (không cần join)
- Mức độ: **TRUNG BÌNH** - Có thể lấy qua Shop, nhưng nên thêm trực tiếp

**PcPosVariation** (`model.pc.pos.variation.go`)
- Có `ProductId` → Có thể lấy `organizationId` từ `PcPosProduct`
- **Khuyến nghị**: Thêm `organizationId` trực tiếp để tối ưu query
- Mức độ: **TRUNG BÌNH** - Có thể lấy qua Product, nhưng nên thêm trực tiếp

**PcOrder** (`model.pc.order.go`)
- Không có relationship rõ ràng
- **Khuyến nghị**: CẦN thêm `organizationId` nếu đơn hàng thuộc về organization
- Mức độ: **CAO** - Cần xác định business logic

**FbMessageItem** (`model.fb.message.item.go`)
- Có `ConversationId` → Có thể lấy `organizationId` từ `FbConversation`
- **Khuyến nghị**: Thêm `organizationId` trực tiếp để tối ưu query
- Mức độ: **TRUNG BÌNH** - Có thể lấy qua Conversation, nhưng nên thêm trực tiếp

#### 5. Collections Cần Xác Định Lại

**AccessTokens** (`model.pc.access_token.go`)
- **CẦN THÊM**: `OrganizationID primitive.ObjectID` với index `single:1`
- **Lý do**: Cần phân quyền theo organization
- **Mức độ**: **CAO** - Access tokens cần được phân quyền theo organization

**Customer** (`model.customer.go`)
- **CẦN THÊM**: `OrganizationID primitive.ObjectID` với index `single:1`
- **Lý do**: Cần phân quyền theo organization (nếu vẫn còn sử dụng)
- **Mức độ**: **CAO** - Customer data cần phân quyền
- **Lưu ý**: Theo comment có thể deprecated, nhưng nếu vẫn dùng thì cần thêm organizationId

## 🎯 Đề Xuất Phương Án

### Phương Án 1: Bổ Sung `organizationId` Cho Tất Cả Business Data Models (Khuyến Nghị)

#### Nguyên Tắc

1. **Bắt buộc (Required)**: Đối với dữ liệu nghiệp vụ chính
   - Customer, FbCustomer, PcPosCustomer
   - PcPosOrder, PcPosShop, PcPosProduct, PcPosWarehouse
   - FbPage, FbPost, FbConversation, FbMessage

2. **Nullable (Optional)**: Đối với dữ liệu có thể global
   - Agent (có thể global hoặc thuộc organization)

3. **Index**: Tất cả field `organizationId` cần có index để tối ưu query

#### Cấu Trúc Field

```go
// Cho các model bắt buộc
OrganizationID primitive.ObjectID `json:"organizationId" bson:"organizationId" index:"single:1"` // ID tổ chức sở hữu dữ liệu

// Cho các model optional
OrganizationID *primitive.ObjectID `json:"organizationId,omitempty" bson:"organizationId,omitempty" index:"single:1"` // ID tổ chức (null = global/shared)
```

#### Danh Sách Model Cần Bổ Sung

**Priority 1 - Business Critical (Bắt buộc):**
1. ✅ FbCustomer
2. ✅ PcPosCustomer
3. ✅ PcPosOrder
4. ✅ PcPosShop
5. ✅ PcPosProduct
6. ✅ PcPosWarehouse
7. ✅ FbPage
8. ✅ FbPost
9. ✅ FbConversation
10. ✅ FbMessage

**Priority 2 - Tối Ưu Query (Nên thêm):**
11. ✅ PcPosCategory (có thể lấy qua Shop, nhưng nên thêm trực tiếp)
12. ✅ PcPosVariation (có thể lấy qua Product, nhưng nên thêm trực tiếp)
13. ✅ FbMessageItem (có thể lấy qua Conversation, nhưng nên thêm trực tiếp)

**Priority 3 - Cần Xác Định Business Logic:**
14. ❓ PcOrder - Cần xác định đơn hàng có thuộc organization không

**Priority 4 - Optional:**
15. ⚠️ Agent (nullable - có thể global hoặc thuộc organization)

**Priority 5 - Cần Phân Quyền:**
16. ✅ AccessTokens - Cần phân quyền theo organization
17. ✅ Customer - Cần phân quyền theo organization (nếu vẫn còn sử dụng)

### Phương Án 2: Sử Dụng Relationship Qua PageId/ShopId

#### Ý Tưởng

Thay vì thêm `organizationId` vào mọi model, có thể:
- FbPage có `organizationId`
- Các model khác liên kết qua `pageId` → FbPage → `organizationId`

#### Ưu Điểm
- Giảm số lượng field cần thêm
- Tập trung ownership ở một nơi

#### Nhược Điểm
- Query phức tạp hơn (cần join)
- Không áp dụng được cho model không có `pageId` (như Customer, Agent)
- Performance kém hơn (cần lookup)

#### Kết Luận
**Không khuyến nghị** - Phương án 1 tốt hơn về performance và đơn giản hơn.

### Phương Án 3: Hybrid - Kết Hợp Cả Hai

- Model có `pageId` hoặc `shopId`: Dùng relationship
- Model không có: Thêm `organizationId` trực tiếp

#### Kết Luận
**Không khuyến nghị** - Tạo sự không nhất quán, khó maintain.

## 📝 Kế Hoạch Triển Khai

### Bước 1: Migration Script

Tạo script migration để:
1. Thêm field `organizationId` vào các collection
2. Gán giá trị mặc định cho dữ liệu cũ (có thể null hoặc organization mặc định)
3. Tạo index cho field mới

### Bước 2: Cập Nhật Models

1. Thêm field `OrganizationID` vào các model Go
2. Cập nhật index tags
3. Cập nhật validation logic

### Bước 3: Cập Nhật Services & Handlers

1. Thêm logic tự động gán `organizationId` khi tạo mới
2. Thêm filter theo `organizationId` trong các query
3. Thêm middleware để tự động filter theo organization của user hiện tại

### Bước 4: Cập Nhật API Documentation

1. Cập nhật schema documentation
2. Cập nhật API examples
3. Cập nhật migration guide

### Bước 5: Testing

1. Unit tests cho các model mới
2. Integration tests cho multi-tenant scenarios
3. Performance tests cho queries với index mới

## 🔒 Bảo Mật & Phân Quyền

### Scope Permissions trong RolePermission

Trong hệ thống, mỗi `RolePermission` có field `Scope` (byte) quy định phạm vi quyền:

- **Scope = 0 (Self)**: Chỉ thấy dữ liệu của organization mà role thuộc về
- **Scope = 1 (Children)**: Thấy dữ liệu của organization đó + tất cả các organization con (dùng Path regex)

**Lưu ý**: Không có Scope = 2. Nếu muốn xem tất cả dữ liệu, chỉ cần có role trong **System Organization** (root, level = -1) với Scope = 1. Vì System Organization là root, tất cả organizations khác đều là children của nó, nên Scope = 1 sẽ tự động bao phủ toàn bộ hệ thống.

### Logic Filter Theo Scope

Khi user thực hiện query, hệ thống cần:

1. **Lấy tất cả permissions của user** (từ cache hoặc database):
   - User → UserRole(s) → Role(s) → RolePermission(s) → Permission + Scope
   - Mỗi permission có scope riêng, gắn với organization của role

2. **Tính toán danh sách organizationIds được phép truy cập**:
   ```go
   // Pseudo code
   allowedOrgIDs := []primitive.ObjectID{}
   
   for each userRole {
       role := getRole(userRole.RoleID)
       orgID := role.OrganizationID
       
       for each rolePermission {
           if rolePermission.Scope == 0 {
               // Scope 0: Chỉ organization của role
               allowedOrgIDs = append(allowedOrgIDs, orgID)
           } else if rolePermission.Scope == 1 {
               // Scope 1: Organization + children
               allowedOrgIDs = append(allowedOrgIDs, orgID)
               childrenIDs := getChildrenIDs(orgID) // Dùng OrganizationService.GetChildrenIDs
               allowedOrgIDs = append(allowedOrgIDs, childrenIDs...)
               
               // Nếu role thuộc System Organization (root), childrenIDs sẽ bao gồm tất cả organizations
               // => Tự động có quyền xem tất cả
           }
       }
   }
   
   // Remove duplicates
   allowedOrgIDs = unique(allowedOrgIDs)
   ```

3. **Áp dụng filter vào query**:
   ```go
   // Luôn filter theo danh sách organizationIds
   // Nếu role thuộc System Organization với Scope = 1, allowedOrgIDs sẽ chứa tất cả organizations
   filter := bson.M{
       "$and": []bson.M{
           originalFilter,
           {"organizationId": bson.M{"$in": allowedOrgIDs}},
       },
   }
   ```

### Service Helper Function

Tạo helper function trong service để tính toán allowed organization IDs:

```go
// GetUserAllowedOrganizationIDs lấy danh sách organization IDs mà user có quyền truy cập
// dựa trên permissions và scope của user
// Nếu role thuộc System Organization với Scope = 1, sẽ trả về tất cả organizations
func (s *BaseService) GetUserAllowedOrganizationIDs(ctx context.Context, userID primitive.ObjectID, permissionName string) ([]primitive.ObjectID, error) {
    // 1. Lấy tất cả UserRoles của user
    userRoles, err := s.userRoleService.Find(ctx, bson.M{"userId": userID}, nil)
    if err != nil {
        return nil, err
    }
    
    allowedOrgIDs := make(map[primitive.ObjectID]bool)
    
    // 2. Duyệt qua từng role
    for _, userRole := range userRoles {
        // Lấy role
        role, err := s.roleService.FindOneById(ctx, userRole.RoleID)
        if err != nil {
            continue
        }
        
        orgID := role.OrganizationID
        
        // 3. Lấy RolePermissions của role
        rolePermissions, err := s.rolePermissionService.Find(ctx, bson.M{"roleId": role.ID}, nil)
        if err != nil {
            continue
        }
        
        // 4. Kiểm tra permission cụ thể
        for _, rp := range rolePermissions {
            permission, err := s.permissionService.FindOneById(ctx, rp.PermissionID)
            if err != nil {
                continue
            }
            
            // Chỉ xét permission được yêu cầu
            if permission.Name != permissionName {
                continue
            }
            
            // 5. Xử lý theo scope
            if rp.Scope == 0 {
                // Scope 0: Chỉ organization của role
                allowedOrgIDs[orgID] = true
            } else if rp.Scope == 1 {
                // Scope 1: Organization + children
                allowedOrgIDs[orgID] = true
                childrenIDs, err := s.organizationService.GetChildrenIDs(ctx, orgID)
                if err == nil {
                    for _, childID := range childrenIDs {
                        allowedOrgIDs[childID] = true
                    }
                }
                // Nếu role thuộc System Organization (level = -1), childrenIDs sẽ bao gồm tất cả
                // => Tự động có quyền xem tất cả organizations
            }
        }
    }
    
    // Convert map to slice
    result := make([]primitive.ObjectID, 0, len(allowedOrgIDs))
    for orgID := range allowedOrgIDs {
        result = append(result, orgID)
    }
    
    return result, nil
}
```

### Middleware Tự Động Filter

Tạo middleware hoặc helper function trong BaseHandler để tự động thêm filter:

```go
// applyOrganizationFilter thêm filter organizationId vào query dựa trên permission scope
func (h *BaseHandler[T, CreateInput, UpdateInput]) applyOrganizationFilter(
    c fiber.Ctx, 
    permissionName string,
    baseFilter map[string]interface{},
) (map[string]interface{}, error) {
    // Lấy user từ context
    userIDStr, ok := c.Locals("user_id").(string)
    if !ok {
        return nil, common.ErrUnauthorized
    }
    
    userID, err := primitive.ObjectIDFromHex(userIDStr)
    if err != nil {
        return nil, err
    }
    
    // Lấy allowed organization IDs
    allowedOrgIDs, err := h.getUserAllowedOrganizationIDs(c.Context(), userID, permissionName)
    if err != nil {
        return nil, err
    }
    
    // Nếu không có quyền truy cập organization nào, trả về filter rỗng (không có kết quả)
    if len(allowedOrgIDs) == 0 {
        return bson.M{"_id": bson.M{"$exists": false}}, nil // Filter không match gì cả
    }
    
    // Thêm filter organizationId
    // Nếu role thuộc System Organization với Scope = 1, allowedOrgIDs sẽ chứa tất cả organizations
    // => Filter vẫn hoạt động bình thường, nhưng sẽ match tất cả records
    if baseFilter == nil {
        baseFilter = make(map[string]interface{})
    }
    
    // Merge với filter hiện có
    baseFilter["organizationId"] = bson.M{"$in": allowedOrgIDs}
    
    return baseFilter, nil
}
```

### Tự Động Gán OrganizationId Khi Tạo Mới

Khi tạo mới record, cần xác định `organizationId` để gán cho record. Có 4 phương án chính:

## 🌍 Cách Các Tổ Chức Thế Giới Xử Lý

### Các Hệ Thống Lớn Sử Dụng Context Switching

1. **GitHub**: User chọn Organization/Workspace → Tất cả actions trong context đó
2. **Slack**: User chọn Workspace → Messages, channels trong workspace đó
3. **Microsoft 365**: User chọn Tenant → Data thuộc tenant đó
4. **AWS**: User chọn Account/Role → Resources trong account đó
5. **Google Workspace**: User chọn Organization → Data thuộc organization đó
6. **Notion**: User chọn Workspace → Pages trong workspace đó
7. **Figma**: User chọn Team → Files trong team đó

**Pattern chung**: User phải **chọn context (role/organization)** trước khi làm việc, context này được lưu trong session/header và áp dụng cho tất cả requests.

#### Phương Án 4: Context Switching - Chọn Role/Organization (Khuyến Nghị - Theo Chuẩn Quốc Tế)

**Ý tưởng**: User phải chọn một role (tương ứng với một organization) để làm việc. Context này được lưu và áp dụng cho tất cả requests.

**Flow:**
1. User đăng nhập → Lấy danh sách roles của user
2. User chọn role để làm việc → Lưu `activeRoleId` và `activeOrganizationId` vào session/header
3. Mỗi request tự động dùng `activeOrganizationId` để filter và gán

**Implementation:**

**A. Lưu Context trong Header (Stateless - Khuyến Nghị)**

```go
// Middleware để đọc và validate context từ header
func OrganizationContextMiddleware() fiber.Handler {
    return func(c fiber.Ctx) error {
        // Lấy activeRoleId từ header
        activeRoleIDStr := c.Get("X-Active-Role-ID")
        if activeRoleIDStr == "" {
            // Nếu không có, lấy role đầu tiên của user
            userIDStr, _ := c.Locals("user_id").(string)
            userRoles, err := getUserRoles(userIDStr)
            if err == nil && len(userRoles) > 0 {
                activeRoleIDStr = userRoles[0].RoleID.Hex()
            } else {
                return common.NewError(
                    common.ErrCodeAuthRole,
                    "Vui lòng chọn role để làm việc",
                    common.StatusBadRequest,
                    nil,
                )
            }
        }
        
        activeRoleID, err := primitive.ObjectIDFromHex(activeRoleIDStr)
        if err != nil {
            return common.NewError(
                common.ErrCodeValidationFormat,
                "X-Active-Role-ID không đúng định dạng",
                common.StatusBadRequest,
                nil,
            )
        }
        
        // Validate user có role này không
        userIDStr, _ := c.Locals("user_id").(string)
        userID, _ := primitive.ObjectIDFromHex(userIDStr)
        hasRole, err := validateUserHasRole(userID, activeRoleID)
        if err != nil || !hasRole {
            return common.NewError(
                common.ErrCodeAuthRole,
                "User không có quyền sử dụng role này",
                common.StatusForbidden,
                nil,
            )
        }
        
        // Lấy organization từ role
        role, err := getRole(activeRoleID)
        if err != nil {
            return err
        }
        
        // Lưu vào context
        c.Locals("active_role_id", activeRoleID)
        c.Locals("active_organization_id", role.OrganizationID)
        c.Locals("active_role", role)
        
        return c.Next()
    }
}

// Trong InsertOne handler
func (h *BaseHandler[T, CreateInput, UpdateInput]) InsertOne(c fiber.Ctx) error {
    // Lấy active organization từ context
    activeOrgID, ok := c.Locals("active_organization_id").(primitive.ObjectID)
    if !ok {
        return common.NewError(
            common.ErrCodeAuthRole,
            "Không xác định được organization context",
            common.StatusBadRequest,
            nil,
        )
    }
    
    // Gán organizationId vào model
    if model, ok := input.(interface{ SetOrganizationID(primitive.ObjectID) }); ok {
        model.SetOrganizationID(activeOrgID)
    }
    
    // ... continue insert ...
}
```

**B. API Endpoints**

```go
// GET /api/v1/auth/roles - Lấy danh sách roles của user
func GetUserRoles(c fiber.Ctx) error {
    userIDStr, _ := c.Locals("user_id").(string)
    userID, _ := primitive.ObjectIDFromHex(userIDStr)
    
    userRoles, err := getUserRolesWithDetails(userID)
    // Trả về: [{roleId, roleName, organizationId, organizationName, ...}]
    return c.JSON(userRoles)
}

// POST /api/v1/auth/switch-context - Chuyển đổi context (optional, nếu dùng session)
func SwitchContext(c fiber.Ctx) error {
    var req struct {
        RoleID string `json:"roleId"`
    }
    // ... validate và lưu vào session ...
}
```

**C. Frontend Implementation (Client-Side Context)**

```javascript
// Context được lưu ở CLIENT (localStorage/state), không phải server
// Mỗi client (browser tab, mobile app) có thể có context riêng

// 1. Sau khi login, lấy danh sách roles
const roles = await api.get('/auth/roles');

// 2. Nếu có nhiều roles, hiển thị cho user chọn
if (roles.length > 1) {
    const selectedRole = await showRoleSelector(roles);
    // Lưu vào localStorage (mỗi client có localStorage riêng)
    localStorage.setItem('activeRoleId', selectedRole.id);
    localStorage.setItem('activeOrganizationId', selectedRole.organizationId);
} else if (roles.length === 1) {
    // Tự động chọn role duy nhất
    localStorage.setItem('activeRoleId', roles[0].id);
    localStorage.setItem('activeOrganizationId', roles[0].organizationId);
}

// 3. Mỗi request gửi kèm header
axios.defaults.headers.common['X-Active-Role-ID'] = localStorage.getItem('activeRoleId');

// 4. User có thể đổi context bất cứ lúc nào
function switchContext(newRoleId) {
    localStorage.setItem('activeRoleId', newRoleId);
    // Reload data với context mới
    window.location.reload(); // hoặc update state
}
```

**D. Multi-Client Support (Quan Trọng)**

✅ **Một user có thể làm việc với nhiều client với nhiều role khác nhau:**

```
User A:
├── Browser Tab 1 → Role: Manager (Org: Company A)
├── Browser Tab 2 → Role: Employee (Org: Company B)  
├── Mobile App → Role: Admin (Org: System)
└── Desktop App → Role: Manager (Org: Company A)
```

**Cách hoạt động:**
1. Mỗi client (tab/device) có localStorage riêng
2. Mỗi client chọn và lưu context riêng
3. Mỗi request từ client gửi kèm `X-Active-Role-ID` của client đó
4. Backend validate mỗi request độc lập (stateless)
5. User có thể mở nhiều tab với các role khác nhau cùng lúc

**Ví dụ thực tế:**
- Tab 1: User làm việc với Company A (Role: Manager)
- Tab 2: User làm việc với Company B (Role: Employee)
- Cả 2 tab hoạt động độc lập, không ảnh hưởng nhau

**Backend không cần lưu session** - Mỗi request tự validate:
```go
// Mỗi request validate độc lập
func OrganizationContextMiddleware() fiber.Handler {
    // 1. Đọc X-Active-Role-ID từ header
    // 2. Validate user có role đó không
    // 3. Lưu vào context cho request này
    // Không cần lưu vào database/session
}
```

**Ưu điểm:**
- ✅ Rõ ràng: User biết đang làm việc với organization nào
- ✅ An toàn: Validate user có role đó trước khi dùng
- ✅ Linh hoạt: User có thể đổi context khi cần
- ✅ Theo chuẩn: Giống GitHub, Slack, Microsoft 365
- ✅ Stateless: Dùng header, không cần session storage

**Nhược điểm:**
- ⚠️ User phải chọn role (nhưng chỉ 1 lần, lưu vào localStorage)
- ⚠️ Nếu user chỉ có 1 role, vẫn phải gửi header (có thể tự động)

**Cải tiến:**
- Nếu user chỉ có 1 role → Tự động chọn, không cần user chọn
- Nếu user có nhiều roles → Bắt buộc chọn (hoặc dùng role đầu tiên làm default)

#### Phương Án 1: User Gửi organizationId (Fallback)

Cho phép user gửi `organizationId` trong request body, nhưng phải validate user có quyền với organization đó:

```go
// Trong InsertOne handler
func (h *BaseHandler[T, CreateInput, UpdateInput]) InsertOne(c fiber.Ctx) error {
    // ... parse input ...
    
    // Lấy user từ context
    userIDStr, ok := c.Locals("user_id").(string)
    if !ok {
        return common.ErrUnauthorized
    }
    
    userID, err := primitive.ObjectIDFromHex(userIDStr)
    if err != nil {
        return err
    }
    
    // Lấy permission name từ route (ví dụ: "Customer.Create")
    permissionName := c.Locals("permission").(string) // Cần lưu trong middleware
    
    // Lấy allowed organization IDs của user
    allowedOrgIDs, err := h.getUserAllowedOrganizationIDs(c.Context(), userID, permissionName)
    if err != nil {
        return err
    }
    
    var targetOrgID primitive.ObjectID
    
    // 1. Kiểm tra nếu input có organizationId
    if orgIDFromInput := h.getOrganizationIDFromInput(input); orgIDFromInput != nil {
        // Validate user có quyền với organization này không
        hasPermission := false
        for _, allowedID := range allowedOrgIDs {
            if allowedID == *orgIDFromInput {
                hasPermission = true
                break
            }
        }
        
        if !hasPermission {
            return common.NewError(
                common.ErrCodeAuthRole,
                "Không có quyền tạo dữ liệu cho organization này",
                common.StatusForbidden,
                nil,
            )
        }
        
        targetOrgID = *orgIDFromInput
    } else {
        // 2. Nếu không có, lấy từ role đầu tiên của user
        userRoles, err := h.userRoleService.Find(c.Context(), bson.M{"userId": userID}, nil)
        if err != nil || len(userRoles) == 0 {
            return common.NewError(
                common.ErrCodeAuthRole,
                "User không có role nào",
                common.StatusForbidden,
                nil,
            )
        }
        
        role, err := h.roleService.FindOneById(c.Context(), userRoles[0].RoleID)
        if err != nil {
            return err
        }
        
        targetOrgID = role.OrganizationID
    }
    
    // Gán organizationId vào model
    if model, ok := input.(interface{ SetOrganizationID(primitive.ObjectID) }); ok {
        model.SetOrganizationID(targetOrgID)
    }
    
    // ... continue insert ...
}
```

#### Phương Án 2: Tự Động Từ Role Đầu Tiên

Luôn lấy từ role đầu tiên của user (đơn giản hơn nhưng ít linh hoạt):

```go
// Helper function
func (h *BaseHandler[T, CreateInput, UpdateInput]) getUserPrimaryOrganizationID(ctx context.Context, userID primitive.ObjectID) (primitive.ObjectID, error) {
    userRoles, err := h.userRoleService.Find(ctx, bson.M{"userId": userID}, nil)
    if err != nil || len(userRoles) == 0 {
        return primitive.NilObjectID, common.NewError(
            common.ErrCodeAuthRole,
            "User không có role nào",
            common.StatusForbidden,
            nil,
        )
    }
    
    role, err := h.roleService.FindOneById(ctx, userRoles[0].RoleID)
    if err != nil {
        return primitive.NilObjectID, err
    }
    
    return role.OrganizationID, nil
}
```

#### Phương Án 3: Từ Header/Query Parameter

Cho phép gửi qua header `X-Organization-ID` hoặc query parameter `organizationId`:

```go
// Lấy từ header hoặc query
orgIDStr := c.Get("X-Organization-ID")
if orgIDStr == "" {
    orgIDStr = c.Query("organizationId")
}

if orgIDStr != "" {
    orgID, err := primitive.ObjectIDFromHex(orgIDStr)
    if err == nil {
        // Validate và sử dụng
    }
}
```

#### So Sánh Các Phương Án

| Tiêu chí | Phương Án 4 (Context) | Phương Án 1 (Gửi ID) | Phương Án 2 (Tự động) | Phương Án 3 (Header) |
|----------|----------------------|---------------------|---------------------|---------------------|
| **Rõ ràng** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐ |
| **An toàn** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| **Linh hoạt** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐⭐ |
| **UX** | ⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ |
| **Theo chuẩn** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐⭐ |
| **Độ phức tạp** | Trung bình | Thấp | Rất thấp | Trung bình |

#### Khuyến Nghị

**Sử dụng Phương Án 4 (Context Switching)** vì:
- ✅ Theo chuẩn quốc tế: GitHub, Slack, Microsoft 365 đều dùng cách này
- ✅ Rõ ràng: User biết đang làm việc với organization nào
- ✅ An toàn: Validate role trước khi dùng
- ✅ Linh hoạt: User có thể đổi context khi cần
- ✅ Stateless: Dùng header, không cần session storage

**Fallback Strategy:**
- Nếu user chỉ có 1 role → Tự động chọn, không cần user chọn
- Nếu user có nhiều roles → Bắt buộc chọn (hoặc dùng role đầu tiên làm default)
- Nếu không có header → Lấy role đầu tiên (backward compatibility)

**Lưu ý quan trọng:**
- User có thể có nhiều roles thuộc nhiều organizations khác nhau
- Phải validate user có quyền với organization được chọn (dựa trên `allowedOrgIDs`)
- Nếu user chỉ có 1 role, tự động dùng organization của role đó
- Nếu user có nhiều roles, ưu tiên organization từ input, nếu không có thì dùng role đầu tiên

## 🔄 Hierarchical Data Sharing - Dữ Liệu Dùng Chung

### Vấn Đề

**Scenario:**
- 2 team sale (Team A và Team B) cùng cần xem khách hàng chung
- Nếu để dữ liệu ở cấp Team → Team khác không thấy
- Nếu để dữ liệu ở cấp Company → Nhân viên cấp thấp (Scope 0) không truy cập được

**Cấu trúc tổ chức:**
```
Company (Level 1)
├── Sales Department (Level 2)
│   ├── Team A (Level 3)
│   └── Team B (Level 3)
└── Marketing Department (Level 2)
```

### Cách Các Tổ Chức Lớn Giải Quyết

#### 1. **Hierarchical Data Ownership** (Khuyến Nghị)

**Nguyên tắc**: Dữ liệu có thể thuộc về parent organization, và được chia sẻ với children thông qua Scope.

**Ví dụ:**
- Khách hàng chung → Thuộc **Company** (Level 1)
- Khách hàng riêng Team A → Thuộc **Team A** (Level 3)

**Access Control:**
- User có role ở **Company** với **Scope = 1** → Thấy tất cả khách hàng của Company + tất cả teams
- User có role ở **Team A** với **Scope = 0** → Chỉ thấy khách hàng của Team A
- User có role ở **Team A** với **Scope = 1** → Thấy khách hàng của Team A + các team con (nếu có)

**Implementation:**
```go
// Khi tạo khách hàng, user chọn organization level phù hợp
// - Khách hàng chung → Company level
// - Khách hàng riêng → Team level

// Filter tự động dựa trên scope:
// - Scope 0: Chỉ organization của role
// - Scope 1: Organization + children (tự động share với children)
```

#### 2. **Shared Workspaces** (Advanced)

Một số hệ thống cho phép "share" dữ liệu giữa các organizations:

**Option A: Field `SharedWith`**
```go
type Customer struct {
    OrganizationID primitive.ObjectID   `json:"organizationId" bson:"organizationId"`
    SharedWith     []primitive.ObjectID `json:"sharedWith,omitempty" bson:"sharedWith,omitempty"` // Danh sách organizations được share
}
```

**Option B: Field `VisibilityLevel`**
```go
type Customer struct {
    OrganizationID primitive.ObjectID `json:"organizationId" bson:"organizationId"`
    VisibilityLevel string            `json:"visibilityLevel" bson:"visibilityLevel"` // "private", "team", "department", "company"
}
```

**Nhược điểm:**
- Phức tạp hơn
- Cần logic phức tạp để query
- Không tận dụng được scope hiện có

#### 3. **Permission Inheritance** (Đã có sẵn)

Hệ thống hiện tại đã có **Scope = 1** cho phép xem children:
- Role ở **Company** với **Scope = 1** → Tự động thấy tất cả teams
- Role ở **Department** với **Scope = 1** → Tự động thấy tất cả teams trong department

### Vấn Đề Với Scope Hiện Tại

**Scope = 1 chỉ cho phép xem từ cha xuống con (parent → children), KHÔNG phải từ con lên cha (children → parent):**

```
Cấu trúc:
Sales Department (Level 2)
├── Team A (Level 3)
└── Team B (Level 3)

Khách hàng chung ở Sales Department:
- organizationId: Sales Department ID
- User Team A (Scope 1) → Chỉ thấy Team A + children của Team A
- User Team A KHÔNG thể thấy dữ liệu của parent (Sales Department) ❌
```

### Giải Pháp: Inverse Lookup - Tìm Parent Organizations

**Nguyên tắc**: Khi query, cần tìm cả **parent organizations** của organization hiện tại.

**Logic mới:**

1. **Lấy allowedOrgIDs từ scope** (như hiện tại)
2. **Thêm parent organizations** vào allowedOrgIDs
3. **Query filter**: `organizationId IN [allowedOrgIDs + parentOrgIDs]`

**Implementation:**

```go
// GetParentIDs lấy tất cả ID của organization cha (dùng cho inverse lookup)
func (s *OrganizationService) GetParentIDs(ctx context.Context, childID primitive.ObjectID) ([]primitive.ObjectID, error) {
    // Lấy organization con
    child, err := s.FindOneById(ctx, childID)
    if err != nil {
        return nil, err
    }
    
    if child.ParentID == nil {
        // Không có parent (root)
        return []primitive.ObjectID{}, nil
    }
    
    parentIDs := []primitive.ObjectID{}
    currentID := *child.ParentID
    
    // Đi ngược lên cây để lấy tất cả parents
    for {
        parent, err := s.FindOneById(ctx, currentID)
        if err != nil {
            break
        }
        
        parentIDs = append(parentIDs, parent.ID)
        
        if parent.ParentID == nil {
            break // Đã đến root
        }
        
        currentID = *parent.ParentID
    }
    
    return parentIDs, nil
}

// GetUserAllowedOrganizationIDs - Cập nhật để bao gồm parents
func GetUserAllowedOrganizationIDs(ctx context.Context, userID primitive.ObjectID, permissionName string) ([]primitive.ObjectID, error) {
    // ... logic hiện tại để lấy allowedOrgIDs từ scope ...
    
    // Thêm parent organizations
    allAllowedOrgIDs := make(map[primitive.ObjectID]bool)
    
    for _, orgID := range allowedOrgIDs {
        allAllowedOrgIDs[orgID] = true
        
        // Lấy parents của organization này
        parentIDs, err := organizationService.GetParentIDs(ctx, orgID)
        if err == nil {
            for _, parentID := range parentIDs {
                allAllowedOrgIDs[parentID] = true
            }
        }
    }
    
    // Convert to slice
    result := make([]primitive.ObjectID, 0, len(allAllowedOrgIDs))
    for orgID := range allAllowedOrgIDs {
        result = append(result, orgID)
    }
    
    return result, nil
}
```

**Ví dụ với logic mới:**

```
Cấu trúc:
Sales Department (Level 2, ID: dept_123)
├── Team A (Level 3, ID: team_a)
└── Team B (Level 3, ID: team_b)

Khách hàng chung:
- organizationId: dept_123 (Sales Department)

User Team A (Scope 0):
- allowedOrgIDs từ scope: [team_a]
- parentOrgIDs: [dept_123] (parent của team_a)
- Final allowedOrgIDs: [team_a, dept_123]
- Query: organizationId IN [team_a, dept_123]
- → Thấy được khách hàng chung ✅

User Team A (Scope 1):
- allowedOrgIDs từ scope: [team_a, children_of_team_a]
- parentOrgIDs: [dept_123]
- Final allowedOrgIDs: [team_a, children_of_team_a, dept_123]
- → Thấy được khách hàng chung ✅
```

### Giải Pháp Khuyến Nghị

**Sử dụng Hierarchical Data Ownership + Inverse Parent Lookup:**

1. **Dữ liệu chung → Để ở cấp cao hơn (Company/Department)**
   - Khách hàng chung của 2 team sale → Thuộc **Sales Department**
   - User Team A → Tự động thấy (vì Department là parent của Team A)

2. **Dữ liệu riêng → Để ở cấp thấp (Team)**
   - Khách hàng riêng Team A → Thuộc **Team A**
   - User Team A → Thấy được
   - User Team B → Không thấy (trừ khi có Scope 1 ở Department level)

3. **Query Logic:**
   - Lấy allowedOrgIDs từ scope (như hiện tại)
   - Thêm parent organizations vào allowedOrgIDs
   - Filter: `organizationId IN [allowedOrgIDs + parentOrgIDs]`

**Ví dụ thực tế:**

```
Khách hàng "ABC Corp" (chung cho cả 2 team):
- organizationId: Sales Department ID
- User Team A (Scope 0) → Thấy được ✅ (Department là parent của Team A)
- User Team B (Scope 0) → Thấy được ✅ (Department là parent của Team B)

Khách hàng "XYZ Ltd" (riêng Team A):
- organizationId: Team A ID
- User Team A (Scope 0) → Thấy được ✅
- User Team B (Scope 0) → KHÔNG thấy ❌ (Team B không phải parent của Team A)
```

### Implementation

**Cần thêm method `GetParentIDs()`:**

1. ✅ Organization hierarchy (parent-child) - Đã có
2. ✅ Scope 0 (self) và Scope 1 (children) - Đã có
3. ✅ `GetChildrenIDs()` - Đã có sẵn
4. ❌ `GetParentIDs()` - **CẦN THÊM** để inverse lookup

**Cần làm:**
1. Thêm method `GetParentIDs()` vào `OrganizationService`
2. Cập nhật `GetUserAllowedOrganizationIDs()` để bao gồm parent organizations
3. User chọn organization level phù hợp khi tạo dữ liệu
4. Frontend hỗ trợ chọn organization từ danh sách organizations user có quyền
5. Backend validate và gán `organizationId` tương ứng

## 🤝 Collaborative Data - Dữ Liệu Cộng Tác

### Vấn Đề

**Scenario:**
- Khách hàng "ABC Corp" là dữ liệu chung, nhiều bộ phận cùng đóng góp:
  - Nhân viên MKT góp ý về chiến dịch marketing
  - Nhân viên Sale ghi chú về lịch sử gặp gỡ
  - Nhân viên Kho ghi chú về đơn hàng
- Tất cả đều cần xem và chỉnh sửa cùng một record khách hàng

**Cấu trúc tổ chức:**
```
Company (Level 1)
├── Marketing Department (Level 2)
├── Sales Department (Level 2)
│   ├── Team A (Level 3)
│   └── Team B (Level 3)
└── Warehouse Department (Level 2)
```

### Cách Các Tổ Chức Lớn Giải Quyết

#### 1. **Shared Ownership + Activity/Notes Pattern** (Khuyến Nghị - Như Salesforce, HubSpot)

**Nguyên tắc:**
- Dữ liệu chính (Customer) thuộc về **parent organization** (Company/Department)
- Mỗi bộ phận thêm **Notes/Activities/Comments** vào dữ liệu chung
- Tất cả bộ phận có quyền xem và đóng góp

**Ví dụ:**
```
Customer "ABC Corp":
- organizationId: Company ID (Level 1) - Dữ liệu chung
- Notes: [
    {userId: mkt_user, organizationId: mkt_dept, content: "Góp ý marketing"},
    {userId: sale_user, organizationId: sale_dept, content: "Ghi chú sale"},
    {userId: warehouse_user, organizationId: warehouse_dept, content: "Ghi chú kho"}
  ]
```

**Access Control:**
- User có role ở bất kỳ organization nào trong Company → Thấy được customer
- User có thể thêm notes/activities vào customer
- Notes có `organizationId` để track bộ phận nào đóng góp

#### 2. **Workspace/Project-Based** (Như Notion, Asana)

**Nguyên tắc:**
- Dữ liệu thuộc về một **Workspace/Project**
- Nhiều teams được mời vào workspace
- Tất cả teams trong workspace có quyền xem và chỉnh sửa

**Implementation:**
```go
type Customer struct {
    OrganizationID primitive.ObjectID `json:"organizationId"` // Workspace/Project organization
    SharedWith     []primitive.ObjectID `json:"sharedWith"`   // Teams được mời
}
```

**Nhược điểm:**
- Phức tạp hơn
- Cần quản lý danh sách `sharedWith`

#### 3. **Multi-Organization Ownership** (Như GitHub Organizations)

**Nguyên tắc:**
- Dữ liệu có thể thuộc nhiều organizations
- Mỗi organization có quyền xem và chỉnh sửa

**Implementation:**
```go
type Customer struct {
    OrganizationIDs []primitive.ObjectID `json:"organizationIds"` // Nhiều organizations
}
```

**Nhược điểm:**
- Query phức tạp hơn (cần `$in` với array)
- Khó quản lý ownership

### Giải Pháp Khuyến Nghị: Shared Ownership + Activity Pattern

**Sử dụng kết hợp 2 patterns:**

#### Pattern 1: Dữ Liệu Chính Thuộc Parent Organization

```
Customer "ABC Corp":
- organizationId: Company ID (Level 1) - Dữ liệu chung
- Tất cả bộ phận trong Company đều thấy được (nhờ Inverse Parent Lookup)
```

#### Pattern 2: Activity/Notes Collection Riêng

```go
// CustomerActivity - Lưu các hoạt động/ghi chú của từng bộ phận
type CustomerActivity struct {
    ID             primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
    CustomerID     primitive.ObjectID `json:"customerId" bson:"customerId" index:"single:1"`
    OrganizationID primitive.ObjectID `json:"organizationId" bson:"organizationId" index:"single:1"` // Bộ phận đóng góp
    UserID         primitive.ObjectID `json:"userId" bson:"userId" index:"single:1"`                 // User đóng góp
    Type           string             `json:"type" bson:"type"`                                      // "note", "comment", "activity"
    Content        string             `json:"content" bson:"content"`
    CreatedAt      int64              `json:"createdAt" bson:"createdAt"`
}
```

**Ví dụ thực tế:**

```
Customer "ABC Corp":
- organizationId: Company ID
- Tất cả bộ phận thấy được

CustomerActivity:
- {customerId: abc_corp, organizationId: mkt_dept, userId: mkt_user, content: "Góp ý marketing"}
- {customerId: abc_corp, organizationId: sale_dept, userId: sale_user, content: "Ghi chú sale"}
- {customerId: abc_corp, organizationId: warehouse_dept, userId: warehouse_user, content: "Ghi chú kho"}
```

**Query:**
```go
// Lấy customer
customer := getCustomer(customerId)

// Lấy tất cả activities của customer
activities := getCustomerActivities(customerId)

// Filter activities theo organization nếu cần
mktActivities := filterActivitiesByOrg(activities, mktDeptID)
```

### Implementation

**Option A: Activity Collection Riêng (Khuyến Nghị)**

```go
// Collection: customer_activities
type CustomerActivity struct {
    ID             primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
    CustomerID     primitive.ObjectID `json:"customerId" bson:"customerId" index:"single:1"`
    OrganizationID primitive.ObjectID `json:"organizationId" bson:"organizationId" index:"single:1"`
    UserID         primitive.ObjectID `json:"userId" bson:"userId" index:"single:1"`
    Type           string             `json:"type" bson:"type"` // "note", "comment", "activity"
    Content        string             `json:"content" bson:"content"`
    Metadata       map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"`
    CreatedAt      int64              `json:"createdAt" bson:"createdAt"`
}
```

**Option B: Embedded Activities trong Customer**

```go
type Customer struct {
    // ... fields hiện tại ...
    OrganizationID primitive.ObjectID `json:"organizationId" bson:"organizationId"`
    Activities     []CustomerActivity `json:"activities,omitempty" bson:"activities,omitempty"` // Embedded
}
```

**Khuyến nghị: Option A** vì:
- Tách biệt concerns
- Dễ query và filter
- Không làm document quá lớn
- Có thể scale tốt hơn

### Best Practices

1. **Dữ liệu chung → Cấp cao (Company/Department)**
   - Customer chung → Company level
   - Tất cả bộ phận thấy được (nhờ Inverse Parent Lookup)

2. **Activities/Notes → Collection riêng**
   - Mỗi bộ phận thêm notes vào collection riêng
   - Track `organizationId` và `userId` để biết ai đóng góp

3. **Sử dụng Scope = 1 cho managers** để tự động thấy children
4. **Sử dụng Scope = 0 cho employees** + Inverse Parent Lookup để thấy parent data

## 🔗 Kết Hợp Dữ Liệu Riêng & Dữ Liệu Chung Trong Hệ Thống Phân Cấp

### Tổng Quan

Hệ thống cần hỗ trợ **cả 2 loại dữ liệu**:
1. **Dữ liệu riêng** (Team level) - Chỉ team đó thấy và quản lý
2. **Dữ liệu chung** (Company/Department level) - Nhiều teams cùng thấy và đóng góp

### Cấu Trúc Tổ Chức

```
Company (Level 1, ID: company_123)
├── Marketing Department (Level 2, ID: mkt_dept)
├── Sales Department (Level 2, ID: sales_dept)
│   ├── Team A (Level 3, ID: team_a)
│   └── Team B (Level 3, ID: team_b)
└── Warehouse Department (Level 2, ID: warehouse_dept)
```

### Quy Tắc Phân Loại Dữ Liệu

#### 1. Dữ Liệu Riêng (Private Data)
**Thuộc về:** Team/Division level (Level 3+)

**Đặc điểm:**
- Chỉ team đó sở hữu và quản lý
- Các teams khác không thấy (trừ khi có Scope 1 ở parent level)
- Ví dụ: Khách hàng riêng của Team A, không chia sẻ với Team B

**Ví dụ:**
```
Customer "XYZ Ltd" (riêng Team A):
- organizationId: team_a (Level 3)
- Chỉ Team A thấy được
- Team B không thấy (trừ manager có Scope 1 ở sales_dept)
```

#### 2. Dữ Liệu Chung (Shared Data)
**Thuộc về:** Company/Department level (Level 1-2)

**Đặc điểm:**
- Nhiều teams cùng sở hữu và đóng góp
- Tất cả teams trong parent organization đều thấy được
- Mỗi team có thể thêm activities/notes riêng

**Ví dụ:**
```
Customer "ABC Corp" (chung cho cả Sales Department):
- organizationId: sales_dept (Level 2)
- Team A thấy được ✅ (vì sales_dept là parent của team_a)
- Team B thấy được ✅ (vì sales_dept là parent của team_b)
- Cả 2 teams có thể thêm notes/activities
```

### Logic Query Kết Hợp (Đơn Giản - Tự Động)

**✅ NGUYÊN TẮC ĐƠN GIẢN: Tự động xem được dữ liệu cấp trên (trong cùng cây)**

**Nguyên tắc:**
1. **Dữ liệu riêng** → Để ở cấp thấp nhất (Team/Division level)
   - Chỉ team đó và các teams con (nếu có Scope 1) thấy được
   
2. **Dữ liệu chung** → Để ở cấp trên (Department/Company level)
   - Tất cả teams trong parent organization **tự động thấy được**
   - Không cần permission, không cần đánh dấu `isShared`

3. **Tự động thấy parent data** → User tự động thấy dữ liệu của tất cả parent organizations (trong cùng cây)

**Logic Query (Đơn Giản):**

```go
// GetUserAllowedOrganizationIDs - Tự động bao gồm parent
func GetUserAllowedOrganizationIDs(ctx context.Context, userID primitive.ObjectID, permissionName string) ([]primitive.ObjectID, error) {
    allowedOrgIDs := []primitive.ObjectID{}
    
    // 1. Lấy allowedOrgIDs từ scope (như hiện tại)
    // - Scope 0: [team_a]
    // - Scope 1: [team_a, children_of_team_a]
    
    // 2. Tự động thêm parent organizations (KHÔNG cần permission)
    parentOrgIDs := []primitive.ObjectID{}
    for _, orgID := range allowedOrgIDs {
        parents, _ := organizationService.GetParentIDs(ctx, orgID)
        parentOrgIDs = append(parentOrgIDs, parents...)
    }
    
    // 3. Kết hợp: allowedOrgIDs + parentOrgIDs
    finalOrgIDs := unique(append(allowedOrgIDs, parentOrgIDs...))
    
    return finalOrgIDs, nil
}

// Query filter - Đơn giản, không cần isShared
filter := bson.M{
    "organizationId": bson.M{"$in": finalOrgIDs}
}
```

**Kết quả:**
- User Team A (Scope 0):
  - ✅ Dữ liệu riêng của Team A (`organizationId = team_a`)
  - ✅ Dữ liệu chung của Sales Department (`organizationId = sales_dept`) - **Tự động thấy**
  - ✅ Dữ liệu chung của Company (`organizationId = company_123`) - **Tự động thấy**
  - ❌ Dữ liệu của Team B (`organizationId = team_b`) - **KHÔNG thấy** (không phải parent)

- User Team A (Scope 1):
  - ✅ Tất cả dữ liệu trên
  - ✅ Dữ liệu của các teams con (nếu có)

### Ví Dụ Thực Tế

**Scenario 1: User Team A (Scope 0) - Tự động thấy parent data**

```
Cấu trúc:
Company (company_123)
└── Sales Department (sales_dept)
    ├── Team A (team_a) ← User ở đây
    └── Team B (team_b)

Dữ liệu:
1. Customer "XYZ Ltd" - organizationId: team_a (riêng Team A)
2. Customer "ABC Corp" - organizationId: sales_dept (chung Sales Department)
3. Customer "DEF Inc" - organizationId: company_123 (chung Company)
4. Customer "GHI Ltd" - organizationId: team_b (riêng Team B)

Query của User Team A:
- allowedOrgIDs từ scope: [team_a]
- Tự động thêm parentOrgIDs: [sales_dept, company_123]
- finalOrgIDs: [team_a, sales_dept, company_123]

Kết quả:
✅ Customer "XYZ Ltd" - Thấy được (riêng Team A)
✅ Customer "ABC Corp" - Thấy được (chung Sales Department) - Tự động thấy
✅ Customer "DEF Inc" - Thấy được (chung Company) - Tự động thấy
❌ Customer "GHI Ltd" - KHÔNG thấy (riêng Team B, không phải parent)
```

**Scenario 2: User Team A (Scope 1) - Thấy cả children**

```
Cấu trúc: (giống như trên)

Dữ liệu: (giống như trên)

Query của User Team A:
- allowedOrgIDs từ scope: [team_a, children_of_team_a] (nếu có)
- Tự động thêm parentOrgIDs: [sales_dept, company_123]
- finalOrgIDs: [team_a, children_of_team_a, sales_dept, company_123]

Kết quả:
✅ Customer "XYZ Ltd" - Thấy được (riêng Team A)
✅ Customer "ABC Corp" - Thấy được (chung Sales Department)
✅ Customer "DEF Inc" - Thấy được (chung Company)
❌ Customer "GHI Ltd" - KHÔNG thấy (riêng Team B, không phải parent/children)
```

**Scenario 3: Multi-Company (Công ty khác nhau) - Chỉ thấy trong cùng cây**

```
Cấu trúc:
Group (group_123)
├── Company A (company_a)
│   └── Sales Department (sales_dept_a)
│       └── Team A (team_a) ← User ở đây
└── Company B (company_b)
    └── Sales Department (sales_dept_b)

Dữ liệu:
1. Customer "XYZ Ltd" - organizationId: team_a (riêng Team A)
2. Customer "ABC Corp" - organizationId: sales_dept_a (chung Sales Department A)
3. Customer "DEF Inc" - organizationId: company_a (chung Company A)
4. Customer "GHI Ltd" - organizationId: company_b (riêng Company B)

Query của User Team A:
- allowedOrgIDs từ scope: [team_a]
- Tự động thêm parentOrgIDs: [sales_dept_a, company_a, group_123]
- finalOrgIDs: [team_a, sales_dept_a, company_a, group_123]

Kết quả:
✅ Customer "XYZ Ltd" - Thấy được (riêng Team A)
✅ Customer "ABC Corp" - Thấy được (chung Sales Department A) - Tự động thấy
✅ Customer "DEF Inc" - Thấy được (chung Company A) - Tự động thấy
❌ Customer "GHI Ltd" - KHÔNG thấy (riêng Company B, không phải parent trong cùng cây)
```

### Quy Tắc Chọn Organization Level Khi Tạo Dữ Liệu

**Nguyên tắc đơn giản:**
- **Dữ liệu riêng** → Để ở cấp thấp nhất (Team/Division level)
- **Dữ liệu chung** → Để ở cấp trên (Department/Company level)

**Frontend cho phép user chọn:**

1. **Dữ liệu riêng** → Chọn Team/Division level (cấp thấp nhất)
   - Chỉ team đó và các teams con (nếu có Scope 1) thấy được
   - Các teams khác không thấy

2. **Dữ liệu chung** → Chọn Department/Company level (cấp trên)
   - Tất cả teams trong parent organization **tự động thấy được**
   - Không cần đánh dấu gì, chỉ cần để ở cấp trên

**UI/UX:**
```
Tạo khách hàng mới:
┌─────────────────────────────┐
│ Tên khách hàng: [ABC Corp] │
│                             │
│ Thuộc tổ chức:             │
│ ○ Riêng Team A             │
│ ● Chung Sales Department   │ ← User chọn (cấp trên)
│ ○ Chung Company            │
│                             │
│ [Tạo]                      │
└─────────────────────────────┘
```

**Backend tự động gán:**
```go
// User chọn organization level
customer.OrganizationID = selectedOrgID

// Không cần field isShared nữa
// Logic đơn giản: Dữ liệu ở cấp trên tự động visible cho cấp dưới
```

### Kết Hợp Với Activity Pattern

**Dữ liệu chung + Activities:**

```
Customer "ABC Corp":
- organizationId: sales_dept (Level 2) - Dữ liệu chung
- Tất cả teams trong Sales Department thấy được

CustomerActivity:
- {customerId: abc_corp, organizationId: team_a, userId: sale_user_a, content: "Ghi chú từ Team A"}
- {customerId: abc_corp, organizationId: team_b, userId: sale_user_b, content: "Ghi chú từ Team B"}
- {customerId: abc_corp, organizationId: mkt_dept, userId: mkt_user, content: "Góp ý marketing"}

Query activities:
- User Team A → Thấy tất cả activities (vì customer thuộc sales_dept, parent của team_a)
- Có thể filter theo organizationId nếu chỉ muốn xem activities của team mình
```

### Implementation Summary

**1. Query Logic (Đơn giản - Tự động):**
```go
// User query customers
allowedOrgIDs = [team_a] // Từ scope

// Tự động thêm parent organizations (KHÔNG cần permission)
parentOrgIDs = [sales_dept, company_123] // Inverse lookup
finalOrgIDs = [team_a, sales_dept, company_123]

// Filter đơn giản
filter = {"organizationId": {"$in": finalOrgIDs}}

// Kết quả: 
// - Dữ liệu riêng của team mình
// - Dữ liệu chung của tất cả parent organizations (tự động thấy)
```

**2. Create Logic (User chọn - Đơn giản):**
```go
// User tạo customer - Chỉ cần chọn organization level
customer.OrganizationID = selectedOrgID

// Không cần field isShared
// Logic: Dữ liệu ở cấp trên tự động visible cho cấp dưới
```

**3. Activity Pattern (Cho dữ liệu chung):**
```go
// User thêm note vào customer chung
activity := CustomerActivity{
    CustomerID: customerId,
    OrganizationID: team_a, // Team đóng góp
    UserID: userId,
    Content: "Ghi chú từ Team A",
}
```

### Best Practices Kết Hợp

1. **Dữ liệu riêng** → Team/Division level (Level 3+) - Cấp thấp nhất
2. **Dữ liệu chung** → Department/Company level (Level 1-2) - Cấp trên
3. **Query tự động** → Tự động bao gồm parent organizations (không cần permission)
4. **Activities** → Collection riêng, track `organizationId` của team đóng góp
5. **UI** → Cho phép user chọn organization level khi tạo dữ liệu
6. **Không cần `isShared`** → Logic đơn giản: Cấp trên tự động visible cho cấp dưới

### Kết Luận

**Hệ thống đơn giản và tự nhiên:**
- ✅ Dữ liệu riêng: Để ở Team level (cấp thấp nhất) → Chỉ team đó và children thấy
- ✅ Dữ liệu chung: Để ở Department/Company level (cấp trên) → Tất cả teams trong parent tree tự động thấy
- ✅ Query tự động: Tự động bao gồm parent organizations → User tự động thấy dữ liệu cấp trên
- ✅ Đơn giản: Không cần permission ViewParent, không cần field `isShared`
- ✅ Bảo mật: Chỉ thấy trong cùng cây (hierarchical), không thấy sibling organizations
- ✅ Activities: Collection riêng cho dữ liệu chung → Mỗi team đóng góp độc lập

**Cần implement:**
1. Thêm `GetParentIDs()` để inverse lookup
2. Cập nhật `GetUserAllowedOrganizationIDs()` để tự động thêm parent organizations
3. User chọn organization level phù hợp khi tạo dữ liệu
4. Frontend hỗ trợ chọn organization level
5. **KHÔNG cần** field `isShared` và permission `Data.ViewParent` nữa

## 📊 Tác Động

### Performance

- **Index**: Thêm index `organizationId` sẽ cải thiện query performance
- **Storage**: Tăng ~12 bytes per document (ObjectID)
- **Query**: Có thể filter nhanh hơn với index

### Backward Compatibility

- Dữ liệu cũ: Cần migration script để gán giá trị mặc định
- API: Có thể giữ backward compatibility bằng cách cho phép `organizationId` optional trong một thời gian

## ✅ Checklist Triển Khai Chi Tiết

### Phase 1: Middleware & Context Management

#### 1.1. Tạo OrganizationContextMiddleware
- [ ] **File mới**: `api/internal/api/middleware/middleware.organization_context.go`
  - [ ] Function `OrganizationContextMiddleware()` - Đọc `X-Active-Role-ID` từ header
  - [ ] Validate user có role đó không
  - [ ] Lấy organization từ role
  - [ ] Lưu vào `c.Locals("active_role_id")`, `c.Locals("active_organization_id")`
  - [ ] Fallback: Nếu không có header, lấy role đầu tiên của user

#### 1.2. Cập nhật AuthManager
- [ ] **File**: `api/internal/api/middleware/middleware.auth.go`
  - [ ] Thêm method `GetUserRolesWithDetails(userID)` - Lấy roles với thông tin organization
  - [ ] Thêm method `ValidateUserHasRole(userID, roleID)` - Validate user có role không

### Phase 2: API Endpoints

#### 2.1. Endpoint Lấy Danh Sách Roles
- [ ] **File**: `api/internal/api/handler/handler.auth.user.go` hoặc tạo file mới
  - [ ] Handler `GetUserRoles(c fiber.Ctx)` - `GET /api/v1/auth/roles`
  - [ ] Trả về: `[{roleId, roleName, organizationId, organizationName, organizationCode, ...}]`

#### 2.2. Cập nhật Router
- [ ] **File**: `api/internal/api/router/routes.go`
  - [ ] Thêm route `GET /api/v1/auth/roles` với `AuthMiddleware("")`
  - [ ] Áp dụng `OrganizationContextMiddleware()` vào các routes cần thiết (sau `AuthMiddleware`)

### Phase 3: Database & Models

#### 3.1. Cập nhật Models (Không cần Migration vì dữ liệu trắng)
- [ ] Chỉ cần thêm field vào models, MongoDB sẽ tự động tạo index khi có tag `index:"single:1"`

#### 3.2. Cập nhật Models (Priority 1 - Bắt buộc)
- [ ] **File**: `api/internal/api/models/mongodb/model.fb.customer.go`
  - [ ] Thêm field: `OrganizationID primitive.ObjectID` với index `single:1`
- [ ] **File**: `api/internal/api/models/mongodb/model.pc.pos.customer.go`
  - [ ] Thêm field: `OrganizationID primitive.ObjectID` với index `single:1`
- [ ] **File**: `api/internal/api/models/mongodb/model.pc.pos.order.go`
  - [ ] Thêm field: `OrganizationID primitive.ObjectID` với index `single:1`
- [ ] **File**: `api/internal/api/models/mongodb/model.pc.pos.shop.go`
  - [ ] Thêm field: `OrganizationID primitive.ObjectID` với index `single:1`
- [ ] **File**: `api/internal/api/models/mongodb/model.pc.pos.product.go`
  - [ ] Thêm field: `OrganizationID primitive.ObjectID` với index `single:1`
- [ ] **File**: `api/internal/api/models/mongodb/model.pc.pos.warehouse.go`
  - [ ] Thêm field: `OrganizationID primitive.ObjectID` với index `single:1`
- [ ] **File**: `api/internal/api/models/mongodb/model.fb.page.go`
  - [ ] Thêm field: `OrganizationID primitive.ObjectID` với index `single:1`
- [ ] **File**: `api/internal/api/models/mongodb/model.fb.post.go`
  - [ ] Thêm field: `OrganizationID primitive.ObjectID` với index `single:1`
- [ ] **File**: `api/internal/api/models/mongodb/model.fb.conversation.go`
  - [ ] Thêm field: `OrganizationID primitive.ObjectID` với index `single:1`
- [ ] **File**: `api/internal/api/models/mongodb/model.fb.message.go`
  - [ ] Thêm field: `OrganizationID primitive.ObjectID` với index `single:1`
- [ ] **File**: `api/internal/api/models/mongodb/model.auth.agent.go`
  - [ ] Thêm field: `OrganizationID *primitive.ObjectID` (nullable) với index `single:1`

#### 3.3. Cập nhật Models (Priority 2 - Tối ưu query)
- [ ] **File**: `api/internal/api/models/mongodb/model.pc.pos.category.go`
  - [ ] Thêm field: `OrganizationID primitive.ObjectID` với index `single:1`
- [ ] **File**: `api/internal/api/models/mongodb/model.pc.pos.variation.go`
  - [ ] Thêm field: `OrganizationID primitive.ObjectID` với index `single:1`
- [ ] **File**: `api/internal/api/models/mongodb/model.fb.message.item.go`
  - [ ] Thêm field: `OrganizationID primitive.ObjectID` với index `single:1`

#### 3.4. Cập nhật Models (Priority 3 - Cần xác định)
- [ ] **File**: `api/internal/api/models/mongodb/model.pc.order.go`
  - [ ] Xác định business logic: Đơn hàng có thuộc organization không?
  - [ ] Nếu có: Thêm field `OrganizationID primitive.ObjectID` với index `single:1`

### Phase 4: Services

#### 4.1. Helper Functions trong BaseService
- [ ] **File**: `api/internal/api/services/service.base.mongo.go`
  - [ ] Method `GetUserAllowedOrganizationIDs(ctx, userID, permissionName)` - Tính toán allowed org IDs dựa trên scope
  - [ ] **Tự động thêm parent organizations** vào allowedOrgIDs (không cần permission)
  - [ ] Method `ApplyOrganizationFilter(baseFilter, allowedOrgIDs)` - Thêm filter organizationId

#### 4.2. Cập nhật OrganizationService
- [ ] **File**: `api/internal/api/services/service.auth.organization.go`
  - [ ] Đảm bảo method `GetChildrenIDs()` hoạt động đúng (đã có)
  - [ ] **Thêm method `GetParentIDs(ctx, childID)`** - Lấy tất cả parent IDs (inverse lookup)

### Phase 5: Handlers

#### 5.1. Cập nhật BaseHandler
- [ ] **File**: `api/internal/api/handler/handler.base.go`
  - [ ] Method `getActiveOrganizationID(c)` - Lấy active organization từ context
  - [ ] Method `applyOrganizationFilter(c, permissionName, baseFilter)` - Tự động filter theo scope

#### 5.2. Cập nhật InsertOne trong BaseHandler
- [ ] **File**: `api/internal/api/handler/handler.base.crud.go`
  - [ ] Trong `InsertOne()`: Tự động gán `organizationId` từ `active_organization_id` trong context
  - [ ] Validate model có field `OrganizationID` không (dùng reflection)

#### 5.3. Cập nhật Find/Query Methods trong BaseHandler
- [ ] **File**: `api/internal/api/handler/handler.base.crud.go`
  - [ ] Trong `Find()`: Tự động thêm filter `organizationId` dựa trên scope
  - [ ] Trong `FindWithPagination()`: Tự động thêm filter `organizationId`
  - [ ] Trong `FindOne()`: Tự động thêm filter `organizationId`
  - [ ] Trong `FindOneById()`: Validate record thuộc organization được phép
  - [ ] Trong `UpdateOne()`: Validate và filter theo organization
  - [ ] Trong `DeleteOne()`: Validate và filter theo organization

#### 5.4. Cập nhật Specific Handlers (nếu cần override)
- [ ] Kiểm tra các handlers có override `InsertOne()` không:
  - [ ] `handler.customer.go`
  - [ ] `handler.fb.*.go`
  - [ ] `handler.pc.pos.*.go`
  - [ ] Các handlers khác

### Phase 6: Router & Middleware Chain

#### 6.1. Cập nhật Router
- [ ] **File**: `api/internal/api/router/routes.go`
  - [ ] Thêm `OrganizationContextMiddleware()` vào middleware chain
  - [ ] Đảm bảo thứ tự: `AuthMiddleware` → `OrganizationContextMiddleware` → Handler
  - [ ] Áp dụng cho tất cả routes cần organization context (trừ auth routes)

### Phase 7: Testing

#### 7.1. Unit Tests
- [ ] Test `OrganizationContextMiddleware()` với các scenarios:
  - [ ] Có header `X-Active-Role-ID`
  - [ ] Không có header (fallback)
  - [ ] User không có role
  - [ ] User có role nhưng không có quyền
- [ ] Test `GetUserAllowedOrganizationIDs()` với scope 0 và 1
- [ ] Test `applyOrganizationFilter()` với các scenarios

#### 7.2. Integration Tests
- [ ] Test insert với organization context
- [ ] Test query với organization filter
- [ ] Test multi-role user với context switching
- [ ] Test scope 0 (self) và scope 1 (children)

### Phase 8: Documentation

#### 8.1. API Documentation
- [ ] Cập nhật API docs với header `X-Active-Role-ID`
- [ ] Document endpoint `GET /api/v1/auth/roles`
- [ ] Cập nhật examples với organization context

#### 8.2. Frontend Documentation
- [ ] Hướng dẫn implement context switching ở frontend
- [ ] Example code cho việc lưu và gửi context
- [ ] Hướng dẫn xử lý multi-client scenarios

### Phase 9: Deployment

#### 9.1. Deployment
- [ ] Deploy backend với middleware mới
- [ ] Deploy frontend với context management
- [ ] Monitor errors và performance
- [ ] Rollback plan nếu có vấn đề

## 📋 Thứ Tự Ưu Tiên Triển Khai

### Priority 1 (Core - Phải làm trước)
1. ✅ Middleware `OrganizationContextMiddleware`
2. ✅ Endpoint `GET /api/v1/auth/roles`
3. ✅ Cập nhật BaseHandler để tự động gán `organizationId` khi insert
4. ✅ Cập nhật BaseHandler để tự động filter khi query

### Priority 2 (Models - Cần cho data mới)
5. ✅ Cập nhật các models (Customer, FbCustomer, PcPosCustomer, PcPosOrder, ...)
   - Không cần migration script vì dữ liệu trắng
   - Chỉ cần thêm field vào models, MongoDB sẽ tự động tạo index

### Priority 3 (Services - Tối ưu)
7. ✅ Helper functions trong services
8. ✅ Cập nhật query methods với organization filter

### Priority 4 (Testing & Docs)
9. ✅ Tests
10. ✅ Documentation

## 📚 Tài Liệu Tham Khảo

- [Organization Structure](./organization.md)
- [Database Schema](./database.md)
- [RBAC System](./rbac.md)


