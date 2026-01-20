# Tổng Hợp Endpoint Đặc Thù - Handler và Service

## Tổng Quan

Tài liệu này thống kê tất cả các endpoint đặc thù (không phải CRUD chuẩn) còn lại trong hệ thống, bao gồm cả handler endpoints và service methods.

---

## 1. Custom Handler Endpoints (Không Phải CRUD Chuẩn)

### 1.1. AI & Workflow (4 endpoints)

1. **RenderPrompt**
   - Handler: `AIStepHandler.RenderPrompt`
   - Endpoint: `POST /api/v2/ai/steps/:id/render-prompt`
   - Lý do: Action nghiệp vụ - render prompt template với variable substitution

2. **ClaimPendingCommands (AI Workflow)**
   - Handler: `AIWorkflowCommandHandler.ClaimPendingCommands`
   - Endpoint: `POST /api/v1/ai/workflow-commands/claim-pending`
   - Lý do: Atomic operation - claim commands với atomic update

3. **UpdateHeartbeat (AI Workflow)**
   - Handler: `AIWorkflowCommandHandler.UpdateHeartbeat`
   - Endpoint: `POST /api/v1/ai/workflow-commands/update-heartbeat`
   - Lý do: Real-time update - agent cập nhật heartbeat

4. **ReleaseStuckCommands (AI Workflow)**
   - Handler: `AIWorkflowCommandHandler.ReleaseStuckCommands`
   - Endpoint: `POST /api/v1/ai/workflow-commands/release-stuck`
   - Lý do: Background job - giải phóng commands bị stuck

### 1.2. Agent Command (3 endpoints)

5. **ClaimPendingCommands (Agent)**
   - Handler: `AgentCommandHandler.ClaimPendingCommands`
   - Endpoint: `POST /api/v1/agent-management/command/claim-pending`
   - Lý do: Atomic operation

6. **UpdateHeartbeat (Agent)**
   - Handler: `AgentCommandHandler.UpdateHeartbeat`
   - Endpoint: `POST /api/v1/agent-management/command/update-heartbeat`
   - Lý do: Real-time update

7. **ReleaseStuckCommands (Agent)**
   - Handler: `AgentCommandHandler.ReleaseStuckCommands`
   - Endpoint: `POST /api/v1/agent-management/command/release-stuck`
   - Lý do: Background job

### 1.3. Content & Draft (4 endpoints)

8. **GetTree**
   - Handler: `ContentNodeHandler.GetTree`
   - Endpoint: `GET /api/v1/content/nodes/tree/:id`
   - Lý do: Logic đệ quy - build tree structure

9. **CommitDraftNode**
   - Handler: `DraftContentNodeHandler.CommitDraftNode`
   - Endpoint: `POST /api/v1/drafts/nodes/:id/commit`
   - Lý do: Cross-collection operation - commit draft → production

10. **ApproveDraftWorkflowRun**
    - Handler: `DraftApprovalHandler.ApproveDraftWorkflowRun`
    - Endpoint: `POST /api/v1/content/drafts/approvals/:id/approve`
    - Lý do: Workflow action - approve draft

11. **RejectDraftWorkflowRun**
    - Handler: `DraftApprovalHandler.RejectDraftWorkflowRun`
    - Endpoint: `POST /api/v1/content/drafts/approvals/:id/reject`
    - Lý do: Workflow action - reject draft

### 1.4. Public / Tracking (4 endpoints)

12. **TrackCTAClick**
    - Handler: `CTATrackHandler.TrackCTAClick`
    - Endpoint: Public (không auth)
    - Lý do: Public endpoint - HTTP redirect

13. **HandleTrackOpen**
    - Handler: `NotificationTrackHandler.HandleTrackOpen`
    - Endpoint: Public
    - Lý do: Public tracking - track email open

14. **HandleTrackClick**
    - Handler: `NotificationTrackHandler.HandleTrackClick`
    - Endpoint: Public
    - Lý do: Public tracking - track email click

15. **HandleTrackConfirm**
    - Handler: `NotificationTrackHandler.HandleTrackConfirm`
    - Endpoint: Public
    - Lý do: Public tracking - track email confirm

