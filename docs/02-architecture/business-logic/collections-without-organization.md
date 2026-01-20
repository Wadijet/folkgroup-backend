# Danh Sách Collections KHÔNG CÓ OrganizationID

## 📋 Tổng Quan

Tài liệu này liệt kê các collection **KHÔNG CẦN** field `organizationId` và **KHÔNG ÁP DỤNG** phân quyền dữ liệu theo tổ chức.

## ✅ Collections KHÔNG CẦN `organizationId` (System/Global)

### 1. Authentication & Authorization Models

#### **Users** (`model.auth.user.go`)
- **Lý do**: User là global, có thể thuộc nhiều organizations qua UserRoles
- **Phân quyền**: Qua UserRoles → Role → OrganizationID
- **CRUD**: Không filter theo organizationId

#### **Permissions** (`model.auth.permission.go`)
- **Lý do**: System-wide, không thuộc organization cụ thể
- **Phân quyền**: Qua RolePermissions → Role → OrganizationID
- **CRUD**: Không filter theo organizationId

#### **Organizations** (`model.auth.organization.go`)
- **Lý do**: Chính nó là organization, không cần field organizationId
- **Phân quyền**: Không cần (chính nó là organization)
- **CRUD**: Không filter theo organizationId

#### **UserRoles** (`model.auth.user_role.go`)
- **Lý do**: Mapping table, đã có organizationId gián tiếp qua Role
- **Phân quyền**: UserRole → Role → OrganizationID
- **CRUD**: Không filter theo organizationId (có thể filter qua Role nếu cần)

#### **RolePermissions** (`model.auth.role_permission.go`)
- **Lý do**: Mapping table, đã có organizationId gián tiếp qua Role
- **Phân quyền**: RolePermission → Role → OrganizationID
- **CRUD**: Không filter theo organizationId (có thể filter qua Role nếu cần)

~~#### **AccessTokens** (`model.pc.access_token.go`)~~ - **CẦN THÊM organizationId**
- ~~**Lý do**: Global hoặc user-specific, không cần organizationId~~
- **Cập nhật**: Cần phân quyền theo organization → Cần thêm `OrganizationID`

~~#### **Customer** (`model.customer.go`)~~ - **CẦN THÊM organizationId**
- ~~**Lý do**: Deprecated - dùng FbCustomers và PcPosCustomers~~
- **Cập nhật**: Cần phân quyền theo organization → Cần thêm `OrganizationID` (nếu vẫn còn sử dụng)

## 📊 Tổng Kết

### Collections KHÔNG CẦN OrganizationID (4 collections)

1. ✅ **Users** - Global, phân quyền qua UserRoles
2. ✅ **Permissions** - System-wide
3. ✅ **Organizations** - Chính nó là organization
4. ✅ **UserRoles** - Mapping table, có organizationId gián tiếp
5. ✅ **RolePermissions** - Mapping table, có organizationId gián tiếp

## 🔧 Ảnh Hưởng Đến CRUD

### Các Collection Này Sẽ:

✅ **InsertOne()** - Không tự động gán `organizationId` (vì không có field)
✅ **Find()** - Không tự động filter theo `organizationId` (vì không có field)
✅ **UpdateOne()** - Không validate `organizationId` (vì không có field)
✅ **DeleteOne()** - Không filter theo `organizationId` (vì không có field)
✅ **Tất cả CRUD operations hoạt động bình thường như trước**

### Logic Check

BaseHandler sẽ tự động detect và bỏ qua các collection không có field `OrganizationID`:

```go
// Helper function check field có tồn tại không
func (h *BaseHandler[T, CreateInput, UpdateInput]) hasOrganizationIDField() bool {
    // ... check bằng reflection
}

// Mọi function đều check trước:
if !h.hasOrganizationIDField() {
    return // Không có field, không làm gì cả
}
```

## 📝 Lưu Ý

1. **Backward Compatibility**: 
   - Các collection này sẽ **KHÔNG bị ảnh hưởng** bởi logic organization filtering
   - CRUD operations hoạt động **hoàn toàn bình thường** như trước

2. **Phân Quyền Gián Tiếp**:
   - UserRoles, RolePermissions có organizationId **gián tiếp** qua Role
   - Nếu cần filter, có thể filter qua Role → OrganizationID

3. **System-Wide Data**:
   - Users, Permissions, Organizations là system-wide
   - Không cần phân quyền theo organization

4. **Deprecated**:
   - Customer collection đã deprecated
   - Không cần cập nhật nếu không còn sử dụng

## ✅ Kết Luận

**Tổng cộng: 4 collections không cần organizationId**

Tất cả các collection này sẽ:
- ✅ Hoạt động bình thường với CRUD base functions
- ✅ Không bị ảnh hưởng bởi logic organization filtering
- ✅ Backward compatible 100%

## ⚠️ Lưu Ý

**AccessTokens và Customer đã được chuyển sang danh sách cần thêm organizationId** vì cần phân quyền theo organization.

