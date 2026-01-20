# XỬ LÝ TRÙNG LẶP TÀI KHOẢN KHI BỔ SUNG THÔNG TIN

Tài liệu này mô tả cách xử lý trường hợp user đăng ký email và số điện thoại độc lập thành 2 tài khoản riêng, sau đó bổ sung thông tin mới phát hiện ra trùng lặp.

---

## 1. VẤN ĐỀ

### 1.1. Scenario xảy ra

**Trường hợp 1:**
```
User A: Đăng ký bằng Email/Password
  - Email: user@example.com
  - Phone: (chưa có)

User B: Đăng nhập bằng Phone OTP
  - Email: (chưa có)
  - Phone: +84123456789

→ User A muốn bổ sung phone: +84123456789
→ Phát hiện phone đã được sử dụng bởi User B
```

**Trường hợp 2:**
```
User A: Đăng nhập bằng Phone OTP
  - Email: (chưa có)
  - Phone: +84123456789

User B: Đăng ký bằng Email/Password
  - Email: user@example.com
  - Phone: (chưa có)

→ User A muốn bổ sung email: user@example.com
→ Phát hiện email đã được sử dụng bởi User B
```

**Trường hợp 3:**
```
User A: Đăng nhập bằng Google OAuth
  - Email: user@gmail.com (từ Google)
  - Phone: (chưa có)

User B: Đăng nhập bằng Phone OTP
  - Email: (chưa có)
  - Phone: +84123456789

→ User A muốn bổ sung phone: +84123456789
→ Phát hiện phone đã được sử dụng bởi User B
```

---

## 2. PHƯƠNG ÁN XỬ LÝ

### 2.1. Phương án 1: Merge Tài khoản (Khuyến nghị)

**Ý tưởng:** Tự động merge 2 tài khoản thành 1

**Flow:**

```
User A muốn bổ sung phone: +84123456789
    ↓
Backend phát hiện phone đã được sử dụng bởi User B
    ↓
Backend merge User B vào User A:
    - Giữ User A (tài khoản hiện tại)
    - Copy thông tin từ User B sang User A
    - Xóa User B
    ↓
User A có đầy đủ thông tin từ cả 2 tài khoản
```

**Ưu điểm:**
- ✅ User không mất dữ liệu
- ✅ Tự động, không cần user can thiệp
- ✅ User experience tốt

**Nhược điểm:**
- ⚠️ Cần xử lý cẩn thận để không mất dữ liệu
- ⚠️ Cần merge các quan hệ (UserRole, etc.)

---

### 2.2. Phương án 2: Yêu cầu Xác nhận

**Ý tưởng:** Yêu cầu user xác nhận merge tài khoản

**Flow:**

```
User A muốn bổ sung phone: +84123456789
    ↓
Backend phát hiện phone đã được sử dụng bởi User B
    ↓
Backend trả về thông tin User B
    ↓
Frontend hiển thị dialog xác nhận:
    "Số điện thoại này đã được sử dụng bởi tài khoản khác.
     Bạn có muốn hợp nhất 2 tài khoản không?"
    ↓
User xác nhận → Merge tài khoản
User từ chối → Trả về lỗi
```

**Ưu điểm:**
- ✅ User có quyền quyết định
- ✅ An toàn hơn

**Nhược điểm:**
- ⚠️ User phải thao tác thêm
- ⚠️ Có thể gây confusion

---

### 2.3. Phương án 3: Từ chối và Yêu cầu Đăng nhập

**Ý tưởng:** Từ chối bổ sung, yêu cầu user đăng nhập bằng tài khoản kia

**Flow:**

```
User A muốn bổ sung phone: +84123456789
    ↓
Backend phát hiện phone đã được sử dụng bởi User B
    ↓
Backend trả về lỗi:
    "Số điện thoại này đã được sử dụng.
     Vui lòng đăng nhập bằng số điện thoại này,
     sau đó liên kết email vào tài khoản đó."
    ↓
User phải đăng nhập bằng phone → User B
Sau đó liên kết email từ User A vào User B
```

