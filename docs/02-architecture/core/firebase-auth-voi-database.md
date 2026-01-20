# FIREBASE AUTHENTICATION VỚI DATABASE CỦA BẠN

Tài liệu này giải thích cách sử dụng Firebase Authentication hoàn toàn và vẫn lưu user trên database của bạn.

---

## 1. FIREBASE AUTHENTICATION CHỈ QUẢN LÝ AUTHENTICATION

### 1.1. Firebase Authentication làm gì?

Firebase Authentication **CHỈ** quản lý:
- ✅ Xác thực người dùng (Email/Password, Google, Facebook, Phone OTP)
- ✅ Account linking tự động
- ✅ Session management
- ✅ Token generation (ID token)

Firebase Authentication **KHÔNG** lưu:
- ❌ Profile data (name, avatar, settings)
- ❌ Business data (orders, transactions)
- ❌ Custom fields
- ❌ Relationships với các collection khác

---

## 2. VẪN CẦN LƯU USER TRÊN DATABASE CỦA BẠN

### 2.1. Tại sao vẫn cần database?

Bạn vẫn cần lưu user trên database của bạn để:

1. **Lưu Profile Data:**
   - Name, avatar, settings
   - Preferences, language
   - Custom fields

2. **Lưu Business Data:**
   - Orders, transactions
   - Cart, wishlist
   - User roles, permissions

3. **Relationships:**
   - UserRole, UserPermission
   - Orders, Transactions
   - Các collection khác có reference đến user

4. **JWT Token Management:**
   - Lưu JWT token cho backend
   - Quản lý tokens theo device (hwid)
   - Token refresh logic

---

## 3. KIẾN TRÚC: FIREBASE AUTH + MONGODB

### 3.1. Flow tổng quát

```
┌─────────────┐
│   Frontend  │
└──────┬──────┘
       │
       │ 1. User đăng nhập bằng Email/Google/Phone
       │    → Firebase Authentication SDK
       │
       ▼
┌─────────────┐
│   Firebase  │
│ Authentication│
└──────┬──────┘
       │
       │ 2. Firebase xử lý authentication
       │    → Trả về Firebase ID token
       │
       ▼
┌─────────────┐
│   Frontend  │
└──────┬──────┘
       │
       │ 3. Gửi Firebase ID token đến Backend
       │    POST /api/v1/auth/login
       │    { "idToken": "firebase_id_token", "hwid": "..." }
       │
       ▼
┌─────────────┐
│   Backend   │
└──────┬──────┘
       │
       │ 4. Verify Firebase ID token
       │    → Lấy Firebase UID
       │
       │ 5. Tìm user trong MongoDB theo Firebase UID
       │    filter = { firebaseUid: "firebase_uid" }
       │
       │ 6. Nếu không tìm thấy:
       │    → Tạo user mới trong MongoDB
       │    → Lưu Firebase UID
       │
       │ 7. Tạo JWT token cho backend
       │
       │ 8. Trả về user và JWT token
       │
       ▼
┌─────────────┐
│   MongoDB   │
│  (Your DB)  │
└─────────────┘
```

---

## 4. CẤU TRÚC USER TRONG MONGODB

### 4.1. User Model (Đơn giản hơn)

