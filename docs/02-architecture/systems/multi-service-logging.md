# Chạy Nhiều Service Đồng Thời và Quản Lý Log

## 📋 Tổng Quan

Khi có nhiều service trong cùng workspace (api, api-worker, agent_pancake), cần tổ chức log để không bị rối khi chạy đồng thời.

## ✅ Chạy Đồng Thời - CÓ THỂ

### Go Workspace Cho Phép

Go workspace **hoàn toàn cho phép** chạy nhiều module đồng thời:

```bash
# Terminal 1: Chạy API server
cd api
go run cmd/server/main.go

# Terminal 2: Chạy Worker
cd api-worker  
go run cmd/worker/main.go

# Terminal 3: Chạy agent_pancake
cd agent_pancake
go run main.go
```

**Mỗi module chạy độc lập**, không ảnh hưởng nhau về:
- Process riêng biệt
- Port riêng (nếu có HTTP server)
- Dependencies riêng (mỗi module có go.mod riêng)
- Memory riêng

## ⚠️ Vấn Đề: Log Bị Rối

### Vấn Đề Hiện Tại

Nếu tất cả service ghi vào cùng 1 file log → **SẼ RỐI**:

```
logs/
└── app.log  ← Cả 3 service ghi vào đây → RỐI!
```

**Ví dụ log bị rối:**
```
[INFO] [2025-01-15 10:00:01.000] [api-server] Starting server...
[INFO] [2025-01-15 10:00:01.100] [worker] Running job...
[INFO] [2025-01-15 10:00:01.200] [agent] Syncing conversations...
[INFO] [2025-01-15 10:00:01.300] [api-server] Request received...
[INFO] [2025-01-15 10:00:01.400] [worker] Job completed...
```

→ **Khó debug**, không biết log nào của service nào!

## ✅ Giải Pháp: Log Riêng Cho Mỗi Service

### Cấu Trúc Log Đề Xuất

```
ff_be_auth/
└── logs/
    ├── api-server.log      # Log của API server
    ├── api-worker.log       # Log của worker
    ├── agent-pancake.log    # Log của agent_pancake
    └── combined.log         # Tất cả (optional)
```

### Cách 1: Log File Riêng (Khuyến Nghị)

**Mỗi service có log file riêng:**

```go
// api/cmd/server/main.go
func initLogger() {
    logFile := filepath.Join(logPath, "api-server.log")
    // ...
}

// api-worker/cmd/worker/main.go
func initLogger() {
    logFile := filepath.Join(logPath, "api-worker.log")
    // ...
}

// agent_pancake/main.go
func initLogger() {
    logFile := filepath.Join(logPath, "agent-pancake.log")
    // ...
}
```

**Ưu điểm:**
- ✅ Tách biệt rõ ràng
- ✅ Dễ tìm log của từng service
- ✅ Có thể xóa log của 1 service mà không ảnh hưởng service khác

### Cách 2: Log với Prefix/Service Name

**Thêm service name vào mỗi log entry:**

```go
// api/cmd/server/main.go
logrus.WithFields(logrus.Fields{
    "service": "api-server",
    "port": 8080,
}).Info("Server starting")

// api-worker/cmd/worker/main.go
logrus.WithFields(logrus.Fields{
    "service": "api-worker",
    "job": "conversation-monitor",
}).Info("Job started")
```

**Kết quả:**
```
[INFO] [2025-01-15 10:00:01.000] service=api-server Starting server...
[INFO] [2025-01-15 10:00:01.100] service=api-worker job=conversation-monitor Running...
[INFO] [2025-01-15 10:00:01.200] service=agent-pancake action=sync Syncing...
```

### Cách 3: Structured Logging với JSON

**Log dạng JSON, dễ filter:**

```go
logrus.SetFormatter(&logrus.JSONFormatter{})

logrus.WithFields(logrus.Fields{
    "service": "api-server",
    "level": "info",
    "message": "Server starting",
    "port": 8080,
}).Info()
```

**Kết quả:**
```json
{"level":"info","service":"api-server","message":"Server starting","port":8080,"time":"2025-01-15T10:00:01Z"}
{"level":"info","service":"api-worker","job":"conversation-monitor","time":"2025-01-15T10:00:01Z"}
```

**Filter log:**
```bash
# Chỉ xem log của api-server
cat logs/combined.log | jq 'select(.service=="api-server")'

# Chỉ xem ERROR
cat logs/combined.log | jq 'select(.level=="error")'
```

## 🔧 Implementation

### Cải Thiện Logger Hiện Tại

**File:** `api/core/logger/logger.go`

