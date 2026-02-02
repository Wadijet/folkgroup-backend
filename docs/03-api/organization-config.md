# API Cấu Hình Tổ Chức (Organization Config)

Tài liệu API cho cấu hình riêng theo từng tổ chức: lấy config raw, config đã merge theo cây (resolved), cập nhật và xóa.

## Tổng quan

- **Collection:** `auth_organization_configs`
- **Quan hệ:** 1 document / 1 tổ chức (khóa bởi `ownerOrganizationId`).
- **Config theo cây:** Tổ chức cấp dưới kế thừa config từ tổ chức cấp trên; key có `allowOverride: false` ở cấp trên thì cấp dưới không được ghi đè.
- **Config hệ thống:** Config của tổ chức hệ thống (root) không cho xóa (`IsSystem` hoặc `ownerOrganizationId` = System Org).

## Cấu trúc dữ liệu

### Document (raw)

| Trường | Kiểu | Mô tả |
|--------|------|--------|
| `_id` | ObjectID | ID document |
| `ownerOrganizationId` | ObjectID | Tổ chức sở hữu config (unique) |
| `config` | map[string]interface{} | Giá trị từng key (ví dụ: `timezone`, `businessHours`) |
| `configMeta` | map[string]ConfigKeyMeta | Metadata từng key: tên, mô tả, loại, ràng buộc, `allowOverride` |
| `isSystem` | bool | `true` = config hệ thống, không xóa (chỉ nội bộ) |
| `createdAt` | int64 | Unix ms |
| `updatedAt` | int64 | Unix ms |

### ConfigKeyMeta

| Trường | Kiểu | Mô tả |
|--------|------|--------|
| `name` | string | Tên hiển thị (ví dụ: "Múi giờ") |
| `description` | string | Mô tả mục đích/cách dùng |
| `dataType` | string | `string`, `number`, `boolean`, `object`, `array` |
| `constraints` | string | Ràng buộc: enum, min, max, pattern... |
| `allowOverride` | bool | `true` = cấp dưới được ghi đè; `false` = khóa |

## Endpoints

Base path: `/api/v1/organization`. Tất cả endpoint cần **Bearer token** và **Organization context** (header/query theo quy ước dự án).

### 1. Lấy config raw của tổ chức

**GET** `/api/v1/organization/:id/config`

- **:id** — ObjectID của tổ chức.
- **Permission:** `OrganizationConfig.Read`
- **Response (có config):** Document đầy đủ (config, configMeta, isSystem, createdAt, updatedAt).
- **Response (chưa có config):** `ownerOrganizationId`, `config: null`, `configMeta: null`, `isSystem: false`.

### 2. Lấy config đã merge theo cây (resolved)

**GET** `/api/v1/organization/:id/config/resolved`

- **:id** — ObjectID của tổ chức.
- **Permission:** `OrganizationConfig.Read`
- **Response:** `{ "config": { "key1": value1, ... } }` — chỉ giá trị config đã merge từ root → org hiện tại; key bị khóa (`allowOverride: false`) ở cấp trên thì cấp dưới không ghi đè.

### 3. Tạo / cập nhật config (upsert)

**PUT** `/api/v1/organization/:id/config`

- **:id** — ObjectID của tổ chức.
- **Permission:** `OrganizationConfig.Update`
- **Body:** `OrganizationConfigUpdateInput` (JSON).

```json
{
  "config": {
    "timezone": "Asia/Ho_Chi_Minh",
    "businessHours": { "start": "09:00", "end": "18:00" }
  },
  "configMeta": {
    "timezone": {
      "name": "Múi giờ",
      "description": "Múi giờ mặc định",
      "dataType": "string",
      "constraints": "",
      "allowOverride": true
    },
    "businessHours": {
      "name": "Giờ làm việc",
      "description": "Giờ bắt đầu và kết thúc",
      "dataType": "object",
      "constraints": "",
      "allowOverride": false
    }
  }
}
```

- **Validation:** Key nằm trong `configMeta` của tổ chức cha với `allowOverride: false` thì không được ghi đè (trả lỗi 403).
- **Response:** Document config sau khi upsert.

### 4. Xóa config tổ chức

**DELETE** `/api/v1/organization/:id/config`

- **:id** — ObjectID của tổ chức.
- **Permission:** `OrganizationConfig.Delete`
- **Validation:** Không cho xóa config hệ thống (`isSystem: true` hoặc tổ chức là System Org).
- **Response:** `{ "message": "Đã xóa config tổ chức." }`

## Permissions (Init)

Khi gọi init permissions, hệ thống sẽ có sẵn:

- `OrganizationConfig.Read` — Xem cấu hình tổ chức (raw và resolved).
- `OrganizationConfig.Update` — Cập nhật cấu hình tổ chức.
- `OrganizationConfig.Delete` — Xóa cấu hình tổ chức (không áp dụng cho config hệ thống).

## Quy tắc nghiệp vụ

1. **Sở hữu:** Mỗi tổ chức có tối đa một document config (`ownerOrganizationId` unique).
2. **Kế thừa:** `GetResolvedConfig` merge theo chuỗi root → … → org; key có `allowOverride: false` ở cấp trên thì giá trị đó không bị cấp dưới ghi đè.
3. **Khóa key:** Khi cập nhật, nếu key đã bị khóa bởi tổ chức cha thì API trả 403 với message rõ ràng.
4. **Config hệ thống:** Không cho xóa (API trả 403 nếu thử xóa).
