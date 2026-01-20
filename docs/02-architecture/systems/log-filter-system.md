# Hệ Thống Filter Log

## 📋 Tổng Quan

Hệ thống filter log cho phép bạn bật/tắt log theo các tiêu chí cụ thể để debug hiệu quả hơn, tránh log quá nhiều và loạn. Bạn có thể filter theo:

- **Module**: Tên module (ví dụ: `auth`, `notification`, `delivery`, `content`, `ai`)
- **Collection**: Tên collection MongoDB (ví dụ: `users`, `orders`, `notifications`)
- **Endpoint**: Đường dẫn endpoint (ví dụ: `/api/v1/users`, `/api/v1/orders`)
- **Method**: HTTP method (ví dụ: `GET`, `POST`, `PUT`, `DELETE`)
- **Log Type**: Loại log (ví dụ: `info`, `debug`, `warn`, `error`, `fatal`)

---

## 🏗️ Kiến Trúc Triển Khai

### 1. LogConfig Extension (`api/core/logger/config.go`)

Thêm các fields filter vào `LogConfig`:
- `FilterModules`: Filter theo module
- `FilterCollections`: Filter theo collection
- `FilterEndpoints`: Filter theo endpoint
- `FilterMethods`: Filter theo HTTP method
- `FilterLogTypes`: Filter theo log type

Tất cả filters mặc định là `"*"` (cho phép tất cả).

### 2. FilterHook (`api/core/logger/filter.go`)

Hook mới để lọc log entries:
- Parse filter config thành map để lookup nhanh
- Kiểm tra từng filter theo thứ tự: log type → module → collection → endpoint → method
- Đánh dấu entry bị filter bằng field `_filtered = true`
- Thread-safe với mutex

### 3. AsyncHook Integration (`api/core/logger/hook.go`)

Cập nhật `AsyncHook` để:
- Kiểm tra field `_filtered` trước khi ghi log
- Loại bỏ field `_filtered` khỏi entry trước khi format (không ghi vào log output)

### 4. Logger Integration (`api/core/logger/logger.go`)

Cập nhật `createLogger` để:
- Thêm `FilterHook` trước `AsyncHook` (filter trước khi đưa vào async queue)
- Filter áp dụng cho tất cả loggers (app, audit, performance, error)

### 5. Helper Functions (`api/core/logger/context.go`)

Thêm các helper functions:
- `WithModule(module string)`: Set module vào log entry
- `WithCollection(collection string)`: Set collection vào log entry
- `WithEndpoint(endpoint string)`: Set endpoint vào log entry
- `WithMethod(method string)`: Set HTTP method vào log entry
- `WithModuleAndCollection(module, collection string)`: Set cả module và collection
- `WithRequestInfo(c fiber.Ctx, module, collection string)`: Set đầy đủ thông tin request

---

## 🚀 Cấu Hình

### Environment Variables

Thêm các biến môi trường sau vào file `.env`:

```env
# Filter theo Module (comma-separated hoặc "*" cho tất cả)
LOG_FILTER_MODULES=*                    # Cho phép tất cả modules
# LOG_FILTER_MODULES=auth,notification  # Chỉ log từ auth và notification modules

# Filter theo Collection (comma-separated hoặc "*" cho tất cả)
LOG_FILTER_COLLECTIONS=*                # Cho phép tất cả collections
# LOG_FILTER_COLLECTIONS=users,orders   # Chỉ log từ users và orders collections

# Filter theo Endpoint (comma-separated hoặc "*" cho tất cả)
LOG_FILTER_ENDPOINTS=*                  # Cho phép tất cả endpoints
# LOG_FILTER_ENDPOINTS=/api/v1/users,/api/v1/orders  # Chỉ log từ các endpoints này

# Filter theo HTTP Method (comma-separated hoặc "*" cho tất cả)
LOG_FILTER_METHODS=*                    # Cho phép tất cả methods
# LOG_FILTER_METHODS=GET,POST           # Chỉ log GET và POST requests

# Filter theo Log Type (comma-separated hoặc "*" cho tất cả)
LOG_FILTER_LOG_TYPES=*                  # Cho phép tất cả log types
# LOG_FILTER_LOG_TYPES=error,warn       # Chỉ log errors và warnings
```

### Giá Trị Mặc Định

- Tất cả filters mặc định là `*` (cho phép tất cả)
- Nếu không set environment variable, filter sẽ không hoạt động (cho phép tất cả)

---

## 📝 Cách Sử Dụng Trong Code

### 1. Log với Module

```go
import "meta_commerce/core/logger"

// Log với module name
logger.WithModule("auth").Info("User authenticated successfully")

// Log với module và fields khác
logger.WithModule("notification").WithFields(map[string]interface{}{
    "eventType": "message_sent",
    "channelId": "123",
}).Info("Notification sent")
```

### 2. Log với Collection

```go
// Log với collection name
logger.WithCollection("users").Info("User created")

// Log với collection và fields khác
logger.WithCollection("orders").WithFields(map[string]interface{}{
    "orderId": "123",
    "total": 1000,
}).Info("Order created")
```