### 1.5. Notification (2 endpoints)

16. **HandleSend**
    - Handler: `DeliverySendHandler.HandleSend`
    - Endpoint: `POST /api/v1/delivery/send`
    - Lý do: Cross-service operation - gửi notification real-time

17. **HandleTriggerNotification**
    - Handler: `NotificationTriggerHandler.HandleTriggerNotification`
    - Endpoint: `POST /api/v1/notifications/trigger`
    - Lý do: Cross-service operation - trigger notification workflow

### 1.6. User & Authentication (5 endpoints)

18. **HandleLoginWithFirebase**
    - Handler: `UserHandler.HandleLoginWithFirebase`
    - Endpoint: `POST /api/v1/auth/login/firebase`
    - Lý do: Authentication flow - verify Firebase token

19. **HandleLogout**
    - Handler: `UserHandler.HandleLogout`
    - Endpoint: `POST /api/v1/auth/logout`
    - Lý do: Authentication action - invalidate session

20. **HandleGetProfile**
    - Handler: `UserHandler.HandleGetProfile`
    - Endpoint: `GET /api/v1/auth/profile`
    - Lý do: Lấy profile của authenticated user

21. **HandleUpdateProfile**
    - Handler: `UserHandler.HandleUpdateProfile`
    - Endpoint: `PUT /api/v1/auth/profile`
    - Lý do: Update profile của authenticated user

22. **HandleGetUserRoles**
    - Handler: `UserHandler.HandleGetUserRoles`
    - Endpoint: `GET /api/v1/auth/user-roles`
    - Lý do: Lấy roles của authenticated user

### 1.7. Role & Permission (2 endpoints)

23. **HandleUpdateUserRoles**
    - Handler: `UserRoleHandler.HandleUpdateUserRoles`
    - Endpoint: `PUT /api/v1/auth/user-roles/update`
    - Lý do: Atomic operation - replace all user roles

24. **HandleUpdateRolePermissions**
    - Handler: `RolePermissionHandler.HandleUpdateRolePermissions`
    - Endpoint: `PUT /api/v1/auth/role-permissions/update`
    - Lý do: Atomic operation - replace all role permissions

### 1.8. Organization Share (3 endpoints)

25. **CreateShare**
    - Handler: `OrganizationShareHandler.CreateShare`
    - Endpoint: `POST /api/v1/organization-shares`
    - Lý do: Business logic - validate duplicate với set comparison

26. **DeleteShare**
    - Handler: `OrganizationShareHandler.DeleteShare`
    - Endpoint: `DELETE /api/v1/organization-shares/:id`
    - Lý do: Business logic - authorization check

27. **ListShares**
    - Handler: `OrganizationShareHandler.ListShares`
    - Endpoint: `GET /api/v1/organization-shares?ownerOrganizationId=xxx hoặc ?toOrgId=xxx`
    - Lý do: Query phức tạp - filter với $or operator

### 1.9. Facebook Integration (7 endpoints)

28. **HandleUpsertMessages**
    - Handler: `FbMessageHandler.HandleUpsertMessages`
    - Endpoint: `POST /api/v1/facebook/message/upsert-messages`
    - Lý do: Batch operation - upsert nhiều messages

29. **HandleFindByConversationId**
    - Handler: `FbMessageItemHandler.HandleFindByConversationId`
    - Endpoint: `GET /api/v1/facebook/message-items/by-conversation/:conversationId`
    - Lý do: Query convenience - tìm bằng external ID

30. **HandleFindOneByMessageId**
    - Handler: `FbMessageItemHandler.HandleFindOneByMessageId`
    - Endpoint: `GET /api/v1/facebook/message-items/by-message/:messageId`
    - Lý do: Query convenience - tìm bằng external ID

31. **HandleFindAllSortByApiUpdate**
    - Handler: `FbConversationHandler.HandleFindAllSortByApiUpdate`
    - Endpoint: `GET /api/v1/facebook/conversations/sort-by-api-update`
    - Lý do: Query đặc biệt - sort theo apiUpdate

32. **HandleFindOneByPostID**
    - Handler: `FbPostHandler.HandleFindOneByPostID`
    - Endpoint: `GET /api/v1/facebook/posts/by-post-id/:postId`
    - Lý do: Query convenience - tìm bằng external ID

