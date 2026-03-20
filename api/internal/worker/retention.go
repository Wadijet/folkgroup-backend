// Package worker - Cấu hình retention (số ngày giữ log) cho workers cleanup.
package worker

import (
	"os"
	"strconv"
	"strings"
	"sync"
)

// workerRetentionOverrides override retention qua API. Key: worker name.
var (
	workerRetentionOverrides   = make(map[string]int64)
	workerRetentionOverridesMu sync.RWMutex
)

// SetWorkerRetentionOverride đặt override retention (số ngày) cho worker.
func SetWorkerRetentionOverride(workerName string, retentionDays int64) {
	name := strings.ToLower(strings.TrimSpace(workerName))
	if name == "" {
		return
	}
	workerRetentionOverridesMu.Lock()
	defer workerRetentionOverridesMu.Unlock()
	if retentionDays >= 1 {
		workerRetentionOverrides[name] = retentionDays
	} else {
		delete(workerRetentionOverrides, name)
	}
}

// ClearWorkerRetentionOverride xóa override retention.
func ClearWorkerRetentionOverride(workerName string) {
	SetWorkerRetentionOverride(workerName, 0)
}

// GetWorkerRetentionOverrides trả về override hiện tại (để API GET).
func GetWorkerRetentionOverrides() map[string]int64 {
	workerRetentionOverridesMu.RLock()
	defer workerRetentionOverridesMu.RUnlock()
	out := make(map[string]int64, len(workerRetentionOverrides))
	for k, v := range workerRetentionOverrides {
		out[k] = v
	}
	return out
}

// GetEffectiveWorkerRetention trả về retention hiệu dụng. Thứ tự: API override > env > default.
// Env: WORKER_<NAME>_RETENTION_DAYS (vd: WORKER_NOTIFICATION_AGENT_ACTIVITY_CLEANUP_RETENTION_DAYS).
func GetEffectiveWorkerRetention(workerName string, defaultDays int64) int64 {
	canonical := strings.ToLower(strings.TrimSpace(workerName))
	workerRetentionOverridesMu.RLock()
	if n, ok := workerRetentionOverrides[canonical]; ok && n >= 1 {
		workerRetentionOverridesMu.RUnlock()
		return n
	}
	workerRetentionOverridesMu.RUnlock()

	envKey := "WORKER_" + strings.ToUpper(strings.ReplaceAll(canonical, "-", "_")) + "_RETENTION_DAYS"
	if v := os.Getenv(envKey); v != "" {
		n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err == nil && n >= 1 {
			return n
		}
	}
	return defaultDays
}

// workerRetentionDefaults retention mặc định cho workers có retention.
var workerRetentionDefaults = map[string]int64{
	WorkerAgentActivityCleanup: 1,
}

// GetAllWorkerRetentions trả về retention hiệu dụng cho workers có retention (để API GET).
func GetAllWorkerRetentions() map[string]int64 {
	out := make(map[string]int64)
	for name, def := range workerRetentionDefaults {
		out[name] = GetEffectiveWorkerRetention(name, def)
	}
	return out
}
