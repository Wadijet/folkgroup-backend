// Package metrics - Đo lường thời gian thực hiện từng loại job, lưu in-memory.
package metrics

import (
	"sync"
	"time"
)

const (
	// BufferSize số mẫu tối đa mỗi job type (1k gần nhất).
	BufferSize = 1000
	// Window1Hour khoảng 1 giờ để đếm countLastHour.
	Window1Hour = time.Hour
)

// entry lưu timestamp và duration của 1 job.
type entry struct {
	ts int64 // Unix nano
	ns int64 // duration nanoseconds
}

// bucket ring buffer cho 1 job type.
type bucket struct {
	mu       sync.RWMutex
	entries  [BufferSize]entry
	head     int   // vị trí ghi tiếp theo
	count    int   // số phần tử hiện có (0..BufferSize)
	sumNs    int64 // tổng duration (để tính avg O(1))
}

var (
	buckets   = make(map[string]*bucket)
	bucketsMu sync.RWMutex
)

// getBucket trả về bucket cho jobType, tạo mới nếu chưa có.
func getBucket(jobType string) *bucket {
	if jobType == "" {
		jobType = "unknown"
	}
	bucketsMu.RLock()
	b := buckets[jobType]
	bucketsMu.RUnlock()
	if b != nil {
		return b
	}
	bucketsMu.Lock()
	defer bucketsMu.Unlock()
	if b = buckets[jobType]; b == nil {
		b = &bucket{}
		buckets[jobType] = b
	}
	return b
}

// RecordDuration ghi nhận thời gian thực hiện 1 job. Gọi từ worker sau khi xử lý xong.
func RecordDuration(jobType string, duration time.Duration) {
	b := getBucket(jobType)
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now().UnixNano()
	ns := duration.Nanoseconds()

	// Nếu buffer đã đầy, trừ phần tử cũ nhất khỏi sum
	if b.count == BufferSize {
		old := b.entries[b.head]
		b.sumNs -= old.ns
	} else {
		b.count++
	}

	b.entries[b.head] = entry{ts: now, ns: ns}
	b.sumNs += ns
	b.head = (b.head + 1) % BufferSize
}

// JobMetric kết quả metrics cho 1 job type.
type JobMetric struct {
	AvgMs         int64 `json:"avgMs"`
	MinMs         int64 `json:"minMs"`
	MaxMs         int64 `json:"maxMs"`
	SampleCount   int   `json:"sampleCount"`
	CountLastHour int   `json:"countLastHour"`
}

// GetAll trả về tất cả metrics — dùng cho API duy nhất.
func GetAll() map[string]JobMetric {
	bucketsMu.RLock()
	names := make([]string, 0, len(buckets))
	for k := range buckets {
		names = append(names, k)
	}
	bucketsMu.RUnlock()

	result := make(map[string]JobMetric, len(names))
	threshold := time.Now().Add(-Window1Hour).UnixNano()

	for _, jobType := range names {
		b := getBucket(jobType)
		b.mu.RLock()
		avgMs := int64(0)
		minMs := int64(0)
		maxMs := int64(0)
		if b.count > 0 {
			avgMs = (b.sumNs / int64(b.count)) / int64(time.Millisecond)
			// Tính min, max bằng cách duyệt toàn bộ buffer
			for i := 0; i < b.count; i++ {
				var idx int
				if b.count < BufferSize {
					idx = i
				} else {
					idx = (b.head + i) % BufferSize
				}
				ms := b.entries[idx].ns / int64(time.Millisecond)
				if i == 0 || ms < minMs {
					minMs = ms
				}
				if i == 0 || ms > maxMs {
					maxMs = ms
				}
			}
		}
		countLastHour := 0
		for i := 0; i < b.count; i++ {
			var idx int
			if b.count < BufferSize {
				idx = i
			} else {
				idx = (b.head + i) % BufferSize
			}
			if b.entries[idx].ts >= threshold {
				countLastHour++
			}
		}
		sampleCount := b.count
		b.mu.RUnlock()

		result[jobType] = JobMetric{
			AvgMs:         avgMs,
			MinMs:         minMs,
			MaxMs:         maxMs,
			SampleCount:   sampleCount,
			CountLastHour: countLastHour,
		}
	}
	return result
}