33. **HandleFindOneByPageID**
    - Handler: `FbPageHandler.HandleFindOneByPageID`
    - Endpoint: `GET /api/v1/facebook/pages/by-page-id/:pageId`
    - Lý do: Query convenience - tìm bằng external ID

34. **HandleUpdateToken**
    - Handler: `FbPageHandler.HandleUpdateToken`
    - Endpoint: `PUT /api/v1/facebook/pages/:id/token`
    - Lý do: Business logic - update Facebook page token

### 1.10. Webhook (2 endpoints)

35. **HandlePancakeWebhook**
    - Handler: `PancakeWebhookHandler.HandlePancakeWebhook`
    - Endpoint: `POST /api/v1/webhooks/pancake`
    - Lý do: Webhook - nhận webhook từ Pancake

36. **HandlePancakePosWebhook**
    - Handler: `PancakePosWebhookHandler.HandlePancakePosWebhook`
    - Endpoint: `POST /api/v1/webhooks/pancake-pos`
    - Lý do: Webhook - nhận webhook từ Pancake POS

### 1.11. Admin (5 endpoints)

37. **HandleSetRole**
    - Handler: `AdminHandler.HandleSetRole`
    - Endpoint: `POST /api/v1/admin/users/:id/set-role`
    - Lý do: Admin operation - set role cho user

38. **HandleBlockUser**
    - Handler: `AdminHandler.HandleBlockUser`
    - Endpoint: `POST /api/v1/admin/users/:id/block`
    - Lý do: Admin operation - block user

39. **HandleUnBlockUser**
    - Handler: `AdminHandler.HandleUnBlockUser`
    - Endpoint: `POST /api/v1/admin/users/:id/unblock`
    - Lý do: Admin operation - unblock user

40. **HandleAddAdministrator**
    - Handler: `AdminHandler.HandleAddAdministrator`
    - Endpoint: `POST /api/v1/admin/administrators/add`
    - Lý do: Admin operation - add administrator

41. **HandleSyncAdministratorPermissions**
    - Handler: `AdminHandler.HandleSyncAdministratorPermissions`
    - Endpoint: `POST /api/v1/admin/administrators/sync-permissions`
    - Lý do: Admin operation - sync permissions

### 1.12. System / Init (8 endpoints)

42. **HandleSetAdministrator**
    - Handler: `InitHandler.HandleSetAdministrator`
    - Endpoint: `POST /api/v1/init/set-administrator`
    - Lý do: System initialization

43. **HandleInitOrganization**
    - Handler: `InitHandler.HandleInitOrganization`
    - Endpoint: `POST /api/v1/init/organization`
    - Lý do: System initialization

44. **HandleInitPermissions**
    - Handler: `InitHandler.HandleInitPermissions`
    - Endpoint: `POST /api/v1/init/permissions`
    - Lý do: System initialization

45. **HandleInitRoles**
    - Handler: `InitHandler.HandleInitRoles`
    - Endpoint: `POST /api/v1/init/roles`
    - Lý do: System initialization

46. **HandleInitAdminUser**
    - Handler: `InitHandler.HandleInitAdminUser`
    - Endpoint: `POST /api/v1/init/admin-user`
    - Lý do: System initialization

47. **HandleInitAll**
    - Handler: `InitHandler.HandleInitAll`
    - Endpoint: `POST /api/v1/init/all`
    - Lý do: System initialization

48. **HandleInitStatus**
    - Handler: `InitHandler.HandleInitStatus`
    - Endpoint: `GET /api/v1/init/status`
    - Lý do: System initialization - check status

49. **HandleHealth**
    - Handler: `SystemHandler.HandleHealth`
    - Endpoint: `GET /api/v1/system/health`
    - Lý do: Health check

### 1.13. Agent Management (2 endpoints)

50. **HandleEnhancedCheckIn**
    - Handler: `AgentManagementHandler.HandleEnhancedCheckIn`
    - Endpoint: `POST /api/v1/agent/check-in`
    - Lý do: Agent management - check-in với enhanced logic

