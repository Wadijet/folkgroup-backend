# Đề Xuất Hệ Thống Logging Chuyên Nghiệp

## 📋 Tổng Quan

Tài liệu này đề xuất một hệ thống logging chuyên nghiệp, có cấu trúc và dễ quản lý cho dự án.

## 🔍 Phân Tích Hiện Trạng

### Vấn Đề Hiện Tại

1. **Duplicate Code**: Có 2 cách khởi tạo logger (`initLogger()` trong main.go và `GetLogger()` trong logger.go)
2. **Không có Log Rotation**: File log sẽ lớn dần, khó quản lý
3. **Level Log Cố Định**: Luôn là DebugLevel, không configurable
4. **Format Đơn Giản**: Chỉ có text format, không có JSON cho production
5. **Thiếu Context**: Không có request ID, user ID, organization ID trong log
6. **Inconsistent**: Một số nơi dùng `log.Printf`, một số dùng `logrus`
7. **Không có Audit Log**: Không log các thao tác quan trọng
8. **Không có Performance Logging**: Không track slow queries, request duration

## ✅ Phương Án Đề Xuất

### 1. Unified Logging System

**Mục tiêu**: Một package logger thống nhất, dễ sử dụng

**Tính năng**:
- Package `core/logger` duy nhất
- Hỗ trợ nhiều logger instances (app, worker, audit, etc.)
- Thread-safe với sync.Mutex
- Singleton pattern để đảm bảo consistency

### 2. Log Rotation

**Mục tiêu**: Quản lý file log tự động, tránh file quá lớn

**Tính năng**:
- Rotate theo size (ví dụ: 100MB)
- Rotate theo time (ví dụ: hàng ngày)
- Giữ lại N file logs cũ (ví dụ: 7 ngày)
- Sử dụng thư viện `gopkg.in/natefinch/lumberjack.v2`

**Cấu hình**:
```go
type LogConfig struct {
    MaxSize    int  // MB
    MaxBackups int  // Số file cũ giữ lại
    MaxAge     int  // Số ngày giữ lại
    Compress   bool // Nén file cũ
}
```

### 3. Structured Logging với Context

**Mục tiêu**: Log có cấu trúc, dễ query và phân tích

**Tính năng**:
- JSON format cho production
- Text format cho development
- Context fields tự động: requestID, userID, organizationID, service, etc.
- Middleware để inject context vào logger

**Ví dụ**:
```go
logger.WithContext(ctx).WithFields(logrus.Fields{
    "user_id": userID,
    "action": "create_user",
}).Info("User created successfully")
```

### 4. Configurable Log Levels

**Mục tiêu**: Điều chỉnh log level theo môi trường

**Tính năng**:
- Config từ environment variable `LOG_LEVEL`
- Default: `DEBUG` cho development, `INFO` cho production
- Hỗ trợ: TRACE, DEBUG, INFO, WARN, ERROR, FATAL

**Cấu hình**:
```env
LOG_LEVEL=info
LOG_FORMAT=json  # json hoặc text
LOG_OUTPUT=file  # file, stdout, hoặc both
```

### 5. Request Context Logging

**Mục tiêu**: Tự động thêm request context vào mọi log

**Tính năng**:
- Middleware để inject request ID vào logger context
- Tự động thêm user ID, organization ID nếu có
- Helper functions để log với context

**Implementation**:
```go
// Middleware
func LoggingMiddleware(c fiber.Ctx) error {
    ctx := context.WithValue(c.UserContext(), "requestID", c.Get("X-Request-ID"))
    c.SetUserContext(ctx)
    return c.Next()
}

// Usage
logger.WithRequest(c).Info("Processing request")
```

### 6. Performance Logging

**Mục tiêu**: Track performance và slow operations

**Tính năng**:
- Slow query logging (queries > threshold)
- Request duration logging
- Memory/CPU metrics (optional)
- Separate performance log file

**Ví dụ**:
```go
logger.WithPerformance().WithFields(logrus.Fields{
    "duration_ms": 1500,
    "query": "SELECT * FROM users",
    "threshold_ms": 1000,
}).Warn("Slow query detected")
```

### 7. Error Tracking

**Mục tiêu**: Log errors với đầy đủ context và stack trace

**Tính năng**:
- Structured error logging
- Stack trace tự động cho errors
- Error categorization
- Có thể tích hợp Sentry sau

**Ví dụ**:
```go
logger.WithError(err).WithFields(logrus.Fields{
    "error_code": "AUTH_001",
    "error_type": "authentication",
}).Error("Authentication failed")
```

