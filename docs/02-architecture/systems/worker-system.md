# Hệ Thống Worker và Xử Lý Logic Định Kỳ

## 📋 Tổng Quan

Tài liệu này mô tả kiến trúc hệ thống worker để xử lý các logic định kỳ như:
- Rà soát conversation chưa được trả lời để nhắc nhở sale
- Kiểm tra trạng thái online của agent
- Xử lý các task background khác

## 🏗️ Kiến Trúc Đề Xuất

### Phương Án 1: Worker Service Riêng Biệt (Khuyến Nghị)

**Ưu điểm:**
- Tách biệt hoàn toàn với HTTP server
- Có thể scale độc lập
- Dễ quản lý và monitor
- Không ảnh hưởng đến performance của API server

**Cấu trúc:**
```
api/
├── cmd/
│   ├── server/          # HTTP API Server
│   └── worker/          # Background Worker Service
│       ├── main.go      # Entry point của worker
│       ├── init.go      # Khởi tạo dependencies
│       └── scheduler.go # Quản lý scheduled tasks
├── core/
│   ├── api/             # API layer (existing)
│   └── worker/          # Worker layer (NEW)
│       ├── jobs/        # Các job cụ thể
│       │   ├── conversation_monitor.go
│       │   └── agent_status_check.go
│       ├── scheduler/   # Scheduler logic
│       │   └── cron.go
│       └── notification/ # Notification services
│           └── alert.go
```

**Cách hoạt động:**
1. Worker chạy độc lập như một service riêng
2. Sử dụng cron scheduler để chạy các job định kỳ
3. Có thể deploy cùng server hoặc server riêng

### Phương Án 2: Worker Chạy Trong Server (Đơn Giản)

**Ưu điểm:**
- Đơn giản, không cần deploy riêng
- Dễ phát triển và debug
- Chia sẻ dependencies với server

**Nhược điểm:**
- Có thể ảnh hưởng đến performance của API
- Khó scale độc lập

**Cấu trúc:**
```
api/
├── cmd/
│   └── server/
│       ├── main.go      # Khởi động cả server và worker
│       └── worker.go    # Worker logic
├── core/
│   └── worker/          # Worker layer
│       └── jobs/        # Các job cụ thể
```

### Phương Án 3: Hybrid - Worker Service với Shared Core

**Ưu điểm:**
- Tách biệt deployment nhưng chia sẻ code
- Linh hoạt nhất
- Có thể chạy worker trong server khi cần

**Cấu trúc:**
```
api/
├── cmd/
│   ├── server/          # HTTP API Server
│   └── worker/          # Background Worker Service
├── core/
│   ├── api/             # API layer
│   └── worker/          # Worker layer (shared)
│       ├── jobs/        # Các job cụ thể
│       └── scheduler/    # Scheduler logic
```

## 🎯 Khuyến Nghị: Phương Án 1 - Worker Service Riêng Biệt

### Lý Do:
1. **Tách biệt concerns**: API server chỉ xử lý HTTP requests
2. **Scalability**: Có thể scale worker và server độc lập
3. **Reliability**: Worker crash không ảnh hưởng API server
4. **Monitoring**: Dễ monitor và debug từng service riêng

## 📁 Cấu Trúc Chi Tiết

### 1. Worker Entry Point

**File:** `api/cmd/worker/main.go`

```go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    
    "meta_commerce/internal/worker"
    "meta_commerce/internal/global"
    "github.com/sirupsen/logrus"
)

func main() {
    // Khởi tạo logger
    initLogger()
    
    // Khởi tạo global dependencies
    InitGlobal()
    
    // Khởi tạo registry
    InitRegistry()
    
    // Khởi tạo worker scheduler
    scheduler := worker.NewScheduler()
    
    // Đăng ký các jobs
    registerJobs(scheduler)
    
    // Chạy scheduler
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    // Graceful shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    go func() {
        <-sigChan
        logrus.Info("Shutting down worker...")
        cancel()
    }()
    
    if err := scheduler.Start(ctx); err != nil {
        logrus.Fatalf("Worker failed: %v", err)
    }
}
```

### 2. Worker Core

**File:** `api/internal/worker/scheduler/cron.go`