51. **HandleUpdateConfigData**
    - Handler: `AgentConfigHandler.HandleUpdateConfigData`
    - Endpoint: `PUT /api/v1/agent-management/config/:agentId/update-data`
    - Lý do: Agent config - update config data

---

## 2. Custom Service Methods (Không Phải CRUD Chuẩn)

### 2.1. AI Services (4 methods)

1. **RenderPromptForStep**
   - Service: `AIStepService.RenderPromptForStep`
   - Lý do: Render prompt template với variable substitution

2. **ClaimPendingCommands (AI Workflow)**
   - Service: `AIWorkflowCommandService.ClaimPendingCommands`
   - Lý do: Atomic operation - claim commands

3. **UpdateHeartbeat (AI Workflow)**
   - Service: `AIWorkflowCommandService.UpdateHeartbeat`
   - Lý do: Real-time update - update heartbeat

4. **ReleaseStuckCommands (AI Workflow)**
   - Service: `AIWorkflowCommandService.ReleaseStuckCommands`
   - Lý do: Background job - release stuck commands

### 2.2. Agent Services (7 methods)

5. **GetPendingCommand**
   - Service: `AgentCommandService.GetPendingCommand`
   - Lý do: Lấy command pending đầu tiên

6. **GetPendingCommands**
   - Service: `AgentCommandService.GetPendingCommands`
   - Lý do: Lấy danh sách commands pending

7. **CreateCommand**
   - Service: `AgentCommandService.CreateCommand`
   - Lý do: Tạo command với business logic

8. **ReportCommandResult**
   - Service: `AgentCommandService.ReportCommandResult`
   - Lý do: Báo cáo kết quả thực thi command

9. **ClaimPendingCommands (Agent)**
   - Service: `AgentCommandService.ClaimPendingCommands`
   - Lý do: Atomic operation

10. **UpdateHeartbeat (Agent)**
    - Service: `AgentCommandService.UpdateHeartbeat`
    - Lý do: Real-time update

11. **ReleaseStuckCommands (Agent)**
    - Service: `AgentCommandService.ReleaseStuckCommands`
    - Lý do: Background job

### 2.3. Content & Draft Services (5 methods)

12. **GetChildren**
    - Service: `ContentNodeService.GetChildren`
    - Lý do: Query convenience - lấy children của node

13. **GetAncestors**
    - Service: `ContentNodeService.GetAncestors`
    - Lý do: Query convenience - lấy ancestors của node

14. **CommitDraftNode**
    - Service: `DraftContentNodeService.CommitDraftNode`
    - Lý do: Cross-collection operation - commit draft → production

15. **GetDraftsByWorkflowRunID**
    - Service: `DraftContentNodeService.GetDraftsByWorkflowRunID`
    - Lý do: Query convenience - lấy drafts theo workflow run ID

16. **InsertOne (DraftContentNode)** - Override
    - Service: `DraftContentNodeService.InsertOne`
    - Lý do: Business logic - set default values, validate

### 2.4. User & Auth Services (2 methods)

17. **Logout**
    - Service: `UserService.Logout`
    - Lý do: Authentication action - invalidate session

18. **LoginWithFirebase**
    - Service: `UserService.LoginWithFirebase`
    - Lý do: Authentication flow - verify Firebase token

### 2.5. Agent Config Services (4 methods)

19. **SubmitConfig**
    - Service: `AgentConfigService.SubmitConfig`
    - Lý do: Business logic - submit config với validation

20. **GetCurrentConfig**
    - Service: `AgentConfigService.GetCurrentConfig`
    - Lý do: Query convenience - lấy config hiện tại

21. **UpdateConfig**
    - Service: `AgentConfigService.UpdateConfig`
    - Lý do: Business logic - update config với change log

22. **ReportConfigApplied**
    - Service: `AgentConfigService.ReportConfigApplied`
    - Lý do: Báo cáo config đã được apply

### 2.6. Agent Registry Services (3 methods)

23. **FindOrCreateByAgentID**
    - Service: `AgentRegistryService.FindOrCreateByAgentID`
    - Lý do: Business logic - find or create pattern