**Ưu điểm:**
- ✅ Đơn giản, không cần merge
- ✅ An toàn

**Nhược điểm:**
- ❌ User phải thao tác nhiều bước
- ❌ User experience không tốt
- ❌ Có thể gây confusion

---

## 3. PHƯƠNG ÁN ĐƯỢC CHỌN: MERGE TÀI KHOẢN (CÓ XÁC NHẬN)

### 3.1. Flow chi tiết

```
┌─────────┐
│ User A  │ (Đã đăng nhập, có JWT token)
└────┬────┘
     │ 1. User A muốn bổ sung phone: +84123456789
     │
     │ 2. Gửi request
     │    POST /api/v1/auth/phone/link
     │    Headers: { Authorization: "Bearer {jwt_token_userA}" }
     │    Body: {
     │      "idToken": "firebase_id_token",
     │      "phone": "+84123456789"
     │    }
     │
     ▼
┌─────────┐
│ Backend │
└────┬────┘
     │ 3. Verify JWT token → Lấy User A
     │
     │ 4. Verify ID token với Firebase
     │
     │ 5. Tìm user khác có phone: +84123456789
     │    filter = {
     │      phone: "+84123456789",
     │      _id: { "$ne": userA.ID }
     │    }
     │
     │ 6. Tìm thấy User B?
     │    ├─ NO → Bổ sung phone bình thường
     │    └─ YES → Phát hiện trùng lặp
     │
     │ 7. Kiểm tra User B có dữ liệu quan trọng không?
     │    - Có UserRole?
     │    - Có dữ liệu liên quan?
     │    - Có OAuth providers?
     │
     │ 8. Trả về thông tin User B và đề xuất merge
     │    {
     │      "conflict": true,
     │      "conflictType": "phone",
     │      "conflictValue": "+84123456789",
     │      "existingUser": {
     │        "id": "userB_id",
     │        "email": "userB@example.com",
     │        "phone": "+84123456789",
     │        "hasRoles": true,
     │        "hasOAuth": false
     │      },
     │      "mergeRequired": true
     │    }
     │
     ▼
┌─────────┐
│ Frontend│
└────┬────┘
     │ 9. Hiển thị dialog xác nhận merge
     │
     │    ┌─────────────────────────────────────┐
     │    │  PHÁT HIỆN TÀI KHOẢN TRÙNG LẶP      │
     │    ├─────────────────────────────────────┤
     │    │                                     │
     │    │  Số điện thoại +84123456789 đã      │
     │    │  được sử dụng bởi tài khoản khác.  │
     │    │                                     │
     │    │  Thông tin tài khoản đó:            │
     │    │  - Email: userB@example.com         │
     │    │  - Phone: +84123456789              │
     │    │                                     │
     │    │  Bạn có muốn hợp nhất 2 tài khoản   │
     │    │  thành 1 không?                     │
     │    │                                     │
     │    │  ⚠️ Lưu ý: Tài khoản cũ sẽ bị xóa   │
     │    │                                     │
     │    │  [Hủy]  [Hợp nhất tài khoản]       │
     │    └─────────────────────────────────────┘
     │
     │ 10. User click "Hợp nhất tài khoản"
     │
     │ 11. Gửi request merge
     │     POST /api/v1/auth/merge-account
     │     Headers: { Authorization: "Bearer {jwt_token_userA}" }
     │     Body: {
     │       "conflictType": "phone",
     │       "conflictValue": "+84123456789",
     │       "targetUserId": "userB_id",
     │       "confirm": true
     │     }
     │
     ▼
┌─────────┐
│ Backend │
└────┬────┘
     │ 12. Verify JWT token → Lấy User A
     │
     │ 13. Verify User B tồn tại và có phone trùng
     │
     │ 14. Merge User B vào User A:
     │     a. Copy thông tin từ User B:
     │        - Phone (nếu User A chưa có)
     │        - FirebaseUID (nếu User A chưa có)
     │        - OAuthProviders (merge vào array)
     │        - Name (nếu User A chưa có)
     │
     │     b. Merge các quan hệ:
     │        - UserRole: Update userId từ User B → User A
     │        - Các collection khác có reference đến User B
     │
     │     c. Xóa User B
     │
     │     d. Cập nhật User A với thông tin đã merge
     │
     │ 15. Trả về User A đã được merge
     │
     ▼
┌─────────┐
│ Frontend│
└─────────┘
     │ 16. Hiển thị thông báo "Đã hợp nhất tài khoản thành công"
     │
     │ 17. Refresh thông tin user
```

