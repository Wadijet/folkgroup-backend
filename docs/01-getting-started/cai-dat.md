# Cài Đặt và Cấu Hình

Hướng dẫn chi tiết về cách cài đặt và cấu hình hệ thống FolkForm Auth Backend từ đầu.

## 📋 Yêu Cầu Hệ Thống

### Phần Mềm Cần Thiết

- **Go**: Phiên bản 1.23 trở lên
  - Tải về: https://golang.org/dl/
  - Kiểm tra: `go version`
  
- **MongoDB**: Phiên bản 4.4 trở lên
  - Tải về: https://www.mongodb.com/try/download/community
  - Hoặc sử dụng Docker: `docker run -d -p 27017:27017 mongo:latest`
  
- **Firebase Project**: Cần có Firebase project với Authentication enabled
  - Tạo project: https://console.firebase.google.com/
  - Xem hướng dẫn: [Hướng Dẫn Đăng Ký Firebase](../04-deployment/huong-dan-dang-ky-firebase.md)

### Hệ Điều Hành

- Windows 10/11
- Linux (Ubuntu 20.04+)
- macOS 10.15+

## 🚀 Cài Đặt

### Bước 1: Clone Repository

```bash
git clone <repository-url>
cd ff_be_auth
```

### Bước 2: Cài Đặt Dependencies

```bash
# Di chuyển vào thư mục api
cd api

# Tải dependencies
go mod download

# Hoặc sử dụng go mod tidy để tự động cập nhật
go mod tidy
```

### Bước 3: Cấu Hình MongoDB

1. **Khởi động MongoDB:**
```bash
# Windows (nếu cài đặt local)
mongod

# Linux/macOS
sudo systemctl start mongod
# hoặc
mongod --dbpath /path/to/data
```

2. **Kiểm tra kết nối:**
```bash
mongosh
# hoặc
mongo
```

### Bước 4: Cấu Hình Firebase

1. **Tạo Firebase Project:**
   - Truy cập https://console.firebase.google.com/
   - Tạo project mới hoặc chọn project có sẵn
   - Bật Authentication với các providers: Email/Password, Google, Facebook, Phone

2. **Tải Service Account Key:**
   - Vào Project Settings > Service Accounts
   - Click "Generate new private key"
   - Lưu file JSON vào `api/config/firebase/service-account.json`

3. **Lấy Firebase API Key:**
   - Vào Project Settings > General
   - Copy "Web API Key"

Xem chi tiết tại [Hướng Dẫn Cài Đặt Firebase](../04-deployment/huong-dan-cai-dat-firebase.md)

### Bước 5: Cấu Hình Môi Trường

1. **Copy file cấu hình mẫu:**
```bash
cd api/config/env
cp development.env development.env.local
```

2. **Chỉnh sửa file `development.env.local`:**

```env
# Server Configuration
INITMODE=true
ADDRESS=8080

# JWT Configuration
JWT_SECRET=your-secret-key-here-change-in-production

# MongoDB Configuration
MONGODB_CONNECTION_URI=mongodb://localhost:27017
MONGODB_DBNAME_AUTH=folkform_auth
MONGODB_DBNAME_STAGING=folkform_staging
MONGODB_DBNAME_DATA=folkform_data

# CORS Configuration
CORS_ORIGINS=*
CORS_ALLOW_CREDENTIALS=false

# Rate Limiting
RATE_LIMIT_MAX=100
RATE_LIMIT_WINDOW=60

# Firebase Configuration
FIREBASE_PROJECT_ID=your-project-id
FIREBASE_CREDENTIALS_PATH=config/firebase/service-account.json
FIREBASE_API_KEY=your-api-key

# Frontend URL
FRONTEND_URL=http://localhost:3000
```

**Lưu ý quan trọng:**
- Thay đổi `JWT_SECRET` thành một chuỗi ngẫu nhiên mạnh
- Cập nhật `FIREBASE_PROJECT_ID` và `FIREBASE_API_KEY` từ Firebase Console
- Đảm bảo đường dẫn `FIREBASE_CREDENTIALS_PATH` đúng với vị trí file service-account.json

### Bước 6: Chạy Server

```bash
# Từ thư mục api
go run cmd/server/main.go
```

Hoặc build và chạy:

```bash
# Build
go build -o server.exe cmd/server/main.go

# Chạy
./server.exe
```

Server sẽ khởi động tại `http://localhost:8080`

### Bước 7: Kiểm Tra

1. **Kiểm tra health endpoint:**
```bash
curl http://localhost:8080/api/v1/system/health
```

Kết quả mong đợi:
```json
{
  "status": "ok",
  "timestamp": "2024-01-01T00:00:00Z"
}
```

2. **Kiểm tra log:**
   - Log được ghi vào `api/logs/app.log`
   - Kiểm tra xem có lỗi nào không

## 🔧 Cấu Hình Nâng Cao

### Cấu Hình Logging

Log được cấu hình trong `cmd/server/main.go`. Mặc định:
- Log level: `Debug`
- Log file: `logs/app.log`
- Format: Text với timestamp và caller info

### Cấu Hình CORS

Trong file `.env`:
```env
# Cho phép tất cả origins (development)
CORS_ORIGINS=*

# Production: chỉ định domain cụ thể
CORS_ORIGINS=https://yourdomain.com,https://www.yourdomain.com
CORS_ALLOW_CREDENTIALS=true
```

### Cấu Hình Rate Limiting

```env
# Số request tối đa trong một window
RATE_LIMIT_MAX=100

# Thời gian window (giây)
RATE_LIMIT_WINDOW=60
```

## ✅ Xác Nhận Cài Đặt

Sau khi cài đặt, bạn nên:

1. ✅ Server khởi động thành công
2. ✅ Health endpoint trả về status "ok"
3. ✅ MongoDB kết nối thành công (kiểm tra log)
4. ✅ Firebase được khởi tạo (kiểm tra log)
5. ✅ Không có lỗi trong log file

## 🐛 Xử Lý Lỗi

### Lỗi Kết Nối MongoDB

```
Error: cannot connect to MongoDB
```

**Giải pháp:**
- Kiểm tra MongoDB có đang chạy không
- Kiểm tra `MONGODB_CONNECTION_URI` đúng chưa
- Kiểm tra firewall/network

### Lỗi Firebase

```
Error: Firebase initialization failed
```

**Giải pháp:**
- Kiểm tra file `service-account.json` có tồn tại không
- Kiểm tra `FIREBASE_PROJECT_ID` đúng chưa
- Kiểm tra quyền của service account

### Lỗi Port Đã Được Sử Dụng

```
Error: bind: address already in use
```

**Giải pháp:**
- Thay đổi `ADDRESS` trong file `.env`
- Hoặc dừng process đang sử dụng port đó

Xem thêm tại [Xử Lý Sự Cố](../07-troubleshooting/loi-thuong-gap.md)

## 📚 Tài Liệu Liên Quan

- [Cấu Hình Môi Trường](cau-hinh.md) - Chi tiết về biến môi trường
- [Khởi Tạo Hệ Thống](khoi-tao.md) - Quy trình khởi tạo hệ thống lần đầu
- [Hướng Dẫn Cài Đặt Firebase](../04-deployment/huong-dan-cai-dat-firebase.md)
- [Hướng Dẫn Đăng Ký Firebase](../04-deployment/huong-dan-dang-ky-firebase.md)