```go
package scheduler

import (
    "context"
    "time"
    
    "github.com/robfig/cron/v3"
    "github.com/sirupsen/logrus"
)

type Scheduler struct {
    cron *cron.Cron
    jobs []Job
}

type Job interface {
    Name() string
    Schedule() string  // Cron expression
    Run(ctx context.Context) error
}

func NewScheduler() *Scheduler {
    return &Scheduler{
        cron: cron.New(cron.WithSeconds()),
        jobs: make([]Job, 0),
    }
}

func (s *Scheduler) Register(job Job) {
    s.jobs = append(s.jobs, job)
    _, err := s.cron.AddFunc(job.Schedule(), func() {
        ctx := context.Background()
        if err := job.Run(ctx); err != nil {
            logrus.WithFields(logrus.Fields{
                "job": job.Name(),
                "error": err,
            }).Error("Job execution failed")
        }
    })
    if err != nil {
        logrus.Fatalf("Failed to register job %s: %v", job.Name(), err)
    }
}

func (s *Scheduler) Start(ctx context.Context) error {
    s.cron.Start()
    logrus.Info("Worker scheduler started")
    
    <-ctx.Done()
    stopCtx := s.cron.Stop()
    <-stopCtx.Done()
    
    logrus.Info("Worker scheduler stopped")
    return nil
}
```

### 3. Conversation Monitor Job

**File:** `api/internal/worker/jobs/conversation_monitor.go`

```go
package jobs

import (
    "context"
    "time"
    
    models "meta_commerce/internal/api/models/mongodb"
    "meta_commerce/internal/api/services"
    "meta_commerce/internal/worker/notification"
    "go.mongodb.org/mongo-driver/bson"
    "github.com/sirupsen/logrus"
)

type ConversationMonitorJob struct {
    conversationService *services.FbConversationService
    messageItemService  *services.FbMessageItemService
    alertService        *notification.AlertService
    thresholdMinutes    int64  // Số phút chưa trả lời để cảnh báo
}

func NewConversationMonitorJob(
    convService *services.FbConversationService,
    msgService *services.FbMessageItemService,
    alertService *notification.AlertService,
    thresholdMinutes int64,
) *ConversationMonitorJob {
    return &ConversationMonitorJob{
        conversationService: convService,
        messageItemService:  msgService,
        alertService:        alertService,
        thresholdMinutes:    thresholdMinutes,
    }
}

func (j *ConversationMonitorJob) Name() string {
    return "conversation_monitor"
}

func (j *ConversationMonitorJob) Schedule() string {
    // Chạy mỗi 5 phút
    return "*/5 * * * *"
}

func (j *ConversationMonitorJob) Run(ctx context.Context) error {
    logrus.Info("Running conversation monitor job")
    
    // Lấy tất cả conversations
    // TODO: Cần thêm method để query conversations chưa được trả lời
    conversations, err := j.findUnrepliedConversations(ctx)
    if err != nil {
        return err
    }
    
    for _, conv := range conversations {
        if err := j.checkAndAlert(ctx, conv); err != nil {
            logrus.WithFields(logrus.Fields{
                "conversationId": conv.ConversationId,
                "error": err,
            }).Error("Failed to check conversation")
        }
    }
    
    logrus.WithFields(logrus.Fields{
        "count": len(conversations),
    }).Info("Conversation monitor job completed")
    
    return nil
}

func (j *ConversationMonitorJob) findUnrepliedConversations(ctx context.Context) ([]models.FbConversation, error) {
    // Logic để tìm conversations chưa được trả lời
    // 1. Lấy tất cả conversations
    // 2. Với mỗi conversation, kiểm tra message cuối cùng
    // 3. Nếu message cuối cùng là từ customer và đã quá thresholdMinutes thì thêm vào danh sách
    
    // TODO: Implement logic này
    return nil, nil
}

func (j *ConversationMonitorJob) checkAndAlert(ctx context.Context, conv models.FbConversation) error {
    // Lấy message cuối cùng của conversation
    messages, _, err := j.messageItemService.FindByConversationId(ctx, conv.ConversationId, 1, 1)
    if err != nil {
        return err
    }
    
    if len(messages) == 0 {
        return nil
    }
    
    lastMessage := messages[0]
    
    // Kiểm tra message cuối cùng có phải từ customer không
    // TODO: Cần kiểm tra messageData để xác định sender
    isFromCustomer := j.isMessageFromCustomer(lastMessage.MessageData)
    
    if !isFromCustomer {
        // Đã được trả lời rồi
        return nil
    }
    
    // Kiểm tra thời gian
    now := time.Now().Unix()
    timeDiff := now - lastMessage.InsertedAt
    thresholdSeconds := j.thresholdMinutes * 60
    
    if timeDiff > thresholdSeconds {
        // Gửi cảnh báo
        return j.alertService.SendConversationAlert(ctx, conv, timeDiff)
    }
    
    return nil
}

func (j *ConversationMonitorJob) isMessageFromCustomer(messageData map[string]interface{}) bool {
    // TODO: Kiểm tra messageData để xác định sender
    // Có thể dựa vào field "from" hoặc "direction" trong messageData
    // Ví dụ: direction == "incoming" hoặc from.id == customerId
    return false
}
```

