# Backend Tự Động Detect Role

## ✅ Backend CÓ Hỗ Trợ Tự Động Detect Role

Backend middleware `OrganizationContextMiddleware` có logic tự động detect và fallback role trong các trường hợp sau:

## 🔄 Logic Tự Động Detect

### 1. Không Có Header `X-Active-Role-ID`

**Khi nào:** Frontend không gửi header `X-Active-Role-ID`

**Backend xử lý:**
```go
// Không có header, lấy role đầu tiên của user
activeRoleID, err = getFirstUserRoleID(context.Background(), userID)
```

**Kết quả:** Backend tự động lấy role đầu tiên của user và set làm active role.

---

### 2. Header Không Hợp Lệ

**Khi nào:** Header `X-Active-Role-ID` có giá trị nhưng không phải ObjectID hợp lệ

**Backend xử lý:**
```go
// Role ID không hợp lệ, fallback về role đầu tiên
activeRoleID, err = getFirstUserRoleID(context.Background(), userID)
```

**Kết quả:** Backend tự động fallback về role đầu tiên.

---

### 3. User Không Có Role Trong Header

**Khi nào:** Header `X-Active-Role-ID` hợp lệ nhưng user không có role đó

**Backend xử lý:**
```go
// Validate user có role này không
hasRole, err := validateUserHasRole(context.Background(), userID, activeRoleID)
if err != nil || !hasRole {
    // User không có role này, fallback về role đầu tiên
    activeRoleID, err = getFirstUserRoleID(context.Background(), userID)
}
```

**Kết quả:** Backend tự động fallback về role đầu tiên của user.

---

## 📋 Function `getFirstUserRoleID`

```go
// getFirstUserRoleID lấy role ID đầu tiên của user
func getFirstUserRoleID(ctx context.Context, userID primitive.ObjectID) (primitive.ObjectID, error) {
    userRoleService, err := NewUserRoleService()
    if err != nil {
        return primitive.NilObjectID, err
    }

    userRoles, err := userRoleService.Find(ctx, bson.M{"userId": userID}, nil)
    if err != nil {
        return primitive.NilObjectID, err
    }

    if len(userRoles) == 0 {
        return primitive.NilObjectID, common.ErrNotFound
    }

    return userRoles[0].RoleID, nil
}
```

**Lưu ý:** 
- Lấy role **đầu tiên** từ danh sách UserRoles của user
- Nếu user không có role nào → Trả về error
- Backend vẫn cho phép request tiếp tục (không block) nếu không có role

---

## 🎯 Kết Quả

Sau khi tự động detect, backend sẽ:
1. Lấy role đầu tiên của user
2. Từ role, suy ra organization ID
3. Lưu vào context:
   - `active_role_id` = Role ID đầu tiên
   - `active_organization_id` = Organization ID của role đó

---

## 💡 Ý Nghĩa

### Frontend Có Thể:

**Option 1: Gửi Header (Khuyến nghị)**
```javascript
// Frontend chủ động gửi role ID
headers: {
  'X-Active-Role-ID': 'role-id-123'
}
```
- ✅ User có thể chọn role cụ thể
- ✅ Rõ ràng, minh bạch
- ✅ Dễ debug

**Option 2: Không Gửi Header (Tự Động)**
```javascript
// Frontend không gửi header
// Backend tự động dùng role đầu tiên
```
- ✅ Đơn giản hơn cho frontend
- ✅ Tự động fallback
- ⚠️ User không thể chọn role nếu có nhiều roles

---

## ⚠️ Lưu Ý

### Khi Nào Backend KHÔNG Set Context?

Backend sẽ **KHÔNG** set context (nhưng vẫn cho phép request tiếp tục) nếu:
1. Không có user ID (route không cần auth)
2. User không có role nào (`getFirstUserRoleID` trả về error)
3. Không thể lấy role service
4. Role ID không tồn tại trong database

**Trong các trường hợp này:**
- Request vẫn được tiếp tục
- `active_role_id` và `active_organization_id` sẽ không có trong context
- Handler cần tự kiểm tra và xử lý

---

## 📊 Flow Diagram

```
Request đến
    ↓
Có header X-Active-Role-ID?
    ├─ CÓ → Validate role ID
    │       ├─ Hợp lệ? → Validate user có role?
    │       │            ├─ CÓ → Dùng role đó ✅
    │       │            └─ KHÔNG → Fallback role đầu tiên ✅
    │       └─ KHÔNG hợp lệ → Fallback role đầu tiên ✅
    │
    └─ KHÔNG → Lấy role đầu tiên ✅
            ↓
    Set active_role_id và active_organization_id vào context
            ↓
    Request tiếp tục
```

---

## 🔍 Code Reference

**File:** `api/core/api/middleware/middleware.organization_context.go`

**Logic chính:**
- Dòng 36-66: Logic tự động detect và fallback
- Dòng 109-126: Function `getFirstUserRoleID`