---

## 4. CODE IMPLEMENTATION

### 4.1. DTO: Conflict Detection Response

```go
// AccountConflictResponse response khi phát hiện trùng lặp
type AccountConflictResponse struct {
    Conflict      bool                   `json:"conflict"`       // Có trùng lặp không
    ConflictType  string                 `json:"conflictType"`  // "email" hoặc "phone"
    ConflictValue string                 `json:"conflictValue"` // Giá trị trùng (email hoặc phone)
    ExistingUser  ConflictUserInfo       `json:"existingUser"`  // Thông tin user trùng
    MergeRequired bool                   `json:"mergeRequired"` // Cần merge không
}

// ConflictUserInfo thông tin user bị trùng
type ConflictUserInfo struct {
    ID        string   `json:"id"`        // User ID
    Email     string   `json:"email"`      // Email (nếu có)
    Phone     string   `json:"phone"`      // Phone (nếu có)
    Name      string   `json:"name"`       // Tên
    HasRoles  bool     `json:"hasRoles"`   // Có roles không
    HasOAuth  bool     `json:"hasOAuth"`   // Có OAuth providers không
    CreatedAt int64    `json:"createdAt"`  // Thời gian tạo
}

// MergeAccountInput đầu vào merge tài khoản
type MergeAccountInput struct {
    ConflictType  string `json:"conflictType" validate:"required"`  // "email" hoặc "phone"
    ConflictValue string `json:"conflictValue" validate:"required"` // Email hoặc phone
    TargetUserID  string `json:"targetUserId" validate:"required"`  // ID của user cần merge
    Confirm       bool   `json:"confirm" validate:"required"`        // Xác nhận merge
}
```

### 4.2. Service: Detect Conflict

```go
// DetectAccountConflict phát hiện trùng lặp khi bổ sung thông tin
func (s *UserService) DetectAccountConflict(ctx context.Context, currentUserID primitive.ObjectID, conflictType, conflictValue string) (*dto.AccountConflictResponse, error) {
    // 1. Tìm user khác có email/phone trùng
    filter := bson.M{
        conflictType: conflictValue,
        "_id": bson.M{"$ne": currentUserID},
    }
    
    existingUser, err := s.BaseServiceMongoImpl.FindOne(ctx, filter, nil)
    if err != nil {
        if err == common.ErrNotFound {
            // Không có trùng lặp
            return &dto.AccountConflictResponse{
                Conflict: false,
            }, nil
        }
        return nil, err
    }
    
    // 2. Kiểm tra user có dữ liệu quan trọng không
    hasRoles, err := s.checkUserHasRoles(ctx, existingUser.ID)
    if err != nil {
        return nil, err
    }
    
    hasOAuth := len(existingUser.OAuthProviders) > 0
    
    // 3. Trả về thông tin conflict
    return &dto.AccountConflictResponse{
        Conflict:      true,
        ConflictType:  conflictType,
        ConflictValue: conflictValue,
        ExistingUser: dto.ConflictUserInfo{
            ID:        existingUser.ID.Hex(),
            Email:     existingUser.Email,
            Phone:     existingUser.Phone,
            Name:      existingUser.Name,
            HasRoles:  hasRoles,
            HasOAuth:  hasOAuth,
            CreatedAt: existingUser.CreatedAt,
        },
        MergeRequired: true, // Luôn đề xuất merge
    }, nil
}

// checkUserHasRoles kiểm tra user có roles không
func (s *UserService) checkUserHasRoles(ctx context.Context, userID primitive.ObjectID) (bool, error) {
    filter := bson.M{"userId": userID}
    count, err := s.userRoleService.CountDocuments(ctx, filter)
    if err != nil {
        return false, err
    }
    return count > 0, nil
}
```

