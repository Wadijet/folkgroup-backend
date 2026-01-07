package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AgentCommand lưu commands từ server để điều khiển bot
// Collection: agent_commands
type AgentCommand struct {
	ID          primitive.ObjectID   `json:"id,omitempty" bson:"_id,omitempty"`
	AgentID     primitive.ObjectID   `json:"agentId" bson:"agentId" index:"single:1"` // Reference to agent_registry
	Type        string               `json:"type" bson:"type" index:"single:1"`        // "stop", "start", "restart", "reload_config", "shutdown", "run_job", "pause_job", "resume_job", "disable_job", "enable_job", "update_job_schedule"
	Target      string               `json:"target" bson:"target"`                      // "bot" hoặc job name
	Params      map[string]interface{} `json:"params,omitempty" bson:"params,omitempty"`
	Status      string               `json:"status" bson:"status" index:"single:1"`     // "pending", "executing", "completed", "failed", "cancelled"
	Result      map[string]interface{} `json:"result,omitempty" bson:"result,omitempty"`
	Error       string               `json:"error,omitempty" bson:"error,omitempty"`
	CreatedBy   *primitive.ObjectID   `json:"createdBy,omitempty" bson:"createdBy,omitempty"` // User ID nếu admin tạo
	CreatedAt   int64                `json:"createdAt" bson:"createdAt" index:"single:1"`
	ExecutedAt  int64                `json:"executedAt,omitempty" bson:"executedAt,omitempty"`
	CompletedAt int64                `json:"completedAt,omitempty" bson:"completedAt,omitempty"`
}
