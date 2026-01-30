# Luồng Approval Draft Content

## Tổng Quan

Hệ thống quản lý approval draft content với **một status duy nhất** trên từng draft node, không có bước approval riêng (đã bỏ collection `content_draft_approvals`).

## Trình Tự Status

```
draft → pending → approved → (commit) → production
         ↓
      rejected → draft (có thể chỉnh sửa lại)
```

### Chi Tiết Từng Status

| Status | Mô tả | Khi nào | Có thể chuyển sang |
|--------|-------|---------|-------------------|
| **`draft`** | Chưa gửi duyệt (đang chỉnh sửa) | - Mặc định khi AI workflow tạo draft<br>- Sau khi reject, user có thể chỉnh sửa | → `pending` (gửi duyệt) |
| **`pending`** | Chờ duyệt | - User gửi duyệt (update status = pending)<br>- Hoặc tự động khi workflow run completed | → `approved` (duyệt)<br>→ `rejected` (từ chối) |
| **`approved`** | Đã duyệt | - Admin/approver gọi approve endpoint | → (commit) → production<br>→ `draft` (nếu cần chỉnh sửa lại) |
| **`rejected`** | Đã từ chối | - Admin/approver gọi reject endpoint | → `draft` (chỉnh sửa lại) |

## API Endpoints

### 1. Approve Draft

**POST** `/api/v1/content/drafts/nodes/:id/approve`

- **Quyền**: `ContentDraftNodes.Approve`
- **Validation**: Chỉ approve khi status = `pending` hoặc `draft`
- **Logic**: Set `approvalStatus = "approved"`
- **Lưu ý**: Không tự động commit, user phải gọi commit riêng

**Request:**
```json
// Không cần body
```

**Response:**
```json
{
  "id": "...",
  "type": "pillar",
  "approvalStatus": "approved",
  ...
}
```

### 2. Reject Draft

**POST** `/api/v1/content/drafts/nodes/:id/reject`

- **Quyền**: `ContentDraftNodes.Reject`
- **Validation**: Chỉ reject khi status = `pending` hoặc `draft`
- **Logic**: Set `approvalStatus = "rejected"`

**Request:**
```json
{
  "decisionNote": "Lý do từ chối (tùy chọn)"
}
```

**Response:**
```json
{
  "id": "...",
  "type": "pillar",
  "approvalStatus": "rejected",
  "metadata": {
    "decisionNote": "Lý do từ chối"
  },
  ...
}
```

### 3. Commit Draft

**POST** `/api/v1/content/drafts/nodes/:id/commit`

- **Quyền**: `ContentDraftNodes.Commit`
- **Validation**: Chỉ commit khi `approvalStatus = "approved"`
- **Logic**: Copy từ `DraftContentNode` → `ContentNode` (production)

**Request:**
```json
// Không cần body
```

**Response:**
```json
{
  "id": "...",
  "type": "pillar",
  "text": "...",
  // ContentNode đã được tạo trong production
}
```

### 4. Update Approval Status (CRUD)

**PATCH** `/api/v1/content/drafts/nodes/:id`

- **Quyền**: `ContentDraftNodes.Update`
- **Validation**: 
  - ✅ Cho phép: `draft` → `pending`, `rejected` → `pending`, `rejected/approved` → `draft`
  - ❌ Không cho phép: Set `approved` hoặc `rejected` trực tiếp (phải dùng endpoint riêng)

**Request:**
```json
{
  "approvalStatus": "pending"  // Chỉ cho phép một số chuyển đổi
}
```

## Luồng Sử Dụng

### Luồng 1: User/Bot Approve và Commit

```
1. AI workflow tạo drafts (status = "draft")
   ↓
2. User/Bot gửi duyệt: PATCH với approvalStatus = "pending"
   ↓
3. Admin approve: POST /drafts/nodes/:id/approve → status = "approved"
   ↓
4. User/Bot commit: POST /drafts/nodes/:id/commit → tạo ContentNode trong production
```

### Luồng 2: Approve/Reject Nhiều Drafts (Theo Workflow Run)

```
1. Query drafts: GET /drafts/nodes?workflowRunId=...
   ↓
2. Approve từng draft: POST /drafts/nodes/:id/approve (lặp cho mỗi draft)
   ↓
3. Commit từng draft: POST /drafts/nodes/:id/commit (theo thứ tự level L1→L6)
```

### Luồng 3: Reject và Chỉnh Sửa Lại

```
1. Admin reject: POST /drafts/nodes/:id/reject → status = "rejected"
   ↓
2. User chỉnh sửa: PATCH /drafts/nodes/:id với text/name mới
   ↓
3. User set lại status: PATCH với approvalStatus = "draft" hoặc "pending"
   ↓
4. Lặp lại luồng approve/commit
```

## Validation và Bảo Vệ Luồng

### 1. Service Layer Validation

- `ApproveDraft`: Chỉ approve khi status = `pending` hoặc `draft`
- `RejectDraft`: Chỉ reject khi status = `pending` hoặc `draft`
- `CommitDraftNode`: Chỉ commit khi status = `approved`

### 2. CRUD Update Protection

- Override `UpdateById` trong `DraftContentNodeService` để kiểm soát `approvalStatus`
- Không cho phép set `approved`/`rejected` trực tiếp qua CRUD
- Chỉ cho phép một số chuyển đổi hợp lệ

### 3. Sequential Level Constraint

- Commit phải theo thứ tự level (L1 → L2 → ... → L6)
- Parent phải đã được commit (production) trước khi commit child

## Migration

### 1. Đổi Type "layer" → "pillar"

Chạy script: `scripts/migration_layer_to_pillar.js`

```bash
mongo <database_name> scripts/migration_layer_to_pillar.js
```

### 2. Cleanup DraftApproval Collection

Chạy script: `scripts/migration_cleanup_draft_approvals.js`

```bash
# Backup trước
mongoexport --db=<db> --collection=content_draft_approvals --out=backup.json

# Chạy cleanup (uncomment dòng drop() trong script)
mongo <database_name> scripts/migration_cleanup_draft_approvals.js
```

## Lưu Ý

1. **Backward Compatibility**: Data cũ có thể có type = "layer", cần migration script
2. **Approval Status**: DraftApproval collection đã bỏ, chỉ dùng status trên draft
3. **Commit Order**: Phải commit theo level (L1→L6) để đảm bảo parent đã tồn tại
4. **Validation**: Endpoint approve/reject có validation đầy đủ, không thể phá luồng qua CRUD
