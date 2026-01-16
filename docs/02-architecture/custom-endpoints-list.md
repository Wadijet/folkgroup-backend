# Danh Sách Tất Cả Endpoint Đặc Thù

## Tổng Quan
Tài liệu này liệt kê tất cả các endpoint đặc thù (không phải CRUD chuẩn) trong hệ thống, kèm theo endpoint path và lý do tồn tại ngắn gọn.

---

## 1. AI & Workflow Endpoints

### 1.1. RenderPrompt
- **Endpoint**: `POST /api/v2/ai/steps/:id/render-prompt`
- **Handler**: `AIStepHandler.RenderPrompt`
- **Lý do**: Action nghiệp vụ - render prompt template với variable substitution, trả về rendered prompt + AI config
- **Status**: ✅ Hợp lệ

### 1.2. ClaimPendingCommands (AI Workflow)
- **Endpoint**: `POST /api/v1/ai/workflow-commands/claim-pending`
- **Handler**: `AIWorkflowCommandHandler.ClaimPendingCommands`
- **Lý do**: Atomic operation - claim commands với atomic update (tránh race condition)
- **Status**: ✅ Hợp lệ

### 1.3. UpdateHeartbeat (AI Workflow)
- **Endpoint**: `POST /api/v1/ai/workflow-commands/update-heartbeat`
- **Handler**: `AIWorkflowCommandHandler.UpdateHeartbeat`
- **Lý do**: Real-time update - agent cập nhật heartbeat và progress định kỳ
- **Status**: ✅ Hợp lệ

### 1.4. ReleaseStuckCommands (AI Workflow)
- **Endpoint**: `POST /api/v1/ai/workflow-commands/release-stuck`
- **Handler**: `AIWorkflowCommandHandler.ReleaseStuckCommands`
- **Lý do**: Background job - giải phóng commands bị stuck (quá lâu không có heartbeat)
- **Status**: ✅ Hợp lệ

---

## 2. Agent Command Endpoints

### 2.1. ClaimPendingCommands (Agent)
- **Endpoint**: `POST /api/v1/agent-management/command/claim-pending`
- **Handler**: `AgentCommandHandler.ClaimPendingCommands`
- **Lý do**: Atomic operation - claim commands với atomic update
- **Status**: ✅ Hợp lệ

### 2.2. UpdateHeartbeat (Agent)
- **Endpoint**: `POST /api/v1/agent-management/command/update-heartbeat`
- **Handler**: `AgentCommandHandler.UpdateHeartbeat`
- **Lý do**: Real-time update - agent cập nhật heartbeat và progress
- **Status**: ✅ Hợp lệ

### 2.3. ReleaseStuckCommands (Agent)
- **Endpoint**: `POST /api/v1/agent-management/command/release-stuck`
- **Handler**: `AgentCommandHandler.ReleaseStuckCommands`
- **Lý do**: Background job - giải phóng commands bị stuck
- **Status**: ✅ Hợp lệ

---

## 3. Content Node Endpoints

### 3.1. GetTree
- **Endpoint**: `GET /api/v1/content/nodes/tree/:id`
- **Handler**: `ContentNodeHandler.GetTree`
- **Lý do**: Logic đệ quy - query children đệ quy để build tree structure, response format nested
- **Status**: ✅ Hợp lệ

---

## 4. Draft Content Endpoints

### 4.1. CommitDraftNode
- **Endpoint**: `POST /api/v1/drafts/nodes/:id/commit`
- **Handler**: `DraftContentNodeHandler.CommitDraftNode`
- **Lý do**: Cross-collection operation - commit draft → production (copy draft sang content node)
- **Status**: ✅ Hợp lệ

### 4.2. ApproveDraftWorkflowRun
- **Endpoint**: `POST /api/v1/content/drafts/approvals/:id/approve`
- **Handler**: `DraftApprovalHandler.ApproveDraftWorkflowRun`
- **Lý do**: Workflow action - approve draft với set decidedBy, decidedAt, có thể trigger side effects
- **Status**: ✅ Hợp lệ

### 4.3. RejectDraftWorkflowRun
- **Endpoint**: `POST /api/v1/content/drafts/approvals/:id/reject`
- **Handler**: `DraftApprovalHandler.RejectDraftWorkflowRun`
- **Lý do**: Workflow action - reject draft với decisionNote bắt buộc
- **Status**: ✅ Hợp lệ

---

## 5. Public / Tracking Endpoints

