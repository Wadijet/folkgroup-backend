# Cấu Hình Go Version cho Linter

## Vấn Đề

Linter đang báo lỗi:
```
go: go.work requires go >= 1.24.0 (running go 1.23.1)
```

Nguyên nhân: Linter đang sử dụng Go version cũ (1.23.1) trong khi workspace yêu cầu Go 1.24.0+.

## Giải Pháp

### 1. Kiểm Tra Go Version Hiện Tại

```bash
go version
```

Kết quả mong đợi: `go version go1.24.11 windows/amd64` (hoặc tương tự)

### 2. Cấu Hình VS Code/Cursor

File `.vscode/settings.json` đã được tạo với cấu hình đầy đủ. Nếu vẫn còn lỗi:

1. **Reload Window**: 
   - Nhấn `Ctrl+Shift+P` (hoặc `Cmd+Shift+P` trên Mac)
   - Gõ "Reload Window"
   - Chọn "Developer: Reload Window"

2. **Restart Go Language Server**:
   - Nhấn `Ctrl+Shift+P`
   - Gõ "Go: Restart Language Server"
   - Chọn "Go: Restart Language Server"

3. **Kiểm Tra Go Extension**:
   - Đảm bảo Go extension đã được cài đặt và cập nhật
   - Extension: `golang.go` (Go team at Google)

### 3. Cấu Hình GolangCI-Lint (Nếu Dùng)

File `.golangci.yml` đã được tạo. Để sử dụng:

```bash
# Cài đặt golangci-lint (nếu chưa có)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Chạy linter
golangci-lint run ./api/...
```

### 4. Kiểm Tra PATH

Đảm bảo Go 1.24.x có trong PATH:

```bash
# Windows PowerShell
$env:PATH -split ';' | Select-String -Pattern 'go'

# Hoặc kiểm tra Go binary
where go
```

### 5. Cập Nhật Go (Nếu Cần)

Nếu Go version < 1.24.0:

```bash
# Windows (sử dụng Go installer)
# Tải từ: https://go.dev/dl/

# Hoặc sử dụng g (Go version manager)
go install github.com/voidint/g@latest
g install 1.24.11
```

## Xác Nhận

Sau khi cấu hình, kiểm tra lại:

```bash
# Kiểm tra Go version
go version

# Kiểm tra workspace
cd d:\Crossborder\folkform-workspace\folkgroup-backend
go work sync

# Kiểm tra module
cd api
go mod verify
```

Tất cả các lệnh trên phải chạy thành công không có lỗi.

## Lưu Ý

- **Lỗi linter không ảnh hưởng đến việc build/run code**: Code vẫn chạy bình thường nếu Go version trong hệ thống đúng
- **Lỗi chỉ xuất hiện trong IDE**: Đây là vấn đề cấu hình IDE, không phải lỗi code
- **Reload Window thường giải quyết được vấn đề**: IDE có thể cache Go version cũ

## Troubleshooting

Nếu vẫn còn lỗi sau khi reload:

1. **Xóa cache Go**:
   ```bash
   go clean -modcache
   go clean -cache
   ```

2. **Kiểm tra Go toolchain**:
   ```bash
   go env GOROOT
   go env GOVERSION
   ```

3. **Cập nhật Go toolchain**:
   ```bash
   go install golang.org/dl/go1.24.11@latest
   go1.24.11 download
   ```

4. **Restart IDE hoàn toàn**: Đóng và mở lại Cursor/VS Code
