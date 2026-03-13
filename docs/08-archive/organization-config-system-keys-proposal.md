# Đề xuất config keys cho System Organization (root)

Config đặt ở tổ chức **System (root)** sẽ được merge xuống các tổ chức con qua **GetResolvedConfig**. Key có `allowOverride: false` thì cấp dưới không ghi đè.

---

## 1. Múi giờ & locale

| Key | dataType | value mặc định | constraints | allowOverride | Mô tả |
|-----|----------|-----------------|-------------|---------------|--------|
| `timezone` | string | `"Asia/Ho_Chi_Minh"` | `{"enum":["Asia/Ho_Chi_Minh","UTC","Asia/Bangkok","Asia/Singapore","Europe/London"]}` | true | Múi giờ hiển thị và xử lý thời gian |
| `locale` | string | `"vi"` | `{"enum":["vi","en"]}` | true | Ngôn ngữ giao diện / thông báo |
| `dateFormat` | string | `"DD/MM/YYYY"` | `{"enum":["DD/MM/YYYY","YYYY-MM-DD","MM/DD/YYYY"]}` | true | Định dạng ngày hiển thị |

---

## 2. Giới hạn & hiệu năng

| Key | dataType | value mặc định | constraints | allowOverride | Mô tả |
|-----|----------|-----------------|-------------|---------------|--------|
| `paginationDefaultLimit` | number | `10` | `{"minimum":1,"maximum":100}` | true | Số bản ghi mặc định mỗi trang (find/paginate) |
| `paginationMaxLimit` | number | `1000` | `{"minimum":100,"maximum":2000}` | false | Giới hạn tối đa limit để tránh query quá nặng |
| `aiWorkflowStepTimeout` | number | `300` | `{"minimum":60,"maximum":900}` | true | Timeout mặc định mỗi step AI (giây) |
| `aiCommandStuckTimeout` | number | `300` | `{"minimum":60,"maximum":600}` | true | Timeout coi command AI là stuck (giây) |
| `aiClaimCommandsMaxLimit` | number | `100` | `{"minimum":1,"maximum":100}` | true | Số command tối đa mỗi lần claim |

---

## 3. Notification & gửi tin

| Key | dataType | value mặc định | constraints | allowOverride | Mô tả |
|-----|----------|-----------------|-------------|---------------|--------|
| `notificationDefaultChannelTypes` | array | `["email","telegram"]` | `{"minItems":0,"maxItems":5}` | true | Kênh thông báo mặc định khi tạo routing |
| `deliveryRetryMaxAttempts` | number | `3` | `{"minimum":1,"maximum":10}` | true | Số lần retry gửi tin thất bại |
| `rateLimitPerMinute` | number | `60` | `{"minimum":10,"maximum":500}` | false | Rate limit API (request/phút) nếu dùng chung |

---

## 4. Nội dung & approval

| Key | dataType | value mặc định | constraints | allowOverride | Mô tả |
|-----|----------|-----------------|-------------|---------------|--------|
| `contentDraftAutoArchiveDays` | number | `30` | `{"minimum":1,"maximum":365}` | true | Số ngày draft không hoạt động thì có thể auto-archive |
| `contentApprovalRequiredLevels` | string | `"L6,L7"` | `{"pattern":"^(L[1-8](,L[1-8])*)?$","maxLength":50}` | true | Các level bắt buộc approval (ví dụ L6 script, L7 video) |
| `contentMaxDepth` | number | `8` | `{"minimum":5,"maximum":10}` | false | Độ sâu tối đa cây content (L1–L8) |

---

## 5. Tích hợp (Facebook, Pancake, AI)

| Key | dataType | value mặc định | constraints | allowOverride | Mô tả |
|-----|----------|-----------------|-------------|---------------|--------|
| `facebookWebhookVerifyTokenMaxLength` | number | `128` | `{"minimum":32,"maximum":256}` | true | Độ dài tối đa verify token webhook Facebook |
| `pancakeWebhookSecretEnabled` | boolean | `true` | — | true | Bật kiểm tra secret webhook Pancake |
| `aiDefaultMaxTokens` | number | `2000` | `{"minimum":500,"maximum":8000}` | true | Max tokens mặc định cho AI (khi template không set) |
| `aiDefaultTemperature` | number | `0.7` | `{"minimum":0,"maximum":2}` | true | Temperature mặc định cho AI |

---

## 6. Bảo mật & quản trị

| Key | dataType | value mặc định | constraints | allowOverride | Mô tả |
|-----|----------|-----------------|-------------|---------------|--------|
| `sessionIdleTimeoutMinutes` | number | `60` | `{"minimum":15,"maximum":480}` | false | Thời gian idle trước khi session hết hạn (phút) |
| `passwordMinLength` | number | `8` | `{"minimum":6,"maximum":32}` | false | Độ dài tối thiểu mật khẩu (nếu dùng auth nội bộ) |
| `orgConfigKeysAllowList` | string | `""` | `{"maxLength":500}` | false | Danh sách key config cho phép (rỗng = tất cả). Format: `key1,key2` |

---

## 7. Cách seed config System Org

- Gọi API **POST /api/v1/organization-config/upsert-one** với `ownerOrganizationId` = ID của System Organization (root).
- Mỗi key trên tạo **một request** (hoặc script/migration gọi lần lượt).
- Set **IsSystem** = true khi tạo từ init script (backend set `IsSystem = false` từ API thường; cần init script hoặc admin tool để tạo item system).
- **Lưu ý:** Hiện API upsert set `IsSystem = false`. Để có config item **IsSystem = true** cho System Org cần:
  - Script init (insert trực tiếp vào DB), hoặc
  - Thêm endpoint/flag nội bộ cho phép set `IsSystem = true` khi caller là system/init.

---

## 8. Gợi ý triển khai

1. **Init script:** Trong `service.admin.init.go` (hoặc script migration riêng), sau khi có System Org, tạo các `OrganizationConfigItem` với `ownerOrganizationId` = System Org ID, `isSystem: true`, các key/value/constraints/allowOverride như bảng trên.
2. **Đọc config trong code:** Gọi `GetResolvedConfig(ctx, orgID)` để lấy config đã merge (root → org); dùng value theo key, fallback default trong code nếu key không tồn tại.
3. **Key cần lock toàn hệ thống:** Đặt `allowOverride: false` cho `paginationMaxLimit`, `contentMaxDepth`, `sessionIdleTimeoutMinutes`, `passwordMinLength`, `orgConfigKeysAllowList`, `rateLimitPerMinute` để cấp dưới không ghi đè.

---

**Cập nhật:** 2025-02