### 4. Notification Service

**File:** `api/internal/worker/notification/alert.go`

```go
package notification

import (
    "context"
    "fmt"
    
    models "meta_commerce/internal/api/models/mongodb"
    "github.com/sirupsen/logrus"
)

type AlertService struct {
    // Có thể tích hợp với:
    // - Email service
    // - SMS service
    // - Slack webhook
    // - Telegram bot
    // - In-app notification
}

func NewAlertService() *AlertService {
    return &AlertService{}
}

func (s *AlertService) SendConversationAlert(
    ctx context.Context,
    conv models.FbConversation,
    timeDiffSeconds int64,
) error {
    message := fmt.Sprintf(
        "Conversation %s chưa được trả lời trong %d phút",
        conv.ConversationId,
        timeDiffSeconds/60,
    )
    
    logrus.WithFields(logrus.Fields{
        "conversationId": conv.ConversationId,
        "pageId": conv.PageId,
        "timeDiffMinutes": timeDiffSeconds / 60,
    }).Warn(message)
    
    // TODO: Gửi notification thực sự
    // - Gửi email cho sale
    // - Gửi Slack notification
    // - Gửi Telegram message
    // - Tạo in-app notification
    
    return nil
}
```

## 🔧 Triển Khai

### Bước 1: Tạo Cấu Trúc Thư Mục

```bash
mkdir -p api/internal/worker/jobs
mkdir -p api/internal/worker/scheduler
mkdir -p api/internal/worker/notification
```

### Bước 2: Thêm Dependencies

**File:** `api/go.mod`

```go
require (
    github.com/robfig/cron/v3 v3.0.1
    // ... existing dependencies
)
```

### Bước 3: Tạo Worker Entry Point

Tạo các file theo cấu trúc đã đề xuất ở trên.

### Bước 4: Cấu Hình Deployment

**File:** `deploy_notes/worker.service.txt`

```ini
[Unit]
Description=FolkForm Worker Service
After=network.target mongodb.service

[Service]
Type=simple
User=folkform
WorkingDirectory=/opt/folkform-auth/api
ExecStart=/opt/folkform-auth/api/worker
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=folkform-worker

[Install]
WantedBy=multi-user.target
```

## 📊 Monitoring và Logging

### Logging

Worker sẽ log các thông tin:
- Khi nào job được chạy
- Kết quả của mỗi job
- Lỗi nếu có
- Số lượng conversations được kiểm tra

### Metrics (Tùy chọn)

Có thể tích hợp với Prometheus để track:
- Số lượng jobs đã chạy
- Thời gian thực thi của mỗi job
- Số lượng alerts đã gửi
- Error rate

## 🔄 Các Job Khác Có Thể Thêm

1. **Agent Status Check**: Kiểm tra và cập nhật trạng thái online của agent
2. **Data Sync**: Đồng bộ dữ liệu từ Pancake API định kỳ
3. **Cleanup Job**: Dọn dẹp dữ liệu cũ
4. **Report Generation**: Tạo báo cáo định kỳ
5. **Health Check**: Kiểm tra health của các service khác

## 📝 Lưu Ý

1. **Error Handling**: Mỗi job cần có error handling riêng, không để một job lỗi làm crash toàn bộ worker
2. **Context Cancellation**: Sử dụng context để có thể cancel jobs khi shutdown
3. **Resource Management**: Đảm bảo đóng connections và cleanup resources sau mỗi job
4. **Configuration**: Các threshold và schedule nên được config từ environment variables
5. **Testing**: Viết unit tests cho mỗi job

## 🚀 Next Steps

1. Implement conversation monitor job với logic đầy đủ
2. Tích hợp notification service (email/Slack/Telegram)
3. Thêm các job khác theo nhu cầu
4. Setup monitoring và alerting
5. Viết documentation cho từng job


