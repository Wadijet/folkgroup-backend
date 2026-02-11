# XỬ LÝ ADMINISTRATOR VỚI FIREBASE AUTHENTICATION

Tài liệu này mô tả cách xử lý administrator trong hệ thống sau khi chuyển sang Firebase Authentication.

---

## 1. TỔNG QUAN

### 1.1. Vấn đề
- Với Firebase Authentication, user được tạo tự động khi login lần đầu
- Không thể tạo user trực tiếp trong MongoDB như trước
- Cần cách để tự động tạo user admin trong init

### 1.2. Giải pháp
- Thêm config `FIREBASE_ADMIN_UID` để tự động tạo admin user từ Firebase UID
- User phải đã tồn tại trong Firebase Authentication
- Init sẽ tự động tạo user trong MongoDB và gán role Administrator

---

## 2. CÁCH HOẠT ĐỘNG

### 2.1. Init Process
```
1. InitRootOrganization() - Tạo Organization Root
2. InitPermission() - Tạo Permissions
3. CheckPermissionForAdministrator() - Tạo Role Administrator và gán quyền
4. InitAdminUser() - Tạo user admin từ Firebase UID (nếu có config)
```

### 2.2. InitAdminUser Flow
```
1. Kiểm tra FIREBASE_ADMIN_UID có trong config không
2. Nếu có, lấy thông tin user từ Firebase bằng UID
3. Kiểm tra user đã tồn tại trong MongoDB chưa
4. Nếu chưa, tạo user mới từ thông tin Firebase
5. Gán role Administrator cho user
```

---

## 3. CẤU HÌNH

### 3.1. Thêm vào `development.env`
```env
# Firebase Admin UID (tùy chọn)
# User phải đã tồn tại trong Firebase Authentication
# Init sẽ tự động tạo user trong MongoDB và gán role Administrator
FIREBASE_ADMIN_UID=your_firebase_admin_uid_here
```

### 3.2. Lấy Firebase UID
1. Đăng nhập vào [Firebase Console](https://console.firebase.google.com/)
2. Vào **Authentication** > **Users**
3. Tìm user admin (hoặc tạo mới)
4. Copy **UID** của user

---

## 4. CÁCH SỬ DỤNG

### 4.1. Tự động tạo Admin trong Init (Khuyến nghị)

**Bước 1:** Tạo user trong Firebase Console
- Vào Firebase Console > Authentication > Users
- Tạo user mới hoặc sử dụng user có sẵn
- Copy UID của user

**Bước 2:** Thêm vào config
```env
FIREBASE_ADMIN_UID=abc123xyz456...
```

**Bước 3:** Khởi động server
- Init sẽ tự động:
  - Lấy thông tin user từ Firebase
  - Tạo user trong MongoDB
  - Gán role Administrator

### 4.2. Tạo Admin thủ công

**Bước 1:** Đăng nhập với Firebase
```bash
# Sử dụng script hoặc frontend để đăng nhập
POST /auth/login/firebase
{
  "idToken": "firebase_id_token",
  "hwid": "device_id"
}
```

**Bước 2:** Lấy user ID từ response
```json
{
  "data": {
    "id": "user_id_here",
    ...
  }
}
```

**Bước 3:** Set Administrator
```bash
POST /init/set-administrator/:userID
Authorization: Bearer <token_with_Init.SetAdmin_permission>
```

---

## 5. CODE IMPLEMENTATION

### 5.1. Config
```go
// api/config/config.go
FirebaseAdminUID string `env:"FIREBASE_ADMIN_UID"` // Firebase UID của user admin
```

### 5.2. InitAdminUser Method
```go
// api/internal/api/services/service.admin.init.go
func (h *InitService) InitAdminUser(firebaseUID string) error {
    // 1. Kiểm tra user đã tồn tại chưa
    // 2. Nếu chưa, lấy từ Firebase và tạo mới
    // 3. Gán role Administrator
}
```

### 5.3. InitDefaultData
```go
// api/cmd/server/init.data.go
func InitDefaultData() {
    // ... init organization, permissions, roles ...
    
    // Tạo admin user từ Firebase UID (nếu có config)
    if global.MongoDB_ServerConfig.FirebaseAdminUID != "" {
        initService.InitAdminUser(global.MongoDB_ServerConfig.FirebaseAdminUID)
    }
}
```

---

## 6. LƯU Ý

### 6.1. Firebase UID
- User phải đã tồn tại trong Firebase Authentication
- Không thể tạo user mới trong Firebase từ backend
- Cần tạo user trước trong Firebase Console

### 6.2. Permissions
- Endpoint `/init/set-administrator/:id` yêu cầu permission `Init.SetAdmin`
- User đầu tiên có thể không có permission này
- Có 2 cách:
  1. Sử dụng `FIREBASE_ADMIN_UID` để tự động tạo (không cần permission)
  2. Tạm thời bỏ qua permission check cho lần đầu init

### 6.3. Security
- `FIREBASE_ADMIN_UID` chỉ nên set trong development/staging
- Production nên tạo admin thủ công và bảo mật tốt hơn
- Không commit `FIREBASE_ADMIN_UID` vào git

---

## 7. TÓM TẮT

### Đã cập nhật:
- ✅ Thêm `FIREBASE_ADMIN_UID` vào config
- ✅ Thêm `InitAdminUser()` method
- ✅ Cập nhật `InitDefaultData()` để tự động tạo admin

### Cách sử dụng:
1. **Tự động (khuyến nghị):** Set `FIREBASE_ADMIN_UID` trong config
2. **Thủ công:** Login với Firebase, sau đó gọi `/init/set-administrator/:id`

### Kết quả:
- ✅ Init đã hỗ trợ Firebase Authentication
- ✅ Có thể tự động tạo admin user từ Firebase UID
- ✅ Administrator được xử lý phù hợp với login mới

---

**Init đã được cập nhật để hỗ trợ Firebase Authentication! ✅**