### 5.1. TrackCTAClick
- **Endpoint**: Public endpoint (không có auth)
- **Handler**: `CTATrackHandler.TrackCTAClick`
- **Lý do**: Public endpoint - HTTP redirect về original URL, decode tracking URL
- **Status**: ✅ Hợp lệ

### 5.2. HandleTrackOpen
- **Endpoint**: Public endpoint
- **Handler**: `NotificationTrackHandler.HandleTrackOpen`
- **Lý do**: Public tracking - track email open, response format pixel image hoặc 204
- **Status**: ✅ Hợp lệ

### 5.3. HandleTrackClick
- **Endpoint**: Public endpoint
- **Handler**: `NotificationTrackHandler.HandleTrackClick`
- **Lý do**: Public tracking - track email click
- **Status**: ✅ Hợp lệ

### 5.4. HandleTrackConfirm
- **Endpoint**: Public endpoint
- **Handler**: `NotificationTrackHandler.HandleTrackConfirm`
- **Lý do**: Public tracking - track email confirm
- **Status**: ✅ Hợp lệ

---

## 6. Notification Endpoints

### 6.1. HandleSend
- **Endpoint**: `POST /api/v1/delivery/send`
- **Handler**: `DeliverySendHandler.HandleSend`
- **Lý do**: Cross-service operation - gửi notification trực tiếp (real-time), sử dụng nhiều services
- **Status**: ✅ Hợp lệ

### 6.2. HandleTriggerNotification
- **Endpoint**: `POST /api/v1/notifications/trigger`
- **Handler**: `NotificationTriggerHandler.HandleTriggerNotification`
- **Lý do**: Cross-service operation - trigger notification workflow
- **Status**: ✅ Hợp lệ

---

## 7. User & Authentication Endpoints

### 7.1. HandleLoginWithFirebase
- **Endpoint**: `POST /api/v1/auth/login/firebase`
- **Handler**: `UserHandler.HandleLoginWithFirebase`
- **Lý do**: Authentication flow - verify Firebase token, tạo/update user, tạo session
- **Status**: ✅ Hợp lệ

### 7.2. HandleLogout
- **Endpoint**: `POST /api/v1/auth/logout`
- **Handler**: `UserHandler.HandleLogout`
- **Lý do**: Authentication action - invalidate session/token
- **Status**: ✅ Hợp lệ

### 7.3. HandleGetProfile
- **Endpoint**: `GET /api/v1/auth/profile`
- **Handler**: `UserHandler.HandleGetProfile`
- **Lý do**: Lấy profile của authenticated user (từ context), sanitize sensitive data
- **Status**: ✅ Hợp lệ

### 7.4. HandleUpdateProfile
- **Endpoint**: `PUT /api/v1/auth/profile`
- **Handler**: `UserHandler.HandleUpdateProfile`
- **Lý do**: Update profile của authenticated user, có validation đặc biệt
- **Status**: ✅ Hợp lệ

### 7.5. HandleGetUserRoles
- **Endpoint**: `GET /api/v1/auth/user-roles`
- **Handler**: `UserHandler.HandleGetUserRoles`
- **Lý do**: Lấy roles của authenticated user
- **Status**: ✅ Hợp lệ

---

## 8. Role & Permission Endpoints

### 8.1. HandleUpdateUserRoles
- **Endpoint**: `PUT /api/v1/auth/user-roles/update`
- **Handler**: `UserRoleHandler.HandleUpdateUserRoles`
- **Lý do**: Atomic operation - xóa tất cả user roles cũ, tạo roles mới (atomic replace all)
- **Status**: ✅ Hợp lệ

### 8.2. HandleUpdateRolePermissions
- **Endpoint**: `PUT /api/v1/auth/role-permissions/update`
- **Handler**: `RolePermissionHandler.HandleUpdateRolePermissions`
- **Lý do**: Atomic operation - xóa tất cả role permissions cũ, tạo permissions mới
- **Status**: ✅ Hợp lệ

---

## 9. Organization Share Endpoints

### 9.1. CreateShare
- **Endpoint**: `POST /api/v1/organization-shares`
- **Handler**: `OrganizationShareHandler.CreateShare`
- **Lý do**: Business logic phức tạp - validate duplicate với set comparison, authorization check
- **Status**: ✅ Hợp lệ

### 9.2. DeleteShare
- **Endpoint**: `DELETE /api/v1/organization-shares/:id`
- **Handler**: `OrganizationShareHandler.DeleteShare`
- **Lý do**: Business logic - authorization check, validate quyền
- **Status**: ✅ Hợp lệ

