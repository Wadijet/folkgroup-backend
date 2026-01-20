# PHƯƠNG ÁN MULTI-PROVIDER AUTHENTICATION

Tài liệu này mô tả chi tiết cách thiết kế hệ thống để **1 user có thể đăng nhập bằng nhiều cách** và **có thể bổ sung thêm phương thức login giữa chừng**.

---

## 1. TỔNG QUAN

### 1.1. Mục tiêu

- ✅ **1 user có thể đăng nhập bằng nhiều cách:**
  - Email/Password
  - Google OAuth
  - Facebook OAuth
  - Phone OTP (Firebase)
  
- ✅ **Có thể bổ sung thêm phương thức login sau:**
  - User đã có tài khoản Email/Password → Liên kết thêm Google/Facebook/Phone
  - User đã có tài khoản Google → Liên kết thêm Facebook/Phone/Email
  - User đã có tài khoản Phone → Liên kết thêm Email/Google/Facebook

- ✅ **Tự động liên kết nếu email/phone trùng:**
  - User A đăng ký bằng Email → User A đăng nhập bằng Google (cùng email) → Tự động liên kết
  - User B đăng ký bằng Phone → User B đăng nhập bằng Email (cùng phone) → Tự động liên kết

---

## 2. CẤU TRÚC DỮ LIỆU

### 2.1. User Model

```go
type User struct {
    // ... các trường hiện tại ...
    
    // Email/Password Authentication
    Email         string `json:"email,omitempty" bson:"email,omitempty" index:"unique,sparse"` // Email (unique, sparse)
    Password      string `json:"-" bson:"password,omitempty"`                                  // Password (optional)
    Salt          string `json:"-" bson:"salt,omitempty"`                                       // Salt (optional)
    EmailVerified bool   `json:"emailVerified" bson:"emailVerified"`                           // Email đã verify
    
    // Phone Authentication
    Phone         string `json:"phone,omitempty" bson:"phone,omitempty" index:"unique,sparse"` // Phone (unique, sparse)
    PhoneVerified bool   `json:"phoneVerified" bson:"phoneVerified"`                           // Phone đã verify
    FirebaseUID   string `json:"firebaseUid,omitempty" bson:"firebaseUid,omitempty"`          // Firebase User ID
    
    // OAuth Providers (danh sách các provider đã liên kết)
    OAuthProviders []OAuthProvider `json:"oauthProviders" bson:"oauthProviders"` // Danh sách providers
    
    // Metadata
    CreatedAt int64 `json:"createdAt" bson:"createdAt"`
    UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"`
}

// OAuthProvider lưu thông tin từng provider đã liên kết
type OAuthProvider struct {
    ProviderType string `json:"providerType" bson:"providerType"` // "google", "facebook"
    ProviderID   string `json:"providerId" bson:"providerId"`     // ID từ provider
    Email        string `json:"email,omitempty" bson:"email,omitempty"` // Email từ provider (nếu có)
    Name         string `json:"name" bson:"name"`                  // Tên từ provider
    AvatarURL    string `json:"avatarUrl,omitempty" bson:"avatarUrl,omitempty"` // Avatar URL
    LinkedAt     int64  `json:"linkedAt" bson:"linkedAt"`         // Thời gian liên kết
}
```

### 2.2. Đặc điểm thiết kế

- **Sparse Index**: Email và Phone dùng sparse index → Cho phép null, nhưng nếu có thì phải unique
- **Optional Fields**: Password, Salt, Email, Phone đều optional → User có thể không có password nếu chỉ dùng OAuth
- **OAuthProviders Array**: Lưu danh sách các provider đã liên kết → Có thể có nhiều provider

---

## 3. LOGIC XỬ LÝ - ĐĂNG NHẬP BẰNG PROVIDER MỚI

### 3.1. Flow tổng quát

```
User đăng nhập bằng Provider X
    ↓
Backend nhận thông tin từ Provider X
    ↓
Tìm user theo:
    1. ProviderID (nếu là OAuth)
    2. FirebaseUID (nếu là Phone)
    3. Email (nếu có)
    4. Phone (nếu có)
    ↓
Có tìm thấy user?
    ├─ YES → Cập nhật thông tin Provider X → Tạo JWT → Trả về
    └─ NO → Tạo user mới với Provider X → Tạo JWT → Trả về
