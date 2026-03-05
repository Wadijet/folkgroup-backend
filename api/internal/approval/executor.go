// Package approval — Đăng ký executor: delegate sang engine.
package approval

import (
	pkgapproval "meta_commerce/pkg/approval"
)

// RegisterExecutor đăng ký executor cho domain. Gọi khi init (ví dụ từ ads).
func RegisterExecutor(domain string, ex pkgapproval.Executor) {
	Init()
	GetEngine().RegisterExecutor(domain, ex)
}

// RegisterEventTypes đăng ký EventType cho domain (executed, rejected).
func RegisterEventTypes(domain string, types map[string]string) {
	Init()
	GetEngine().RegisterEventTypes(domain, types)
}