### 8. Audit Logging

**Mục tiêu**: Log các thao tác quan trọng để audit

**Tính năng**:
- Separate audit log file
- Log CRUD operations
- Log authentication events
- Log permission changes
- Log data access

**Ví dụ**:
```go
auditLogger.LogAction("user_create", map[string]interface{}{
    "user_id": userID,
    "created_by": adminID,
    "ip": c.IP(),
})
```

## 🏗️ Kiến Trúc

### Cấu Trúc Package

```
api/internal/logger/
├── logger.go          # Main logger package
├── config.go          # Log configuration
├── context.go         # Context helpers
├── rotation.go        # Log rotation
├── formatter.go       # Custom formatters
└── audit.go           # Audit logger
```

### Logger Types

1. **App Logger**: Log chính của ứng dụng
2. **Audit Logger**: Log các thao tác audit
3. **Performance Logger**: Log performance metrics
4. **Error Logger**: Log errors (có thể tích hợp với error tracking service)

### File Structure

```
logs/
├── app.log              # Main application log
├── app.log.2025-01-15   # Rotated logs
├── audit.log            # Audit log
├── performance.log      # Performance log
└── error.log            # Error log (optional)
```

## 📝 API Design

### Basic Usage

```go
import "meta_commerce/internal/logger"

// Get default logger
log := logger.GetLogger("app")

// Log với level
log.Info("Application started")
log.Error("Failed to connect to database")
log.Debug("Processing request")

// Log với fields
log.WithFields(logrus.Fields{
    "user_id": "123",
    "action": "login",
}).Info("User logged in")
```

### Context Logging

```go
// Với request context
log := logger.WithRequest(c)
log.Info("Request processed")

// Với custom context
ctx := context.WithValue(context.Background(), "userID", "123")
log := logger.WithContext(ctx)
log.Info("User action")
```

### Audit Logging

```go
audit := logger.GetAuditLogger()
audit.LogAction("user_create", map[string]interface{}{
    "user_id": userID,
    "created_by": adminID,
    "ip": c.IP(),
    "timestamp": time.Now(),
})
```

## 🔧 Configuration

### Environment Variables

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

# Log Paths
LOG_PATH=./logs
LOG_APP_FILE=app.log
LOG_AUDIT_FILE=audit.log
LOG_PERFORMANCE_FILE=performance.log
```

### Code Configuration

```go
type LogConfig struct {
    Level       string // trace, debug, info, warn, error, fatal
    Format      string // json, text
    Output      string // file, stdout, both
    MaxSize     int    // MB
    MaxBackups  int
    MaxAge      int    // days
    Compress    bool
    LogPath     string
    AppFile     string
    AuditFile   string
    PerfFile    string
}
```

## 🚀 Migration Plan

### Phase 1: Core Logger (Ưu tiên cao)
1. ✅ Tạo unified logger package
2. ✅ Implement log rotation
3. ✅ Configurable log levels
4. ✅ JSON/Text formatters

### Phase 2: Context & Middleware (Ưu tiên cao)
1. ✅ Request context middleware
2. ✅ Context helpers
3. ✅ Replace tất cả log.Printf với logger

### Phase 3: Advanced Features (Ưu tiên trung bình)
1. ⏳ Audit logging
2. ⏳ Performance logging
3. ⏳ Error tracking integration

### Phase 4: Monitoring (Ưu tiên thấp)
1. ⏳ Metrics collection
2. ⏳ Log aggregation (ELK, Loki, etc.)
3. ⏳ Alerting

## 📊 Benefits

1. **Dễ Debug**: Structured logging với context giúp trace issues nhanh hơn
2. **Dễ Quản Lý**: Log rotation tự động, không lo file quá lớn
3. **Production Ready**: JSON format, configurable levels
4. **Audit Trail**: Log đầy đủ các thao tác quan trọng
5. **Performance Insights**: Track slow operations
6. **Scalable**: Dễ tích hợp với log aggregation tools

## 🔗 Dependencies

```go
require (
    github.com/sirupsen/logrus v1.9.3
    gopkg.in/natefinch/lumberjack.v2 v2.2.1
)
```

## 📚 References

- [Logrus Documentation](https://github.com/sirupsen/logrus)
- [Lumberjack Documentation](https://github.com/natefinch/lumberjack)
- [Structured Logging Best Practices](https://www.loggly.com/ultimate-guide/node-logging-basics/)