24. **UpdateByAgentID**
    - Service: `AgentRegistryService.UpdateByAgentID`
    - Lý do: Query convenience - update bằng external ID

25. **UpdateStatus**
    - Service: `AgentRegistryService.UpdateStatus`
    - Lý do: Business logic - update status với validation

### 2.7. Agent Management Services (1 method)

26. **HandleEnhancedCheckIn**
    - Service: `AgentManagementService.HandleEnhancedCheckIn`
    - Lý do: Agent management - check-in với enhanced logic

### 2.8. Delivery Queue Services (5 methods)

27. **FindPending**
    - Service: `DeliveryQueueService.FindPending`
    - Lý do: Query convenience - lấy items pending

28. **UpdateStatus**
    - Service: `DeliveryQueueService.UpdateStatus`
    - Lý do: Business logic - update status cho nhiều items

29. **CleanupFailedItems**
    - Service: `DeliveryQueueService.CleanupFailedItems`
    - Lý do: Background job - cleanup failed items

30. **FindRecentDuplicates**
    - Service: `DeliveryQueueService.FindRecentDuplicates`
    - Lý do: Business logic - tìm duplicates trong time window

31. **FindStuckItems**
    - Service: `DeliveryQueueService.FindStuckItems`
    - Lý do: Background job - tìm items bị stuck

### 2.9. Notification Services (4 methods)

32. **FindByEventType**
    - Service: `NotificationRoutingService.FindByEventType`
    - Lý do: Query convenience - tìm rules theo event type

33. **FindByDomain**
    - Service: `NotificationRoutingService.FindByDomain`
    - Lý do: Query convenience - tìm rules theo domain

34. **getSystemOrganizationID**
    - Service: `NotificationRoutingService.getSystemOrganizationID`
    - Lý do: Helper method - lấy system organization ID

35. **FindByOrganizationID**
    - Service: `NotificationChannelService.FindByOrganizationID`
    - Lý do: Query convenience - tìm channels theo organization

### 2.10. Organization Services (7 methods)

36. **GetChildrenIDs**
    - Service: `OrganizationService.GetChildrenIDs`
    - Lý do: Query convenience - lấy children IDs

37. **GetParentIDs**
    - Service: `OrganizationService.GetParentIDs`
    - Lý do: Query convenience - lấy parent IDs

38. **validateBeforeDelete**
    - Service: `OrganizationService.validateBeforeDelete`
    - Lý do: Business logic - validate trước khi delete

39. **DeleteOne** - Override
    - Service: `OrganizationService.DeleteOne`
    - Lý do: Business logic - validate trước khi delete

40. **DeleteById** - Override
    - Service: `OrganizationService.DeleteById`
    - Lý do: Business logic - validate trước khi delete

41. **DeleteMany** - Override
    - Service: `OrganizationService.DeleteMany`
    - Lý do: Business logic - validate trước khi delete

42. **FindOneAndDelete** - Override
    - Service: `OrganizationService.FindOneAndDelete`
    - Lý do: Business logic - validate trước khi delete

### 2.11. Role Services (5 methods)

43. **validateBeforeDelete**
    - Service: `RoleService.validateBeforeDelete`
    - Lý do: Business logic - validate trước khi delete

44. **DeleteOne** - Override
    - Service: `RoleService.DeleteOne`
    - Lý do: Business logic - validate trước khi delete

45. **DeleteById** - Override
    - Service: `RoleService.DeleteById`
    - Lý do: Business logic - validate trước khi delete

46. **DeleteMany** - Override
    - Service: `RoleService.DeleteMany`
    - Lý do: Business logic - validate trước khi delete

47. **FindOneAndDelete** - Override
    - Service: `RoleService.FindOneAndDelete`
    - Lý do: Business logic - validate trước khi delete

### 2.12. User Role Services (9 methods)

48. **Create**
    - Service: `UserRoleService.Create`
    - Lý do: Business logic - validate administrator role

49. **UpdateUserRoles**
    - Service: `UserRoleService.UpdateUserRoles`
    - Lý do: Atomic operation - replace all user roles

50. **validateCanRemoveAdministratorRole**
    - Service: `UserRoleService.validateCanRemoveAdministratorRole`
    - Lý do: Business logic - validate không thể remove administrator role

