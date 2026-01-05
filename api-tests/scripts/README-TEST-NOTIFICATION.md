# Hướng Dẫn Test Hệ Thống Notification

## Tổng Quan

Hệ thống notification có 2 cách gửi thông báo:

1. **Hệ thống 1 - Direct Send** (`/delivery/send`): Gửi trực tiếp không cần routing rules
2. **Hệ thống 2 - Trigger với Routing** (`/notification/trigger`): Gửi qua routing rules và templates

## Scripts Test

### 1. `test-delivery-send.ps1` ✅ HOẠT ĐỘNG TỐT

**Mô tả**: Test gửi notification trực tiếp qua endpoint `/delivery/send`

**Cách sử dụng**:
```powershell
.\scripts\test-delivery-send.ps1
```

**Kết quả**: 
- ✅ Đã test thành công với Telegram channel
- ✅ Notification đã được queue và gửi đi
- ⚠️ Email sender không active nên không test được email

**Yêu cầu**:
- Token hợp lệ (đã có sẵn trong script)
- Telegram sender active
- Chat ID hợp lệ

### 2. `test-notification-trigger-full.ps1` ⚠️ CẦN KIỂM TRA

**Mô tả**: Test gửi notification qua endpoint `/notification/trigger` với kiểm tra đầy đủ

**Cách sử dụng**:
```powershell
.\scripts\test-notification-trigger-full.ps1
```

**Kết quả hiện tại**:
- ✅ Routing rules match với Telegram channel organization
- ✅ Channel có ChatIDs: `-5139196836`
- ✅ Templates có sẵn cho eventType `system_error`
- ⚠️ Không có notification nào được queue (queued = 0)

**Nguyên nhân có thể**:
1. Router không tìm thấy channels khi query bằng `FindByOrganizationID`
2. Có lỗi trong quá trình tìm template
3. Có lỗi trong quá trình render template

**Cần kiểm tra**:
- Logs của server khi trigger notification
- Xem router có tìm thấy routes không
- Xem có lỗi nào trong quá trình xử lý không

### 3. `test-notification-with-token.ps1`

**Mô tả**: Script test cơ bản với endpoint `/notification/trigger`

**Cách sử dụng**:
```powershell
.\scripts\test-notification-with-token.ps1
```

## Cấu Hình Hiện Tại

### Channels
- **Telegram Channel**: 
  - OwnerOrganizationID: `695aa015c122aac1e4cd28aa`
  - ChatIDs: `-5139196836`
  - Status: Active ✅

### Routing Rules
- **System Events** (system_error, system_warning, etc.):
  - OrganizationIDs: `695aa015c122aac1e4cd28aa` ✅
  - ChannelTypes: `email, telegram, webhook` ✅
  - IsActive: `True` ✅

### Templates
- Có 12 Telegram templates cho các system events ✅
- Templates có sẵn cho eventType `system_error` ✅

## Kết Luận

1. **Endpoint `/delivery/send` hoạt động tốt** - Có thể dùng để gửi notification trực tiếp
2. **Endpoint `/notification/trigger` cần kiểm tra** - Routing rules và templates đều có, nhưng không queue được notification

## Gợi Ý Debug

1. Kiểm tra logs của server khi trigger notification
2. Thêm debug logs vào router để xem có tìm thấy routes không
3. Kiểm tra xem `FindByOrganizationID` có trả về channels không
4. Kiểm tra xem template có được tìm thấy và render thành công không
