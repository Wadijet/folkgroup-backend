# CÁC ID VÀ IDENTIFIER CỦA USER

Tài liệu này mô tả tất cả các ID và identifier có thể có cho một user trong hệ thống multi-provider authentication.

---

## 1. TỔNG QUAN CÁC ID

Một user có thể có các ID sau:

| Loại ID | Tên Field | Mô tả | Unique | Bắt buộc | Ví dụ |
|---------|-----------|-------|--------|----------|-------|
| **Internal ID** | `_id` | MongoDB ObjectID | ✅ | ✅ | `507f1f77bcf86cd799439011` |
| **Email** | `email` | Email address | ✅ (sparse) | ❌ | `user@example.com` |
| **Phone** | `phone` | Số điện thoại | ✅ (sparse) | ❌ | `+84123456789` |
| **Firebase UID** | `firebaseUid` | Firebase User ID | ✅ | ❌ | `firebase_abc123xyz` |
| **Google ID** | `oauthProviders[].providerId` | Google User ID | ✅ | ❌ | `google_123456789` |
| **Facebook ID** | `oauthProviders[].providerId` | Facebook User ID | ✅ | ❌ | `facebook_987654321` |

---

## 2. CHI TIẾT TỪNG ID

### 2.1. Internal ID (`_id`)

**Field:** `_id` (MongoDB ObjectID)

**Đặc điểm:**
- ✅ **Luôn có** - Tự động tạo khi tạo user
- ✅ **Unique** - MongoDB đảm bảo unique
- ✅ **Bất biến** - Không thể thay đổi
- ✅ **Primary Key** - Dùng để reference user

**Ví dụ:**
```go
ID: primitive.ObjectID("507f1f77bcf86cd799439011")
```

**Sử dụng:**
- Reference trong các collection khác (UserRole, etc.)
- Tìm user trong database
- JWT token payload

---

### 2.2. Email (`email`)

**Field:** `email`

**Đặc điểm:**
- ❌ **Optional** - User có thể không có email
- ✅ **Unique (nếu có)** - Sparse index đảm bảo unique khi có giá trị
- ✅ **Sparse Index** - Cho phép nhiều user không có email
- ✅ **Có thể thay đổi** - User có thể đổi email (cần verify)

**Ví dụ:**
```go
Email: "user@example.com"
// hoặc
Email: "" // Không có email
```

**Sử dụng:**
- Đăng nhập bằng Email/Password
- Tự động liên kết khi OAuth có cùng email
- Gửi email verification
- Tìm user: `{ email: "user@example.com" }`

**Validation:**
- Format: Email hợp lệ
- Unique: Không trùng với user khác (nếu có)

---

### 2.3. Phone (`phone`)

**Field:** `phone`

**Đặc điểm:**
- ❌ **Optional** - User có thể không có phone
- ✅ **Unique (nếu có)** - Sparse index đảm bảo unique khi có giá trị
- ✅ **Sparse Index** - Cho phép nhiều user không có phone
- ✅ **Có thể thay đổi** - User có thể đổi phone (cần verify OTP)

**Ví dụ:**
```go
Phone: "+84123456789"
// hoặc
Phone: "" // Không có phone
```

**Sử dụng:**
- Đăng nhập bằng Phone OTP
- Tự động liên kết khi có cùng phone
- Tìm user: `{ phone: "+84123456789" }`

**Validation:**
- Format: E.164 format (ví dụ: `+84123456789`)
- Unique: Không trùng với user khác (nếu có)

---

### 2.4. Firebase UID (`firebaseUid`)

**Field:** `firebaseUid`

**Đặc điểm:**
- ❌ **Optional** - Chỉ có khi user đăng nhập bằng Phone OTP
- ✅ **Unique** - Firebase đảm bảo unique
- ✅ **Bất biến** - Không thể thay đổi (từ Firebase)
- ✅ **1 user = 1 Firebase UID** - Mỗi user chỉ có 1 Firebase UID