```

### 3.2. Chi tiết từng trường hợp

#### Trường hợp 1: User đăng nhập bằng Google OAuth (lần đầu)

**Input:**
- Provider: Google
- Email từ Google: `user@gmail.com`
- Google ID: `google_123456`

**Logic:**
1. Tìm user theo:
   - `oauthProviders.providerId = "google_123456"` → Không tìm thấy
   - `email = "user@gmail.com"` → Tìm thấy user A (đã có email/password)

2. **Xử lý:**
   - User A đã tồn tại với email `user@gmail.com`
   - **Tự động liên kết Google vào user A**
   - Thêm Google vào `userA.OAuthProviders`
   - Cập nhật thông tin (name, avatar) nếu cần
   - Tạo JWT và trả về user A

**Kết quả:**
- User A có thể đăng nhập bằng:
  - ✅ Email/Password
  - ✅ Google OAuth

#### Trường hợp 2: User đăng nhập bằng Phone OTP (lần đầu)

**Input:**
- Provider: Phone
- Phone: `+84123456789`
- Firebase UID: `firebase_abc123`

**Logic:**
1. Tìm user theo:
   - `firebaseUid = "firebase_abc123"` → Không tìm thấy
   - `phone = "+84123456789"` → Không tìm thấy

2. **Xử lý:**
   - Không tìm thấy user nào
   - **Tạo user mới** với:
     - `phone = "+84123456789"`
     - `phoneVerified = true`
     - `firebaseUid = "firebase_abc123"`
   - Tạo JWT và trả về user mới

**Kết quả:**
- User mới chỉ có thể đăng nhập bằng Phone OTP

#### Trường hợp 3: User đăng nhập bằng Phone OTP (đã có email)

**Input:**
- Provider: Phone
- Phone: `+84123456789`
- Firebase UID: `firebase_abc123`
- User B đã có email `user@gmail.com`

**Logic:**
1. Tìm user theo:
   - `firebaseUid = "firebase_abc123"` → Không tìm thấy
   - `phone = "+84123456789"` → Không tìm thấy
   - (Không có email trong input nên không tìm theo email)

2. **Xử lý:**
   - Không tìm thấy user nào
   - **Tạo user mới** với phone
   - User mới và User B là 2 user khác nhau

**Lưu ý:** 
- Nếu muốn liên kết, user phải đăng nhập vào User B và gọi API link phone

#### Trường hợp 4: User đăng nhập bằng Google (đã có Phone)

**Input:**
- Provider: Google
- Email từ Google: `user@gmail.com`
- Google ID: `google_123456`
- User C đã có phone `+84123456789` (không có email)

**Logic:**
1. Tìm user theo:
   - `oauthProviders.providerId = "google_123456"` → Không tìm thấy
   - `email = "user@gmail.com"` → Không tìm thấy (User C không có email)

2. **Xử lý:**
   - Không tìm thấy user nào
   - **Tạo user mới** với Google
   - User mới và User C là 2 user khác nhau

**Lưu ý:**
- Nếu muốn liên kết, user phải đăng nhập vào User C và gọi API link Google

---

## 4. LOGIC XỬ LÝ - LIÊN KẾT PROVIDER SAU

### 4.1. Flow liên kết provider

```
User đã đăng nhập (có JWT token)
    ↓
User gọi API link provider (Google/Facebook/Phone)
    ↓
Backend verify JWT token → Lấy user hiện tại
    ↓
Backend verify provider token/credentials
    ↓
Kiểm tra provider đã được sử dụng chưa?
    ├─ YES → Trả về lỗi "Provider đã được sử dụng"
    └─ NO → Thêm provider vào user.OAuthProviders → Cập nhật user