51. **IsExist**
    - Service: `UserRoleService.IsExist`
    - Lý do: Query convenience - check tồn tại

52. **validateBeforeDeleteAdministratorRole**
    - Service: `UserRoleService.validateBeforeDeleteAdministratorRole`
    - Lý do: Business logic - validate trước khi delete administrator role

53. **validateBeforeDeleteAdministratorRoleByFilter**
    - Service: `UserRoleService.validateBeforeDeleteAdministratorRoleByFilter`
    - Lý do: Business logic - validate trước khi delete

54. **DeleteOne** - Override
    - Service: `UserRoleService.DeleteOne`
    - Lý do: Business logic - validate trước khi delete

55. **DeleteById** - Override
    - Service: `UserRoleService.DeleteById`
    - Lý do: Business logic - validate trước khi delete

56. **DeleteMany** - Override
    - Service: `UserRoleService.DeleteMany`
    - Lý do: Business logic - validate trước khi delete

### 2.13. Role Permission Services (2 methods)

57. **Create**
    - Service: `RolePermissionService.Create`
    - Lý do: Business logic - validate duplicate

58. **IsExist**
    - Service: `RolePermissionService.IsExist`
    - Lý do: Query convenience - check tồn tại

### 2.14. Facebook Services (10 methods)

59. **IsConversationIdExist**
    - Service: `FbConversationService.IsConversationIdExist`
    - Lý do: Query convenience - check tồn tại bằng external ID

60. **FindAllSortByApiUpdate**
    - Service: `FbConversationService.FindAllSortByApiUpdate`
    - Lý do: Query đặc biệt - sort theo apiUpdate

61. **CountByConversationId**
    - Service: `FbMessageItemService.CountByConversationId`
    - Lý do: Query convenience - count bằng external ID

62. **IsMessageExist**
    - Service: `FbMessageService.IsMessageExist`
    - Lý do: Query convenience - check tồn tại

63. **FindOneByConversationID**
    - Service: `FbMessageService.FindOneByConversationID`
    - Lý do: Query convenience - tìm bằng external ID

64. **FindAll**
    - Service: `FbMessageService.FindAll`
    - Lý do: Query convenience - find all với pagination

65. **IsPageExist**
    - Service: `FbPageService.IsPageExist`
    - Lý do: Query convenience - check tồn tại

66. **FindOneByPageID**
    - Service: `FbPageService.FindOneByPageID`
    - Lý do: Query convenience - tìm bằng external ID

67. **FindAll**
    - Service: `FbPageService.FindAll`
    - Lý do: Query convenience - find all với pagination

68. **UpdateToken**
    - Service: `FbPageService.UpdateToken`
    - Lý do: Business logic - update Facebook page token

69. **IsPostExist**
    - Service: `FbPostService.IsPostExist`
    - Lý do: Query convenience - check tồn tại

70. **FindOneByPostID**
    - Service: `FbPostService.FindOneByPostID`
    - Lý do: Query convenience - tìm bằng external ID

71. **FindAll**
    - Service: `FbPostService.FindAll`
    - Lý do: Query convenience - find all với pagination

72. **UpdateToken**
    - Service: `FbPostService.UpdateToken`
    - Lý do: Business logic - update token

### 2.15. Pancake Services (4 methods)

73. **IsPancakeOrderIdExist**
    - Service: `PcOrderService.IsPancakeOrderIdExist`
    - Lý do: Query convenience - check tồn tại bằng external ID

74. **FindOne**
    - Service: `PcOrderService.FindOne`
    - Lý do: Query convenience - find one

75. **Delete**
    - Service: `PcOrderService.Delete`
    - Lý do: Business logic - delete với validation

76. **Update**
    - Service: `PcOrderService.Update`
    - Lý do: Business logic - update với validation

### 2.16. Access Token Services (3 methods)

77. **IsNameExist**
    - Service: `AccessTokenService.IsNameExist`
    - Lý do: Query convenience - check tồn tại

78. **Create**
    - Service: `AccessTokenService.Create`
    - Lý do: Business logic - validate duplicate

79. **Update**
    - Service: `AccessTokenService.Update`
    - Lý do: Business logic - update với validation

