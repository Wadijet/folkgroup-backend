# Hướng Dẫn Xử Lý Crash Ở WriteByte

## Tóm Tắt Vấn Đề

Crash ở hàm `WriteByte` trong `bytes.Buffer` thường do **race condition** khi nhiều goroutine cùng truy cập một buffer instance mà không có synchronization.

## Các Bước Kiểm Tra

### 1. Tìm Nơi Sử Dụng bytes.Buffer

```bash
# Tìm tất cả nơi sử dụng bytes.Buffer
grep -r "bytes.Buffer" . --include="*.go"

# Hoặc trên Windows PowerShell
Select-String -Path "*.go" -Pattern "bytes.Buffer" -Recurse
```

### 2. Kiểm Tra Race Condition

```bash
# Chạy với race detector
go run -race main.go

# Hoặc test
go test -race ./...
```

### 3. Kiểm Tra Stack Trace

Nếu crash xảy ra, tìm trong log:
- `panic: runtime error: index out of range`
- `fatal error: concurrent map writes`
- `nil pointer dereference`

## Giải Pháp Nhanh

### Nếu Buffer Được Dùng Trong Goroutine

**❌ KHÔNG AN TOÀN:**
```go
var sharedBuf bytes.Buffer

go func() {
    sharedBuf.WriteByte('a') // Race condition!
}()

go func() {
    sharedBuf.WriteByte('b') // Race condition!
}()
```

**✅ AN TOÀN - Dùng Mutex:**
```go
type SafeBuffer struct {
    mu  sync.Mutex
    buf bytes.Buffer
}

func (sb *SafeBuffer) WriteByte(c byte) error {
    sb.mu.Lock()
    defer sb.mu.Unlock()
    return sb.buf.WriteByte(c)
}
```

**✅ AN TOÀN - Mỗi Goroutine Dùng Buffer Riêng:**
```go
go func() {
    var buf bytes.Buffer // Buffer riêng
    buf.WriteByte('a')
}()
```

## Kiểm Tra Trong Codebase Hiện Tại

Sau khi kiểm tra, codebase hiện tại:

1. ✅ **`api/core/utility/data.extract.go`**: Dùng `strings.Builder` - an toàn (mỗi lần gọi tạo builder mới)
2. ✅ **Logger system**: Dùng channel và mutex đúng cách
3. ⚠️ **`deploy_notes/client_example_code(golang).txt`**: Chỉ là example code, không phải production

## Hành Động Khuyến Nghị

1. **Chạy race detector ngay:**
   ```bash
   cd api
   go test -race ./...
   ```

2. **Kiểm tra log crash** để xác định vị trí chính xác

3. **Nếu tìm thấy shared buffer**, thêm mutex protection

4. **Xem chi tiết** trong: `docs/analysis/buffer-writebyte-crash-analysis.md`

## Liên Hệ

Nếu vẫn gặp vấn đề sau khi kiểm tra, cung cấp:
- Stack trace đầy đủ
- Đoạn code gây crash
- Kết quả race detector