**Ví dụ:**
```go
FirebaseUID: "firebase_abc123xyz789"
```

**Sử dụng:**
- Verify ID token từ Firebase
- Tìm user: `{ firebaseUid: "firebase_abc123xyz789" }`
- Liên kết Phone OTP với user

**Lưu ý:**
- Chỉ có khi user đã đăng nhập bằng Phone OTP ít nhất 1 lần
- Firebase UID được tạo bởi Firebase, không phải backend

---

### 2.5. OAuth Provider IDs

**Field:** `oauthProviders[].providerId`

**Đặc điểm:**
- ❌ **Optional** - User có thể không có OAuth provider
- ✅ **Unique** - Mỗi provider ID chỉ thuộc về 1 user
- ✅ **Array** - User có thể có nhiều OAuth providers
- ✅ **Bất biến** - Provider ID từ OAuth provider, không thể thay đổi

**Cấu trúc:**
```go
OAuthProviders: []OAuthProvider{
    {
        ProviderType: "google",
        ProviderID:   "google_123456789",
        Email:        "user@gmail.com",
        Name:         "Nguyen Van A",
        AvatarURL:    "https://...",
        LinkedAt:     1234567890,
    },
    {
        ProviderType: "facebook",
        ProviderID:   "facebook_987654321",
        Email:        "user@facebook.com",
        Name:         "Nguyen Van A",
        AvatarURL:    "https://...",
        LinkedAt:     1234567891,
    },
}
```

**Ví dụ:**
```go
// Google ID
ProviderID: "google_123456789"

// Facebook ID
ProviderID: "facebook_987654321"
```

**Sử dụng:**
- Đăng nhập bằng OAuth
- Tìm user: `{ "oauthProviders.providerId": "google_123456789" }`
- Liên kết nhiều OAuth providers

**Validation:**
- Unique: Mỗi provider ID chỉ thuộc về 1 user
- Provider Type: "google" hoặc "facebook"

---

## 3. CÁCH TÌM USER THEO CÁC ID

### 3.1. Tìm theo Internal ID

```go
// Tìm user theo MongoDB ObjectID
user, err := userService.FindOneById(ctx, objectID)
```

**Khi nào dùng:**
- Đã biết chính xác user ID
- Từ JWT token payload
- Reference từ collection khác

---

### 3.2. Tìm theo Email

```go
// Tìm user theo email
filter := bson.M{"email": "user@example.com"}
user, err := userService.FindOne(ctx, filter, nil)
```

**Khi nào dùng:**
- Đăng nhập bằng Email/Password
- Tự động liên kết OAuth (nếu email trùng)
- Kiểm tra email đã tồn tại chưa

---

### 3.3. Tìm theo Phone

```go
// Tìm user theo phone
filter := bson.M{"phone": "+84123456789"}
user, err := userService.FindOne(ctx, filter, nil)
```

**Khi nào dùng:**
- Đăng nhập bằng Phone OTP
- Tự động liên kết Phone
- Kiểm tra phone đã tồn tại chưa

---

### 3.4. Tìm theo Firebase UID

```go
// Tìm user theo Firebase UID
filter := bson.M{"firebaseUid": "firebase_abc123"}
user, err := userService.FindOne(ctx, filter, nil)
```

**Khi nào dùng:**
- Đăng nhập bằng Phone OTP
- Verify ID token từ Firebase
- Liên kết Phone với user

---

### 3.5. Tìm theo OAuth Provider ID

```go
// Tìm user theo Google ID
filter := bson.M{
    "oauthProviders.providerId": "google_123456789",
    "oauthProviders.providerType": "google",
}
user, err := userService.FindOne(ctx, filter, nil)
```

**Khi nào dùng:**
- Đăng nhập bằng OAuth
- Kiểm tra provider đã được sử dụng chưa
- Liên kết OAuth provider

