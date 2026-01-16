package logger

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"
)

// ContextKey là type cho context keys
type ContextKey string

const (
	// RequestIDKey là key cho request ID trong context
	RequestIDKey ContextKey = "requestID"
	// UserIDKey là key cho user ID trong context
	UserIDKey ContextKey = "userID"
	// OrganizationIDKey là key cho organization ID trong context
	OrganizationIDKey ContextKey = "organizationID"
	// ServiceKey là key cho service name trong context
	ServiceKey ContextKey = "service"
)

// WithContext trả về logger entry với context
func WithContext(ctx context.Context) *logrus.Entry {
	logger := GetAppLogger()
	entry := logger.WithContext(ctx)

	// Thêm các fields từ context nếu có
	if requestID := ctx.Value(RequestIDKey); requestID != nil {
		entry = entry.WithField("request_id", requestID)
	}
	if userID := ctx.Value(UserIDKey); userID != nil {
		entry = entry.WithField("user_id", userID)
	}
	if orgID := ctx.Value(OrganizationIDKey); orgID != nil {
		entry = entry.WithField("organization_id", orgID)
	}
	if service := ctx.Value(ServiceKey); service != nil {
		entry = entry.WithField("service", service)
	}

	return entry
}

// WithRequest trả về logger entry với request context từ Fiber
func WithRequest(c fiber.Ctx) *logrus.Entry {
	logger := GetAppLogger()
	entry := logger.WithContext(context.Background())

	// Thêm request ID - Fiber request ID middleware có thể set vào Locals hoặc header
	var requestID string
	
	// Thử lấy từ Locals trước (Fiber request ID middleware thường set vào đây)
	if rid := c.Locals("requestid"); rid != nil {
		if ridStr, ok := rid.(string); ok {
			requestID = ridStr
		}
	}
	
	// Nếu không có trong Locals, thử lấy từ header
	if requestID == "" {
		requestID = c.Get("X-Request-ID")
	}
	
	// Nếu vẫn không có, thử lấy từ response header (middleware có thể đã set)
	if requestID == "" {
		requestID = c.GetRespHeader("X-Request-ID")
	}
	
	// Thêm request ID vào log nếu có
	if requestID != "" {
		entry = entry.WithField("request_id", requestID)
	}

	// Thêm các thông tin request khác
	entry = entry.WithFields(logrus.Fields{
		"method": c.Method(),
		"path":   c.Path(),
		"ip":     c.IP(),
	})

	return entry
}

// WithFields trả về logger entry với các fields bổ sung
func WithFields(fields map[string]interface{}) *logrus.Entry {
	return GetAppLogger().WithFields(logrus.Fields(fields))
}

// WithError trả về logger entry với error
func WithError(err error) *logrus.Entry {
	return GetAppLogger().WithError(err)
}

// WithModule trả về logger entry với module name
// Module: tên module (ví dụ: "auth", "notification", "delivery", "content", "ai")
func WithModule(module string) *logrus.Entry {
	return GetAppLogger().WithField("module", module)
}

// WithCollection trả về logger entry với collection name
// Collection: tên collection MongoDB (ví dụ: "users", "orders", "notifications")
func WithCollection(collection string) *logrus.Entry {
	return GetAppLogger().WithField("collection", collection)
}

// WithEndpoint trả về logger entry với endpoint path
// Endpoint: đường dẫn endpoint (ví dụ: "/api/v1/users", "/api/v1/orders")
func WithEndpoint(endpoint string) *logrus.Entry {
	return GetAppLogger().WithField("endpoint", endpoint)
}

// WithMethod trả về logger entry với HTTP method
// Method: HTTP method (ví dụ: "GET", "POST", "PUT", "DELETE")
func WithMethod(method string) *logrus.Entry {
	return GetAppLogger().WithField("method", method)
}

// WithModuleAndCollection trả về logger entry với module và collection
func WithModuleAndCollection(module, collection string) *logrus.Entry {
	return GetAppLogger().WithFields(logrus.Fields{
		"module":     module,
		"collection": collection,
	})
}

// WithRequestInfo trả về logger entry với đầy đủ thông tin request
// Bao gồm: method, path (endpoint), IP, request_id
// Có thể thêm module và collection nếu cần
func WithRequestInfo(c fiber.Ctx, module, collection string) *logrus.Entry {
	entry := WithRequest(c)
	
	// Thêm module và collection nếu có
	if module != "" {
		entry = entry.WithField("module", module)
	}
	if collection != "" {
		entry = entry.WithField("collection", collection)
	}
	
	return entry
}
