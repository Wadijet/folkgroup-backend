package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AgentRegistry lưu thông tin cơ bản và trạng thái realtime của agent (bot)
// Collection: agent_registry
// Lưu ý: Đã ghép với agent_status để tránh trùng lặp dữ liệu và đơn giản hóa code
type AgentRegistry struct {
	// Thông tin cơ bản (ít thay đổi)
	ID          primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	AgentID     string             `json:"agentId" bson:"agentId" index:"unique"`              // ID của agent (từ ENV AGENT_ID)
	Name        string             `json:"name,omitempty" bson:"name,omitempty"`               // Tên agent (hiển thị cho user)
	DisplayName string             `json:"displayName,omitempty" bson:"displayName,omitempty"` // Tên hiển thị đầy đủ (nếu khác với Name)
	Description string             `json:"description,omitempty" bson:"description,omitempty"` // Mô tả chi tiết về agent
	BotVersion  string             `json:"botVersion,omitempty" bson:"botVersion,omitempty"`   // Version của bot code

	// Thông tin hiển thị (UI-friendly)
	Icon     string   `json:"icon,omitempty" bson:"icon,omitempty"`         // Icon/emoji cho agent (ví dụ: "🤖", "📊", "🔔")
	Color    string   `json:"color,omitempty" bson:"color,omitempty"`       // Màu sắc cho agent (hex color, ví dụ: "#3B82F6")
	Category string   `json:"category,omitempty" bson:"category,omitempty"` // Danh mục agent (ví dụ: "monitoring", "data-sync", "notification")
	Tags     []string `json:"tags,omitempty" bson:"tags,omitempty"`         // Tags để phân loại và tìm kiếm (ví dụ: ["production", "critical", "monitoring"])

	// Status summary (thay đổi thường xuyên nhưng nhẹ)
	Status        string `json:"status" bson:"status" index:"single:1"`             // "online", "offline", "error", "maintenance"
	HealthStatus  string `json:"healthStatus" bson:"healthStatus" index:"single:1"` // "healthy", "degraded", "unhealthy"
	LastCheckInAt int64  `json:"lastCheckInAt" bson:"lastCheckInAt" index:"single:1"`
	FirstSeenAt   int64  `json:"firstSeenAt" bson:"firstSeenAt"`
	LastSeenAt    int64  `json:"lastSeenAt" bson:"lastSeenAt"`

	// Status details (thông tin chi tiết realtime từ agent_status)
	SystemInfo    map[string]interface{}   `json:"systemInfo,omitempty" bson:"systemInfo,omitempty"` // OS, Arch, GoVersion, Uptime, CPU, Memory, Disk
	Metrics       map[string]interface{}   `json:"metrics,omitempty" bson:"metrics,omitempty"`       // Bot-level metrics
	JobStatus     []map[string]interface{} `json:"jobStatus,omitempty" bson:"jobStatus,omitempty"`   // Job statuses (agent tự gửi lên, có thể kèm metadata: displayName, description, icon, color, category, tags)
	ConfigVersion int64                    `json:"configVersion" bson:"configVersion"`               // Version của config đang dùng (Unix timestamp)
	ConfigHash    string                   `json:"configHash" bson:"configHash"`                     // Hash của config đang dùng

	// Timestamps
	CreatedAt int64 `json:"createdAt" bson:"createdAt"`
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"`
}
