# Cấu Hình Môi Trường

Tài liệu chi tiết về các biến môi trường và cách cấu hình hệ thống.

## 📋 Tổng Quan

Hệ thống sử dụng file `.env` để quản lý cấu hình. File mẫu nằm tại `api/config/env/development.env`.

## 🔧 Các Biến Môi Trường

### Server Configuration

| Biến | Mô Tả | Mặc Định | Bắt Buộc |
|------|-------|----------|----------|
| `INITMODE` | Chế độ khởi tạo (true/false) | `true` | Không |
| `ADDRESS` | Port server lắng nghe | `8080` | Có |

**Ví dụ:**
```env
INITMODE=true
ADDRESS=8080
```

### JWT Configuration

| Biến | Mô Tả | Mặc Định | Bắt Buộc |
|------|-------|----------|----------|
| `JWT_SECRET` | Secret key để ký JWT token | - | Có |

**Lưu ý:**
- Phải là chuỗi ngẫu nhiên mạnh (ít nhất 32 ký tự)
- Không được chia sẻ hoặc commit vào git
- Production nên sử dụng secret key khác với development

**Ví dụ:**
```env
JWT_SECRET=your-very-long-and-random-secret-key-here
```

### MongoDB Configuration

| Biến | Mô Tả | Mặc Định | Bắt Buộc |
|------|-------|----------|----------|
| `MONGODB_CONNECTION_URI` | Connection string MongoDB | - | Có |
| `MONGODB_DBNAME_AUTH` | Tên database cho auth | `folkform_auth` | Có |
| `MONGODB_DBNAME_STAGING` | Tên database cho staging | `folkform_staging` | Không |
| `MONGODB_DBNAME_DATA` | Tên database cho data | `folkform_data` | Không |

**Ví dụ:**
```env
MONGODB_CONNECTION_URI=mongodb://localhost:27017
MONGODB_DBNAME_AUTH=folkform_auth
MONGODB_DBNAME_STAGING=folkform_staging
MONGODB_DBNAME_DATA=folkform_data
```

**Connection String Formats:**
- Local: `mongodb://localhost:27017`
- With authentication: `mongodb://username:password@localhost:27017`
- Replica set: `mongodb://host1:27017,host2:27017/?replicaSet=rs0`
- Atlas: `mongodb+srv://username:password@cluster.mongodb.net/`

### CORS Configuration

| Biến | Mô Tả | Mặc Định | Bắt Buộc |
|------|-------|----------|----------|
| `CORS_ORIGINS` | Danh sách origins được phép (phân cách bằng dấu phẩy) | `*` | Không |
| `CORS_ALLOW_CREDENTIALS` | Cho phép credentials (true/false) | `false` | Không |

**Development:**
```env
CORS_ORIGINS=*
CORS_ALLOW_CREDENTIALS=false
```

**Production:**
```env
CORS_ORIGINS=https://yourdomain.com,https://www.yourdomain.com
CORS_ALLOW_CREDENTIALS=true
```

### Rate Limiting Configuration

| Biến | Mô Tả | Mặc Định | Bắt Buộc |
|------|-------|----------|----------|
| `RATE_LIMIT_MAX` | Số request tối đa trong một window | `100` | Không |
| `RATE_LIMIT_WINDOW` | Thời gian window (giây) | `60` | Không |

**Ví dụ:**
```env
RATE_LIMIT_MAX=100
RATE_LIMIT_WINDOW=60
```

Điều này có nghĩa: cho phép tối đa 100 requests trong 60 giây.

### Firebase Configuration

| Biến | Mô Tả | Mặc Định | Bắt Buộc |
|------|-------|----------|----------|
| `FIREBASE_PROJECT_ID` | Firebase Project ID | - | Có |
| `FIREBASE_CREDENTIALS_PATH` | Đường dẫn đến service account JSON | `config/firebase/service-account.json` | Có |
| `FIREBASE_API_KEY` | Firebase Web API Key | - | Có |
| `FIREBASE_ADMIN_UID` | Firebase UID của admin (tùy chọn) | - | Không |