### 4.3. Service: Merge Account

```go
// MergeAccount merge 2 tài khoản thành 1
func (s *UserService) MergeAccount(ctx context.Context, currentUserID primitive.ObjectID, input *dto.MergeAccountInput) (*models.User, error) {
    // 1. Verify input
    if !input.Confirm {
        return nil, common.NewError(
            common.ErrCodeValidation,
            "Cần xác nhận merge tài khoản",
            common.StatusBadRequest,
            nil,
        )
    }
    
    // 2. Lấy user hiện tại (User A - tài khoản chính)
    currentUser, err := s.BaseServiceMongoImpl.FindOneById(ctx, currentUserID)
    if err != nil {
        return nil, err
    }
    
    // 3. Lấy user cần merge (User B - tài khoản sẽ bị xóa)
    targetUserID, err := primitive.ObjectIDFromHex(input.TargetUserID)
    if err != nil {
        return nil, common.NewError(
            common.ErrCodeValidationFormat,
            "Invalid target user ID",
            common.StatusBadRequest,
            err,
        )
    }
    
    targetUser, err := s.BaseServiceMongoImpl.FindOneById(ctx, targetUserID)
    if err != nil {
        return nil, err
    }
    
    // 4. Verify conflict vẫn còn
    filter := bson.M{
        input.ConflictType: input.ConflictValue,
        "_id": targetUserID,
    }
    verifyUser, err := s.BaseServiceMongoImpl.FindOne(ctx, filter, nil)
    if err != nil || verifyUser == nil {
        return nil, common.NewError(
            common.ErrCodeValidation,
            "Conflict không còn tồn tại",
            common.StatusBadRequest,
            nil,
        )
    }
    
    // 5. Merge thông tin từ User B vào User A
    // 5.1. Merge Phone
    if currentUser.Phone == "" && targetUser.Phone != "" {
        currentUser.Phone = targetUser.Phone
        currentUser.PhoneVerified = targetUser.PhoneVerified
        currentUser.FirebaseUID = targetUser.FirebaseUID
    }
    
    // 5.2. Merge Email
    if currentUser.Email == "" && targetUser.Email != "" {
        currentUser.Email = targetUser.Email
        currentUser.EmailVerified = targetUser.EmailVerified
    }
    
    // 5.3. Merge Name
    if currentUser.Name == "" && targetUser.Name != "" {
        currentUser.Name = targetUser.Name
    }
    
    // 5.4. Merge OAuth Providers
    for _, targetProvider := range targetUser.OAuthProviders {
        // Kiểm tra provider đã có chưa
        exists := false
        for _, currentProvider := range currentUser.OAuthProviders {
            if currentProvider.ProviderType == targetProvider.ProviderType &&
               currentProvider.ProviderID == targetProvider.ProviderID {
                exists = true
                break
            }
        }
        
        // Nếu chưa có, thêm vào
        if !exists {
            currentUser.OAuthProviders = append(currentUser.OAuthProviders, targetProvider)
        }
    }
    
    // 5.5. Merge Password (nếu User A chưa có)
    if currentUser.Password == "" && targetUser.Password != "" {
        currentUser.Password = targetUser.Password
        currentUser.Salt = targetUser.Salt
    }
    
    // 6. Merge các quan hệ (UserRole, etc.)
    err = s.mergeUserRelations(ctx, currentUserID, targetUserID)
    if err != nil {
        return nil, fmt.Errorf("failed to merge user relations: %v", err)
    }
    
    // 7. Cập nhật User A
    currentUser.UpdatedAt = time.Now().Unix()
    updatedUser, err := s.BaseServiceMongoImpl.UpdateById(ctx, currentUserID, currentUser)
    if err != nil {
        return nil, err
    }
    
    // 8. Xóa User B
    err = s.BaseServiceMongoImpl.DeleteById(ctx, targetUserID)
    if err != nil {
        // Log error nhưng không fail vì đã merge xong
        logrus.Warnf("Failed to delete merged user %s: %v", targetUserID.Hex(), err)
    }
    
    return updatedUser, nil
}

// mergeUserRelations merge các quan hệ của user
func (s *UserService) mergeUserRelations(ctx context.Context, currentUserID, targetUserID primitive.ObjectID) error {
    // 1. Merge UserRole
    // Update tất cả UserRole có userId = targetUserID → currentUserID
    filter := bson.M{"userId": targetUserID}
    update := bson.M{"$set": bson.M{"userId": currentUserID}}
    
    _, err := s.userRoleService.collection.UpdateMany(ctx, filter, update)
    if err != nil {
        return fmt.Errorf("failed to merge user roles: %v", err)
    }
    
    // 2. Merge các quan hệ khác (nếu có)
    // Ví dụ: Orders, Transactions, etc.
    // Cần update theo từng collection cụ thể
    
    return nil
}
```

