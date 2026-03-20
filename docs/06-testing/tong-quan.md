# Tổng Quan Testing

Tài liệu về hệ thống testing của FolkForm Auth Backend.

## 📋 Tổng Quan

Dự án sử dụng **Go Workspace** để tách biệt module test (`api-tests`) khỏi module chính (`api`). Điều này giúp:
- Tách biệt dependencies
- Dễ quản lý và maintain
- Có thể versioning riêng nếu cần

## 🏗️ Cấu Trúc Test

```
api-tests/
├── cases/                  # Test cases
│   ├── auth_test.go
│   ├── admin_test.go
│   ├── health_test.go
│   └── ...
├── utils/                  # Test utilities
│   ├── http_client.go
│   ├── test_fixtures.go
│   └── get_firebase_token.go
├── scripts/                # Test scripts
│   ├── test_runner.ps1
│   ├── manage_server.ps1
│   └── utils.ps1
├── reports/                # Test reports
├── templates/              # Report templates
├── go.mod                  # Test module dependencies
└── README.md               # Test documentation
```

## 🚀 Chạy Test

### Cách 0: Chỉ Unit Tests (Nhanh, không cần server)

```powershell
.\api-tests\test.ps1 -UnitOnly
```

Chạy unit tests trong `api/` (ví dụ: `service.crm.snapshot_test.go`). Không cần server, MongoDB, Firebase token.

### Cách 1: Script Tự Động (Khuyến Nghị)

```powershell
# Từ root directory
.\api-tests\test.ps1
```

Script sẽ tự động:
1. Kiểm tra server có đang chạy không
2. Khởi động server nếu chưa chạy
3. Đợi server sẵn sàng (tối đa 60 giây)
4. Chạy toàn bộ test suite
5. Tự động dừng server sau khi test xong
6. Hiển thị kết quả chi tiết

### Cách 2: Bỏ Qua Khởi Động Server

Nếu server đã chạy sẵn:

```powershell
.\api-tests\test.ps1 -SkipServer
```

### Cách 3: Quản Lý Server Thủ Công

```powershell
# Khởi động server
.\api-tests\scripts\manage_server.ps1 start

# Kiểm tra trạng thái
.\api-tests\scripts\manage_server.ps1 status

# Dừng server
.\api-tests\scripts\manage_server.ps1 stop
```

Sau đó chạy test ở terminal khác:
```powershell
.\api-tests\test.ps1 -SkipServer
```

### Cách 4: Chạy Trực Tiếp với Go

```powershell
cd api-tests
go test -v ./cases/...
```

## 📝 Test Cases

### Health Test

Kiểm tra health endpoint:

```go
func TestHealth(t *testing.T) {
    resp, err := client.Get("/api/v1/system/health")
    assert.NoError(t, err)
    assert.Equal(t, 200, resp.StatusCode)
}
```

### Auth Test

Test các endpoint authentication:

- `TestLoginWithFirebase` - Đăng nhập bằng Firebase
- `TestLogout` - Đăng xuất
- `TestGetProfile` - Lấy profile
- `TestUpdateProfile` - Cập nhật profile

### Admin Test

Test các endpoint admin:

- `TestCreateUser` - Tạo user
- `TestGetUsers` - Lấy danh sách users
- `TestUpdateUser` - Cập nhật user
- `TestDeleteUser` - Xóa user

### RBAC Test

Test hệ thống phân quyền:

- `TestCreateRole` - Tạo role
- `TestAssignRole` - Gán role cho user
- `TestPermissionCheck` - Kiểm tra permission

## 🛠️ Test Utilities

### HTTP Client

**Vị trí:** `api-tests/utils/http_client.go`

Wrapper cho HTTP client với các tính năng:
- Base URL configuration
- Automatic token management
- Error handling
- Response parsing

### Test Fixtures

**Vị trí:** `api-tests/utils/test_fixtures.go`

Các hàm helper để tạo test data:
- `CreateTestUser()` - Tạo user test
- `CreateTestRole()` - Tạo role test
- `GetFirebaseToken()` - Lấy Firebase token cho test

### Firebase Token Helper

**Vị trí:** `api-tests/utils/get_firebase_token.go`

Helper để lấy Firebase token cho testing:
- Sử dụng Firebase Admin SDK
- Tạo custom token cho test user

## 📊 Test Reports

### Tự Động Tạo Report

Sau khi chạy test, script tự động tạo report trong `api-tests/reports/`:

- Format: Markdown
- Tên file: `test_report_YYYY-MM-DD_HH-MM-SS.md`
- Nội dung:
  - Tổng số test cases
  - Số test passed/failed
  - Pass rate
  - Chi tiết từng test case

### Xem Report

```powershell
# Mở file report mới nhất
Get-ChildItem api-tests\reports\*.md | Sort-Object LastWriteTime -Descending | Select-Object -First 1 | ForEach-Object { notepad $_.FullName }
```

## ✅ Yêu Cầu

### Phần Mềm

- Go 1.23+
- MongoDB đang chạy
- Firebase project đã cấu hình

### Cấu Hình

- File `api/config/env/development.env` phải tồn tại
- Firebase credentials phải được cấu hình
- Server có thể khởi động thành công

## 🐛 Troubleshooting

### Server Không Khởi Động

**Nguyên nhân:**
- MongoDB chưa chạy
- Port 8080 đã được sử dụng
- Cấu hình sai

**Giải pháp:**
- Kiểm tra MongoDB: `mongosh`
- Kiểm tra port: `netstat -ano | findstr :8080`
- Xem log: `api/logs/app.log`

### Test Bị Lỗi Kết Nối

**Nguyên nhân:**
- Server chưa sẵn sàng
- URL sai

**Giải pháp:**
- Đợi server khởi động hoàn toàn
- Kiểm tra health endpoint: `curl http://localhost:8080/api/v1/system/health`

### Firebase Token Lỗi

**Nguyên nhân:**
- Firebase chưa được cấu hình
- Service account key sai

**Giải pháp:**
- Kiểm tra `FIREBASE_CREDENTIALS_PATH` trong `.env`
- Kiểm tra file service account có tồn tại không

## 📚 Tài Liệu Liên Quan

- [Chạy Test Suite](chay-test.md)
- [Viết Test Case](viet-test.md)
- [Báo Cáo Test](bao-cao-test.md)
- [Rà Soát Test Hiện Có](RA_SOAT_TEST_HIEN_CO.md)
- [Đề Xuất Cơ Chế Test Tự Động](DE_XUAT_CO_CHE_TEST_TU_DONG.md)

