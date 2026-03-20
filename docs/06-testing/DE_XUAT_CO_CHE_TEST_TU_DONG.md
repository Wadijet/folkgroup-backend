# Đề Xuất Cơ Chế Test Tự Động

Tài liệu đề xuất cơ chế chạy test tự động cho Folkform Backend, dựa trên phân tích codebase hiện tại.

---

## 1. Tổng Quan Hiện Trạng

### 1.1 Cấu trúc test hiện có

| Loại | Vị trí | Số lượng | Phụ thuộc |
|------|--------|----------|------------|
| **Unit test** | `api/internal/.../*_test.go` | 2 file (crm/snapshot, report/layer3) | Không (pure logic) |
| **Integration/E2E test** | `api-tests/cases/*.go` | 20+ file | Server, MongoDB, Firebase token |

### 1.2 Quy trình chạy test hiện tại

- **Script chính**: `api-tests/test.ps1`
- **Luồng**: Kiểm tra server → Build server nếu cần → Khởi động server → Đợi health → Chạy `go test ./api-tests/cases/...` → Dừng server → Tạo báo cáo Markdown
- **Hạn chế**:
  - Chỉ chạy integration tests, không chạy unit tests trong `api`
  - Phụ thuộc PowerShell (Windows)
  - Không có CI/CD (không có `.github/workflows`, `.gitlab-ci.yml`)
  - Cần `TEST_FIREBASE_ID_TOKEN` thủ công

### 1.3 Công cụ đã dùng

- `go test` (standard)
- `github.com/stretchr/testify` (trong api-tests)
- Go workspace (`go.work`) với `api` và `api-tests`

---

## 2. Đề Xuất Cơ Chế Test Tự Động

### 2.1 Phân tầng test

```
┌─────────────────────────────────────────────────────────────────┐
│  L0: Unit Tests (nhanh, không phụ thuộc)                        │
│  - api/internal/.../*_test.go                                    │
│  - Chạy: go test ./api/... -short                                │
│  - Thời gian: ~5–30 giây                                         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  L1: Integration Tests (cần server + DB)                         │
│  - api-tests/cases/*.go                                          │
│  - Chạy: .\api-tests\test.ps1 hoặc go test ./api-tests/...       │
│  - Thời gian: ~2–5 phút                                          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  L2: Smoke/E2E (tùy chọn, pre-release)                           │
│  - Subset critical paths                                         │
│  - Chạy trước deploy                                             │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 Các điểm kích hoạt tự động

| Điểm kích hoạt | L0 Unit | L1 Integration | Ghi chú |
|----------------|---------|----------------|---------|
| **Pre-commit** | ✅ Bắt buộc | ❌ Không | Nhanh, không cần server |
| **Push / PR** | ✅ | ✅ (hoặc nightly) | CI chạy full |
| **Merge main** | ✅ | ✅ | Đảm bảo main luôn xanh |
| **Nightly** | - | ✅ Full suite | Chạy integration đầy đủ |

---

## 3. Cơ Chế Cụ Thể

### 3.1 Script thống nhất (cross-platform)

Tạo `scripts/test.sh` (Linux/Mac) và mở rộng `api-tests/test.ps1` để hỗ trợ:

```powershell
# api-tests/test.ps1 - Thêm tham số
param(
    [switch]$SkipServer = $false,
    [switch]$UnitOnly = $false,   # Chỉ chạy unit tests trong api
    [switch]$IntegrationOnly = $false  # Chỉ chạy integration (mặc định: cả hai)
)
```

**Luồng mới**:
1. Nếu `-UnitOnly`: `go test ./api/... -short -count=1` → thoát
2. Nếu `-IntegrationOnly` hoặc mặc định: giữ luồng hiện tại (build server, chạy api-tests)

### 3.2 Lệnh chạy nhanh (unit only)

```bash
# Chỉ unit tests - dùng trước commit
go test ./api/... -short -count=1 -v

# Với coverage
go test ./api/... -short -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### 3.3 GitHub Actions (CI)

Tạo `.github/workflows/test.yml`:

```yaml
name: Test

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main, develop]

jobs:
  unit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
          cache-dependency-path: api/go.sum
      - name: Unit tests
        run: go test ./api/... -short -count=1 -v
        working-directory: .

  integration:
    runs-on: ubuntu-latest
    needs: unit
    services:
      mongodb:
        image: mongo:7
        ports:
          - 27017:27017
    env:
      GO_ENV: development
      MONGODB_URI: mongodb://localhost:27017
      TEST_FIREBASE_ID_TOKEN: ${{ secrets.TEST_FIREBASE_ID_TOKEN }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - name: Build server
        run: go build -o server_test ./api/cmd/server/
      - name: Start server
        run: ./server_test &
      - name: Wait for health
        run: |
          for i in {1..60}; do
            curl -s http://localhost:8080/api/v1/system/health && break
            sleep 1
          done
      - name: Integration tests
        run: go test -v ./api-tests/cases/...
```

**Lưu ý**: Integration job cần `TEST_FIREBASE_ID_TOKEN` trong GitHub Secrets. Có thể tách integration thành workflow riêng chạy nightly nếu token khó cấu hình.