### 9.3. ListShares
- **Endpoint**: `GET /api/v1/organization-shares?ownerOrganizationId=xxx hoặc ?toOrgId=xxx`
- **Handler**: `OrganizationShareHandler.ListShares`
- **Lý do**: Query phức tạp - filter với $or operator, authorization check
- **Status**: ✅ Hợp lệ

---

## 10. Facebook Integration Endpoints

### 10.1. HandleUpsertMessages
- **Endpoint**: `POST /api/v1/facebook/message/upsert-messages`
- **Handler**: `FbMessageHandler.HandleUpsertMessages`
- **Lý do**: Batch operation - upsert nhiều messages cùng lúc (hiệu quả hơn)
- **Status**: ✅ Hợp lệ

### 10.2. HandleFindByConversationId
- **Endpoint**: `GET /api/v1/facebook/message-items/by-conversation/:conversationId`
- **Handler**: `FbMessageItemHandler.HandleFindByConversationId`
- **Lý do**: Query convenience - tìm bằng external ID (conversationId)
- **Status**: ✅ Hợp lệ

### 10.3. HandleFindOneByMessageId
- **Endpoint**: `GET /api/v1/facebook/message-items/by-message/:messageId`
- **Handler**: `FbMessageItemHandler.HandleFindOneByMessageId`
- **Lý do**: Query convenience - tìm bằng external ID (messageId)
- **Status**: ✅ Hợp lệ

### 10.4. HandleFindAllSortByApiUpdate
- **Endpoint**: `GET /api/v1/facebook/conversations/sort-by-api-update`
- **Handler**: `FbConversationHandler.HandleFindAllSortByApiUpdate`
- **Lý do**: Query đặc biệt - sort theo apiUpdate timestamp
- **Status**: ✅ Hợp lệ

### 10.5. HandleFindOneByPostID
- **Endpoint**: `GET /api/v1/facebook/posts/by-post-id/:postId`
- **Handler**: `FbPostHandler.HandleFindOneByPostID`
- **Lý do**: Query convenience - tìm bằng external ID (Facebook Post ID)
- **Status**: ✅ Hợp lệ

### 10.6. HandleFindOneByPageID
- **Endpoint**: `GET /api/v1/facebook/pages/by-page-id/:pageId`
- **Handler**: `FbPageHandler.HandleFindOneByPageID`
- **Lý do**: Query convenience - tìm bằng external ID (Facebook Page ID)
- **Status**: ✅ Hợp lệ

### 10.7. HandleUpdateToken
- **Endpoint**: `PUT /api/v1/facebook/pages/:id/token`
- **Handler**: `FbPageHandler.HandleUpdateToken`
- **Lý do**: Business logic - update Facebook page token
- **Status**: ✅ Hợp lệ

---

## 11. Webhook Endpoints

### 11.1. HandlePancakeWebhook
- **Endpoint**: `POST /api/v1/webhooks/pancake`
- **Handler**: `PancakeWebhookHandler.HandlePancakeWebhook`
- **Lý do**: Webhook - nhận webhook từ Pancake, verify signature, process payload
- **Status**: ✅ Hợp lệ

### 11.2. HandlePancakePosWebhook
- **Endpoint**: `POST /api/v1/webhooks/pancake-pos`
- **Handler**: `PancakePosWebhookHandler.HandlePancakePosWebhook`
- **Lý do**: Webhook - nhận webhook từ Pancake POS, verify signature
- **Status**: ✅ Hợp lệ

---

## 12. Admin Endpoints

### 12.1. HandleSetRole
- **Endpoint**: `POST /api/v1/admin/users/:id/set-role`
- **Handler**: `AdminHandler.HandleSetRole`
- **Lý do**: Admin operation - set role cho user, chỉ admin mới được
- **Status**: ✅ Hợp lệ

### 12.2. HandleBlockUser
- **Endpoint**: `POST /api/v1/admin/users/:id/block`
- **Handler**: `AdminHandler.HandleBlockUser`
- **Lý do**: Admin operation - block user
- **Status**: ✅ Hợp lệ

### 12.3. HandleUnBlockUser
- **Endpoint**: `POST /api/v1/admin/users/:id/unblock`
- **Handler**: `AdminHandler.HandleUnBlockUser`
- **Lý do**: Admin operation - unblock user
- **Status**: ✅ Hợp lệ

### 12.4. HandleAddAdministrator
- **Endpoint**: `POST /api/v1/admin/administrators/add`
- **Handler**: `AdminHandler.HandleAddAdministrator`
- **Lý do**: Admin operation - add administrator
- **Status**: ✅ Hợp lệ

