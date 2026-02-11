package logger

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"
)

// AuditAction log một hành động audit
type AuditAction struct {
	Action      string                 `json:"action"`       // Tên hành động (ví dụ: "user_create", "user_delete")
	UserID      string                 `json:"user_id"`      // ID người dùng thực hiện
	ResourceID  string                 `json:"resource_id"`  // ID tài nguyên bị ảnh hưởng
	ResourceType string                `json:"resource_type"` // Loại tài nguyên (ví dụ: "user", "organization")
	IP          string                 `json:"ip"`           // IP address
	UserAgent   string                 `json:"user_agent"`  // User agent
	Details     map[string]interface{} `json:"details"`     // Chi tiết bổ sung
	Timestamp   time.Time              `json:"timestamp"`   // Thời gian
}

// LogAction log một hành động audit
func LogAction(action string, c fiber.Ctx, details map[string]interface{}) {
	auditLogger := GetAuditLogger()

	audit := AuditAction{
		Action:      action,
		IP:          c.IP(),
		UserAgent:   c.Get("User-Agent"),
		Details:     details,
		Timestamp:   time.Now(),
	}

	// Lấy user ID từ context nếu có
	if userID := c.Locals("userID"); userID != nil {
		if uid, ok := userID.(string); ok {
			audit.UserID = uid
		}
	}

	// Lấy organization ID từ context nếu có
	if orgID := c.Locals("organizationID"); orgID != nil {
		if oid, ok := orgID.(string); ok {
			audit.Details["organization_id"] = oid
		}
	}

	// Lấy request ID
	if requestID := c.Get("X-Request-ID"); requestID != "" {
		audit.Details["request_id"] = requestID
	}

	auditLogger.WithFields(logrus.Fields{
		"action":       audit.Action,
		"user_id":      audit.UserID,
		"resource_id":  audit.ResourceID,
		"resource_type": audit.ResourceType,
		"ip":           audit.IP,
		"user_agent":   audit.UserAgent,
		"details":      audit.Details,
		"timestamp":    audit.Timestamp,
	}).Info("Audit log")
}

// LogCRUD log các thao tác CRUD
func LogCRUD(operation string, resourceType string, resourceID string, c fiber.Ctx, details map[string]interface{}) {
	if details == nil {
		details = make(map[string]interface{})
	}
	details["operation"] = operation
	details["resource_type"] = resourceType
	details["resource_id"] = resourceID

	LogAction("crud_"+operation, c, details)
}

// LogAuth log các thao tác authentication
func LogAuth(action string, c fiber.Ctx, details map[string]interface{}) {
	if details == nil {
		details = make(map[string]interface{})
	}
	details["auth_action"] = action

	LogAction("auth_"+action, c, details)
}

// LogPermission log các thay đổi permission
func LogPermission(action string, c fiber.Ctx, details map[string]interface{}) {
	if details == nil {
		details = make(map[string]interface{})
	}
	details["permission_action"] = action

	LogAction("permission_"+action, c, details)
}