---

### 3.6. Tìm theo nhiều điều kiện (OR)

```go
// Tìm user theo bất kỳ identifier nào
filter := bson.M{
    "$or": []bson.M{
        {"email": "user@example.com"},
        {"phone": "+84123456789"},
        {"firebaseUid": "firebase_abc123"},
        {"oauthProviders.providerId": "google_123456789"},
    },
}
user, err := userService.FindOne(ctx, filter, nil)
```

**Khi nào dùng:**
- Đăng nhập bằng provider mới (tự động liên kết)
- Tìm user khi không biết chính xác identifier nào

---

## 4. VÍ DỤ USER VỚI NHIỀU ID

### Ví dụ 1: User có đầy đủ tất cả ID

```go
User{
    ID:        primitive.ObjectID("507f1f77bcf86cd799439011"), // Internal ID
    Email:     "user@example.com",                              // Email
    Phone:     "+84123456789",                                  // Phone
    FirebaseUID: "firebase_abc123xyz",                         // Firebase UID
    OAuthProviders: []OAuthProvider{
        {
            ProviderType: "google",
            ProviderID:   "google_123456789",                   // Google ID
        },
        {
            ProviderType: "facebook",
            ProviderID:   "facebook_987654321",                  // Facebook ID
        },
    },
}
```

**Có thể tìm user bằng:**
- ✅ `_id = "507f1f77bcf86cd799439011"`
- ✅ `email = "user@example.com"`
- ✅ `phone = "+84123456789"`
- ✅ `firebaseUid = "firebase_abc123xyz"`
- ✅ `oauthProviders.providerId = "google_123456789"`
- ✅ `oauthProviders.providerId = "facebook_987654321"`

---

### Ví dụ 2: User chỉ có Email/Password

```go
User{
    ID:        primitive.ObjectID("507f1f77bcf86cd799439012"),
    Email:     "user2@example.com",
    Password:  "hashed_password",
    Phone:     "",                                              // Không có
    FirebaseUID: "",                                            // Không có
    OAuthProviders: []OAuthProvider{},                          // Không có
}
```

**Có thể tìm user bằng:**
- ✅ `_id = "507f1f77bcf86cd799439012"`
- ✅ `email = "user2@example.com"`

---

### Ví dụ 3: User chỉ có Phone OTP

```go
User{
    ID:        primitive.ObjectID("507f1f77bcf86cd799439013"),
    Email:     "",                                              // Không có
    Phone:     "+84987654321",
    FirebaseUID: "firebase_def456uvw",                         // Có Firebase UID
    OAuthProviders: []OAuthProvider{},                          // Không có
}
```

**Có thể tìm user bằng:**
- ✅ `_id = "507f1f77bcf86cd799439013"`
- ✅ `phone = "+84987654321"`
- ✅ `firebaseUid = "firebase_def456uvw"`

---

### Ví dụ 4: User chỉ có Google OAuth

```go
User{
    ID:        primitive.ObjectID("507f1f77bcf86cd799439014"),
    Email:     "user4@gmail.com",                               // Email từ Google
    Phone:     "",                                              // Không có
    FirebaseUID: "",                                            // Không có
    OAuthProviders: []OAuthProvider{
        {
            ProviderType: "google",
            ProviderID:   "google_999888777",                   // Google ID
        },
    },
}
```

**Có thể tìm user bằng:**
- ✅ `_id = "507f1f77bcf86cd799439014"`
- ✅ `email = "user4@gmail.com"`
- ✅ `oauthProviders.providerId = "google_999888777"`

---

## 5. QUAN HỆ GIỮA CÁC ID

### 5.1. 1 Internal ID = 1 User

- Mỗi user có **1 và chỉ 1** Internal ID
- Internal ID là primary key, không thể thay đổi

### 5.2. 1 Email = 1 User (nếu có)

- Nếu user có email, email phải unique
- Nhiều user có thể không có email (sparse index)

