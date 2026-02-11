// Package registry cung cấp implementation của registry pattern với generic type.
// Package này cho phép quản lý các singleton instances trong ứng dụng một cách thread-safe.
// Sử dụng generic type để có thể tái sử dụng cho nhiều loại đối tượng khác nhau.
package registry

import (
	"fmt"
	"meta_commerce/internal/common"
	"sync"
)

// Registry là một thread-safe generic registry pattern implementation.
// Type parameter T cho phép registry quản lý bất kỳ loại object nào.
// Thread-safety được đảm bảo thông qua sync.RWMutex.
//
// Example:
//
//	// Tạo registry cho kiểu string
//	strRegistry := NewRegistry[string]()
//
//	// Đăng ký một item
//	strRegistry.Register("key", "value")
//
//	// Lấy item
//	if value, exists := strRegistry.Get("key"); exists {
//	    fmt.Println(value)
//	}
type Registry[T any] struct {
	items map[string]T // Map lưu trữ các items theo key
	mu    sync.RWMutex // Mutex để đảm bảo thread-safety
}

// NewRegistry tạo và trả về một registry mới.
// Generic type T xác định loại items mà registry sẽ quản lý.
//
// Returns:
//   - *Registry[T]: Registry instance mới, đã được khởi tạo
//
// Example:
//
//	registry := NewRegistry[int]()
func NewRegistry[T any]() *Registry[T] {
	return &Registry[T]{
		items: make(map[string]T),
	}
}

// ====================================
// CÁC PHƯƠNG THỨC CỦA REGISTRY
// ====================================

// Register đăng ký một item mới vào registry.
// Nếu item với name đã tồn tại, nó sẽ bị ghi đè.
//
// Parameters:
//   - name: Định danh duy nhất cho item
//   - item: Item cần đăng ký
//
// Returns:
//   - isNew: true nếu là item mới, false nếu ghi đè item cũ
//   - err: Trả về lỗi nếu name rỗng
//
// Thread-safety: Safe for concurrent use
//
// Example:
//
//	isNew, err := registry.Register("counter", 42)
//	if isNew {
//	    fmt.Println("Đã tạo counter mới")
//	} else {
//	    fmt.Println("Đã ghi đè counter cũ")
//	}
func (r *Registry[T]) Register(name string, item T) (isNew bool, err error) {
	if name == "" {
		return false, fmt.Errorf("name cannot be empty: %w", common.ErrRequiredField)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, exists := r.items[name]
	r.items[name] = item
	return !exists, nil
}

// Get lấy item theo tên.
// Trả về item và một boolean cho biết item có tồn tại hay không.
//
// Parameters:
//   - name: Tên của item cần lấy
//
// Returns:
//   - item: Item nếu tìm thấy, zero value của T nếu không tìm thấy
//   - exists: true nếu item tồn tại, false nếu không
//
// Thread-safety: Safe for concurrent use
//
// Example:
//
//	if item, exists := registry.Get("counter"); exists {
//	    fmt.Printf("Counter value: %d\n", item)
//	}
func (r *Registry[T]) Get(name string) (item T, exists bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, exists = r.items[name]
	return item, exists
}

// GetOrCreate lấy item theo tên, nếu không tồn tại sẽ tạo mới thông qua creator function
//
// Parameters:
//   - name: Tên của item
//   - creator: Function tạo item mới
//
// Returns:
//   - item: Item (existing hoặc newly created)
//   - err: Lỗi nếu có trong quá trình tạo hoặc validate
//
// Thread-safety: Safe for concurrent use
//
// Example:
//
//	item, err := registry.GetOrCreate("counter", func() (int, error) {
//	    return 0, nil
//	})
func (r *Registry[T]) GetOrCreate(name string, creator func() (T, error)) (item T, err error) {
	if name == "" {
		return item, fmt.Errorf("name cannot be empty: %w", common.ErrRequiredField)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if existingItem, exists := r.items[name]; exists {
		return existingItem, nil
	}

	newItem, err := creator()
	if err != nil {
		return item, fmt.Errorf("failed to create item: %w", err)
	}

	r.items[name] = newItem
	return newItem, nil
}

// Update cập nhật item một cách thread-safe
//
// Parameters:
//   - name: Tên của item
//   - updater: Function cập nhật item
//
// Returns:
//   - error: Lỗi nếu có
//
// Thread-safety: Safe for concurrent use
//
// Example:
//
//	err := registry.Update("counter", func(current int) (int, error) {
//	    return current + 1, nil
//	})
func (r *Registry[T]) Update(name string, updater func(T) (T, error)) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	current, exists := r.items[name]
	if !exists {
		return fmt.Errorf("item not found: %s: %w", name, common.ErrNotFound)
	}

	updated, err := updater(current)
	if err != nil {
		return fmt.Errorf("failed to update item: %w", err)
	}

	r.items[name] = updated
	return nil
}

// Clear xóa một item khỏi registry.
// Nếu cleanup function được cung cấp, nó sẽ được gọi trước khi xóa để giải phóng tài nguyên.
//
// Parameters:
//   - name: Tên của item cần xóa
//   - cleanup: Optional function để giải phóng tài nguyên trước khi xóa
//
// Returns:
//   - deleted: true nếu item bị xóa, false nếu item không tồn tại
//   - err: Lỗi nếu có trong quá trình cleanup
//
// Thread-safety: Safe for concurrent use
//
// Example:
//
//	// Xóa item đơn giản
//	deleted, _ := registry.Clear("counter", nil)
//
//	// Xóa database connection với cleanup
//	deleted, err := registry.Clear("db1", func(db *Database) error {
//	    return db.Close()
//	})
func (r *Registry[T]) Clear(name string, cleanup func(T) error) (deleted bool, err error) {
	if name == "" {
		return false, fmt.Errorf("name cannot be empty: %w", common.ErrRequiredField)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	item, exists := r.items[name]
	if !exists {
		return false, nil
	}

	if cleanup != nil {
		if err := cleanup(item); err != nil {
			return false, fmt.Errorf("failed to cleanup item %s: %w", name, err)
		}
	}

	delete(r.items, name)
	return true, nil
}

// ClearAll xóa tất cả items trong registry.
// Nếu cleanup function được cung cấp, nó sẽ được gọi cho mỗi item trước khi xóa.
//
// Parameters:
//   - cleanup: Optional function để giải phóng tài nguyên trước khi xóa
//
// Returns:
//   - count: Số lượng items đã bị xóa
//   - err: Lỗi nếu có trong quá trình cleanup
//
// Thread-safety: Safe for concurrent use
//
// Example:
//
//	// Xóa tất cả items đơn giản
//	count, _ := registry.ClearAll(nil)
//
//	// Xóa tất cả database connections với cleanup
//	count, err := registry.ClearAll(func(db *Database) error {
//	    return db.Close()
//	})
func (r *Registry[T]) ClearAll(cleanup func(T) error) (count int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	count = len(r.items)
	if count == 0 {
		return 0, nil
	}

	if cleanup != nil {
		var errs []error
		for name, item := range r.items {
			if err := cleanup(item); err != nil {
				errs = append(errs, fmt.Errorf("failed to cleanup %s: %w", name, err))
			}
		}
		if len(errs) > 0 {
			return 0, fmt.Errorf("cleanup errors occurred: %v", errs)
		}
	}

	r.items = make(map[string]T)
	return count, nil
}