### 4.4. Handler: Detect Conflict

```go
// HandleDetectConflict phát hiện trùng lặp khi bổ sung thông tin
func (h *UserHandler) HandleDetectConflict(c fiber.Ctx) error {
    userID := c.Locals("user_id")
    if userID == nil {
        h.HandleResponse(c, nil, common.NewError(
            common.ErrCodeAuth,
            "User not authenticated",
            common.StatusUnauthorized,
            nil,
        ))
        return nil
    }
    
    conflictType := c.Query("type") // "email" hoặc "phone"
    conflictValue := c.Query("value") // Email hoặc phone
    
    if conflictType == "" || conflictValue == "" {
        h.HandleResponse(c, nil, common.NewError(
            common.ErrCodeValidation,
            "Missing conflict type or value",
            common.StatusBadRequest,
            nil,
        ))
        return nil
    }
    
    objID, err := primitive.ObjectIDFromHex(userID.(string))
    if err != nil {
        h.HandleResponse(c, nil, common.NewError(
            common.ErrCodeValidationFormat,
            "Invalid user ID",
            common.StatusBadRequest,
            err,
        ))
        return nil
    }
    
    conflict, err := h.userService.DetectAccountConflict(context.Background(), objID, conflictType, conflictValue)
    h.HandleResponse(c, conflict, err)
    return nil
}
```

### 4.5. Handler: Merge Account

```go
// HandleMergeAccount merge 2 tài khoản
func (h *UserHandler) HandleMergeAccount(c fiber.Ctx) error {
    userID := c.Locals("user_id")
    if userID == nil {
        h.HandleResponse(c, nil, common.NewError(
            common.ErrCodeAuth,
            "User not authenticated",
            common.StatusUnauthorized,
            nil,
        ))
        return nil
    }
    
    var input dto.MergeAccountInput
    if err := h.ParseRequestBody(c, &input); err != nil {
        h.HandleResponse(c, nil, err)
        return nil
    }
    
    objID, err := primitive.ObjectIDFromHex(userID.(string))
    if err != nil {
        h.HandleResponse(c, nil, common.NewError(
            common.ErrCodeValidationFormat,
            "Invalid user ID",
            common.StatusBadRequest,
            err,
        ))
        return nil
    }
    
    mergedUser, err := h.userService.MergeAccount(context.Background(), objID, &input)
    h.HandleResponse(c, mergedUser, err)
    return nil
}
```

### 4.6. Handler: Link Phone (Cập nhật)