### 5.3. 1 Phone = 1 User (nếu có)

- Nếu user có phone, phone phải unique
- Nhiều user có thể không có phone (sparse index)

### 5.4. 1 Firebase UID = 1 User (nếu có)

- Nếu user có Firebase UID, Firebase UID phải unique
- Nhiều user có thể không có Firebase UID

### 5.5. 1 Provider ID = 1 User

- Mỗi OAuth Provider ID chỉ thuộc về 1 user
- 1 user có thể có nhiều Provider IDs (từ nhiều providers)

---

## 6. STRATEGY TÌM USER

### 6.1. Khi đăng nhập bằng Email/Password

```go
// Chỉ tìm theo email
filter := bson.M{"email": input.Email}
user, err := userService.FindOne(ctx, filter, nil)
```

---

### 6.2. Khi đăng nhập bằng OAuth

```go
// Tìm theo Provider ID hoặc Email (tự động liên kết)
filter := bson.M{
    "$or": []bson.M{
        {"oauthProviders.providerId": providerID},
        {"email": emailFromProvider},
    },
}
user, err := userService.FindOne(ctx, filter, nil)
```

**Logic:**
1. Tìm theo Provider ID → Nếu có → Đăng nhập user đó
2. Nếu không tìm thấy → Tìm theo Email
3. Nếu tìm thấy theo Email → Tự động liên kết Provider
4. Nếu không tìm thấy → Tạo user mới

---

### 6.3. Khi đăng nhập bằng Phone OTP

```go
// Tìm theo Firebase UID hoặc Phone
filter := bson.M{
    "$or": []bson.M{
        {"firebaseUid": firebaseUID},
        {"phone": phoneNumber},
    },
}
user, err := userService.FindOne(ctx, filter, nil)
```

**Logic:**
1. Tìm theo Firebase UID → Nếu có → Đăng nhập user đó
2. Nếu không tìm thấy → Tìm theo Phone
3. Nếu tìm thấy theo Phone → Cập nhật Firebase UID
4. Nếu không tìm thấy → Tạo user mới

---

## 7. TÓM TẮT

### Các ID của User:

1. **Internal ID** (`_id`)
   - Luôn có, unique, bất biến
   - Primary key

2. **Email** (`email`)
   - Optional, unique (nếu có), có thể thay đổi
   - Dùng cho Email/Password login

3. **Phone** (`phone`)
   - Optional, unique (nếu có), có thể thay đổi
   - Dùng cho Phone OTP login

4. **Firebase UID** (`firebaseUid`)
   - Optional, unique (nếu có), bất biến
   - Dùng cho Phone OTP login

5. **OAuth Provider IDs** (`oauthProviders[].providerId`)
   - Optional, unique, bất biến
   - Dùng cho OAuth login
   - Có thể có nhiều (Google, Facebook, etc.)

### Quy tắc:

- ✅ **1 user = 1 Internal ID** (luôn có)
- ✅ **1 user = 0 hoặc 1 Email** (nếu có thì unique)
- ✅ **1 user = 0 hoặc 1 Phone** (nếu có thì unique)
- ✅ **1 user = 0 hoặc 1 Firebase UID** (nếu có thì unique)
- ✅ **1 user = 0 hoặc nhiều Provider IDs** (mỗi ID unique)

### Tìm user:

- **Theo Internal ID**: Khi đã biết chính xác user
- **Theo Email**: Đăng nhập Email/Password, tự động liên kết OAuth
- **Theo Phone**: Đăng nhập Phone OTP
- **Theo Firebase UID**: Đăng nhập Phone OTP
- **Theo Provider ID**: Đăng nhập OAuth
- **Theo nhiều điều kiện (OR)**: Tự động liên kết khi đăng nhập bằng provider mới

---

**Tất cả các ID này giúp hệ thống linh hoạt trong việc tìm và liên kết user! 🎯**