```

### 4.2. Chi tiết từng trường hợp

#### Liên kết Google OAuth

**API:** `POST /api/v1/auth/oauth/google/link`

**Flow:**
1. User đã đăng nhập (có JWT)
2. Frontend redirect user đến Google OAuth
3. User xác thực với Google
4. Google redirect về callback với code
5. Backend đổi code lấy access token
6. Backend lấy thông tin user từ Google
7. Backend kiểm tra:
   - Google ID đã được sử dụng bởi user khác? → Lỗi
   - Email từ Google trùng với email user hiện tại? → OK, liên kết
   - Email từ Google trùng với email user khác? → Lỗi
8. Thêm Google vào `user.OAuthProviders`
9. Trả về thành công

#### Liên kết Phone OTP

**API:** `POST /api/v1/auth/phone/link`

**Input:**
```json
{
  "idToken": "firebase_id_token",
  "phone": "+84123456789"
}
```

**Flow:**
1. User đã đăng nhập (có JWT)
2. Frontend verify OTP với Firebase → Lấy ID token
3. Frontend gửi ID token đến backend
4. Backend verify ID token với Firebase
5. Backend kiểm tra:
   - Phone đã được sử dụng bởi user khác? → Lỗi
   - Phone trùng với phone user hiện tại? → OK, cập nhật
6. Cập nhật `user.Phone`, `user.PhoneVerified`, `user.FirebaseUID`
7. Trả về thành công

#### Liên kết Email/Password

**API:** `POST /api/v1/auth/email/link`

**Input:**
```json
{
  "email": "user@example.com",
  "password": "password123"
}
```

**Flow:**
1. User đã đăng nhập (có JWT)
2. Backend kiểm tra:
   - Email đã được sử dụng? → Lỗi
   - Email format hợp lệ? → OK
3. Hash password và lưu
4. Cập nhật `user.Email`, `user.Password`, `user.Salt`
5. Gửi email verification
6. Trả về thành công

---

## 5. CODE IMPLEMENTATION

### 5.1. Helper function: Tìm user theo nhiều điều kiện

```go
// FindUserByAnyIdentifier tìm user theo bất kỳ identifier nào
func (s *UserService) FindUserByAnyIdentifier(ctx context.Context, identifiers map[string]string) (*models.User, error) {
    // Tạo filter với OR conditions
    orConditions := []bson.M{}
    
    // Tìm theo OAuth Provider ID
    if providerID, ok := identifiers["providerId"]; ok && providerID != "" {
        orConditions = append(orConditions, bson.M{
            "oauthProviders.providerId": providerID,
        })
    }
    
    // Tìm theo Firebase UID
    if firebaseUID, ok := identifiers["firebaseUid"]; ok && firebaseUID != "" {
        orConditions = append(orConditions, bson.M{
            "firebaseUid": firebaseUID,
        })
    }
    
    // Tìm theo Email
    if email, ok := identifiers["email"]; ok && email != "" {
        orConditions = append(orConditions, bson.M{
            "email": email,
        })
    }
    
    // Tìm theo Phone
    if phone, ok := identifiers["phone"]; ok && phone != "" {
        orConditions = append(orConditions, bson.M{
            "phone": phone,
        })
    }
    
    if len(orConditions) == 0 {
        return nil, common.ErrNotFound
    }
    
    filter := bson.M{"$or": orConditions}
    user, err := s.BaseServiceMongoImpl.FindOne(ctx, filter, nil)
    return user, err
}
```

### 5.2. Login với OAuth - Tự động liên kết

```go
// LoginWithOAuth đăng nhập bằng OAuth (Google/Facebook)
func (s *UserService) LoginWithOAuth(ctx context.Context, providerType string, providerInfo *OAuthProviderInfo) (*models.User, error) {
    // 1. Tìm user theo nhiều điều kiện
    identifiers := map[string]string{
        "providerId": providerInfo.ProviderID,
        "email":     providerInfo.Email,
    }
    
    existingUser, err := s.FindUserByAnyIdentifier(ctx, identifiers)
    
    // 2. Nếu tìm thấy user
    if err == nil && existingUser != nil {
        // Kiểm tra provider đã có chưa
        providerExists := false
        for i, provider := range existingUser.OAuthProviders {
            if provider.ProviderType == providerType && provider.ProviderID == providerInfo.ProviderID {
                // Cập nhật thông tin provider
                existingUser.OAuthProviders[i] = models.OAuthProvider{
                    ProviderType: providerType,
                    ProviderID:   providerInfo.ProviderID,
                    Email:        providerInfo.Email,
                    Name:         providerInfo.Name,
                    AvatarURL:    providerInfo.AvatarURL,
                    LinkedAt:     time.Now().Unix(),
                }
                providerExists = true
                break
            }
        }
        
        // Nếu chưa có, thêm mới
        if !providerExists {
            existingUser.OAuthProviders = append(existingUser.OAuthProviders, models.OAuthProvider{
                ProviderType: providerType,
                ProviderID:   providerInfo.ProviderID,
                Email:        providerInfo.Email,
                Name:         providerInfo.Name,
                AvatarURL:    providerInfo.AvatarURL,
                LinkedAt:     time.Now().Unix(),
            })
        }
        
        // Cập nhật email nếu chưa có
        if existingUser.Email == "" && providerInfo.Email != "" {
            existingUser.Email = providerInfo.Email
            existingUser.EmailVerified = true // OAuth providers đã verify email
        }
        
        // Cập nhật name nếu chưa có
        if existingUser.Name == "" && providerInfo.Name != "" {
            existingUser.Name = providerInfo.Name
        }
        
        existingUser.UpdatedAt = time.Now().Unix()
        updatedUser, err := s.BaseServiceMongoImpl.UpdateById(ctx, existingUser.ID, existingUser)
        if err != nil {
            return nil, err
        }
        
        // Tạo JWT và trả về
        return s.createJWTAndUpdateUser(ctx, updatedUser, input.Hwid)
    }
    
    // 3. Nếu không tìm thấy, tạo user mới
    if err == common.ErrNotFound || existingUser == nil {
        newUser := &models.User{
            Name: providerInfo.Name,
            Email: providerInfo.Email,
            EmailVerified: true, // OAuth providers đã verify email
            OAuthProviders: []models.OAuthProvider{
                {
                    ProviderType: providerType,
                    ProviderID:   providerInfo.ProviderID,
                    Email:        providerInfo.Email,
                    Name:         providerInfo.Name,
                    AvatarURL:    providerInfo.AvatarURL,
                    LinkedAt:     time.Now().Unix(),
                },
            },
            IsBlock:   false,
            CreatedAt: time.Now().Unix(),
            UpdatedAt: time.Now().Unix(),
        }
        
        createdUser, err := s.BaseServiceMongoImpl.InsertOne(ctx, *newUser)
        if err != nil {
            return nil, err
        }
        
        // Tạo JWT và trả về
        return s.createJWTAndUpdateUser(ctx, &createdUser, input.Hwid)
    }
    
    return nil, err
}
```

### 5.3. Link Provider sau

```go
// LinkOAuthProvider liên kết OAuth provider với tài khoản hiện có
func (s *UserService) LinkOAuthProvider(ctx context.Context, userID primitive.ObjectID, providerType string, providerInfo *OAuthProviderInfo) error {
    // 1. Lấy user hiện tại
    user, err := s.BaseServiceMongoImpl.FindOneById(ctx, userID)
    if err != nil {
        return err
    }
    
    // 2. Kiểm tra provider đã được sử dụng bởi user khác chưa
    filter := bson.M{
        "oauthProviders.providerId": providerInfo.ProviderID,
        "_id": bson.M{"$ne": userID}, // Không phải user hiện tại
    }
    existingUser, err := s.BaseServiceMongoImpl.FindOne(ctx, filter, nil)
    if err == nil && existingUser != nil {
        return common.NewError(
            common.ErrCodeDuplicate,
            "Provider này đã được sử dụng bởi tài khoản khác",
            common.StatusConflict,
            nil,
        )
    }
    
    // 3. Kiểm tra email trùng (nếu có)
    if providerInfo.Email != "" && providerInfo.Email != user.Email {
        emailFilter := bson.M{
            "email": providerInfo.Email,
            "_id":   bson.M{"$ne": userID},
        }
        existingUser, err := s.BaseServiceMongoImpl.FindOne(ctx, emailFilter, nil)
        if err == nil && existingUser != nil {
            return common.NewError(
                common.ErrCodeDuplicate,
                "Email này đã được sử dụng bởi tài khoản khác",
                common.StatusConflict,
                nil,
            )
        }
    }
    
    // 4. Kiểm tra provider đã có trong user chưa
    providerExists := false
    for i, provider := range user.OAuthProviders {
        if provider.ProviderType == providerType {
            // Cập nhật provider
            user.OAuthProviders[i] = models.OAuthProvider{
                ProviderType: providerType,
                ProviderID:   providerInfo.ProviderID,
                Email:        providerInfo.Email,
                Name:         providerInfo.Name,
                AvatarURL:    providerInfo.AvatarURL,
                LinkedAt:     time.Now().Unix(),
            }
            providerExists = true
            break
        }
    }
    
    // 5. Nếu chưa có, thêm mới
    if !providerExists {
        user.OAuthProviders = append(user.OAuthProviders, models.OAuthProvider{
            ProviderType: providerType,
            ProviderID:   providerInfo.ProviderID,
            Email:        providerInfo.Email,
            Name:         providerInfo.Name,
            AvatarURL:    providerInfo.AvatarURL,
            LinkedAt:     time.Now().Unix(),
        })
    }
    
    // 6. Cập nhật email nếu chưa có
    if user.Email == "" && providerInfo.Email != "" {
        user.Email = providerInfo.Email
        user.EmailVerified = true
    }
    
    // 7. Cập nhật name nếu chưa có
    if user.Name == "" && providerInfo.Name != "" {
        user.Name = providerInfo.Name
    }
    
    user.UpdatedAt = time.Now().Unix()
    
    // 8. Lưu user
    _, err = s.BaseServiceMongoImpl.UpdateById(ctx, userID, user)
    return err
}
```

---

## 6. API ENDPOINTS

### 6.1. Đăng nhập bằng Provider

```
POST /api/v1/auth/login              - Email/Password
POST /api/v1/auth/oauth/google/callback   - Google OAuth (tự động liên kết)
POST /api/v1/auth/oauth/facebook/callback - Facebook OAuth (tự động liên kết)
POST /api/v1/auth/phone/verify      - Phone OTP (tự động liên kết)
```

### 6.2. Liên kết Provider sau

```
POST /api/v1/auth/oauth/google/link   - Liên kết Google (cần auth)
POST /api/v1/auth/oauth/facebook/link - Liên kết Facebook (cần auth)
POST /api/v1/auth/phone/link         - Liên kết Phone (cần auth)
POST /api/v1/auth/email/link         - Liên kết Email/Password (cần auth)
```

### 6.3. Quản lý Providers

```
GET  /api/v1/auth/providers          - Lấy danh sách providers đã liên kết (cần auth)
POST /api/v1/auth/providers/unlink/:provider - Hủy liên kết provider (cần auth)
```

---

## 7. VÍ DỤ SCENARIOS

### Scenario 1: User đăng ký Email → Liên kết Google → Liên kết Phone

**Bước 1:** User đăng ký bằng Email/Password
```
User: {
  email: "user@example.com",
  password: "hashed_password",
  oauthProviders: []
}
```

**Bước 2:** User đăng nhập bằng Google (cùng email)
```
Backend tự động liên kết:
User: {
  email: "user@example.com",
  password: "hashed_password",
  oauthProviders: [
    { providerType: "google", providerId: "google_123", email: "user@example.com" }
  ]
}
```

**Bước 3:** User liên kết Phone (đã đăng nhập)
```
User: {
  email: "user@example.com",
  password: "hashed_password",
  phone: "+84123456789",
  phoneVerified: true,
  firebaseUid: "firebase_abc",
  oauthProviders: [
    { providerType: "google", providerId: "google_123" }
  ]
}
```

**Kết quả:** User có thể đăng nhập bằng:
- ✅ Email/Password
- ✅ Google OAuth
- ✅ Phone OTP

### Scenario 2: User đăng nhập bằng Phone → Liên kết Email → Liên kết Facebook

**Bước 1:** User đăng nhập bằng Phone OTP
```
User: {
  phone: "+84123456789",
  phoneVerified: true,
  firebaseUid: "firebase_abc",
  oauthProviders: []
}
```

**Bước 2:** User liên kết Email/Password (đã đăng nhập)
```
User: {
  email: "user@example.com",
  password: "hashed_password",
  phone: "+84123456789",
  phoneVerified: true,
  firebaseUid: "firebase_abc",
  oauthProviders: []
}
```

**Bước 3:** User liên kết Facebook (đã đăng nhập)
```
User: {
  email: "user@example.com",
  password: "hashed_password",
  phone: "+84123456789",
  phoneVerified: true,
  firebaseUid: "firebase_abc",
  oauthProviders: [
    { providerType: "facebook", providerId: "fb_456", email: "user@example.com" }
  ]
}
```

**Kết quả:** User có thể đăng nhập bằng:
- ✅ Phone OTP
- ✅ Email/Password
- ✅ Facebook OAuth

---

## 8. RULES VÀ VALIDATION

### 8.1. Rules tự động liên kết

1. **Email trùng → Tự động liên kết:**
   - User A có email `user@gmail.com` (Email/Password)
   - User A đăng nhập bằng Google với email `user@gmail.com`
   - → Tự động liên kết Google vào User A

2. **Provider ID trùng → Tự động liên kết:**
   - User B đã đăng nhập bằng Google (providerId: `google_123`)
   - User B đăng nhập lại bằng Google
   - → Tự động tìm và đăng nhập User B

3. **Phone trùng → Tự động liên kết:**
   - User C có phone `+84123456789`
   - User C đăng nhập bằng Phone OTP với cùng số
   - → Tự động tìm và đăng nhập User C

### 8.2. Rules liên kết thủ công

1. **Provider đã được sử dụng → Lỗi:**
   - User D đã có Google (providerId: `google_123`)
   - User E cố gắng liên kết Google với providerId `google_123`
   - → Lỗi: "Provider đã được sử dụng"

2. **Email đã được sử dụng → Lỗi:**
   - User F có email `user@gmail.com`
   - User G cố gắng liên kết email `user@gmail.com`
   - → Lỗi: "Email đã được sử dụng"

3. **Phone đã được sử dụng → Lỗi:**
   - User H có phone `+84123456789`
   - User I cố gắng liên kết phone `+84123456789`
   - → Lỗi: "Phone đã được sử dụng"

### 8.3. Validation

- **Ít nhất 1 phương thức login:**
  - User phải có ít nhất 1 trong: Email/Password, OAuth Provider, hoặc Phone
  - Không cho phép xóa phương thức cuối cùng

- **Email unique (nếu có):**
  - Nếu user có email, email phải unique
  - Sparse index cho phép nhiều user không có email

- **Phone unique (nếu có):**
  - Nếu user có phone, phone phải unique
  - Sparse index cho phép nhiều user không có phone

---

## 9. FRONTEND INTEGRATION

### 9.1. Kiểm tra providers đã liên kết

```javascript
// Lấy danh sách providers
const response = await fetch('/api/v1/auth/providers', {
  headers: { 'Authorization': `Bearer ${token}` }
});