**Ví dụ:**
```env
FIREBASE_PROJECT_ID=meta-commerce-auth
FIREBASE_CREDENTIALS_PATH=config/firebase/service-account.json
FIREBASE_API_KEY=AIzaSyBZUQETl42lzd3TeytC9wZf-6rDbWJ3Zas
FIREBASE_ADMIN_UID=user-uid-here
```

**Lưu ý:**
- `FIREBASE_ADMIN_UID`: Nếu được set, user với UID này sẽ tự động trở thành admin khi khởi động server
- Nếu không set, user đầu tiên đăng nhập sẽ tự động trở thành admin

### Frontend Configuration

| Biến | Mô Tả | Mặc Định | Bắt Buộc |
|------|-------|----------|----------|
| `FRONTEND_URL` | URL của frontend (cho redirect) | `http://localhost:3000` | Không |

**Ví dụ:**
```env
FRONTEND_URL=http://localhost:3000
```

## 📝 File Cấu Hình Mẫu

File `api/config/env/development.env`:

```env
# Server Configuration
INITMODE=true
ADDRESS=8080

# JWT Configuration
JWT_SECRET=4661408x

# MongoDB Configuration
MONGODB_CONNECTION_URI=mongodb://localhost:27017
MONGODB_DBNAME_AUTH=folkform_auth
MONGODB_DBNAME_STAGING=folkform_staging
MONGODB_DBNAME_DATA=folkform_data

# CORS Configuration
CORS_ORIGINS=*
CORS_ALLOW_CREDENTIALS=false

# Rate Limiting Configuration
RATE_LIMIT_MAX=100
RATE_LIMIT_WINDOW=60

# Firebase Configuration
FIREBASE_PROJECT_ID=meta-commerce-auth
FIREBASE_CREDENTIALS_PATH=config/firebase/service-account.json
FIREBASE_API_KEY=AIzaSyBZUQETl42lzd3TeytC9wZf-6rDbWJ3Zas

# Frontend URL
FRONTEND_URL=http://localhost:3000
```

## 🔒 Bảo Mật

### Development vs Production

**Development:**
- Có thể sử dụng giá trị mặc định hoặc giá trị đơn giản
- File `.env` có thể commit vào git (nếu không chứa thông tin nhạy cảm)

**Production:**
- **KHÔNG BAO GIỜ** commit file `.env` chứa secret keys
- Sử dụng environment variables của hệ thống hoặc secret management service
- `JWT_SECRET` phải là chuỗi ngẫu nhiên mạnh (ít nhất 32 ký tự)
- `CORS_ORIGINS` phải chỉ định domain cụ thể, không dùng `*`
- Sử dụng MongoDB với authentication
- Sử dụng HTTPS

### Best Practices

1. **Tách biệt cấu hình:**
   - `development.env` - Development
   - `staging.env` - Staging
   - `production.env` - Production (không commit)

2. **Sử dụng .gitignore:**
```gitignore
# Environment files
*.env.local
*.env.production
config/env/production.env
```

3. **Secret Management:**
   - Sử dụng secret management service (AWS Secrets Manager, HashiCorp Vault, etc.)
   - Hoặc sử dụng environment variables của hệ điều hành

## 🔍 Kiểm Tra Cấu Hình

### Kiểm Tra Biến Môi Trường

Server sẽ log các cấu hình quan trọng khi khởi động. Kiểm tra log file `logs/app.log`:

```
[INFO] Server starting on port: 8080
[INFO] MongoDB connected: mongodb://localhost:27017
[INFO] Firebase initialized: meta-commerce-auth
```

### Validate Configuration

Các biến bắt buộc sẽ được validate khi server khởi động. Nếu thiếu, server sẽ không khởi động và hiển thị lỗi.

## 📚 Tài Liệu Liên Quan

- [Cài Đặt và Cấu Hình](cai-dat.md)
- [Khởi Tạo Hệ Thống](khoi-tao.md)
- [Hướng Dẫn Cài Đặt Firebase](../04-deployment/huong-dan-cai-dat-firebase.md)

