# Tóm Tắt Phương Án Filter Log System

## 📋 Tổng Quan

Đã triển khai hệ thống filter log cho phép bật/tắt log theo các tiêu chí:
- **Module**: Tên module (auth, notification, delivery, content, ai, ...)
- **Collection**: Tên collection MongoDB (users, orders, notifications, ...)
- **Endpoint**: Đường dẫn endpoint (/api/v1/users, /api/v1/orders, ...)
- **Method**: HTTP method (GET, POST, PUT, DELETE)
- **Log Type**: Loại log (trace, debug, info, warn, error, fatal)

## 🏗️ Kiến Trúc

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

## 🚀 Cách Sử Dụng

### Environment Variables

```env
# Filter theo Module
LOG_FILTER_MODULES=auth,notification

# Filter theo Collection
LOG_FILTER_COLLECTIONS=users,orders

# Filter theo Endpoint
LOG_FILTER_ENDPOINTS=/api/v1/users,/api/v1/orders

# Filter theo Method
LOG_FILTER_METHODS=GET,POST

# Filter theo Log Type
LOG_FILTER_LOG_TYPES=error,warn
```

### Code Example

```go
// Log với module
logger.WithModule("auth").Info("User authenticated")

// Log với collection
logger.WithCollection("users").Info("User created")

// Log với request info (tự động có method, path, IP, request_id)
logger.WithRequestInfo(c, "auth", "users").Info("Creating user")
```

## ⚙️ Cách Hoạt Động

1. **FilterHook** được thêm vào logger trước AsyncHook
2. Khi có log entry mới, FilterHook kiểm tra các filters
3. Nếu entry không pass filter, đánh dấu `_filtered = true`
4. AsyncHook kiểm tra field `_filtered`, nếu true thì bỏ qua không ghi log
5. Nếu entry pass tất cả filters, ghi log bình thường

## 📊 Performance

- Filter được thực hiện trước khi ghi log (trong hook)
- Sử dụng map lookup O(1) cho filter matching
- Thread-safe với mutex
- Không ảnh hưởng đến async logging performance

## 🔍 Filter Logic

- **Nếu filter = `*` hoặc rỗng**: Cho phép tất cả (không filter)
- **Nếu filter có giá trị**: Chỉ cho phép các giá trị khớp
- **So sánh không phân biệt hoa thường**: `AUTH` = `auth` = `Auth`
- **Endpoint matching**: Hỗ trợ prefix matching
- **AND logic**: Tất cả filters phải pass thì log mới được ghi

## 📝 Files Đã Tạo/Sửa

### Files Mới
- `api/core/logger/filter.go`: FilterHook implementation
- `docs/02-architecture/log-filter-system.md`: Documentation đầy đủ
- `docs/02-architecture/log-filter-system-summary.md`: File này

### Files Đã Sửa
- `api/core/logger/config.go`: Thêm filter config fields
- `api/core/logger/logger.go`: Tích hợp FilterHook
- `api/core/logger/hook.go`: Kiểm tra `_filtered` field
- `api/core/logger/context.go`: Thêm helper functions

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

## 🚨 Lưu Ý

1. **Filter chỉ hoạt động nếu log entry có field tương ứng**:
   - Nếu không set `module` trong log, filter module sẽ không áp dụng
   - Nếu không set `collection` trong log, filter collection sẽ không áp dụng

2. **Luôn set module và collection khi log** để filter hoạt động hiệu quả:
   ```go
   // ✅ TỐT
   logger.WithModuleAndCollection("auth", "users").Info("User created")
   
   // ❌ KHÔNG TỐT (filter không hoạt động)
   logger.Info("User created")
   ```

3. **Filter áp dụng cho tất cả loggers**: app, audit, performance, error

4. **Restart server** sau khi thay đổi environment variables để filter có hiệu lực

## 📚 Tài Liệu Tham Khảo

Xem file `docs/02-architecture/log-filter-system.md` để biết chi tiết về cách sử dụng và các ví dụ.