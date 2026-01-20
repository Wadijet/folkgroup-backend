# Authentication Flow

Tài liệu chi tiết về luồng xác thực trong hệ thống FolkForm Auth Backend.

## 📋 Tổng Quan

Hệ thống sử dụng **Firebase Authentication** để xác thực người dùng, sau đó tạo **JWT token** của hệ thống để sử dụng cho các API requests.

## 🔄 Luồng Xác Thực

### 1. Frontend: User Đăng Nhập

User đăng nhập bằng Firebase SDK với một trong các phương thức:
- Email/Password
- Google OAuth
- Facebook OAuth
- Phone OTP

```javascript
// Ví dụ: Đăng nhập bằng Email/Password
import { signInWithEmailAndPassword } from 'firebase/auth';

const userCredential = await signInWithEmailAndPassword(auth, email, password);
const idToken = await userCredential.user.getIdToken();
```

### 2. Frontend: Gửi ID Token Đến Backend

```javascript
const response = await fetch('http://localhost:8080/api/v1/auth/login/firebase', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    idToken: idToken,
    hwid: 'device-hardware-id' // Optional
  })
});

const data = await response.json();
const jwtToken = data.data.token; // JWT token của hệ thống
```

### 3. Backend: Verify Firebase ID Token

**Vị trí:** `api/core/api/services/service.auth.user.go`

```go
func LoginWithFirebase(idToken string, hwid string) (*User, string, error) {
    // 1. Verify Firebase ID token
    firebaseToken, err := firebase.VerifyIDToken(idToken)
    if err != nil {
        return nil, "", errors.New("Invalid Firebase token")
    }
    
    // 2. Lấy Firebase UID
    firebaseUID := firebaseToken.UID
    
    // 3. Tìm hoặc tạo user trong MongoDB
    user, err := findOrCreateUser(firebaseUID, firebaseToken)
    
    // 4. Tạo JWT token của hệ thống
    jwtToken, err := jwt.GenerateToken(user)
    
    return user, jwtToken, nil
}
```

### 4. Backend: Tìm Hoặc Tạo User

```go
func findOrCreateUser(firebaseUID string, firebaseToken *firebase.Token) (*User, error) {
    // Tìm user theo firebaseUid
    user, err := userRepo.FindByFirebaseUID(firebaseUID)
    
    if err == nil && user != nil {
        // User đã tồn tại, cập nhật thông tin từ Firebase
        updateUserFromFirebase(user, firebaseToken)
        return user, nil
    }
    
    // User chưa tồn tại, tạo mới
    user = &User{
        FirebaseUID: firebaseUID,
        Email: firebaseToken.Email,
        EmailVerified: firebaseToken.EmailVerified,
        Phone: firebaseToken.PhoneNumber,
        PhoneVerified: firebaseToken.PhoneNumberVerified,
        AvatarURL: firebaseToken.Picture,
        // ... các fields khác
    }
    
    // Kiểm tra nếu là user đầu tiên → tự động làm admin
    if isFirstUser() {
        user.Roles = []string{"administrator-role-id"}
    }
    
    return userRepo.Create(user)
}
```

### 5. Backend: Tạo JWT Token

**Vị trí:** `api/core/utility/jwt.go`

```go
func GenerateToken(user *User) (string, error) {
    claims := jwt.MapClaims{
        "userId": user.ID,
        "firebaseUid": user.FirebaseUID,
        "email": user.Email,
        "roles": user.Roles,
        "exp": time.Now().Add(24 * time.Hour).Unix(),
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(jwtSecret))
}
```

### 6. Frontend: Lưu JWT Token

```javascript
// Lưu token vào localStorage hoặc secure storage
localStorage.setItem('jwt_token', jwtToken);

// Sử dụng token cho các request tiếp theo
fetch('http://localhost:8080/api/v1/user/profile', {
  headers: {
    'Authorization': `Bearer ${jwtToken}`
  }
});
```

## 🔐 Sử Dụng JWT Token

### Middleware Authentication

**Vị trí:** `api/core/api/middleware/middleware.auth.go`

```go
func AuthMiddleware(c fiber.Ctx) error {
    // 1. Lấy token từ header
    token := c.Get("Authorization")
    if token == "" {
        return c.Status(401).JSON(fiber.Map{
            "error": "Unauthorized",
        })
    }
    
    // 2. Verify JWT token
    claims, err := jwt.VerifyToken(token)
    if err != nil {
        return c.Status(401).JSON(fiber.Map{
            "error": "Invalid token",
        })
    }
    
    // 3. Lưu user info vào context
    c.Locals("userId", claims["userId"])
    c.Locals("firebaseUid", claims["firebaseUid"])
    c.Locals("roles", claims["roles"])
    
    return c.Next()
}
```

### Sử Dụng Trong Handler

```go
func GetProfile(c fiber.Ctx) error {
    // Lấy user ID từ context
    userId := c.Locals("userId").(string)
    
    // Gọi service
    user, err := userService.GetByID(userId)
    if err != nil {
        return c.Status(404).JSON(fiber.Map{
            "error": "User not found",
        })
    }
    
    return c.JSON(fiber.Map{
        "data": user,
    })
}
```

## 🔄 Refresh Token

Hiện tại hệ thống chưa có refresh token mechanism. JWT token có thời hạn 24 giờ.

**Kế hoạch tương lai:**
- Implement refresh token
- Short-lived access token (15 phút)
- Long-lived refresh token (7 ngày)

## 🚪 Logout

### Endpoint

```http
POST /api/v1/auth/logout
Headers: Authorization: Bearer <token>
```

### Xử Lý

```go
func Logout(c fiber.Ctx) error {
    userId := c.Locals("userId").(string)
    
    // Xóa token khỏi cache (nếu có)
    // Hoặc đánh dấu token là invalid
    
    return c.JSON(fiber.Map{
        "message": "Logged out successfully",
    })
}
```

**Lưu ý:** Với JWT stateless, logout chỉ có thể xóa token ở client. Để logout thực sự, cần implement token blacklist hoặc sử dụng refresh token.

## 🔒 Bảo Mật

### Firebase ID Token

- Firebase ID token có thời hạn 1 giờ
- Tự động refresh bởi Firebase SDK
- Backend verify token với Firebase mỗi lần login

### JWT Token

- Secret key được lưu trong biến môi trường
- Token có thời hạn 24 giờ
- Token chứa thông tin user (không chứa password)

### Best Practices

1. **Không lưu token trong localStorage** (nếu có thể, dùng httpOnly cookie)
2. **HTTPS trong production**
3. **Validate token ở mọi protected endpoint**
4. **Rate limiting cho login endpoint**
5. **Log các lần đăng nhập để phát hiện bất thường**

## 📚 Tài Liệu Liên Quan

- [Firebase Authentication với Database](../firebase-auth-voi-database.md)
- [Multi-Provider Authentication](../multi-provider-authentication.md)
- [RBAC System](rbac.md)
- [Tổng Quan Kiến Trúc](tong-quan.md)