```go
// HandlePhoneLink xử lý liên kết số điện thoại (có kiểm tra conflict)
func (h *PhoneHandler) HandlePhoneLink(c fiber.Ctx) error {
    userID := c.Locals("user_id")
    if userID == nil {
        h.HandleResponse(c, nil, common.NewError(
            common.ErrCodeAuth,
            "User not authenticated",
            common.StatusUnauthorized,
            nil,
        ))
        return nil
    }
    
    var input dto.PhoneLinkInput
    if err := h.ParseRequestBody(c, &input); err != nil {
        h.HandleResponse(c, nil, err)
        return nil
    }
    
    objID, err := primitive.ObjectIDFromHex(userID.(string))
    if err != nil {
        h.HandleResponse(c, nil, common.NewError(
            common.ErrCodeValidationFormat,
            "Invalid user ID",
            common.StatusBadRequest,
            err,
        ))
        return nil
    }
    
    // 1. Verify ID token với Firebase
    firebaseService := services.NewFirebaseService()
    token, err := firebaseService.VerifyIDToken(context.Background(), input.IDToken)
    if err != nil {
        h.HandleResponse(c, nil, common.NewError(
            common.ErrCodeAuthCredentials,
            "Token không hợp lệ",
            common.StatusUnauthorized,
            err,
        ))
        return nil
    }
    
    // 2. Kiểm tra conflict
    userService, _ := services.NewUserService()
    conflict, err := userService.DetectAccountConflict(context.Background(), objID, "phone", input.Phone)
    if err != nil {
        h.HandleResponse(c, nil, err)
        return nil
    }
    
    // 3. Nếu có conflict, trả về thông tin conflict
    if conflict.Conflict {
        h.HandleResponse(c, conflict, nil)
        return nil
    }
    
    // 4. Không có conflict, link phone bình thường
    err = userService.LinkPhone(context.Background(), objID, &input)
    h.HandleResponse(c, fiber.Map{
        "message": "Đã liên kết số điện thoại thành công",
    }, err)
    return nil
}
```

### 4.7. Handler: Link Email (Cập nhật)

```go
// HandleLinkEmail xử lý liên kết email (có kiểm tra conflict)
func (h *UserHandler) HandleLinkEmail(c fiber.Ctx) error {
    userID := c.Locals("user_id")
    if userID == nil {
        h.HandleResponse(c, nil, common.NewError(
            common.ErrCodeAuth,
            "User not authenticated",
            common.StatusUnauthorized,
            nil,
        ))
        return nil
    }
    
    var input dto.EmailLinkInput
    if err := h.ParseRequestBody(c, &input); err != nil {
        h.HandleResponse(c, nil, err)
        return nil
    }
    
    objID, err := primitive.ObjectIDFromHex(userID.(string))
    if err != nil {
        h.HandleResponse(c, nil, common.NewError(
            common.ErrCodeValidationFormat,
            "Invalid user ID",
            common.StatusBadRequest,
            err,
        ))
        return nil
    }
    
    // 1. Kiểm tra conflict
    conflict, err := h.userService.DetectAccountConflict(context.Background(), objID, "email", input.Email)
    if err != nil {
        h.HandleResponse(c, nil, err)
        return nil
    }
    
    // 2. Nếu có conflict, trả về thông tin conflict
    if conflict.Conflict {
        h.HandleResponse(c, conflict, nil)
        return nil
    }
    
    // 3. Không có conflict, link email bình thường
    err = h.userService.LinkEmail(context.Background(), objID, &input)
    h.HandleResponse(c, fiber.Map{
        "message": "Đã thêm email thành công. Vui lòng kiểm tra email để verify.",
    }, err)
    return nil
}
```

---

## 5. API ENDPOINTS

### 5.1. Detect Conflict

```
GET /api/v1/auth/conflict/detect?type=phone&value=+84123456789
Headers: { Authorization: "Bearer {jwt_token}" }
```

**Response (Có conflict):**
```json
{
  "message": "Thành công",
  "data": {
    "conflict": true,
    "conflictType": "phone",
    "conflictValue": "+84123456789",
    "existingUser": {
      "id": "507f1f77bcf86cd799439012",
      "email": "userB@example.com",
      "phone": "+84123456789",
      "name": "User B",
      "hasRoles": true,
      "hasOAuth": false,
      "createdAt": 1234567890
    },
    "mergeRequired": true
  }
}
```

