# Phân Tích Tại Sao Server Không Panic Safe

## Vấn Đề Chính

Server có một số nơi **KHÔNG có recover**, khiến panic có thể làm crash toàn bộ server:

### 1. ❌ Logger Hook Goroutine - THIẾU RECOVER

**File:** `api/core/logger/hook.go`

**Vấn đề:** Hàm `processEntries()` chạy trong goroutine riêng nhưng **KHÔNG có recover**:

```go
// processEntries xử lý log entries trong một goroutine riêng
func (h *AsyncHook) processEntries() {
	defer h.wg.Done()

	for entry := range h.entries {
		// ❌ KHÔNG CÓ RECOVER Ở ĐÂY!
		
		if entry.Logger.Formatter != nil {
			data, err = entry.Logger.Formatter.Format(entry) // Có thể panic!
		}
		
		for _, writer := range h.writers {
			_, err = writer.Write(data) // Có thể panic! (ví dụ: bytes.Buffer race condition)
		}
	}
}
```

**Hậu quả:**
- Nếu `Format()` panic → goroutine crash → server có thể crash
- Nếu `Write()` panic (ví dụ: bytes.Buffer race condition) → goroutine crash → server có thể crash
- Logger là critical component, nếu nó crash có thể làm crash toàn bộ server

### 2. ⚠️ Cleanup Job Goroutine - THIẾU RECOVER Ở NGOÀI

**File:** `api/core/delivery/processor.go`

**Vấn đề:** Có recover cho từng item nhưng **KHÔNG có recover cho toàn bộ goroutine**:

```go
go func() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// ❌ KHÔNG CÓ RECOVER Ở ĐÂY!
			
			stuckItems, err := p.queueService.FindStuckItems(ctx, staleMinutes, batchSize)
			// Nếu FindStuckItems panic → goroutine crash
			
			for _, item := range stuckItems {
				func() {
					defer func() {
						if r := recover(); r != nil {
							// ✅ Có recover cho từng item
						}
					}()
					// Xử lý item
				}()
			}
		}
	}
}()
```

**Hậu quả:**
- Nếu `FindStuckItems()` panic → goroutine crash → cleanup job dừng
- Nếu có lỗi ở ngoài loop → goroutine crash

### 3. ✅ Fiber Recover Middleware - CHỈ BẮT PANIC TRONG HTTP HANDLERS

**File:** `api/cmd/server/init.fiber.go`

**Vấn đề:** Recover middleware **CHỈ bắt panic trong HTTP request handlers**, KHÔNG bắt panic trong background goroutines:

```go
app.Use(recover.New(recover.Config{
	// ✅ Bắt panic trong HTTP handlers
	// ❌ KHÔNG bắt panic trong background goroutines
}))
```

**Hậu quả:**
- Panic trong HTTP handler → ✅ Được bắt, trả về 500 error
- Panic trong background goroutine → ❌ KHÔNG được bắt → server crash

## Tại Sao Điều Này Nguy Hiểm?

### Kịch Bản Crash Thực Tế:

1. **Logger goroutine panic:**
   ```
   bytes.Buffer.WriteByte() → race condition → panic
   → Logger goroutine crash
   → Server có thể crash (tùy thuộc vào Go runtime)
   ```

2. **Cleanup job panic:**
   ```
   FindStuckItems() → database panic → goroutine crash
   → Cleanup job dừng
   → Items bị stuck mãi mãi
   ```

3. **Background worker panic:**
   ```
   Worker goroutine panic → không được recover
   → Worker dừng
   → Service không hoạt động
   ```

## Giải Pháp

### ✅ Fix 1: Thêm Recover Vào Logger Hook

```go
// processEntries xử lý log entries trong một goroutine riêng
func (h *AsyncHook) processEntries() {
	defer h.wg.Done()

	for entry := range h.entries {
		// ✅ THÊM RECOVER Ở ĐÂY
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Log panic nhưng không crash server
					// Không thể dùng logger ở đây vì sẽ tạo vòng lặp
					// Có thể ghi vào stderr trực tiếp
					fmt.Fprintf(os.Stderr, "Logger panic: %v\n", r)
					debug.PrintStack()
				}
			}()

			// Format và write như bình thường
			var data []byte
			var err error

			if entry.Logger.Formatter != nil {
				data, err = entry.Logger.Formatter.Format(entry)
			} else {
				line, strErr := entry.String()
				if strErr != nil {
					continue
				}
				data = []byte(line)
			}

			if err != nil {
				continue
			}

			for _, writer := range h.writers {
				_, err = writer.Write(data)
				if err != nil {
					continue
				}
			}
		}()
	}
}
```

### ✅ Fix 2: Thêm Recover Vào Cleanup Job

```go
go func() {
	defer func() {
		if r := recover(); r != nil {
			log := logger.GetAppLogger()
			log.WithFields(map[string]interface{}{
				"panic": r,
			}).Error("📦 [CLEANUP] Cleanup job panic, sẽ restart sau")
			// Có thể restart job sau một khoảng thời gian
		}
	}()

	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Xử lý như bình thường
		}
	}
}()
```

### ✅ Fix 3: Tạo Panic Recovery Wrapper

Tạo một helper function để wrap tất cả background goroutines:

```go
// SafeGo chạy function trong goroutine với recover
func SafeGo(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log := logger.GetAppLogger()
				log.WithFields(map[string]interface{}{
					"panic": r,
					"goroutine": name,
				}).Error(fmt.Sprintf("[%s] Goroutine panic recovered", name))
				debug.PrintStack()
			}
		}()
		fn()
	}()
}

// Sử dụng:
SafeGo("logger-processEntries", func() {
	hook.processEntries()
})
```

## Khuyến Nghị

1. **✅ Thêm recover vào TẤT CẢ background goroutines**
2. **✅ Logger phải cực kỳ an toàn** - không được panic
3. **✅ Test panic scenarios** - đảm bảo server không crash
4. **✅ Monitor goroutine health** - phát hiện khi goroutine crash
5. **✅ Restart mechanism** - tự động restart goroutine khi crash

## Tài Liệu Tham Khảo

- [Go: Recovering from Panics](https://go.dev/blog/defer-panic-and-recover)
- [Fiber: Recover Middleware](https://docs.gofiber.io/api/middleware/recover)
- [Best Practices: Panic Recovery](https://go.dev/doc/effective_go#panic)
