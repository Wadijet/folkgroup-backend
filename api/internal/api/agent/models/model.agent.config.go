package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AgentConfig lưu các version config của bot
// Collection: agent_configs
type AgentConfig struct {
	ID             primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`
	AgentID        string                 `json:"agentId" bson:"agentId" index:"single:1"` // Reference to agent_registry.agentId (string)
	Version        int64                  `json:"version" bson:"version" index:"single:1"` // Unix timestamp (server tự động quyết định)
	ConfigHash     string                 `json:"configHash" bson:"configHash" index:"single:1"`
	ConfigData     map[string]interface{} `json:"configData" bson:"configData"`                   // Config data với metadata inline
	IsActive       bool                   `json:"isActive" bson:"isActive" index:"single:1"`      // Chỉ có 1 active config cho mỗi agent
	SubmittedByBot bool                   `json:"submittedByBot" bson:"submittedByBot"`           // true nếu bot submit, false nếu admin tạo
	ChangedBy      *primitive.ObjectID    `json:"changedBy,omitempty" bson:"changedBy,omitempty"` // User ID nếu admin thay đổi
	ChangedAt      int64                  `json:"changedAt,omitempty" bson:"changedAt,omitempty"`
	ChangeLog      string                 `json:"changeLog,omitempty" bson:"changeLog,omitempty"`
	Description    string                 `json:"description,omitempty" bson:"description,omitempty"`
	AppliedByBot   bool                   `json:"appliedByBot" bson:"appliedByBot"` // Bot đã apply config này chưa
	AppliedAt      int64                  `json:"appliedAt,omitempty" bson:"appliedAt,omitempty"`
	AppliedStatus  string                 `json:"appliedStatus,omitempty" bson:"appliedStatus,omitempty"` // "pending", "applied", "failed"
	AppliedError   string                 `json:"appliedError,omitempty" bson:"appliedError,omitempty"`
	CreatedAt      int64                  `json:"createdAt" bson:"createdAt"`
	UpdatedAt      int64                  `json:"updatedAt" bson:"updatedAt"`
}
