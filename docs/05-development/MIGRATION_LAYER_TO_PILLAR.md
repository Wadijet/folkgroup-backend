# Migration: Layer → Pillar và Bỏ DraftApproval

## Tổng Quan

Migration này thực hiện 2 thay đổi chính:
1. **Đổi type "layer" → "pillar"** trong content nodes và draft nodes
2. **Cleanup DraftApproval collection** (đã bỏ bước approval riêng)

## Bước 1: Backup Data

**QUAN TRỌNG**: Backup data trước khi chạy migration!

```bash
# Backup content nodes
mongoexport --db=<database_name> --collection=content_nodes --out=backup_content_nodes.json

# Backup draft nodes
mongoexport --db=<database_name> --collection=content_draft_nodes --out=backup_draft_nodes.json

# Backup draft approvals (nếu cần)
mongoexport --db=<database_name> --collection=content_draft_approvals --out=backup_draft_approvals.json
```

## Bước 2: Đổi Type "layer" → "pillar"

Chạy script migration:

```bash
mongo <database_name> scripts/migration_layer_to_pillar.js
```

**Script sẽ:**
- Tìm tất cả documents có `type = "layer"` trong:
  - `content_nodes`
  - `content_draft_nodes`
- Đổi `type` từ `"layer"` → `"pillar"`
- Verify kết quả

**Output mẫu:**
```
🚀 Bắt đầu migration: layer → pillar
==========================================

📊 Collection: content_nodes
   - Số documents có type = "layer": 10
   ✅ Đã update: 10 documents
   ✅ Verify: Không còn document nào có type = "layer"
   ✅ Verify: Có 10 documents có type = "pillar"

📊 Collection: content_draft_nodes
   - Số documents có type = "layer": 5
   ✅ Đã update: 5 documents
   ✅ Verify: Không còn document nào có type = "layer"
   ✅ Verify: Có 5 documents có type = "pillar"

==========================================
📊 TỔNG KẾT:
   ✅ Tổng số documents đã update: 15
   ✅ Không có lỗi
==========================================
✅ Migration hoàn tất!
```

## Bước 3: Cleanup DraftApproval Collection

**LƯU Ý**: Chỉ chạy sau khi đã verify approval status đã được migrate sang draft nodes (nếu có).

Chạy script cleanup:

```bash
mongo <database_name> scripts/migration_cleanup_draft_approvals.js
```

**Script sẽ:**
- Đếm số documents trong `content_draft_approvals`
- **KHÔNG tự động xóa** (cần uncomment dòng `drop()` hoặc `deleteMany()`)
- Hướng dẫn backup và xóa

**Để thực sự xóa:**
1. Mở file `scripts/migration_cleanup_draft_approvals.js`
2. Uncomment dòng: `collection.drop();` hoặc `collection.deleteMany({});`
3. Chạy lại script

## Bước 4: Verify

### Verify Type Migration

```javascript
// Trong MongoDB shell
use <database_name>

// Kiểm tra không còn "layer"
db.content_nodes.countDocuments({ type: "layer" })  // Phải = 0
db.content_draft_nodes.countDocuments({ type: "layer" })  // Phải = 0

// Kiểm tra có "pillar"
db.content_nodes.countDocuments({ type: "pillar" })  // Phải > 0
db.content_draft_nodes.countDocuments({ type: "pillar" })  // Phải > 0
```

### Verify API Endpoints

Test các endpoint mới:

```bash
# Approve draft
POST /api/v1/content/drafts/nodes/:id/approve

# Reject draft
POST /api/v1/content/drafts/nodes/:id/reject

# Commit draft
POST /api/v1/content/drafts/nodes/:id/commit
```

## Rollback (Nếu Cần)

Nếu cần rollback, restore từ backup:

```bash
# Restore content nodes
mongoimport --db=<database_name> --collection=content_nodes --file=backup_content_nodes.json

# Restore draft nodes
mongoimport --db=<database_name> --collection=content_draft_nodes --file=backup_draft_nodes.json

# Restore draft approvals (nếu cần)
mongoimport --db=<database_name> --collection=content_draft_approvals --file=backup_draft_approvals.json
```

Sau đó chạy script rollback (đổi "pillar" → "layer"):

```javascript
// Rollback script (tạo file mới hoặc sửa script hiện tại)
db.content_nodes.updateMany(
    { type: "pillar" },
    { $set: { type: "layer" } }
);

db.content_draft_nodes.updateMany(
    { type: "pillar" },
    { $set: { type: "layer" } }
);
```

## Checklist

- [ ] Backup tất cả collections liên quan
- [ ] Chạy migration script đổi "layer" → "pillar"
- [ ] Verify không còn type = "layer"
- [ ] Verify có type = "pillar"
- [ ] Test API endpoints approve/reject/commit
- [ ] Cleanup DraftApproval collection (nếu cần)
- [ ] Update documentation cho team

## Lưu Ý

1. **Downtime**: Migration có thể mất vài phút tùy số lượng documents
2. **Indexes**: Script không tạo/xóa indexes, MongoDB sẽ tự động update
3. **Validation**: Code mới đã validate type = "pillar", không chấp nhận "layer" nữa
4. **Backward Compatibility**: Data cũ có type = "layer" sẽ không hoạt động với code mới, **phải migrate**