### 3. Log với Module và Collection

```go
// Log với cả module và collection
logger.WithModuleAndCollection("auth", "users").Info("User created in auth module")
```

### 4. Log với Endpoint và Method

```go
import "github.com/gofiber/fiber/v3"

// Trong handler
func (h *UserHandler) HandleCreateUser(c fiber.Ctx) error {
    // Log với request info (tự động có method, path, IP, request_id)
    logger.WithRequestInfo(c, "auth", "users").Info("Creating user")
    
    // Hoặc log riêng lẻ
    logger.WithEndpoint("/api/v1/users").WithMethod("POST").Info("Creating user")
    
    // ... business logic
}
```

### 5. Log với Request Context (Tự Động)

```go
// WithRequest tự động thêm method, path, IP, request_id
logger.WithRequest(c).Info("Request received")

// Kết hợp với module và collection
logger.WithRequestInfo(c, "auth", "users").Info("User operation")
```

---

## ⚙️ Cách Filter Hoạt Động

### Logic Filter

1. **FilterHook** được thêm vào logger trước AsyncHook
2. Khi có log entry mới, FilterHook kiểm tra các filters
3. Nếu entry không pass filter, đánh dấu `_filtered = true`
4. AsyncHook kiểm tra field `_filtered`, nếu true thì bỏ qua không ghi log
5. Nếu entry pass tất cả filters, ghi log bình thường

### Thứ Tự Kiểm Tra

1. **Log Type Filter**: Kiểm tra level của log entry (trace, debug, info, warn, error, fatal)
2. **Module Filter**: Kiểm tra field `module` trong log entry
3. **Collection Filter**: Kiểm tra field `collection` trong log entry
4. **Endpoint Filter**: Kiểm tra field `endpoint` hoặc `path` trong log entry
5. **Method Filter**: Kiểm tra field `method` trong log entry

Nếu bất kỳ filter nào không pass, log entry sẽ bị bỏ qua (không được ghi).

### Filter Logic

- **Nếu filter = `*` hoặc rỗng**: Cho phép tất cả (không filter)
- **Nếu filter có giá trị**: Chỉ cho phép các giá trị khớp
- **So sánh không phân biệt hoa thường**: `AUTH` = `auth` = `Auth`
- **Endpoint matching**: Hỗ trợ prefix matching (ví dụ: `/api/v1/users` khớp với `/api/v1/users/123`)
- **AND logic**: Tất cả filters phải pass thì log mới được ghi

---

## 🎯 Ví Dụ Các Trường Hợp Sử Dụng

### Ví Dụ 1: Chỉ Debug Module Auth

```env
LOG_FILTER_MODULES=auth
LOG_LEVEL=debug
```

Chỉ log các entries có `module: "auth"` ở level debug trở lên.

### Ví Dụ 2: Chỉ Log Errors và Warnings

```env
LOG_FILTER_LOG_TYPES=error,warn
```

Chỉ log errors và warnings, bỏ qua info, debug.

### Ví Dụ 3: Chỉ Log POST Requests

```env
LOG_FILTER_METHODS=POST
```

Chỉ log các requests có method POST.

### Ví Dụ 4: Chỉ Log Từ Endpoint Users

```env
LOG_FILTER_ENDPOINTS=/api/v1/users
```

Chỉ log các requests đến endpoint `/api/v1/users` (bao gồm cả sub-paths như `/api/v1/users/123`).

### Ví Dụ 5: Kết Hợp Nhiều Filters

```env
LOG_FILTER_MODULES=auth,notification
LOG_FILTER_COLLECTIONS=users,orders
LOG_FILTER_METHODS=POST,PUT
LOG_FILTER_LOG_TYPES=error,warn,info
```

Chỉ log:
- Từ modules `auth` hoặc `notification`
- Từ collections `users` hoặc `orders`
- Với methods `POST` hoặc `PUT`
- Với log types `error`, `warn`, hoặc `info`

---

## 📊 Performance

- Filter được thực hiện trước khi ghi log (trong hook)
- Sử dụng map lookup O(1) cho filter matching
- Thread-safe với mutex
- Không ảnh hưởng đến async logging performance

---

## 🔍 Debug Filter

### Kiểm Tra Filter Có Hoạt Động Không

Nếu bạn set filter nhưng vẫn thấy log không mong muốn:

1. **Kiểm tra log entry có field tương ứng không**:
   - Nếu không có field `module`, filter module sẽ không áp dụng
   - Nếu không có field `collection`, filter collection sẽ không áp dụng

2. **Kiểm tra giá trị có đúng format không**:
   - Module: lowercase (ví dụ: `auth` không phải `Auth`)
   - Collection: lowercase (ví dụ: `users` không phải `Users`)
   - Method: uppercase (ví dụ: `GET` không phải `get`)
   - Endpoint: đúng path (ví dụ: `/api/v1/users`)

3. **Kiểm tra environment variables**:
   ```bash
   # Trên Linux/Mac
   env | grep LOG_FILTER
   
   # Trên Windows PowerShell
   Get-ChildItem Env: | Where-Object {$_.Name -like "LOG_FILTER*"}
   ```

