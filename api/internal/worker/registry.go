// Package worker — Registry quản lý thống nhất tất cả background workers.
// Cung cấp interface chuẩn và khởi động tập trung với panic recovery.
package worker

import (
	"context"
	"sync"

	"meta_commerce/internal/logger"
)

// Worker interface chuẩn cho tất cả background workers.
// Mọi worker phải implement Start(ctx) để chạy trong vòng lặp cho đến khi context bị cancel.
type Worker interface {
	Start(ctx context.Context)
}

// Entry mô tả một worker đã đăng ký (tên + instance).
type Entry struct {
	Name   string
	Worker Worker
}

// Registry quản lý danh sách workers và khởi động tập trung.
type Registry struct {
	mu      sync.Mutex
	workers []Entry
}

var defaultRegistry *Registry
var registryOnce sync.Once

// DefaultRegistry trả về singleton Registry.
func DefaultRegistry() *Registry {
	registryOnce.Do(func() {
		defaultRegistry = &Registry{workers: make([]Entry, 0)}
	})
	return defaultRegistry
}

// Register đăng ký worker vào registry.
// Nếu w là nil (do lỗi khi tạo), worker sẽ bị bỏ qua khi StartAll.
func (r *Registry) Register(name string, w Worker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workers = append(r.workers, Entry{Name: name, Worker: w})
}

// StartAll khởi động tất cả workers đã đăng ký trong goroutine riêng.
// Mỗi worker chạy với panic recovery; khi ctx bị cancel, workers sẽ dừng.
func (r *Registry) StartAll(ctx context.Context) {
	r.mu.Lock()
	list := make([]Entry, len(r.workers))
	copy(list, r.workers)
	r.mu.Unlock()

	log := logger.GetAppLogger()
	for _, e := range list {
		if e.Worker == nil {
			log.WithFields(map[string]interface{}{"worker": e.Name}).Warn("Worker bỏ qua (không tạo được)")
			continue
		}
		entry := e
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithFields(map[string]interface{}{
						"panic":  r,
						"worker": entry.Name,
					}).Error("Worker panic, đã dừng")
				}
			}()
			entry.Worker.Start(ctx)
			log.WithFields(map[string]interface{}{"worker": entry.Name}).Warn("Worker đã dừng")
		}()
	}
}

// Count trả về số workers đã đăng ký (kể cả nil).
func (r *Registry) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.workers)
}
