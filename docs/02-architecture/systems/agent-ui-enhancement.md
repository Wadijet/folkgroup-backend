# Cải Thiện Giao Diện Thân Thiện Cho Hệ Thống Agent

## 📋 Tổng Quan

Tài liệu này mô tả các cải thiện để làm cho hệ thống agent thân thiện hơn với người dùng, bao gồm:
- Thông tin mô tả cho agent
- Thông tin mô tả cho từng job
- Các thông tin bổ sung để giao diện thân thiện hơn

## 🤖 Agent Registry - Thông Tin Thân Thiện

### Các Trường Đã Thêm

#### 1. Thông Tin Cơ Bản
- **Name** (`string`): Tên agent (hiển thị cho user)
- **DisplayName** (`string`): Tên hiển thị đầy đủ (nếu khác với Name)
- **Description** (`string`): Mô tả chi tiết về agent, chức năng, mục đích sử dụng

#### 2. Thông Tin Hiển Thị (UI-friendly)
- **Icon** (`string`): Icon/emoji cho agent
  - Ví dụ: "🤖" (bot), "📊" (monitoring), "🔔" (notification), "🔄" (sync)
  - Có thể dùng emoji hoặc icon class name (nếu dùng icon library)
  
- **Color** (`string`): Màu sắc cho agent (hex color)
  - Ví dụ: "#3B82F6" (blue), "#10B981" (green), "#F59E0B" (amber)
  - Dùng để highlight agent trong danh sách, badge, status indicator
  
- **Category** (`string`): Danh mục agent
  - Ví dụ: "monitoring", "data-sync", "notification", "backup", "cleanup"
  - Dùng để nhóm agent theo chức năng
  
- **Tags** (`[]string`): Tags để phân loại và tìm kiếm
  - Ví dụ: ["production", "critical", "monitoring", "high-priority"]
  - Cho phép filter và search agent dễ dàng

### Ví Dụ Sử Dụng

```json
{
  "agentId": "monitoring-bot-001",
  "name": "Monitoring Bot",
  "displayName": "Production Monitoring Bot - Server 01",
  "description": "Bot giám sát trạng thái hệ thống, kiểm tra health check, và gửi cảnh báo khi có sự cố",
  "icon": "📊",
  "color": "#3B82F6",
  "category": "monitoring",
  "tags": ["production", "critical", "monitoring", "server-01"]
}
```

## 📝 Job Structure - Cấu Trúc Job Với Mô Tả

### Cấu Trúc Job Trong ConfigData

Jobs được lưu trong `AgentConfig.ConfigData.jobs` dưới dạng array của job objects.

### Cấu Trúc Job Chuẩn

**⚠️ LƯU Ý QUAN TRỌNG**: Metadata chung của job (displayName, description, icon, color, category, tags) **KHÔNG được lưu trong config** nữa. Metadata chung được lưu riêng trong `AgentRegistry.JobMetadata`.

**Lưu ý**: Metadata của các field trong config (ví dụ: params có thể có metadata riêng như description cho từng param) vẫn được giữ trong config như trước.

**Config chỉ chứa job definition (không có metadata chung của job):**
```json
{
  "configData": {
    "jobs": [
      {
        "name": "conversation_monitor",
        "enabled": true,
        "schedule": "0 */5 * * * *",
        "timeout": 300,
        "retries": 3,
        "params": {
          "threshold": 300,
          "alertChannels": ["email", "slack"]
          // Metadata của params (nếu có) vẫn được giữ trong config
        }
        // KHÔNG có metadata chung của job ở đây (displayName, description, icon, color, category, tags)
      }
    ]
  }
}
```

**Metadata được lưu trong AgentRegistry:**
```json
{
  "jobMetadata": {
    "conversation_monitor": {
      "displayName": "Giám Sát Conversation",
      "description": "Job kiểm tra các conversation chưa được trả lời và gửi cảnh báo cho sale",
      "icon": "💬",
      "color": "#10B981",
      "category": "monitoring",
      "tags": ["conversation", "alert", "critical"]
    }
  }
}
```

### Các Trường Job

#### Trường Bắt Buộc
- **name** (`string`): Tên job (unique identifier)
- **enabled** (`boolean`): Job có được bật hay không
- **schedule** (`string`): Cron expression cho lịch chạy job

#### Trường Tùy Chọn - Metadata Chung Của Job (⚠️ LƯU Ý: Metadata chung KHÔNG được lưu trong config)
- **displayName** (`string`): Tên hiển thị đầy đủ cho user - **Lưu trong AgentRegistry.JobMetadata**
- **description** (`string`): Mô tả chi tiết về job - **Lưu trong AgentRegistry.JobMetadata**
- **category** (`string`): Danh mục job - **Lưu trong AgentRegistry.JobMetadata**
- **tags** (`[]string`): Tags để phân loại - **Lưu trong AgentRegistry.JobMetadata**
- **icon** (`string`): Icon/emoji cho job - **Lưu trong AgentRegistry.JobMetadata**
- **color** (`string`): Màu sắc cho job - **Lưu trong AgentRegistry.JobMetadata**

