# Changelog: Approval Workflow và Layer → Pillar

## Tổng Quan Thay Đổi

### 1. Bỏ Bước Approval Riêng
- ❌ **Xóa**: Collection `content_draft_approvals` (không dùng nữa)
- ❌ **Xóa**: Model `DraftApproval`, Service `DraftApprovalService`, Handler `DraftApprovalHandler`
- ❌ **Xóa**: DTO `dto.content.draft.approval.go`
- ❌ **Xóa**: Routes `/api/v1/content/drafts/approvals/*`
- ✅ **Giữ**: Chỉ dùng `approvalStatus` trên từng draft node

### 2. Thêm Endpoint Approve/Reject Riêng
- ✅ **Thêm**: `POST /api/v1/content/drafts/nodes/:id/approve` (với validation)
- ✅ **Thêm**: `POST /api/v1/content/drafts/nodes/:id/reject` (với validation)
- ✅ **Bảo vệ**: Không cho phép update `approvalStatus = approved/rejected` qua CRUD

### 3. Đổi L1 Layer → Pillar
- ✅ **Đổi**: Constant `ContentNodeTypeLayer` → `ContentNodeTypePillar`
- ✅ **Đổi**: Type value `"layer"` → `"pillar"`
- ✅ **Đổi**: Tất cả comments, prompts, workflows từ "Layer" → "Pillar"

## Files Đã Thay Đổi

### Models
- `api/core/api/models/mongodb/model.content.node.go` - Đổi constant và comment
- `api/core/api/models/mongodb/model.draft.content.node.go` - Đổi comment
- `api/core/api/models/mongodb/model.ai.workflow.go` - Đổi comment
- `api/core/api/models/mongodb/model.ai.workflow.run.go` - Đổi comment
- `api/core/api/models/mongodb/model.ai.workflow.command.go` - Đổi comment

### Services
- `api/core/api/services/service.draft.content.node.go` - Thêm `ApproveDraft`, `RejectDraft`, override `UpdateById`
- `api/core/api/services/service.ai.workflow.command.go` - Đổi `isStepCreateLayer` → `isStepCreatePillar`
- `api/core/api/services/service.ai.workflow.run.go` - Đổi error messages
- `api/core/api/services/service.admin.init.go` - Đổi prompts, steps, workflows từ Layer → Pillar

### Handlers
- `api/core/api/handler/handler.draft.content.node.go` - Thêm `ApproveDraft`, `RejectDraft`
- `api/core/api/handler/handler.content.node.go` - Đổi comment

### DTOs
- `api/core/api/dto/dto.content.node.go` - Đổi comment
- `api/core/api/dto/dto.draft.content.node.go` - Đổi comment, thêm `ApproveDraftParams`, `RejectDraftParams`, `RejectDraftInput`
- `api/core/api/dto/dto.ai.workflow.command.go` - Đổi comment
- `api/core/api/dto/dto.ai.workflow.run.go` - Đổi comment
- `api/core/api/dto/dto.ai.workflow.go` - Đổi comment

### Routes
- `api/core/api/router/routes.go` - Bỏ routes `/drafts/approvals/*`, thêm `/drafts/nodes/:id/approve|reject`

### Utility
- `api/core/utility/content.level.go` - Đổi `ContentLevelMap`, comments, error messages

### Global & Init
- `api/core/global/global.vars.go` - Bỏ `DraftApprovals`
- `api/cmd/server/init.go` - Bỏ collection name và index cho DraftApprovals
- `api/cmd/server/init.registry.go` - Bỏ `content_draft_approvals` khỏi registry

### Permissions
- `api/core/api/services/service.admin.init.go` - Bỏ permissions `ContentDraftApprovals.*`, dùng `ContentDraftNodes.Approve` và `ContentDraftNodes.Reject`

### Files Đã Xóa
- `api/core/api/models/mongodb/model.content.draft.approval.go`
- `api/core/api/services/service.content.draft.approval.go`
- `api/core/api/handler/handler.content.draft.approval.go`
- `api/core/api/dto/dto.content.draft.approval.go`

## Files Mới

### Migration Scripts
- `scripts/migration_layer_to_pillar.js` - Đổi type "layer" → "pillar"
- `scripts/migration_cleanup_draft_approvals.js` - Cleanup DraftApproval collection

### Documentation
- `docs/05-development/luong-approval-draft-content.md` - Tài liệu luồng approval mới
- `docs/05-development/MIGRATION_LAYER_TO_PILLAR.md` - Hướng dẫn migration
- `docs/05-development/CHANGELOG_APPROVAL_AND_PILLAR.md` - File này

## Checklist Triển Khai

### Pre-Deployment
- [ ] Review code changes
- [ ] Test approve/reject endpoints
- [ ] Test commit workflow
- [ ] Verify validation logic

### Migration
- [ ] Backup database
- [ ] Chạy `migration_layer_to_pillar.js`
- [ ] Verify không còn type = "layer"
- [ ] Verify có type = "pillar"
- [ ] Cleanup `content_draft_approvals` (nếu cần)

### Deployment
- [ ] Deploy code mới
- [ ] Verify server start thành công
- [ ] Test API endpoints
- [ ] Monitor logs

### Post-Deployment
- [ ] Verify approve/reject workflow hoạt động
- [ ] Verify commit workflow hoạt động
- [ ] Update API documentation cho frontend team
- [ ] Thông báo team về thay đổi

## Breaking Changes

### API Changes
1. **Xóa routes**:
   - `POST /api/v1/content/drafts/approvals` (CRUD)
   - `POST /api/v1/content/drafts/approvals/:id/approve`
   - `POST /api/v1/content/drafts/approvals/:id/reject`

2. **Thêm routes**:
   - `POST /api/v1/content/drafts/nodes/:id/approve`
   - `POST /api/v1/content/drafts/nodes/:id/reject`

3. **Type value change**:
   - Content nodes với `type = "layer"` sẽ không hoạt động với code mới
   - Phải migrate sang `type = "pillar"`

### Data Changes
1. **DraftApproval collection**: Không dùng nữa, có thể xóa
2. **Content type**: `"layer"` → `"pillar"` (phải migrate)

## Rollback Plan

Nếu cần rollback:

1. **Code**: Revert commit về version cũ
2. **Data**: 
   - Restore từ backup
   - Hoặc chạy rollback script (đổi "pillar" → "layer")

## Testing Checklist

- [ ] Approve draft với status = "pending" → thành công
- [ ] Approve draft với status = "draft" → thành công
- [ ] Approve draft với status = "approved" → lỗi (đã approve rồi)
- [ ] Reject draft với status = "pending" → thành công
- [ ] Reject draft với status = "rejected" → lỗi (đã reject rồi)
- [ ] Update approvalStatus qua CRUD với "approved" → lỗi (phải dùng endpoint)
- [ ] Update approvalStatus qua CRUD với "pending" từ "draft" → thành công
- [ ] Commit draft với status = "approved" → thành công
- [ ] Commit draft với status = "pending" → lỗi (chưa approve)
- [ ] Query drafts với type = "pillar" → trả về kết quả
- [ ] Query drafts với type = "layer" → không trả về (sau migration)

## Notes

- Migration script an toàn, chỉ update không xóa data
- Cleanup script cần uncomment để thực sự xóa
- Code mới không backward compatible với type = "layer", **phải migrate**
