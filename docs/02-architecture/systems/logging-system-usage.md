# Hướng Dẫn Sử Dụng Hệ Thống Logging

## 📋 Tổng Quan

Hệ thống logging mới cung cấp:
- ✅ Log rotation tự động
- ✅ Structured logging với context
- ✅ Configurable log levels
- ✅ JSON format cho production, text format cho development
- ✅ Audit logging
- ✅ Performance logging

## 🚀 Khởi Tạo

Logger được khởi tạo tự động trong `main.go`:

```go
import "meta_commerce/core/logger"

func main() {
    // Logger tự động khởi tạo với cấu hình từ environment
    logger.Init(nil) // nil = sử dụng cấu hình mặc định
    
    // Hoặc với cấu hình tùy chỉnh
    cfg := &logger.LogConfig{
        Level:  "info",
        Format: "json",
        Output: "both",
    }
    logger.Init(cfg)
}
```

## 📝 Cấu Hình

### Environment Variables

Thêm vào file `.env`:

```env
# Log Level: trace, debug, info, warn, error, fatal
LOG_LEVEL=info

# Log Format: json, text
LOG_FORMAT=json

# Log Output: file, stdout, both
LOG_OUTPUT=both

# Log Rotation
LOG_MAX_SIZE=100        # MB
LOG_MAX_BACKUPS=7       # Số file cũ giữ lại
LOG_MAX_AGE=7           # Số ngày giữ lại
LOG_COMPRESS=true       # Nén file cũ

# Log Paths (tùy chọn)
LOG_PATH=./logs
LOG_APP_FILE=app.log
LOG_AUDIT_FILE=audit.log
LOG_PERFORMANCE_FILE=performance.log
```

### Mặc Định Theo Môi Trường

- **Development**: `LOG_LEVEL=debug`, `LOG_FORMAT=text`
- **Production**: `LOG_LEVEL=info`, `LOG_FORMAT=json`

## 🔧 Sử Dụng

### Basic Logging

```go
import "meta_commerce/core/logger"

// Lấy logger chính
log := logger.GetAppLogger()

// Log với các levels
log.Info("Application started")
log.Debug("Debug information")
log.Warn("Warning message")
log.Error("Error occurred")
log.Fatal("Fatal error - will exit")

// Log với fields
log.WithFields(map[string]interface{}{
    "user_id": "123",
    "action":  "login",
}).Info("User logged in")
```

### Request Context Logging

Tự động thêm request ID, method, path, IP vào log:

```go
import "meta_commerce/core/logger"

func handler(c fiber.Ctx) error {
    // Log với request context
    logger.WithRequest(c).Info("Processing request")
    
    // Log với fields bổ sung
    logger.WithRequest(c).WithFields(map[string]interface{}{
        "duration_ms": 150,
    }).Info("Request completed")
    
    return nil
}
```

### Context Logging

```go
import (
    "context"
    "meta_commerce/core/logger"
)

// Tạo context với thông tin
ctx := context.WithValue(context.Background(), logger.RequestIDKey, "req-123")
ctx = context.WithValue(ctx, logger.UserIDKey, "user-456")
ctx = context.WithValue(ctx, logger.OrganizationIDKey, "org-789")

// Log với context
logger.WithContext(ctx).Info("User action")
```

### Error Logging

```go
import "meta_commerce/core/logger"

err := someFunction()
if err != nil {
    // Log error với stack trace
    logger.WithError(err).Error("Failed to process")
    
    // Hoặc với request context
    logger.WithRequest(c).WithError(err).Error("Request failed")
}
```

### Audit Logging

Log các thao tác quan trọng để audit:

```go
import "meta_commerce/core/logger"

// Log CRUD operations
logger.LogCRUD("create", "user", userID, c, map[string]interface{}{
    "email": user.Email,
    "role":  user.Role,
})

// Log authentication
logger.LogAuth("login", c, map[string]interface{}{
    "method": "email",
})

// Log permission changes
logger.LogPermission("grant", c, map[string]interface{}{
    "role_id":       roleID,
    "permission_id": permID,
})

// Log custom action
logger.LogAction("custom_action", c, map[string]interface{}{
    "details": "custom details",
})
```

### Performance Logging

```go
import "meta_commerce/core/logger"

perfLogger := logger.GetPerformanceLogger()

start := time.Now()
// ... do work ...
duration := time.Since(start)

if duration > 1*time.Second {
    perfLogger.WithFields(map[string]interface{}{
        "duration_ms": duration.Milliseconds(),
        "operation":   "database_query",
        "query":       query,
    }).Warn("Slow operation detected")
}
```

## 📂 Cấu Trúc File Log