---

## 📌 Best Practices

### 1. Sử Dụng Module và Collection Trong Code

Luôn set module và collection khi log để filter hoạt động hiệu quả:

```go
// ✅ TỐT: Có module và collection
logger.WithModuleAndCollection("auth", "users").Info("User created")

// ❌ KHÔNG TỐT: Không có module/collection, filter không hoạt động
logger.Info("User created")
```

### 2. Sử Dụng WithRequestInfo Cho Handlers

Trong handlers, luôn dùng `WithRequestInfo` để tự động có đầy đủ thông tin:

```go
func (h *UserHandler) HandleCreateUser(c fiber.Ctx) error {
    // ✅ TỐT: Tự động có method, path, IP, request_id, module, collection
    logger.WithRequestInfo(c, "auth", "users").Info("Creating user")
    
    // ... business logic
}
```

### 3. Filter Theo Mục Đích Debug

- **Development**: Filter theo module đang debug
- **Production**: Filter theo log types (chỉ errors/warnings)
- **Performance Debug**: Filter theo endpoint chậm

### 4. Kết Hợp Với Log Level

```env
# Chỉ debug module auth ở level debug
LOG_LEVEL=debug
LOG_FILTER_MODULES=auth

# Chỉ log errors từ tất cả modules
LOG_LEVEL=error
LOG_FILTER_LOG_TYPES=error
```

---

## 🚨 Lưu Ý Quan Trọng

1. **Filter chỉ hoạt động nếu log entry có field tương ứng**:
   - Nếu không set `module` trong log, filter module sẽ không áp dụng
   - Nếu không set `collection` trong log, filter collection sẽ không áp dụng

2. **Filter là AND logic**:
   - Nếu set nhiều filters, tất cả phải pass thì log mới được ghi
   - Ví dụ: `LOG_FILTER_MODULES=auth` + `LOG_FILTER_METHODS=POST` = chỉ log POST requests từ auth module

3. **Filter không ảnh hưởng đến performance**:
   - Filter được thực hiện trước khi ghi log (trong hook)
   - Không ảnh hưởng đến async logging performance

4. **Filter áp dụng cho tất cả loggers**:
   - Filter áp dụng cho tất cả loggers (app, audit, performance, error)
   - Nếu muốn filter riêng, cần set field tương ứng trong log entry

---

## 📚 Ví Dụ Thực Tế

### Scenario 1: Debug Module Notification

```env
LOG_LEVEL=debug
LOG_FILTER_MODULES=notification
```

Code:
```go
logger.WithModule("notification").Debug("Sending notification")
logger.WithModule("notification").Info("Notification sent")
```

Kết quả: Chỉ log từ notification module ở level debug trở lên.

### Scenario 2: Chỉ Log Errors Từ Collection Users

```env
LOG_FILTER_COLLECTIONS=users
LOG_FILTER_LOG_TYPES=error
```

Code:
```go
logger.WithCollection("users").Error("Failed to create user")
logger.WithCollection("orders").Error("Failed to create order") // Bị filter
```

Kết quả: Chỉ log errors từ users collection.

### Scenario 3: Chỉ Log POST Requests Đến Endpoint Users

```env
LOG_FILTER_ENDPOINTS=/api/v1/users
LOG_FILTER_METHODS=POST
```

Code:
```go
// Trong handler
logger.WithRequestInfo(c, "auth", "users").Info("Creating user") // Được log
logger.WithRequestInfo(c, "auth", "users").Info("Getting user")  // Bị filter (GET)
```

Kết quả: Chỉ log POST requests đến `/api/v1/users`.

---

## 🔄 Cập Nhật Filter Runtime

Filter có thể được cập nhật runtime (nếu cần), nhưng thông thường nên restart server sau khi thay đổi environment variables.

---

## 📝 Files Đã Tạo/Sửa

### Files Mới
- `api/core/logger/filter.go`: FilterHook implementation

### Files Đã Sửa
- `api/core/logger/config.go`: Thêm filter config fields
- `api/core/logger/logger.go`: Tích hợp FilterHook
- `api/core/logger/hook.go`: Kiểm tra `_filtered` field
- `api/core/logger/context.go`: Thêm helper functions

---

## ✅ Testing Checklist

- [ ] Test filter theo module
- [ ] Test filter theo collection
- [ ] Test filter theo endpoint
- [ ] Test filter theo method
- [ ] Test filter theo log type
- [ ] Test kết hợp nhiều filters
- [ ] Test với filter = "*" (cho phép tất cả)
- [ ] Test với filter rỗng (cho phép tất cả)
- [ ] Test performance với nhiều log entries
- [ ] Test thread-safety

---

## 📞 Hỗ Trợ

Nếu có vấn đề với filter system, kiểm tra:
1. Environment variables có được set đúng không
2. Log entries có field tương ứng không
3. Giá trị filter có đúng format không (lowercase/uppercase)
4. Xem log initialization để biết filter nào đang active