const { data } = await response.json();
// data.oauthProviders = [
//   { providerType: 'google', ... },
//   { providerType: 'facebook', ... }
// ]
// data.phone = '+84123456789'
// data.email = 'user@example.com'
```

### 9.2. Hiển thị UI

```javascript
// Hiển thị các phương thức login đã có
if (user.email) {
  showLoginOption('Email/Password');
}
if (user.oauthProviders.some(p => p.providerType === 'google')) {
  showLoginOption('Google');
}
if (user.phone) {
  showLoginOption('Phone OTP');
}

// Hiển thị nút liên kết cho các phương thức chưa có
if (!user.oauthProviders.some(p => p.providerType === 'google')) {
  showLinkButton('Link Google');
}
```

---

## 10. TÓM TẮT

### ✅ Tính năng chính:

1. **1 user có thể đăng nhập bằng nhiều cách**
   - Email/Password
   - Google OAuth
   - Facebook OAuth
   - Phone OTP

2. **Có thể bổ sung thêm phương thức login sau**
   - API link provider (cần auth)
   - Tự động validate và liên kết

3. **Tự động liên kết nếu trùng email/phone**
   - Email trùng → Tự động liên kết
   - Phone trùng → Tự động liên kết
   - Provider ID trùng → Tự động đăng nhập

4. **Quản lý providers**
   - Xem danh sách providers đã liên kết
   - Hủy liên kết provider (giữ lại ít nhất 1)

### 🎯 Lợi ích:

- ✅ User experience tốt: Không cần nhớ nhiều tài khoản
- ✅ Linh hoạt: Có thể thêm/bớt phương thức login
- ✅ An toàn: Validate và kiểm tra trùng lặp
- ✅ Dễ mở rộng: Có thể thêm provider mới dễ dàng

---

**Phương án này đảm bảo 1 user có thể dùng nhiều cách login và có thể bổ sung thêm giữa chừng! 🚀**

