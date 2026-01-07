package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AgentActivityLog lưu log các hoạt động của bot
// Collection: agent_activity_logs
type AgentActivityLog struct {
	ID           primitive.ObjectID   `json:"id,omitempty" bson:"_id,omitempty"`
	AgentID      primitive.ObjectID   `json:"agentId" bson:"agentId" index:"single:1"`
	ActivityType string               `json:"activityType" bson:"activityType" index:"single:1"` // "check_in", "command_executed", "config_applied", "job_run", "error"
	Timestamp    int64                `json:"timestamp" bson:"timestamp" index:"single:1"`
	Data         map[string]interface{} `json:"data,omitempty" bson:"data,omitempty"`
	Message      string                `json:"message,omitempty" bson:"message,omitempty"`
	Severity     string                `json:"severity,omitempty" bson:"severity,omitempty"` // "info", "warning", "error"
}
