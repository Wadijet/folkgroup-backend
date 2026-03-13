# Khởi Tạo Hệ Thống

Hướng dẫn về quy trình khởi tạo hệ thống lần đầu, bao gồm việc tạo admin user và các dữ liệu mặc định.

## 📋 Tổng Quan

Khi khởi động hệ thống lần đầu, cần khởi tạo các thành phần cơ bản:

1. **Organization Root** - Tổ chức cấp cao nhất
2. **Permissions** - Các quyền mặc định của hệ thống
3. **Roles** - Vai trò Administrator
4. **Admin User** - User quản trị hệ thống

## 🔄 Quy Trình Tự Động

### Khi Server Khởi Động

Khi server khởi động, hàm `InitDefaultData()` trong `api/cmd/server/init.data.go` sẽ tự động chạy:

```go
func InitDefaultData() {
    // 1. Khởi tạo Organization Root
    initService.InitRootOrganization()
    
    // 2. Khởi tạo Permissions
    initService.InitPermission()
    
    // 3. Tạo Role Administrator và gán quyền
    initService.CheckPermissionForAdministrator()
    
    // 4. Tạo admin user từ Firebase UID (nếu có config) - Tùy chọn
    if FIREBASE_ADMIN_UID != "" {
        initService.InitAdminUser(FIREBASE_ADMIN_UID)
    }
}
```

### User Đầu Tiên Tự Động Trở Thành Admin

Khi user đầu tiên đăng nhập với Firebase, hệ thống sẽ:
- Tự động kiểm tra xem đã có admin chưa
- Nếu chưa có admin, tự động set user này làm admin
- Đây là phương án phổ biến: **"First user becomes admin"**

## 🎯 Các Phương Án Khởi Tạo Admin

### Phương Án 1: First User Becomes Admin (Khuyến Nghị)

**Cách hoạt động:**
- User đầu tiên đăng nhập tự động trở thành Administrator
- Logic trong `LoginWithFirebase()` service

**Ưu điểm:**
- Đơn giản, không cần cấu hình
- Phù hợp cho development và production
- Tự động hóa hoàn toàn

**Cách sử dụng:**
1. Khởi động server
2. Đăng nhập bằng Firebase (user đầu tiên)
3. User này tự động trở thành admin

### Phương Án 2: Từ Firebase UID

**Cách hoạt động:**
- Set `FIREBASE_ADMIN_UID` trong file `.env`
- Server tự động tạo admin user khi khởi động (nếu chưa có)

**Cách sử dụng:**
1. Lấy Firebase UID từ Firebase Console
2. Thêm vào file `.env`:
```env
FIREBASE_ADMIN_UID=your-firebase-uid-here
```
3. Khởi động server
4. Admin user sẽ được tạo tự động

**Lưu ý:**
- User với UID này phải đã tồn tại trong Firebase Authentication
- Nếu user chưa đăng nhập lần nào, sẽ được tạo trong MongoDB khi đăng nhập

### Phương Án 3: Init Endpoints (Chỉ Khi Chưa Có Admin)

**Cách hoạt động:**
- Khi server khởi động, kiểm tra đã có admin chưa
- Nếu chưa có admin → Đăng ký tất cả init endpoints
- Nếu đã có admin → Init endpoints trả về 404

**Init Endpoints (chỉ khi chưa có admin):**

1. **Kiểm tra trạng thái:**
```http
GET /api/v1/init/status
```

2. **Khởi tạo Organization Root:**
```http
POST /api/v1/init/organization
```

3. **Khởi tạo Permissions:**
```http
POST /api/v1/init/permissions
```

4. **Khởi tạo Roles:**
```http
POST /api/v1/init/roles
```

5. **Tạo admin từ Firebase UID:**
```http
POST /api/v1/init/admin-user
Body: { "firebaseUid": "user-uid-here" }
```

6. **One-click setup (khởi tạo tất cả):**
```http
POST /api/v1/init/all
```

7. **Set admin lần đầu (không cần quyền):**
```http
POST /api/v1/init/set-administrator/:id
```

**Cách sử dụng:**
1. Khởi động server
2. Gọi các init endpoints theo thứ tự
3. Hoặc gọi `/init/all` để khởi tạo tất cả

### Phương Án 4: Admin Endpoints (Khi Đã Có Admin)

**Cách hoạt động:**
- Khi đã có admin, sử dụng admin endpoints
- Yêu cầu quyền `Init.SetAdmin`

**Admin Endpoint:**
```http
POST /api/v1/admin/user/set-administrator/:id
Headers: Authorization: Bearer <admin-token>
```

**Cách sử dụng:**
1. Đăng nhập với admin account
2. Lấy user ID cần set làm admin
3. Gọi endpoint với admin token

## 📝 Quy Trình Khởi Tạo Chi Tiết

### Bước 1: Khởi Tạo Organization Root

Tạo tổ chức cấp cao nhất trong hệ thống:

```json
{
  "name": "Root Organization",
  "code": "ROOT",
  "parentId": null
}
```

### Bước 2: Khởi Tạo Permissions

Tạo các quyền mặc định của hệ thống:

- `User.Read`, `User.Create`, `User.Update`, `User.Delete`
- `Role.Read`, `Role.Create`, `Role.Update`, `Role.Delete`
- `Permission.Read`, `Permission.Create`, `Permission.Update`, `Permission.Delete`
- `Organization.Read`, `Organization.Create`, `Organization.Update`, `Organization.Delete`
- Và nhiều quyền khác...

### Bước 3: Tạo Role Administrator

Tạo role Administrator và gán tất cả permissions:

```json
{
  "name": "Administrator",
  "code": "ADMIN",
  "organizationId": "root-org-id",
  "permissions": ["all-permissions"]
}
```

### Bước 4: Tạo Admin User

Tạo user và gán role Administrator:

```json
{
  "firebaseUid": "user-uid",
  "email": "admin@example.com",
  "roles": ["administrator-role-id"]
}
```

## ✅ Xác Nhận Khởi Tạo

Sau khi khởi tạo, kiểm tra:

1. **Kiểm tra Organization:**
```http
GET /api/v1/organization
```

2. **Kiểm tra Permissions:**
```http
GET /api/v1/permission
```

3. **Kiểm tra Roles:**
```http
GET /api/v1/role
```

4. **Kiểm tra Admin User:**
```http
GET /api/v1/user
Headers: Authorization: Bearer <admin-token>
```

## 🔒 Bảo Mật

### Init Endpoints Tự Động Tắt

- Khi đã có admin, tất cả init endpoints tự động trả về 404
- Điều này ngăn chặn việc khởi tạo lại hệ thống

### Admin Endpoints Yêu Cầu Quyền

- Admin endpoints yêu cầu quyền `Init.SetAdmin`
- Chỉ admin mới có thể tạo admin mới

## 🐛 Xử Lý Lỗi

### Lỗi: Init Endpoints Trả Về 404

**Nguyên nhân:** Đã có admin trong hệ thống

**Giải pháp:**
- Sử dụng admin endpoints thay vì init endpoints
- Hoặc xóa admin hiện tại (không khuyến nghị)

### Lỗi: Không Thể Tạo Admin

**Nguyên nhân:**
- Firebase UID không tồn tại
- User chưa đăng nhập lần nào

**Giải pháp:**
- Đảm bảo user đã đăng nhập ít nhất một lần
- Hoặc sử dụng phương án "First user becomes admin"

## 📚 Tài Liệu Liên Quan

- [Xử Lý Admin với Firebase](../08-archive/xu-ly-admin-voi-firebase.md)