### 3.4 Pre-commit hook (tùy chọn)

Tạo `.husky/pre-commit` hoặc script trong `.git/hooks/pre-commit`:

```bash
#!/bin/sh
go test ./api/... -short -count=1
exit $?
```

Hoặc dùng [pre-commit](https://pre-commit.com/) framework.

### 3.5 Makefile (tùy chọn)

Để thống nhất lệnh trên mọi môi trường:

```makefile
# Makefile
.PHONY: test test-unit test-integration test-all

test-unit:
	go test ./api/... -short -count=1 -v

test-integration:
	./api-tests/test.ps1 -SkipServer  # Hoặc script tương đương

test-all: test-unit test-integration

test: test-unit
```

---

## 4. Mở Rộng Unit Tests

### 4.1 Ưu tiên viết unit test

| Module | Hàm/Logic | Lý do |
|--------|------------|-------|
| `crm/service` | `BuildSnapshotWithChanges`, `diffSnapshot`, `buildMetricsSnapshot` | Đã có snapshot_test, mở rộng thêm |
| `ruleintel/engine` | `Execute`, evaluate rules | Logic phức tạp, dễ mock |
| `report/layer3` | Các hàm tính layer | Pure logic |
| `ads/service` | Throttle, circuit breaker | Logic nghiệp vụ quan trọng |
| `utility/data.extract` | Các hàm extract | Pure function |

### 4.2 Pattern unit test

```go
// Ví dụ: service.crm.snapshot_test.go
func TestBuildSnapshotWithChanges_NoChange_ReturnsNil(t *testing.T) {
	ctx := context.Background()
	c := &crmmodels.CrmCustomer{OrderCount: 1}
	profile := buildProfileSnapshot(c)
	metrics := buildMetricsSnapshot(ctx, c)
	result := BuildSnapshotWithChanges(ctx, c, profile, metrics, 0, nil, nil)
	assert.Nil(t, result)
}
```

### 4.3 Mock cho dependency

- **MongoDB**: Dùng interface, inject mock trong test
- **Firebase**: Mock `VerifyIDToken` bằng test double
- **External API**: Dùng `httptest.Server` hoặc interface

---

## 5. Cải Thiện Integration Tests

### 5.1 Chạy không cần Firebase token (smoke only)

Tạo subset test không cần auth:

```go
// cases/smoke_test.go
func TestHealthAndPublicEndpoints(t *testing.T) {
	client := utils.NewHTTPClient("http://localhost:8080/api/v1", 5)
	resp, _, err := client.GET("/system/health")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
```

CI có thể chạy smoke trước, full integration khi có token.

### 5.2 Docker Compose cho môi trường test

```yaml
# docker-compose.test.yml
services:
  mongodb:
    image: mongo:7
    ports:
      - "27017:27017"
  # Server chạy từ host hoặc trong container
```

Giúp dev và CI dùng cùng môi trường.

---

## 6. Báo Cáo và Coverage

### 6.1 Coverage report

```bash
# Unit coverage
go test ./api/... -short -coverprofile=api/coverage.out
go tool cover -html=api/coverage.out -o coverage.html

# Integration (nếu hỗ trợ)
go test ./api-tests/... -coverprofile=api-tests/coverage.out
```

### 6.2 Mục tiêu coverage

- **Unit**: 70%+ cho các package core (crm, ruleintel, report)
- **Integration**: Duy trì ~60%, hướng tới 80% endpoint (theo endpoint-coverage-checklist.md)

---

## 7. Lộ Trình Triển Khai

| Giai đoạn | Nội dung | Ước lượng |
|-----------|----------|-----------|
| **Phase 1** | Thêm `-UnitOnly` vào test.ps1, chạy unit trước integration | 0.5 ngày |
| **Phase 2** | Tạo GitHub Actions cho unit tests (không cần token) | 0.5 ngày |
| **Phase 3** | Thêm unit tests cho 2–3 package ưu tiên | 2–3 ngày |
| **Phase 4** | Cấu hình integration trong CI (cần MongoDB + token) | 1 ngày |
| **Phase 5** | Pre-commit hook + Makefile (tùy chọn) | 0.5 ngày |

---

## 8. Tóm Tắt

| Thành phần | Đề xuất |
|------------|---------|
| **Unit tests** | Chạy trong `api/`, dùng `-short`, không phụ thuộc server/DB |
| **Integration tests** | Giữ `api-tests/`, chạy qua test.ps1 hoặc CI |
| **CI** | GitHub Actions: unit luôn chạy, integration khi có MongoDB + token |
| **Pre-commit** | Chỉ unit tests (nhanh) |
| **Báo cáo** | Giữ report Markdown hiện tại, thêm coverage HTML |
| **Mở rộng** | Tăng unit tests cho crm, ruleintel, report, ads |

---

## 9. Tài Liệu Liên Quan

- [Tổng Quan Testing](tong-quan.md)
- [Chạy Test Suite](chay-test.md)
- [Viết Test Case](viet-test.md)
- [Endpoint Coverage Checklist](endpoint-coverage-checklist.md)