**Response (Không có conflict):**
```json
{
  "message": "Thành công",
  "data": {
    "conflict": false
  }
}
```

### 5.2. Merge Account

```
POST /api/v1/auth/merge-account
Headers: { Authorization: "Bearer {jwt_token}" }
Body: {
  "conflictType": "phone",
  "conflictValue": "+84123456789",
  "targetUserId": "507f1f77bcf86cd799439012",
  "confirm": true
}
```

**Response:**
```json
{
  "message": "Đã hợp nhất tài khoản thành công",
  "data": {
    "id": "507f1f77bcf86cd799439011",
    "name": "User A",
    "email": "userA@example.com",
    "phone": "+84123456789",
    "phoneVerified": true,
    "oauthProviders": [
      {
        "providerType": "google",
        "providerId": "google_123"
      }
    ]
  }
}
```

---

## 6. FRONTEND IMPLEMENTATION

### 6.1. Component: Conflict Detection và Merge

```javascript
// MergeAccountDialog.jsx
import { useState } from 'react';
import { useAuth } from '@/hooks/useAuth';

export function MergeAccountDialog({ conflict, onConfirm, onCancel }) {
  const { token } = useAuth();
  const [loading, setLoading] = useState(false);
  
  const handleMerge = async () => {
    setLoading(true);
    
    try {
      const response = await fetch('/api/v1/auth/merge-account', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
          conflictType: conflict.conflictType,
          conflictValue: conflict.conflictValue,
          targetUserId: conflict.existingUser.id,
          confirm: true
        })
      });
      
      const data = await response.json();
      
      if (response.ok) {
        alert('Đã hợp nhất tài khoản thành công');
        onConfirm(data.data);
      } else {
        alert(data.message || 'Có lỗi xảy ra');
      }
    } catch (error) {
      alert('Có lỗi xảy ra');
    } finally {
      setLoading(false);
    }
  };
  
  return (
    <div className="merge-dialog">
      <h3>Phát hiện tài khoản trùng lặp</h3>
      
      <p>
        {conflict.conflictType === 'phone' 
          ? `Số điện thoại ${conflict.conflictValue}`
          : `Email ${conflict.conflictValue}`
        } đã được sử dụng bởi tài khoản khác.
      </p>
      
      <div className="existing-user-info">
        <h4>Thông tin tài khoản đó:</h4>
        <ul>
          {conflict.existingUser.email && (
            <li>Email: {conflict.existingUser.email}</li>
          )}
          {conflict.existingUser.phone && (
            <li>Số điện thoại: {conflict.existingUser.phone}</li>
          )}
          <li>Tên: {conflict.existingUser.name}</li>
          {conflict.existingUser.hasRoles && (
            <li>⚠️ Có vai trò và quyền</li>
          )}
          {conflict.existingUser.hasOAuth && (
            <li>⚠️ Có liên kết OAuth</li>
          )}
        </ul>
      </div>
      
      <div className="warning">
        <p>⚠️ Lưu ý: Tài khoản cũ sẽ bị xóa sau khi hợp nhất.</p>
        <p>Bạn có muốn hợp nhất 2 tài khoản thành 1 không?</p>
      </div>
      
      <div className="actions">
        <button onClick={onCancel} disabled={loading}>
          Hủy
        </button>
        <button onClick={handleMerge} disabled={loading}>
          {loading ? 'Đang xử lý...' : 'Hợp nhất tài khoản'}
        </button>
      </div>
    </div>
  );
}
```

### 6.2. Component: Link Phone (Cập nhật)