**Lưu ý**: 
- Khi bot submit config hoặc admin update config, server sẽ tự động loại bỏ metadata chung của job (displayName, description, icon, color, category, tags) khỏi config
- Metadata của các field trong config (ví dụ: params có thể có metadata riêng) vẫn được giữ trong config như trước

#### Trường Tùy Chọn - Cấu Hình
- **timeout** (`number`): Timeout cho job (giây)
- **retries** (`number`): Số lần retry khi job fail
- **params** (`object`): Tham số bổ sung cho job
  - Có thể có metadata riêng (ví dụ: description cho từng param) - metadata này được giữ trong config

### Ví Dụ Các Job Khác

**Job Definition trong Config (không có metadata chung, nhưng có thể có metadata của params):**
```json
{
  "name": "data_sync",
  "enabled": true,
  "schedule": "0 0 */6 * * *",
  "timeout": 600,
  "retries": 2,
  "params": {
    "apiEndpoint": "https://api.pancake.vn",
    "syncInterval": 3600,
    "fields": {
      "orders": {
        "enabled": true,
        "description": "Đồng bộ thông tin đơn hàng"
      },
      "customers": {
        "enabled": true,
        "description": "Đồng bộ thông tin khách hàng"
      }
    }
    // Metadata của params (như description cho từng field) vẫn được giữ trong config
  }
}
```

**Metadata trong AgentRegistry:**
```json
{
  "jobMetadata": {
    "data_sync": {
      "displayName": "Đồng Bộ Dữ Liệu Pancake",
      "description": "Job đồng bộ dữ liệu từ Pancake API định kỳ, cập nhật thông tin đơn hàng và khách hàng",
      "category": "data-sync",
      "tags": ["pancake", "sync", "order", "customer"],
      "icon": "🔄",
      "color": "#8B5CF6"
    },
    "cleanup_old_logs": {
      "displayName": "Dọn Dẹp Log Cũ",
      "description": "Job xóa các log cũ hơn 30 ngày để giải phóng dung lượng database",
      "category": "cleanup",
      "tags": ["cleanup", "logs", "maintenance"],
      "icon": "🧹",
      "color": "#6B7280"
    }
  }
}
```

## 💡 Đề Xuất Các Thông Tin Khác

### 1. Agent Status Display

