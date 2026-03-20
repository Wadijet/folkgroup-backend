# API Tests

Test suite cho Folkform Backend API.

## 🚀 Quick Start

### 1. Setup Environment

**Cách 1: Đăng nhập bằng email/password (khuyến nghị)**

Script `test.ps1` tự động load `api/config/env/development.env` và dùng email/password mặc định:
- Mặc định: `daomanhdung86@gmail.com` / `12345678`
- Có thể override: `$env:TEST_EMAIL="your@email.com"; $env:TEST_PASSWORD="yourpass"`
- Cần `FIREBASE_API_KEY` trong development.env (đã có sẵn)

**Cách 2: Firebase ID Token (thủ công)**

```bash
# Windows PowerShell
$env:TEST_FIREBASE_ID_TOKEN="your-firebase-id-token"

# Linux/Mac
export TEST_FIREBASE_ID_TOKEN="your-firebase-id-token"
```

**Cách lấy Firebase ID Token:**
- Sử dụng script: `scripts/get-firebase-token.ps1` (Windows) hoặc `scripts/get-firebase-token.sh` (Linux/Mac)
- Hoặc lấy từ Firebase Console > Authentication > Users

### 2. Chạy Server

```bash
cd ../api
go run cmd/server/main.go
```

### 3. Chạy Tests

```bash
# Chạy tất cả integration tests (cần server + Firebase token)
go test -v ./cases/...

# Chạy test cụ thể
go test -v ./cases/admin_full_test.go
go test -v ./cases/notification_test.go
```

**Chỉ chạy unit tests trong api** (không cần server, nhanh):

```powershell
.\api-tests\test.ps1 -UnitOnly
```

Hoặc trực tiếp:

```bash
go test ./api/... -short -count=1 -v
```

## 📝 Tạo User Có Quyền Để Test

### Phương Án 1: Sử Dụng Helper Function (Khuyến Nghị) ⭐

**File**: `utils/test_helper.go`

```go
func TestMyFeature(t *testing.T) {
    baseURL := "http://localhost:8080/api/v1"
    
    // Setup với admin user có full quyền
    fixtures, adminEmail, adminToken, client, err := utils.SetupTestWithAdminUser(t, baseURL)
    if err != nil {
        t.Fatalf("❌ Không thể setup test: %v", err)
    }
    
    // client đã được set token và active role, sẵn sàng để test
    resp, body, err := client.GET("/some/endpoint")
    // ...
}
```

**Helper Functions:**

- **`SetupTestWithAdminUser()`** - Tạo admin user với full quyền
  - Tự động: wait for health, init data, create admin user, set active role
  - Trả về: fixtures, adminEmail, adminToken, client

- **`SetupTestWithRegularUser()`** - Tạo user thường
  - Tự động: wait for health, init data, create user, set active role
  - Trả về: fixtures, userEmail, userToken, client

### Phương Án 2: Sử Dụng TestFixtures Trực Tiếp

```go
fixtures := utils.NewTestFixtures(baseURL)
firebaseIDToken := utils.GetTestFirebaseIDToken()

// Tạo admin user
adminEmail, _, adminToken, userID, err := fixtures.CreateAdminUser(firebaseIDToken)

// Tạo client
client := utils.NewHTTPClient(baseURL, 10)
client.SetToken(adminToken)
```

### Phương Án 3: First User Becomes Admin

User đầu tiên đăng nhập tự động trở thành admin (nếu chưa có admin trong hệ thống).

```go
// Tạo user đầu tiên
email, _, token, err := fixtures.CreateTestUser(firebaseIDToken)
// User này tự động trở thành admin
```

## 📚 Test Files

### Admin Tests
- `admin_full_test.go` - Test các API admin với user có full quyền
- `admin_test.go` - Test các API admin cơ bản

### Notification Tests
- `notification_test.go` - Test các API notification (sender, channel, template, routing, history, trigger)

### Auth Tests
- `auth_test.go` - Test authentication (login, logout, profile)
- `auth_additional_test.go` - Test auth bổ sung

### Organization Tests
- `organization_ownership_test.go` - Test phân quyền dữ liệu theo organization
- `organization_ownership_full_test.go` - Test đầy đủ phân quyền dữ liệu
- `organization_data_access_test.go` - Test truy cập dữ liệu theo organization
- `organization_sharing_test.go` - Test chia sẻ dữ liệu giữa organizations
- `organization_sharing_simple_test.go` - Test chia sẻ đơn giản

### RBAC Tests
- `rbac_test.go` - Test Role-Based Access Control
- `scope_permissions_test.go` - Test permissions và scopes

### CRUD Tests
- `crud_operations_test.go` - Test các thao tác CRUD cơ bản

### Other Tests
- `health_test.go` - Test health check endpoint
- `middleware_test.go` - Test middleware
- `endpoint_middleware_test.go` - Test endpoint middleware
- `error_handling_test.go` - Test xử lý lỗi
- `facebook_test.go` - Test Facebook integration
- `pancake_test.go` - Test Pancake integration
- `agent_test.go` - Test Agent functionality

## 🛠️ Utilities

### TestFixtures (`utils/test_fixtures.go`)

Helper functions để setup test data:

- `CreateTestUser()` - Tạo user test
- `CreateAdminUser()` - Tạo admin user với full quyền
- `CreateTestRole()` - Tạo role test
- `CreateTestPermission()` - Tạo permission test
- `GetRootOrganizationID()` - Lấy Root Organization ID
- `InitData()` - Khởi tạo dữ liệu mặc định

### TestHelper (`utils/test_helper.go`)

Helper functions để setup test environment:

- `SetupTestWithAdminUser()` - Setup test với admin user
- `SetupTestWithRegularUser()` - Setup test với regular user
- `waitForHealth()` - Đợi server sẵn sàng
- `initTestData()` - Khởi tạo dữ liệu test

### HTTPClient (`utils/http_client.go`)

HTTP client wrapper với các tiện ích:

- `SetToken()` - Set authentication token
- `SetActiveRoleID()` - Set active role ID (organization context)
- `GET()`, `POST()`, `PUT()`, `DELETE()` - HTTP methods
- `GetToken()` - Lấy token hiện tại

## 📖 Documentation

- `docs/SETUP_TEST_USER_WITH_FULL_PERMISSIONS.md` - Hướng dẫn chi tiết tạo user có quyền

## ⚠️ Lưu Ý

1. **Firebase ID Token**: Bắt buộc phải có `TEST_FIREBASE_ID_TOKEN` environment variable
2. **Server Phải Chạy**: Server phải đang chạy trước khi chạy tests
3. **Database**: Tests sẽ tự động init data nếu chưa có admin
4. **First User Becomes Admin**: User đầu tiên đăng nhập tự động trở thành admin

## 🔍 Debug

### Xem Logs

Tests sẽ log các thông tin quan trọng:
- ✅ Setup thành công
- ⚠️ Warnings
- ❌ Errors

### Kiểm Tra User Có Quyền

```go
// Test admin API
resp, _, err := client.GET("/admin/users")
if resp.StatusCode == http.StatusOK {
    // User có quyền admin
}
```

## 📝 Ví Dụ

Xem các file test trong `cases/` để biết cách sử dụng:

- `admin_full_test.go` - Ví dụ test với admin user
- `notification_test.go` - Ví dụ test notification APIs
- `organization_ownership_full_test.go` - Ví dụ test phân quyền dữ liệu
