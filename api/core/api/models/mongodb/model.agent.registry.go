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
	AgentID     string             `json:"agentId" bson:"agentId" index:"unique"` // ID của agent (từ ENV AGENT_ID)
	Name        string             `json:"name,omitempty" bson:"name,omitempty"`  // Tên agent (optional)
	Description string             `json:"description,omitempty" bson:"description,omitempty"`
	BotVersion  string             `json:"botVersion,omitempty" bson:"botVersion,omitempty"` // Version của bot code

	// Status summary (thay đổi thường xuyên nhưng nhẹ)
	Status        string `json:"status" bson:"status" index:"single:1"`            // "online", "offline", "error", "maintenance"
	HealthStatus  string `json:"healthStatus" bson:"healthStatus" index:"single:1"` // "healthy", "degraded", "unhealthy"
	LastCheckInAt int64  `json:"lastCheckInAt" bson:"lastCheckInAt" index:"single:1"`
	FirstSeenAt   int64  `json:"firstSeenAt" bson:"firstSeenAt"`
	LastSeenAt    int64  `json:"lastSeenAt" bson:"lastSeenAt"`

	// Status details (thông tin chi tiết realtime từ agent_status)
	SystemInfo    map[string]interface{}   `json:"systemInfo,omitempty" bson:"systemInfo,omitempty"` // OS, Arch, GoVersion, Uptime, CPU, Memory, Disk
	Metrics       map[string]interface{}   `json:"metrics,omitempty" bson:"metrics,omitempty"`       // Bot-level metrics
	JobStatus     []map[string]interface{} `json:"jobStatus,omitempty" bson:"jobStatus,omitempty"`   // Job statuses
	ConfigVersion int64                    `json:"configVersion" bson:"configVersion"`               // Version của config đang dùng (Unix timestamp)
	ConfigHash    string                   `json:"configHash" bson:"configHash"`                      // Hash của config đang dùng

	// Timestamps
	CreatedAt int64 `json:"createdAt" bson:"createdAt"`
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"`
}
