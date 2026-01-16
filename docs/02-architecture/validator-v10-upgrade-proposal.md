# Đề Xuất Upgrade Validator v9 → v10

## Tình Trạng Hiện Tại
- **Version hiện tại**: `gopkg.in/go-playground/validator.v9`
- **Version mới nhất**: `github.com/go-playground/validator/v10` (v10.30.1 - Dec 2025)

## Lợi Ích Upgrade

### 1. **Cải Thiện Performance**
- V10 được tối ưu hóa tốt hơn, nhanh hơn v9
- Hỗ trợ Go modules đầy đủ

### 2. **Tính Năng Mới**
- Validation tags mới: `keys`, `endkeys` (map key validation)
- Cải thiện struct-level validation
- Hỗ trợ translation tốt hơn (bao gồm tiếng Việt)

### 3. **Bảo Mật & Bảo Trì**
- Nhận được security patches và bug fixes
- Community support tốt hơn

## Breaking Changes

### 1. **Import Path**
```go
// V9 (cũ)
import "gopkg.in/go-playground/validator.v9"

// V10 (mới)
import "github.com/go-playground/validator/v10"
```

### 2. **API Thay Đổi Nhỏ**
- `validator.FieldLevel` interface tương tự, nhưng có thể có methods mới
- Error handling tương tự: `err.(validator.ValidationErrors)`

### 3. **Custom Validators**
- Custom validators hiện tại (`no_xss`, `no_sql_injection`, `strong_password`) sẽ hoạt động tương tự
- Signature: `func(fl validator.FieldLevel) bool` - không đổi

## Files Cần Sửa

### 1. `api/core/global/validator.go`
```go
// Thay đổi import
- import "gopkg.in/go-playground/validator.v9"
+ import "github.com/go-playground/validator/v10"
```

### 2. `api/core/global/global.vars.go`
```go
// Thay đổi import
- validator "gopkg.in/go-playground/validator.v9"
+ validator "github.com/go-playground/validator/v10"
```

### 3. `api/core/api/handler/handler.base.go`
```go
// Thay đổi import
- import "gopkg.in/go-playground/validator.v9"
+ import "github.com/go-playground/validator/v10"
```

### 4. `api/cmd/server/init.go` (nếu có dùng)
```go
// Thay đổi import
- validator "gopkg.in/go-playground/validator.v9"
+ validator "github.com/go-playground/validator/v10"
```

## Migration Steps

### Bước 1: Update go.mod
```bash
go get github.com/go-playground/validator/v10@latest
go mod tidy
```

### Bước 2: Update Imports
- Thay tất cả `gopkg.in/go-playground/validator.v9` → `github.com/go-playground/validator/v10`

### Bước 3: Test
- Chạy tests để đảm bảo không có breaking changes
- Test các custom validators
- Test validation error messages

### Bước 4: Cleanup
- Xóa dependency cũ: `go mod tidy` sẽ tự động xóa nếu không còn dùng

## Rủi Ro

### Thấp
- API tương tự, ít breaking changes
- Custom validators hiện tại sẽ hoạt động tương tự
- Error handling không đổi

### Cần Kiểm Tra
- Test tất cả validation flows
- Đảm bảo error messages vẫn đúng format
- Kiểm tra performance (nên tốt hơn)

## Khuyến Nghị

✅ **Nên upgrade** vì:
- Version mới hơn, được maintain tốt hơn
- Performance tốt hơn
- Có tính năng mới hữu ích
- Breaking changes ít, migration đơn giản

## Timeline

- **Ước tính**: 1-2 giờ
- **Risk**: Thấp
- **Priority**: Medium (không urgent nhưng nên làm)