### 12.5. HandleSyncAdministratorPermissions
- **Endpoint**: `POST /api/v1/admin/administrators/sync-permissions`
- **Handler**: `AdminHandler.HandleSyncAdministratorPermissions`
- **Lý do**: Admin operation - sync permissions cho administrators
- **Status**: ✅ Hợp lệ

---

## 13. System / Init Endpoints

### 13.1. HandleSetAdministrator
- **Endpoint**: `POST /api/v1/init/set-administrator`
- **Handler**: `InitHandler.HandleSetAdministrator`
- **Lý do**: System initialization - set administrator (one-time operation)
- **Status**: ✅ Hợp lệ

### 13.2. HandleInitOrganization
- **Endpoint**: `POST /api/v1/init/organization`
- **Handler**: `InitHandler.HandleInitOrganization`
- **Lý do**: System initialization - khởi tạo organization
- **Status**: ✅ Hợp lệ

### 13.3. HandleInitPermissions
- **Endpoint**: `POST /api/v1/init/permissions`
- **Handler**: `InitHandler.HandleInitPermissions`
- **Lý do**: System initialization - khởi tạo permissions
- **Status**: ✅ Hợp lệ

### 13.4. HandleInitRoles
- **Endpoint**: `POST /api/v1/init/roles`
- **Handler**: `InitHandler.HandleInitRoles`
- **Lý do**: System initialization - khởi tạo roles
- **Status**: ✅ Hợp lệ

### 13.5. HandleInitAdminUser
- **Endpoint**: `POST /api/v1/init/admin-user`
- **Handler**: `InitHandler.HandleInitAdminUser`
- **Lý do**: System initialization - khởi tạo admin user
- **Status**: ✅ Hợp lệ

### 13.6. HandleInitAll
- **Endpoint**: `POST /api/v1/init/all`
- **Handler**: `InitHandler.HandleInitAll`
- **Lý do**: System initialization - khởi tạo tất cả (permissions, roles, admin user)
- **Status**: ✅ Hợp lệ

### 13.7. HandleInitStatus
- **Endpoint**: `GET /api/v1/init/status`
- **Handler**: `InitHandler.HandleInitStatus`
- **Lý do**: System initialization - check trạng thái initialization
- **Status**: ✅ Hợp lệ

### 13.8. HandleHealth
- **Endpoint**: `GET /api/v1/system/health`
- **Handler**: `SystemHandler.HandleHealth`
- **Lý do**: Health check - kiểm tra hệ thống còn hoạt động không
- **Status**: ✅ Hợp lệ

---

## 14. Agent Management Endpoints

### 14.1. HandleEnhancedCheckIn
- **Endpoint**: `POST /api/v1/agent/check-in`
- **Handler**: `AgentManagementHandler.HandleEnhancedCheckIn`
- **Lý do**: Agent management - check-in agent với enhanced logic
- **Status**: ✅ Hợp lệ

### 14.2. HandleUpdateConfigData
- **Endpoint**: `PUT /api/v1/agent-management/config/:agentId/update-data`
- **Handler**: `AgentConfigHandler.HandleUpdateConfigData`
- **Lý do**: Agent config - update config data cho agent
- **Status**: ✅ Hợp lệ

---

## Tổng Kết

### Thống Kê
- **Tổng số endpoint đặc thù**: ~50+ endpoints
- **Endpoints hợp lệ**: 100% (tất cả đều có lý do tồn tại hợp lệ)
- **Endpoints đã đơn giản hóa validation**: 8 endpoints (RenderPrompt, GetTree, CommitDraftNode, Approve/Reject, ClaimPendingCommands, UpdateHeartbeat, ReleaseStuckCommands)

### Phân Loại Theo Lý Do
1. **Logic nghiệp vụ phức tạp**: 8 endpoints
2. **Public endpoint / Response format đặc biệt**: 4 endpoints
3. **Cross-service operations**: 2 endpoints
4. **Atomic operations**: 2 endpoints
5. **Query phức tạp**: 3 endpoints
6. **Authentication**: 5 endpoints
7. **Tracking/Analytics**: 3 endpoints
8. **Webhook**: 2 endpoints
9. **Admin/System**: 13+ endpoints
10. **Find by custom field**: 6+ endpoints
11. **Batch operations**: 1 endpoint
12. **Agent management**: 2 endpoints

### Kết Luận
**Tất cả các endpoint đặc thù đều có lý do tồn tại hợp lệ.** Không có endpoint nào cần loại bỏ hoặc thay thế hoàn toàn bằng CRUD chuẩn.