```go
package logger

import (
    "fmt"
    "io"
    "os"
    "path/filepath"
    "runtime"
    "strings"
    "sync"
    
    "github.com/sirupsen/logrus"
)

var (
    loggers   = make(map[string]*logrus.Logger)
    loggersMu sync.Mutex
    rootDir   string
)

// GetLogger trả về logger theo tên service
func GetLogger(serviceName string) *logrus.Logger {
    loggersMu.Lock()
    defer loggersMu.Unlock()
    
    if logger, ok := loggers[serviceName]; ok {
        return logger
    }
    
    // Tạo logger mới với file riêng
    logPath := filepath.Join(getRootDir(), "logs")
    logFile := filepath.Join(logPath, fmt.Sprintf("%s.log", serviceName))
    
    if err := os.MkdirAll(logPath, 0755); err != nil {
        panic(fmt.Sprintf("Could not create logs directory: %v", err))
    }
    
    file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
    if err != nil {
        panic(fmt.Sprintf("Could not open log file: %v", err))
    }
    
    logger := logrus.New()
    logger.SetFormatter(&logrus.TextFormatter{
        FullTimestamp:   true,
        TimestampFormat: "2006-01-02 15:04:05.000",
        CallerPrettyfier: func(f *runtime.Frame) (string, string) {
            s := strings.Split(f.Function, ".")
            funcName := s[len(s)-1]
            return funcName, filepath.Base(f.File)
        },
    })
    
    // Ghi vào cả stdout và file riêng
    mw := io.MultiWriter(os.Stdout, file)
    logger.SetOutput(mw)
    logger.SetReportCaller(true)
    logger.SetLevel(logrus.DebugLevel)
    
    // Thêm service name vào mỗi log
    logger = logger.WithField("service", serviceName).Logger
    
    loggers[serviceName] = logger
    return logger
}

func getRootDir() string {
    if rootDir != "" {
        return rootDir
    }
    executable, err := os.Executable()
    if err != nil {
        panic(fmt.Sprintf("Could not get executable path: %v", err))
    }
    rootDir = filepath.Dir(filepath.Dir(filepath.Dir(executable)))
    return rootDir
}
```

### Sử Dụng Trong Mỗi Service

**api/cmd/server/main.go:**
```go
import "meta_commerce/core/logger"

func main() {
    log := logger.GetLogger("api-server")
    log.Info("API Server starting...")
    // ...
}
```

**api-worker/cmd/worker/main.go:**
```go
import "meta_commerce/core/logger"

func main() {
    log := logger.GetLogger("api-worker")
    log.Info("Worker starting...")
    // ...
}
```

**agent_pancake/main.go:**
```go
import "meta_commerce/core/logger"

func main() {
    log := logger.GetLogger("agent-pancake")
    log.Info("Agent starting...")
    // ...
}
```

## 🐛 Debug Khi Chạy Nhiều Service

### Cách 1: Xem Log Riêng

```bash
# Xem log của API server
tail -f logs/api-server.log

# Xem log của worker
tail -f logs/api-worker.log

# Xem log của agent
tail -f logs/agent-pancake.log
```

### Cách 2: Xem Tất Cả Log (Multi-tail)

**Windows (PowerShell):**
```powershell
# Xem nhiều file cùng lúc
Get-Content logs/api-server.log, logs/api-worker.log, logs/agent-pancake.log -Wait
```

**Linux/Mac:**
```bash
# Cài đặt multitail
sudo apt install multitail  # Ubuntu
brew install multitail       # Mac

# Xem nhiều file
multitail logs/api-server.log logs/api-worker.log logs/agent-pancake.log
```

### Cách 3: Filter Log Theo Service

**Với structured logging (JSON):**
```bash
# Chỉ xem log của api-server
cat logs/combined.log | jq 'select(.service=="api-server")'

# Chỉ xem ERROR
cat logs/combined.log | jq 'select(.level=="error")'

# Chỉ xem log của worker có ERROR
cat logs/combined.log | jq 'select(.service=="api-worker" and .level=="error")'
```

### Cách 4: Debug với IDE

**VS Code - Multiple Terminals:**

1. Mở terminal cho mỗi service
2. Chạy từng service trong terminal riêng
3. Xem log trong Output panel riêng

**Cấu hình launch.json:**
```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "API Server",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "${workspaceFolder}/api/cmd/server",
            "console": "integratedTerminal",
            "env": {
                "LOG_SERVICE": "api-server"
            }
        },
        {
            "name": "Worker",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "${workspaceFolder}/api-worker/cmd/worker",
            "console": "integratedTerminal",
            "env": {
                "LOG_SERVICE": "api-worker"
            }
        }
    ],
    "compounds": [
        {
            "name": "All Services",
            "configurations": ["API Server", "Worker"],
            "stopAll": true
        }
    ]
}
```

## 📊 So Sánh Các Phương Án

| Phương án | Độ phức tạp | Dễ debug | Performance | Khuyến nghị |
|-----------|-------------|----------|-------------|-------------|
| **Log file riêng** | Thấp | ✅ Rất dễ | ✅ Tốt | ⭐⭐⭐⭐⭐ |
| **Prefix/Service name** | Trung bình | ✅ Dễ | ✅ Tốt | ⭐⭐⭐⭐ |
| **JSON structured** | Cao | ✅ Rất dễ | ⚠️ Chậm hơn | ⭐⭐⭐ |

## 🎯 Khuyến Nghị

**Kết hợp:**
1. **Log file riêng** cho mỗi service (chính)
2. **Service name trong log** để dễ filter
3. **JSON format** (optional) nếu cần query phức tạp

**Cấu trúc log:**
```
logs/
├── api-server.log          # API server logs
├── api-worker.log          # Worker logs  
├── agent-pancake.log       # Agent logs
└── combined.log            # Tất cả (optional, nếu cần)
```

## 📝 Checklist

- [ ] Mỗi service có log file riêng
- [ ] Service name trong mỗi log entry
- [ ] Cấu hình log level riêng cho từng service
- [ ] Script để xem log của nhiều service cùng lúc
- [ ] Rotation log để tránh file quá lớn
- [ ] Cleanup log cũ (optional)

## 🔗 Tài Liệu Liên Quan

- [Worker System Architecture](worker-system.md)
- [Logging Documentation](../07-troubleshooting/phan-tich-log.md)


