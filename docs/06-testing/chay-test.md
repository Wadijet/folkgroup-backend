# Chạy Test Suite

Hướng dẫn chi tiết về cách chạy test suite.

## 📋 Tổng Quan

Test suite được tổ chức trong module `api-tests` và sử dụng PowerShell scripts để tự động hóa.

## 🚀 Cách Chạy

### Cách 0: Chỉ Unit Tests (Nhanh, không cần server)

```powershell
.\api-tests\test.ps1 -UnitOnly
```

Chạy unit tests trong `api/` (crm/snapshot, report/layer3). Không cần server, MongoDB, Firebase token.

### Cách 1: Script Tự Động (Khuyến Nghị)

```powershell
# Từ root directory
.\api-tests\test.ps1
```

Script sẽ tự động:
1. Kiểm tra server có đang chạy không
2. Khởi động server nếu chưa chạy
3. Đợi server sẵn sàng (tối đa 60 giây)
4. Chạy toàn bộ test suite
5. Tự động dừng server sau khi test xong
6. Hiển thị kết quả chi tiết

### Cách 2: Bỏ Qua Khởi Động Server

Nếu server đã chạy sẵn:

```powershell
.\api-tests\test.ps1 -SkipServer
```

### Cách 3: Quản Lý Server Thủ Công

```powershell
# Khởi động server
.\api-tests\scripts\manage_server.ps1 start

# Kiểm tra trạng thái
.\api-tests\scripts\manage_server.ps1 status

# Dừng server
.\api-tests\scripts\manage_server.ps1 stop
```

Sau đó chạy test ở terminal khác:
```powershell
.\api-tests\test.ps1 -SkipServer
```

### Cách 4: Chạy Trực Tiếp với Go

```powershell
cd api-tests
go test -v ./cases/...
```

## 📊 Kết Quả

Script sẽ hiển thị:
- Tổng số test cases
- Số test passed
- Số test failed
- Pass rate (%)

Báo cáo chi tiết được lưu trong `api-tests/reports/`.

## 🐛 Troubleshooting

### Server Không Khởi Động

- Kiểm tra MongoDB có đang chạy không
- Kiểm tra port 8080 có bị chiếm không
- Xem log trong `api/logs/app.log`

### Test Bị Lỗi Kết Nối

- Đảm bảo server đã sẵn sàng
- Kiểm tra health endpoint: `http://localhost:8080/api/v1/system/health`

## 📚 Tài Liệu Liên Quan

- [Tổng Quan Testing](tong-quan.md)
- [Viết Test Case](viet-test.md)
- [Báo Cáo Test](bao-cao-test.md)

