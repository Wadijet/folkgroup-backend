package logger

import (
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// FilterHook là một hook để lọc log entries dựa trên các tiêu chí:
// - Module (ví dụ: auth, notification, delivery)
// - Collection (ví dụ: users, orders)
// - Endpoint (ví dụ: /api/v1/users)
// - Method (GET, POST, PUT, DELETE)
// - Log Type (trace, debug, info, warn, error, fatal)
type FilterHook struct {
	// Các filter sets (map[string]bool để lookup nhanh)
	// Nếu map rỗng hoặc "*" trong config, cho phép tất cả
	allowedModules    map[string]bool
	allowedCollections map[string]bool
	allowedEndpoints   map[string]bool
	allowedMethods     map[string]bool
	allowedLogTypes    map[string]bool

	// Flags để kiểm tra xem có filter nào được bật không
	hasModuleFilter    bool
	hasCollectionFilter bool
	hasEndpointFilter   bool
	hasMethodFilter     bool
	hasLogTypeFilter    bool

	mu sync.RWMutex
}

// NewFilterHook tạo một filter hook mới với cấu hình
func NewFilterHook(cfg *LogConfig) *FilterHook {
	hook := &FilterHook{
		allowedModules:     make(map[string]bool),
		allowedCollections: make(map[string]bool),
		allowedEndpoints:   make(map[string]bool),
		allowedMethods:     make(map[string]bool),
		allowedLogTypes:    make(map[string]bool),
	}

	// Parse và set filters từ config
	hook.updateFilters(cfg)

	return hook
}

// updateFilters cập nhật filters từ config
func (h *FilterHook) updateFilters(cfg *LogConfig) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Parse modules filter
	h.allowedModules = parseFilter(cfg.FilterModules)
	h.hasModuleFilter = len(h.allowedModules) > 0 && !h.allowedModules["*"]

	// Parse collections filter
	h.allowedCollections = parseFilter(cfg.FilterCollections)
	h.hasCollectionFilter = len(h.allowedCollections) > 0 && !h.allowedCollections["*"]

	// Parse endpoints filter
	h.allowedEndpoints = parseFilter(cfg.FilterEndpoints)
	h.hasEndpointFilter = len(h.allowedEndpoints) > 0 && !h.allowedEndpoints["*"]

	// Parse methods filter
	h.allowedMethods = parseFilter(cfg.FilterMethods)
	h.hasMethodFilter = len(h.allowedMethods) > 0 && !h.allowedMethods["*"]

	// Parse log types filter
	h.allowedLogTypes = parseFilter(cfg.FilterLogTypes)
	h.hasLogTypeFilter = len(h.allowedLogTypes) > 0 && !h.allowedLogTypes["*"]
}

// parseFilter parse filter string thành map
// Format: "value1,value2,value3" hoặc "*" cho tất cả
// Trả về map với key là giá trị filter, value là true
func parseFilter(filterStr string) map[string]bool {
	result := make(map[string]bool)

	// Nếu rỗng hoặc "*", cho phép tất cả
	if filterStr == "" || filterStr == "*" {
		result["*"] = true
		return result
	}

	// Parse comma-separated values
	values := strings.Split(filterStr, ",")
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			// Chuyển thành lowercase để so sánh không phân biệt hoa thường
			result[strings.ToLower(v)] = true
		}
	}

	return result
}

// Levels trả về các log levels mà hook này xử lý
func (h *FilterHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire được gọi mỗi khi có log entry mới
// Đánh dấu entry bị filter bằng cách set field "_filtered" = true
// AsyncHook sẽ kiểm tra field này và bỏ qua entry nếu bị filter
func (h *FilterHook) Fire(entry *logrus.Entry) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Kiểm tra log type filter
	if h.hasLogTypeFilter {
		levelStr := strings.ToLower(entry.Level.String())
		if !h.allowedLogTypes[levelStr] {
			// Entry bị filter, đánh dấu để bỏ qua
			entry.Data["_filtered"] = true
			return nil
		}
	}

	// Kiểm tra module filter
	if h.hasModuleFilter {
		module, ok := entry.Data["module"].(string)
		if !ok || module == "" {
			// Nếu không có module field, bỏ qua filter này (cho phép log)
			// Hoặc có thể filter nếu muốn bắt buộc phải có module
		} else {
			moduleLower := strings.ToLower(module)
			if !h.allowedModules[moduleLower] {
				// Entry bị filter, đánh dấu để bỏ qua
				entry.Data["_filtered"] = true
				return nil
			}
		}
	}

	// Kiểm tra collection filter
	if h.hasCollectionFilter {
		collection, ok := entry.Data["collection"].(string)
		if !ok || collection == "" {
			// Nếu không có collection field, bỏ qua filter này
		} else {
			collectionLower := strings.ToLower(collection)
			if !h.allowedCollections[collectionLower] {
				// Entry bị filter, đánh dấu để bỏ qua
				entry.Data["_filtered"] = true
				return nil
			}
		}
	}

	// Kiểm tra endpoint filter
	if h.hasEndpointFilter {
		endpoint, ok := entry.Data["endpoint"].(string)
		if !ok || endpoint == "" {
			// Nếu không có endpoint field, thử lấy từ "path"
			endpoint, ok = entry.Data["path"].(string)
		}
		if ok && endpoint != "" {
			endpointLower := strings.ToLower(endpoint)
			// Kiểm tra exact match hoặc prefix match
			matched := false
			for allowedEndpoint := range h.allowedEndpoints {
				if allowedEndpoint == "*" {
					matched = true
					break
				}
				// Exact match
				if endpointLower == allowedEndpoint {
					matched = true
					break
				}
				// Prefix match (ví dụ: /api/v1/users khớp với /api/v1/users)
				if strings.HasPrefix(endpointLower, allowedEndpoint) {
					matched = true
					break
				}
			}
			if !matched {
				// Entry bị filter, đánh dấu để bỏ qua
				entry.Data["_filtered"] = true
				return nil
			}
		}
	}

	// Kiểm tra method filter
	if h.hasMethodFilter {
		method, ok := entry.Data["method"].(string)
		if !ok || method == "" {
			// Nếu không có method field, bỏ qua filter này
		} else {
			methodUpper := strings.ToUpper(method)
			if !h.allowedMethods[strings.ToLower(methodUpper)] {
				// Entry bị filter, đánh dấu để bỏ qua
				entry.Data["_filtered"] = true
				return nil
			}
		}
	}

	// Tất cả filters đều pass, cho phép log
	return nil
}

// UpdateFilters cập nhật filters từ config mới (có thể gọi runtime)
func (h *FilterHook) UpdateFilters(cfg *LogConfig) {
	h.updateFilters(cfg)
}