### 2.17. Admin Services (3 methods)

80. **SetRole**
    - Service: `AdminService.SetRole`
    - Lý do: Admin operation - set role cho user

81. **BlockUser**
    - Service: `AdminService.BlockUser`
    - Lý do: Admin operation - block user

82. **UnBlockUser**
    - Service: `AdminService.UnBlockUser`
    - Lý do: Admin operation - unblock user

### 2.18. Webhook Log Services (2 methods)

83. **CreateWebhookLog**
    - Service: `WebhookLogService.CreateWebhookLog`
    - Lý do: Business logic - tạo webhook log

84. **UpdateProcessedStatus**
    - Service: `WebhookLogService.UpdateProcessedStatus`
    - Lý do: Business logic - update processed status

### 2.19. Agent Activity Services (1 method)

85. **LogActivity**
    - Service: `AgentActivityService.LogActivity`
    - Lý do: Business logic - log activity với severity

### 2.20. Init Services (10 methods - Internal)

86-95. **Init methods** (initAIProviderProfiles, initAIPromptTemplates, initAISteps, etc.)
    - Service: `InitService.*`
    - Lý do: System initialization - internal methods, không phải public API

---

## 3. CRUD Override (Đã Được Audit)

### 3.1. Handler Overrides

**Đã xóa (không cần thiết)**:
- `AIRunHandler.InsertOne` ❌
- `AIStepRunHandler.InsertOne` ❌
- `AIPromptTemplateHandler.InsertOne/UpdateOne` ❌ (sau khi có nested_struct)
- `AIProviderProfileHandler.UpdateOne` ❌ (sau khi có nested_struct)

**Còn lại (hợp lệ)**:
- `AIStepHandler.InsertOne` ✅ - Validate schema phức tạp
- `AIWorkflowHandler.InsertOne` ✅ - Convert nested structures
- `AIWorkflowRunHandler.InsertOne` ✅ - Set default values + validate RootRefID
- `AIWorkflowCommandHandler.InsertOne` ✅ - Validate CommandType, StepID, RootRefID
- `AIProviderProfileHandler.InsertOne` ⚠️ - Vẫn giữ lại (user revert)
- `DraftApprovalHandler.InsertOne` ✅ - Validate cross-field
- `OrganizationHandler.InsertOne` ✅ - Tính Path và Level
- `NotificationChannelHandler.InsertOne` ✅ - Validate uniqueness
- `NotificationRoutingHandler.InsertOne` ✅ - Validate uniqueness

### 3.2. Service Overrides

**Delete operations với validation**:
- `OrganizationService.DeleteOne/DeleteById/DeleteMany/FindOneAndDelete` ✅
- `RoleService.DeleteOne/DeleteById/DeleteMany/FindOneAndDelete` ✅
- `UserRoleService.DeleteOne/DeleteById/DeleteMany` ✅

**Insert operations với business logic**:
- `DraftContentNodeService.InsertOne` ✅

---

## 4. Tổng Kết

### 4.1. Handler Endpoints

- **Tổng số custom handler endpoints**: **51 endpoints**
- **Phân loại**:
  - AI & Workflow: 4
  - Agent Command: 3
  - Content & Draft: 4
  - Public / Tracking: 4
  - Notification: 2
  - User & Authentication: 5
  - Role & Permission: 2
  - Organization Share: 3
  - Facebook Integration: 7
  - Webhook: 2
  - Admin: 5
  - System / Init: 8
  - Agent Management: 2

### 4.2. Service Methods

- **Tổng số custom service methods**: **~85 methods** (bao gồm cả init methods)
- **Public service methods** (không phải init): **~75 methods**
- **Phân loại**:
  - AI Services: 4
  - Agent Services: 7
  - Content & Draft: 5
  - User & Auth: 2
  - Agent Config: 4
  - Agent Registry: 3
  - Agent Management: 1
  - Delivery Queue: 5
  - Notification: 4
  - Organization: 7
  - Role: 5
  - User Role: 9
  - Role Permission: 2
  - Facebook: 10
  - Pancake: 4
  - Access Token: 3
  - Admin: 3
  - Webhook Log: 2
  - Agent Activity: 1
  - Init (internal): 10