```javascript
// LinkPhoneForm.jsx (cập nhật)
export function LinkPhoneForm() {
  const { token } = useAuth();
  const [conflict, setConflict] = useState(null);
  const [showMergeDialog, setShowMergeDialog] = useState(false);
  
  const handleVerifyOTP = async (idToken, phone) => {
    try {
      const response = await fetch('/api/v1/auth/phone/link', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
          idToken,
          phone
        })
      });
      
      const data = await response.json();
      
      if (response.ok) {
        // Kiểm tra có conflict không
        if (data.data?.conflict) {
          setConflict(data.data);
          setShowMergeDialog(true);
        } else {
          alert('Đã thêm số điện thoại thành công');
          window.location.reload();
        }
      } else {
        alert(data.message || 'Có lỗi xảy ra');
      }
    } catch (error) {
      alert('Có lỗi xảy ra');
    }
  };
  
  const handleMergeConfirm = (mergedUser) => {
    setShowMergeDialog(false);
    setConflict(null);
    // Refresh user info
    window.location.reload();
  };
  
  return (
    <div>
      {/* Form nhập phone và OTP */}
      {/* ... */}
      
      {showMergeDialog && conflict && (
        <MergeAccountDialog
          conflict={conflict}
          onConfirm={handleMergeConfirm}
          onCancel={() => {
            setShowMergeDialog(false);
            setConflict(null);
          }}
        />
      )}
    </div>
  );
}
```

---

## 7. RULES VÀ VALIDATION

### 7.1. Rules Merge

1. **User A (tài khoản hiện tại) là tài khoản chính:**
   - Giữ lại User A
   - Merge thông tin từ User B vào User A
   - Xóa User B

2. **Merge thông tin:**
   - Phone: Nếu User A chưa có → Copy từ User B
   - Email: Nếu User A chưa có → Copy từ User B
   - OAuth Providers: Merge vào array (không trùng)
   - Name: Nếu User A chưa có → Copy từ User B
   - Password: Nếu User A chưa có → Copy từ User B

3. **Merge quan hệ:**
   - UserRole: Update userId từ User B → User A
   - Các collection khác: Update reference từ User B → User A

4. **Xóa User B:**
   - Sau khi merge xong
   - Xóa User B khỏi database

### 7.2. Validation

- **Verify conflict vẫn còn:**
  - Trước khi merge, verify lại conflict
  - Tránh race condition

- **Verify user có quyền merge:**
  - User phải đã đăng nhập (có JWT)
  - User phải là User A (tài khoản hiện tại)

- **Verify target user tồn tại:**
  - User B phải tồn tại
  - User B phải có email/phone trùng

---

## 8. EDGE CASES

### 8.1. User B có nhiều dữ liệu quan trọng

**Xử lý:**
- Hiển thị cảnh báo rõ ràng trong dialog
- Liệt kê các dữ liệu sẽ bị ảnh hưởng
- Yêu cầu user xác nhận kỹ

### 8.2. User B đang online

**Xử lý:**
- Có thể invalidate token của User B
- Hoặc yêu cầu User B logout trước
- Hoặc force logout User B khi merge

### 8.3. Merge thất bại giữa chừng

**Xử lý:**
- Sử dụng transaction (nếu MongoDB hỗ trợ)
- Hoặc rollback từng bước
- Log lỗi để debug

### 8.4. User B có nhiều OAuth providers

**Xử lý:**
- Merge tất cả OAuth providers vào User A
- Kiểm tra không trùng provider type

---

## 9. TÓM TẮT

### 9.1. Flow xử lý conflict

1. **Phát hiện conflict:**
   - User muốn bổ sung email/phone
   - Backend phát hiện email/phone đã được sử dụng
   - Trả về thông tin user trùng

2. **Hiển thị dialog:**
   - Frontend hiển thị thông tin user trùng
   - Yêu cầu user xác nhận merge

3. **Merge tài khoản:**
   - User xác nhận
   - Backend merge User B vào User A
   - Xóa User B
   - Trả về User A đã merge

### 9.2. API Endpoints

```
GET  /api/v1/auth/conflict/detect?type={email|phone}&value={value}
POST /api/v1/auth/merge-account
```

### 9.3. Lợi ích

- ✅ User không mất dữ liệu
- ✅ Tự động merge, user experience tốt
- ✅ Có xác nhận để an toàn
- ✅ Xử lý đầy đủ các quan hệ

---

**Phương án này đảm bảo xử lý tốt trường hợp trùng lặp tài khoản! 🎯**