#### Thông Tin Hiển Thị Trạng Thái
- **Status Badge**: Hiển thị trạng thái với màu sắc tương ứng
  - Online: Green (#10B981)
  - Offline: Gray (#6B7280)
  - Error: Red (#EF4444)
  - Maintenance: Yellow (#F59E0B)

- **Health Indicator**: Hiển thị health status với icon
  - Healthy: ✅
  - Degraded: ⚠️
  - Unhealthy: ❌

- **Last Check-in Time**: Hiển thị thời gian check-in cuối cùng
  - Format: "2 phút trước", "1 giờ trước", "Hôm qua"
  - Color: Green nếu < 5 phút, Yellow nếu < 15 phút, Red nếu > 15 phút

### 2. Job Status Display

#### Thông Tin Hiển Thị Job Status
- **Job Status Badge**: Hiển thị trạng thái job
  - Running: Blue (#3B82F6) với icon ⏳
  - Success: Green (#10B981) với icon ✅
  - Failed: Red (#EF4444) với icon ❌
  - Paused: Gray (#6B7280) với icon ⏸️
  - Disabled: Gray (#9CA3AF) với icon 🚫

- **Last Run Time**: Thời gian chạy cuối cùng
  - Format: "2 phút trước", "1 giờ trước"
  - Hiển thị kèm kết quả (success/failed)

- **Next Run Time**: Thời gian chạy tiếp theo
  - Format: "Trong 3 phút", "Lúc 14:30 hôm nay"
  - Dựa trên schedule và last run time

- **Run Statistics**: Thống kê chạy job
  - Total runs: Số lần đã chạy
  - Success rate: Tỷ lệ thành công (%)
  - Average duration: Thời gian chạy trung bình
  - Last 24h runs: Số lần chạy trong 24h qua

### 3. Agent Metrics Display

#### Thông Tin Metrics Hiển Thị
- **System Resources**: Hiển thị CPU, Memory, Disk usage
  - Progress bar với màu sắc (Green/Yellow/Red)
  - Tooltip với giá trị chi tiết

- **Uptime**: Thời gian agent đã chạy
  - Format: "2 ngày 5 giờ", "1 tuần 3 ngày"

- **Performance Metrics**: 
  - Response time: Thời gian phản hồi trung bình
  - Throughput: Số requests/jobs xử lý mỗi giờ
  - Error rate: Tỷ lệ lỗi (%)

### 4. Quick Actions

#### Các Hành Động Nhanh
- **Start/Stop Agent**: Bật/tắt agent
- **Restart Agent**: Khởi động lại agent
- **View Logs**: Xem logs của agent
- **Edit Config**: Chỉnh sửa config
- **Run Job Now**: Chạy job ngay lập tức
- **Pause/Resume Job**: Tạm dừng/tiếp tục job

### 5. Filtering & Search

#### Tính Năng Tìm Kiếm và Lọc
- **Search by Name**: Tìm kiếm theo tên agent/job
- **Filter by Category**: Lọc theo danh mục
- **Filter by Tags**: Lọc theo tags
- **Filter by Status**: Lọc theo trạng thái
- **Filter by Health**: Lọc theo health status
- **Sort Options**: Sắp xếp theo tên, status, last check-in, etc.

### 6. Notifications & Alerts

#### Thông Báo và Cảnh Báo
- **Agent Offline Alert**: Cảnh báo khi agent offline > 5 phút
- **Job Failure Alert**: Cảnh báo khi job fail
- **High Resource Usage Alert**: Cảnh báo khi CPU/Memory > 80%
- **Config Change Notification**: Thông báo khi config thay đổi

### 7. Dashboard Overview

#### Tổng Quan Dashboard
- **Total Agents**: Tổng số agents
- **Online Agents**: Số agents đang online
- **Total Jobs**: Tổng số jobs
- **Running Jobs**: Số jobs đang chạy
- **Failed Jobs (24h)**: Số jobs fail trong 24h
- **System Health**: Tổng quan health của toàn bộ hệ thống

## 📊 Ví Dụ Response API Với Thông Tin Thân Thiện

### Agent Registry Response

```json
{
  "code": 200,
  "message": "Thành công",
  "data": {
    "id": "65a1b2c3d4e5f6a7b8c9d0e1",
    "agentId": "monitoring-bot-001",
    "name": "Monitoring Bot",
    "displayName": "Production Monitoring Bot - Server 01",
    "description": "Bot giám sát trạng thái hệ thống, kiểm tra health check, và gửi cảnh báo khi có sự cố",
    "icon": "📊",
    "color": "#3B82F6",
    "category": "monitoring",
    "tags": ["production", "critical", "monitoring", "server-01"],
    "status": "online",
    "healthStatus": "healthy",
    "lastCheckInAt": 1704700800,
    "lastCheckInAgo": "2 phút trước",
    "systemInfo": {
      "os": "linux",
      "arch": "amd64",
      "goVersion": "go1.21.0",
      "uptime": 172800,
      "uptimeDisplay": "2 ngày",
      "cpu": 25.5,
      "memory": 45.2,
      "disk": 60.1
    },
    "jobStatus": [
      {
        "name": "conversation_monitor",
        "displayName": "Giám Sát Conversation",
        "status": "running",
        "lastRunAt": 1704700500,
        "lastRunAgo": "5 phút trước",
        "nextRunAt": 1704700800,
        "nextRunIn": "Trong 3 phút",
        "successRate": 98.5,
        "icon": "💬",
        "color": "#10B981"
      }
    ]
  },
  "status": "success"
}
```

## 🔄 Migration & Backward Compatibility

### Migration
- Các trường mới đều là optional, không ảnh hưởng đến dữ liệu cũ
- Có thể thêm các trường mới dần dần cho từng agent/job
- Frontend nên handle trường hợp thiếu các trường mới (fallback về giá trị mặc định)

### Backward Compatibility
- Nếu không có `displayName`, dùng `name`
- Nếu không có `icon`, dùng icon mặc định theo category
- Nếu không có `color`, dùng color mặc định theo status
- Nếu không có `description`, hiển thị "Không có mô tả"

## 📝 Best Practices

### 1. Naming
- **Name**: Ngắn gọn, dễ nhớ (ví dụ: "Monitoring Bot")
- **DisplayName**: Đầy đủ, mô tả rõ (ví dụ: "Production Monitoring Bot - Server 01")
- **Description**: Chi tiết, giải thích rõ chức năng và mục đích

### 2. Icons & Colors
- Chọn icon phù hợp với chức năng
- Dùng màu sắc nhất quán (ví dụ: monitoring = blue, critical = red)
- Tránh dùng quá nhiều màu sắc khác nhau

### 3. Categories & Tags
- Categories: Dùng để nhóm agent/job theo chức năng
- Tags: Dùng để filter và search, có thể có nhiều tags
- Nên có danh sách categories và tags chuẩn

### 4. Descriptions
- Viết bằng Tiếng Việt, rõ ràng, dễ hiểu
- Mô tả chức năng, mục đích, và các trường hợp sử dụng
- Tránh quá dài, nên ngắn gọn nhưng đầy đủ thông tin

## 🚀 Next Steps

1. **Frontend Implementation**: 
   - Cập nhật UI để hiển thị các thông tin mới
   - Thêm filter và search
   - Thêm dashboard overview

2. **Admin Tools**:
   - Form để edit agent metadata (name, description, icon, color, etc.)
   - Form để edit job metadata trong config
   - Bulk edit tools

3. **Documentation**:
   - Hướng dẫn cách thêm metadata cho agent/job
   - Best practices cho naming, icons, colors

4. **Validation**:
   - Validate color format (hex color)
   - Validate icon (emoji hoặc icon class)
   - Validate category và tags (có thể dùng enum hoặc whitelist)
