package utility

import (
	"sync"
	"time"
)

// Cache là struct để quản lý cache với thời gian sống và thời gian dọn dẹp
type Cache struct {
	items    map[string]interface{}
	mu       sync.RWMutex
	ttl      time.Duration
	cleanup  time.Duration
	stopChan chan struct{}
}

// NewCache tạo một instance mới của Cache
func NewCache(ttl, cleanup time.Duration) *Cache {
	cache := &Cache{
		items:    make(map[string]interface{}),
		ttl:      ttl,
		cleanup:  cleanup,
		stopChan: make(chan struct{}),
	}
	go cache.cleanupLoop()
	return cache
}

// Set lưu giá trị vào cache
func (c *Cache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = value
}

// Get lấy giá trị từ cache
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, exists := c.items[key]
	return value, exists
}

// cleanupLoop dọn dẹp cache định kỳ
func (c *Cache) cleanupLoop() {
	ticker := time.NewTicker(c.cleanup)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			for k := range c.items {
				delete(c.items, k)
			}
			c.mu.Unlock()
		case <-c.stopChan:
			return
		}
	}
}