```
logs/
├── app.log              # Main application log
├── app.log.2025-01-15   # Rotated logs (theo ngày)
├── app.log.2025-01-14
├── audit.log            # Audit log
├── performance.log      # Performance log
└── error.log            # Error log (optional)
```

## 🔍 Log Rotation

Log rotation tự động:
- **Theo size**: Khi file đạt `LOG_MAX_SIZE` MB
- **Theo time**: Mỗi ngày (nếu có thay đổi)
- **Giữ lại**: `LOG_MAX_BACKUPS` file cũ
- **Nén**: File cũ được nén nếu `LOG_COMPRESS=true`

## 📊 Log Format

### Text Format (Development)

```
time="2025-01-15 10:30:45.123" level=info msg="User logged in" service=app func=handler file=handler.go:123 request_id=req-123 user_id=user-456
```

### JSON Format (Production)

```json
{
  "timestamp": "2025-01-15 10:30:45.123",
  "level": "info",
  "message": "User logged in",
  "service": "app",
  "function": "handler",
  "file": "handler.go:123",
  "request_id": "req-123",
  "user_id": "user-456"
}
```

## 🎯 Best Practices

1. **Sử dụng log levels phù hợp**:
   - `Debug`: Thông tin debug chi tiết
   - `Info`: Thông tin quan trọng về flow
   - `Warn`: Cảnh báo nhưng không phải lỗi
   - `Error`: Lỗi cần xử lý
   - `Fatal`: Lỗi nghiêm trọng, sẽ exit

2. **Luôn thêm context**:
   ```go
   // ❌ Không tốt
   log.Info("User created")
   
   // ✅ Tốt
   logger.WithRequest(c).WithFields(map[string]interface{}{
       "user_id": userID,
   }).Info("User created")
   ```

3. **Log errors với đầy đủ thông tin**:
   ```go
   // ❌ Không tốt
   log.Error("Failed")
   
   // ✅ Tốt
   logger.WithRequest(c).WithError(err).WithFields(map[string]interface{}{
       "operation": "create_user",
   }).Error("Failed to create user")
   ```

4. **Sử dụng audit logging cho các thao tác quan trọng**:
   ```go
   // CRUD operations
   logger.LogCRUD("create", "user", userID, c, details)
   
   // Authentication
   logger.LogAuth("login", c, details)
   
   // Permission changes
   logger.LogPermission("grant", c, details)
   ```

5. **Performance logging cho slow operations**:
   ```go
   if duration > threshold {
       perfLogger.Warn("Slow operation", fields)
   }
   ```

## 🔗 Migration từ Logrus Cũ

### Trước (Logrus trực tiếp)

```go
import "github.com/sirupsen/logrus"

logrus.WithFields(logrus.Fields{
    "user_id": userID,
}).Info("User created")
```

### Sau (Logger mới)

```go
import "meta_commerce/core/logger"

logger.WithRequest(c).WithFields(map[string]interface{}{
    "user_id": userID,
}).Info("User created")
```

## 📚 API Reference

### Functions

- `Init(cfg *LogConfig) error`: Khởi tạo logger system
- `GetLogger(name string) *logrus.Logger`: Lấy logger theo tên
- `GetAppLogger() *logrus.Logger`: Lấy app logger
- `GetAuditLogger() *logrus.Logger`: Lấy audit logger
- `GetPerformanceLogger() *logrus.Logger`: Lấy performance logger
- `GetErrorLogger() *logrus.Logger`: Lấy error logger
- `WithContext(ctx context.Context) *logrus.Entry`: Logger với context
- `WithRequest(c fiber.Ctx) *logrus.Entry`: Logger với request context
- `WithFields(fields map[string]interface{}) *logrus.Entry`: Logger với fields
- `WithError(err error) *logrus.Entry`: Logger với error
- `LogAction(action string, c fiber.Ctx, details map[string]interface{})`: Log audit action
- `LogCRUD(operation, resourceType, resourceID string, c fiber.Ctx, details map[string]interface{})`: Log CRUD
- `LogAuth(action string, c fiber.Ctx, details map[string]interface{})`: Log auth
- `LogPermission(action string, c fiber.Ctx, details map[string]interface{})`: Log permission

## 🐛 Troubleshooting

### Log không xuất hiện

1. Kiểm tra `LOG_LEVEL` - có thể level quá cao
2. Kiểm tra `LOG_OUTPUT` - có thể chỉ ghi file
3. Kiểm tra quyền ghi vào thư mục `logs/`

### File log quá lớn

1. Kiểm tra `LOG_MAX_SIZE` - giảm nếu cần
2. Kiểm tra log rotation có hoạt động không
3. Xóa file log cũ thủ công nếu cần

### Log format không đúng

1. Kiểm tra `LOG_FORMAT` - phải là `json` hoặc `text`
2. Kiểm tra environment variables có được load không