```go
type User struct {
    ID        primitive.ObjectID `json:"id" bson:"_id"`
    
    // Firebase UID (Primary key để link với Firebase)
    FirebaseUID string `json:"firebaseUid" bson:"firebaseUid" index:"unique"`
    
    // Profile Data (từ Firebase hoặc custom)
    Name      string `json:"name" bson:"name"`
    Email     string `json:"email" bson:"email"`           // Sync từ Firebase
    Phone     string `json:"phone" bson:"phone"`           // Sync từ Firebase
    AvatarURL string `json:"avatarUrl" bson:"avatarUrl"`
    
    // Verification Status (từ Firebase)
    EmailVerified bool `json:"emailVerified" bson:"emailVerified"`
    PhoneVerified bool `json:"phoneVerified" bson:"phoneVerified"`
    
    // JWT Token Management (cho backend)
    Token  string        `json:"token" bson:"token"`
    Tokens []models.Token `json:"tokens" bson:"tokens"`
    
    // Business Data
    IsBlock bool `json:"isBlock" bson:"isBlock"`
    
    // Metadata
    CreatedAt int64 `json:"createdAt" bson:"createdAt"`
    UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"`
}
```

**Đặc điểm:**
- ✅ **FirebaseUID là unique** → Primary key để link với Firebase
- ✅ **Không cần OAuthProviders array** → Firebase quản lý
- ✅ **Không cần Password/Salt** → Firebase quản lý
- ✅ **Vẫn có JWT token** → Cho backend authentication

---

## 5. CODE IMPLEMENTATION

### 5.1. Service: Login với Firebase

```go
// LoginWithFirebase đăng nhập bằng Firebase ID token
func (s *UserService) LoginWithFirebase(ctx context.Context, idToken string, hwid string) (*models.User, error) {
    // 1. Verify Firebase ID token
    firebaseService := services.NewFirebaseService()
    token, err := firebaseService.VerifyIDToken(ctx, idToken)
    if err != nil {
        return nil, common.NewError(
            common.ErrCodeAuthCredentials,
            "Token không hợp lệ",
            common.StatusUnauthorized,
            err,
        )
    }
    
    // 2. Lấy thông tin user từ Firebase
    firebaseUser, err := firebaseService.GetUser(ctx, token.UID)
    if err != nil {
        return nil, err
    }
    
    // 3. Tìm user trong MongoDB theo Firebase UID
    filter := bson.M{"firebaseUid": token.UID}
    user, err := s.BaseServiceMongoImpl.FindOne(ctx, filter, nil)
    
    // 4. Nếu không tìm thấy, tạo user mới
    if err == common.ErrNotFound || user == nil {
        newUser := &models.User{
            FirebaseUID:    token.UID,
            Email:          firebaseUser.Email,
            EmailVerified:  firebaseUser.EmailVerified,
            Phone:          firebaseUser.PhoneNumber,
            PhoneVerified:  firebaseUser.PhoneNumber != "",
            Name:           firebaseUser.DisplayName,
            AvatarURL:      firebaseUser.PhotoURL,
            IsBlock:        false,
            Tokens:         []models.Token{},
            CreatedAt:      time.Now().Unix(),
            UpdatedAt:      time.Now().Unix(),
        }
        
        user, err = s.BaseServiceMongoImpl.InsertOne(ctx, *newUser)
        if err != nil {
            return nil, err
        }
    } else if err != nil {
        return nil, err
    } else {
        // 5. Nếu tìm thấy, sync thông tin từ Firebase (nếu có thay đổi)
        updated := false
        
        if user.Email != firebaseUser.Email {
            user.Email = firebaseUser.Email
            updated = true
        }
        
        if user.EmailVerified != firebaseUser.EmailVerified {
            user.EmailVerified = firebaseUser.EmailVerified
            updated = true
        }
        
        if user.Phone != firebaseUser.PhoneNumber {
            user.Phone = firebaseUser.PhoneNumber
            user.PhoneVerified = firebaseUser.PhoneNumber != ""
            updated = true
        }
        
        if user.Name != firebaseUser.DisplayName && firebaseUser.DisplayName != "" {
            user.Name = firebaseUser.DisplayName
            updated = true
        }
        
        if user.AvatarURL != firebaseUser.PhotoURL && firebaseUser.PhotoURL != "" {
            user.AvatarURL = firebaseUser.PhotoURL
            updated = true
        }
        
        if updated {
            user.UpdatedAt = time.Now().Unix()
            user, err = s.BaseServiceMongoImpl.UpdateById(ctx, user.ID, user)
            if err != nil {
                return nil, err
            }
        }
    }
    
    // 6. Kiểm tra user bị block
    if user.IsBlock {
        return nil, common.NewError(
            common.ErrCodeUserBlocked,
            "Tài khoản đã bị khóa",
            common.StatusForbidden,
            nil,
        )
    }
    
    // 7. Tạo JWT token cho backend
    rdNumber := rand.Intn(100)
    currentTime := time.Now().Unix()
    
    tokenMap, err := utility.CreateToken(
        global.MongoDB_ServerConfig.JwtSecret,
        user.ID.Hex(),
        strconv.FormatInt(currentTime, 16),
        strconv.Itoa(rdNumber),
    )
    if err != nil {
        return nil, err
    }
    
    // 8. Cập nhật token vào user
    user.Token = tokenMap["token"]
    
    // Cập nhật hoặc thêm token vào tokens array (theo hwid)
    var idTokenExist int = -1
    for i, _token := range user.Tokens {
        if _token.Hwid == hwid {
            idTokenExist = i
            break
        }
    }
    
    if idTokenExist == -1 {
        user.Tokens = append(user.Tokens, models.Token{
            Hwid:     hwid,
            JwtToken: tokenMap["token"],
        })
    } else {
        user.Tokens[idTokenExist].JwtToken = tokenMap["token"]
    }
    
    // 9. Lưu user
    updatedUser, err := s.BaseServiceMongoImpl.UpdateById(ctx, user.ID, user)
    if err != nil {
        return nil, err
    }
    
    return updatedUser, nil
}
```

### 5.2. Handler: Login với Firebase

```go
// HandleLoginWithFirebase xử lý đăng nhập bằng Firebase
func (h *AuthHandler) HandleLoginWithFirebase(c fiber.Ctx) error {
    var input dto.FirebaseLoginInput
    if err := h.ParseRequestBody(c, &input); err != nil {
        h.HandleResponse(c, nil, err)
        return nil
    }
    
    userService, _ := services.NewUserService()
    user, err := userService.LoginWithFirebase(context.Background(), input.IDToken, input.Hwid)
    
    h.HandleResponse(c, user, err)
    return nil
}
```

### 5.3. DTO: Firebase Login Input

```go
// FirebaseLoginInput đầu vào đăng nhập bằng Firebase
type FirebaseLoginInput struct {
    IDToken string `json:"idToken" validate:"required"` // Firebase ID token
    Hwid    string `json:"hwid" validate:"required"`     // Device hardware ID
}
```

---

## 6. SO SÁNH VỚI PHƯƠNG ÁN HIỆN TẠI

### 6.1. Phương án hiện tại (Tự quản lý)

**User Model:**
```go
type User struct {
    Email         string
    Password      string  // Hash password
    Salt          string
    Phone         string
    FirebaseUID   string  // Chỉ cho Phone OTP
    OAuthProviders []OAuthProvider  // Tự quản lý
    // ...
}
```

**Phức tạp:**
- ⚠️ Phải quản lý password hashing
- ⚠️ Phải quản lý OAuth providers
- ⚠️ Phải xử lý account linking
- ⚠️ Phải xử lý merge logic

---

### 6.2. Phương án Firebase Authentication hoàn toàn

**User Model:**
```go
type User struct {
    FirebaseUID   string  // Primary key
    Email         string  // Sync từ Firebase
    Phone         string  // Sync từ Firebase
    Name          string
    // Không cần Password, Salt, OAuthProviders
    // ...
}
```

**Đơn giản:**
- ✅ Firebase quản lý password
- ✅ Firebase quản lý OAuth providers
- ✅ Firebase tự động account linking
- ✅ Không cần merge logic

---

## 7. LỢI ÍCH VÀ HẠN CHẾ

### 7.1. Lợi ích

1. **Đơn giản hơn:**
   - Không cần quản lý password hashing
   - Không cần quản lý OAuth providers
   - Không cần account linking logic
   - Không cần merge logic

2. **Bảo mật tốt:**
   - Firebase xử lý bảo mật tốt
   - Account linking tự động và an toàn
   - Session management tốt

3. **User Experience:**
   - Firebase xử lý UX tốt
   - Account linking tự động
   - Không cần user thao tác nhiều

4. **Vẫn có control:**
   - Vẫn lưu user trên database của bạn
   - Vẫn có JWT token cho backend
   - Vẫn có business data

---

### 7.2. Hạn chế

1. **Phụ thuộc Firebase:**
   - Phụ thuộc vào Firebase Authentication
   - Khó tách ra sau này
   - Chi phí (nếu vượt free tier)

2. **Vẫn cần database:**
   - Vẫn phải lưu user trên MongoDB
   - Vẫn phải sync thông tin từ Firebase
   - Vẫn phải quản lý JWT token

3. **Custom logic:**
   - Khó customize authentication flow
   - Phải follow Firebase patterns

---

## 8. MIGRATION PATH

### 8.1. Nếu đã có user trong database

**Bước 1: Thêm FirebaseUID vào User model**
```go
type User struct {
    // ... existing fields ...
    FirebaseUID string `json:"firebaseUid" bson:"firebaseUid" index:"unique,sparse"`
}
```

**Bước 2: Migrate existing users**
```go
// Script migration
func MigrateUsersToFirebase() {
    // 1. Lấy tất cả users chưa có FirebaseUID
    users := getUsersWithoutFirebaseUID()
    
    // 2. Với mỗi user:
    for _, user := range users {
        // 2.1. Tạo user trong Firebase (nếu có email/password)
        if user.Email != "" && user.Password != "" {
            firebaseUser := createFirebaseUser(user.Email, user.Password)
            user.FirebaseUID = firebaseUser.UID
        }
        
        // 2.2. Hoặc tạo user với phone (nếu có phone)
        if user.Phone != "" {
            firebaseUser := createFirebaseUserWithPhone(user.Phone)
            user.FirebaseUID = firebaseUser.UID
        }
        
        // 2.3. Update user trong MongoDB
        updateUser(user)
    }
}
```

**Bước 3: Update login flow**
- Thay đổi login endpoints để sử dụng Firebase
- Giữ backward compatibility nếu cần

---

## 9. API ENDPOINTS

### 9.1. Login với Firebase

```
POST /api/v1/auth/login/firebase
Body: {
  "idToken": "firebase_id_token",
  "hwid": "device_hwid"
}
```

**Response:**
```json
{
  "message": "Đăng nhập thành công",
  "data": {
    "id": "507f1f77bcf86cd799439011",
    "firebaseUid": "firebase_abc123",
    "name": "Nguyen Van A",
    "email": "user@example.com",
    "emailVerified": true,
    "phone": "+84123456789",
    "phoneVerified": true,
    "token": "jwt_token_here"
  }
}
```

---

## 10. TÓM TẮT

### ✅ Vẫn cần lưu user trên database của bạn vì:

1. **Profile Data:** Name, avatar, settings
2. **Business Data:** Orders, transactions, cart
3. **Relationships:** UserRole, UserPermission, etc.
4. **JWT Token:** Quản lý token cho backend

### ✅ Firebase Authentication chỉ quản lý:

1. **Authentication:** Email/Password, Google, Facebook, Phone OTP
2. **Account Linking:** Tự động liên kết các providers
3. **Session Management:** Quản lý session
4. **Token Generation:** Firebase ID token

### ✅ Kiến trúc:

```
Firebase Authentication (Auth only)
    ↓
Firebase UID (Link key)
    ↓
MongoDB User (Profile + Business data)
```

### ✅ Lợi ích:

- ✅ Đơn giản hơn nhiều
- ✅ Firebase xử lý authentication phức tạp
- ✅ Vẫn có control trên database
- ✅ Vẫn có JWT token cho backend

---

**Firebase Authentication + MongoDB = Đơn giản + Linh hoạt! 🎯**

