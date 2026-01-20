# Thiết Kế Metadata Cho Job - Tách Ra AgentRegistry

## 📋 Vấn Đề Hiện Tại

Hiện tại metadata của job (displayName, description, icon, color, category, tags) được lưu trong `AgentConfig.ConfigData.jobs` cùng với job definition.

**Vấn đề:**
1. ❌ **Duplicate qua config versions**: Mỗi config version đều có metadata → duplicate dữ liệu
2. ❌ **Metadata bị rollback**: Khi rollback config, metadata cũng bị rollback
3. ❌ **Khó quản lý**: Không thể update metadata mà không tạo config version mới
4. ❌ **Không persistent**: Metadata mất khi config thay đổi

## ✅ Giải Pháp: Tách Metadata Ra AgentRegistry

### Cấu Trúc Mới

#### AgentRegistry - Thêm JobMetadata
```typescript
interface AgentRegistry {
  // ... existing fields ...
  
  // Job Metadata (🤖 MỚI)
  jobMetadata?: Record<string, JobMetadata>; // Key = job name, Value = metadata
}

interface JobMetadata {
  displayName?: string;    // Tên hiển thị đầy đủ
  description?: string;     // Mô tả chi tiết
  icon?: string;           // Icon/emoji
  color?: string;          // Màu sắc (hex)
  category?: string;       // Danh mục
  tags?: string[];         // Tags
}
```

#### AgentConfig.ConfigData - Chỉ Giữ Job Definition
```typescript
{
  "configData": {
    "jobs": [
      {
        "name": "conversation_monitor",  // Required - để map với metadata
        "enabled": true,
        "schedule": "0 */5 * * * *",
        "timeout": 300,
        "retries": 3,
        "params": { ... }
        // KHÔNG có metadata ở đây nữa
      }
    ]
  }
}
```

### Lợi Ích

1. ✅ **Không duplicate**: Metadata chỉ lưu 1 lần trong AgentRegistry
2. ✅ **Persistent**: Metadata không bị ảnh hưởng khi config thay đổi
3. ✅ **Dễ quản lý**: Admin có thể update metadata độc lập
4. ✅ **Không rollback**: Rollback config không ảnh hưởng metadata
5. ✅ **Query nhanh**: Có thể query metadata mà không cần load config

### Cách Hoạt Động

#### 1. Khi Bot Submit Config
- Bot gửi job definition (không có metadata)
- Server validate và lưu vào config
- Server tự động tạo metadata mặc định nếu job mới (dựa vào job name)

#### 2. Khi Admin Update Metadata
- Admin update `AgentRegistry.jobMetadata[jobName]`
- Không cần tạo config version mới
- Metadata được update ngay lập tức

#### 3. Khi Frontend Hiển Thị
- Load config để lấy job definitions
- Load AgentRegistry để lấy job metadata
- Merge metadata vào job definitions khi hiển thị

#### 4. Khi Job Bị Xóa Khỏi Config
- Metadata vẫn còn trong AgentRegistry (orphaned metadata)
- Có thể cleanup metadata của jobs không còn tồn tại (optional)

### Migration Strategy

#### Option 1: Hybrid Approach (Khuyến Nghị)
- Hỗ trợ cả 2 cách: metadata trong config (backward compatible) và metadata trong registry
- Ưu tiên metadata trong registry nếu có
- Fallback về metadata trong config nếu không có trong registry

#### Option 2: Full Migration
- Migrate tất cả metadata từ config sang registry
- Xóa metadata khỏi config
- Chỉ dùng metadata trong registry

### Implementation

#### 1. Update AgentRegistry Model
```go
type AgentRegistry struct {
  // ... existing fields ...
  
  // Job Metadata (🤖 MỚI)
  JobMetadata map[string]JobMetadata `json:"jobMetadata,omitempty" bson:"jobMetadata,omitempty"`
}

type JobMetadata struct {
  DisplayName string   `json:"displayName,omitempty" bson:"displayName,omitempty"`
  Description string   `json:"description,omitempty" bson:"description,omitempty"`
  Icon        string   `json:"icon,omitempty" bson:"icon,omitempty"`
  Color       string   `json:"color,omitempty" bson:"color,omitempty"`
  Category    string   `json:"category,omitempty" bson:"category,omitempty"`
  Tags        []string `json:"tags,omitempty" bson:"tags,omitempty"`
}
```

#### 2. Helper Functions
```go
// EnrichJobsWithMetadata merge metadata từ registry vào jobs
func EnrichJobsWithMetadata(jobs []interface{}, jobMetadata map[string]JobMetadata) []interface{} {
  // Merge logic
}

// SyncJobMetadata tự động tạo metadata mặc định cho jobs mới
func SyncJobMetadata(agentID string, jobs []interface{}) error {
  // Sync logic
}
```

#### 3. API Endpoints
- `PUT /api/v1/agent-management/registry/:agentId/job-metadata/:jobName` - Update metadata cho 1 job
- `GET /api/v1/agent-management/registry/:agentId/job-metadata` - Lấy tất cả job metadata
- `DELETE /api/v1/agent-management/registry/:agentId/job-metadata/:jobName` - Xóa metadata

## 🔄 So Sánh

| Tiêu Chí | Metadata Trong Config | Metadata Trong Registry |
|----------|----------------------|------------------------|
| Duplicate | ❌ Có (qua versions) | ✅ Không |
| Persistent | ❌ Không | ✅ Có |
| Update độc lập | ❌ Không | ✅ Có |
| Rollback ảnh hưởng | ❌ Có | ✅ Không |
| Query nhanh | ❌ Phải load config | ✅ Query trực tiếp |
| Phức tạp | ✅ Đơn giản | ⚠️ Phức tạp hơn |

## 💡 Khuyến Nghị

**Nên tách metadata ra AgentRegistry** vì:
1. Metadata là thông tin UI, không phải config logic
2. Metadata thay đổi ít hơn config
3. Metadata cần persistent qua các config version
4. Dễ quản lý và query hơn

**Implementation Strategy:**
1. Bắt đầu với Hybrid Approach (backward compatible)
2. Migrate dần metadata từ config sang registry
3. Sau đó có thể chuyển sang Full Migration

## 📝 Next Steps

1. ✅ Update AgentRegistry model với JobMetadata
2. ✅ Tạo helper functions để merge metadata
3. ✅ Update service để sync metadata khi submit config
4. ✅ Tạo API endpoints để quản lý metadata
5. ✅ Update frontend để merge metadata khi hiển thị
6. ✅ Migration script để migrate metadata từ config sang registry