### 4.3. CRUD Overrides

- **Handler overrides còn lại**: **9 overrides** (tất cả đều hợp lệ)
- **Service overrides còn lại**: **~15 overrides** (tất cả đều hợp lệ)

---

## 5. Phân Loại Theo Lý Do

### 5.1. Logic Nghiệp Vụ Phức Tạp
- RenderPrompt, CommitDraftNode, Approve/Reject, CreateShare, DeleteShare, ListShares
- **Handler**: ~10 endpoints
- **Service**: ~15 methods

### 5.2. Atomic Operations
- ClaimPendingCommands, UpdateHeartbeat, ReleaseStuckCommands, UpdateUserRoles, UpdateRolePermissions
- **Handler**: 5 endpoints
- **Service**: 6 methods

### 5.3. Query Convenience / Find By Custom Field
- GetTree, GetChildren, GetAncestors, FindByConversationId, FindOneByMessageId, FindOneByPostID, etc.
- **Handler**: ~10 endpoints
- **Service**: ~20 methods

### 5.4. Public Endpoint / Response Format Đặc Biệt
- TrackCTAClick, HandleTrackOpen, HandleTrackClick, HandleTrackConfirm
- **Handler**: 4 endpoints
- **Service**: 0 methods (không có service layer)

### 5.5. Cross-Service Operations
- HandleSend, HandleTriggerNotification
- **Handler**: 2 endpoints
- **Service**: 0 methods (logic trong handler)

### 5.6. Authentication Flow
- HandleLoginWithFirebase, HandleLogout, HandleGetProfile, HandleUpdateProfile, HandleGetUserRoles
- **Handler**: 5 endpoints
- **Service**: 2 methods

### 5.7. Admin / System Operations
- HandleSetRole, HandleBlockUser, HandleUnBlockUser, HandleAddAdministrator, HandleSyncAdministratorPermissions
- HandleInitOrganization, HandleInitPermissions, HandleInitRoles, HandleInitAdminUser, HandleInitAll, HandleInitStatus
- HandleHealth
- **Handler**: 13 endpoints
- **Service**: 3 methods (không tính init methods)

### 5.8. Webhook
- HandlePancakeWebhook, HandlePancakePosWebhook
- **Handler**: 2 endpoints
- **Service**: 2 methods (webhook log)

### 5.9. Batch Operations
- HandleUpsertMessages
- **Handler**: 1 endpoint
- **Service**: 0 methods

### 5.10. Background Jobs
- ReleaseStuckCommands, CleanupFailedItems, FindStuckItems
- **Handler**: 2 endpoints
- **Service**: 3 methods

---

## 6. Kết Luận

### 6.1. Tổng Số

- **Custom handler endpoints**: **51 endpoints**
- **Custom service methods**: **~75 methods** (public, không tính init)
- **CRUD overrides (handler)**: **9 overrides** (tất cả hợp lệ)
- **CRUD overrides (service)**: **~15 overrides** (tất cả hợp lệ)

### 6.2. Đánh Giá

**Tất cả các endpoint đặc thù đều có lý do tồn tại hợp lệ:**
- Logic nghiệp vụ phức tạp không thể thay thế bằng CRUD chuẩn
- Atomic operations cần đảm bảo consistency
- Query convenience giúp client dễ sử dụng
- Public endpoints cần response format đặc biệt
- Cross-service operations cần orchestration
- Authentication flow có logic đặc biệt
- Admin/System operations cần quyền đặc biệt

### 6.3. Khả Năng Đơn Giản Hóa

**Đã đơn giản hóa validation** (8 endpoints):
- RenderPrompt, GetTree, CommitDraftNode, Approve/Reject, ClaimPendingCommands (2), UpdateHeartbeat (2), ReleaseStuckCommands (2)

**Có thể đơn giản hóa thêm** (nếu cần):
- Organization Share endpoints (3) - có thể refactor sang CRUD nếu move logic xuống service layer

**Không thể đơn giản hóa**:
- Tất cả các endpoint còn lại đều có logic nghiệp vụ phức tạp, atomic operations, hoặc response format đặc biệt
