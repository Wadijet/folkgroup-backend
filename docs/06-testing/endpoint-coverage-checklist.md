# Endpoint Coverage Checklist

Tài liệu này liệt kê tất cả các endpoints và trạng thái test coverage.

## 📊 Tổng Quan

- **Tổng số endpoints**: ~150+
- **Endpoints đã có test**: ✅
- **Endpoints chưa có test**: ⚠️
- **Endpoints không cần test**: ❌

---

## 🔐 Auth Routes

### ✅ Đã có test
- `POST /auth/login/firebase` - `auth_test.go`
- `POST /auth/logout` - `auth_test.go`
- `GET /auth/profile` - `auth_test.go`
- `PUT /auth/profile` - `auth_test.go`
- `GET /auth/roles` - `organization_data_access_test.go` ✅ **MỚI**

---

## 👥 RBAC Routes

### ✅ Đã có test
- `GET /user/find` - `crud_operations_test.go`
- `GET /permission/find` - `rbac_test.go`
- `GET /permission/by-category/:category` - `rbac_test.go`
- `GET /permission/by-group/:group` - `rbac_test.go`
- `POST /role/insert-one` - `crud_operations_test.go`, `rbac_test.go`
- `GET /role/find` - `crud_operations_test.go`
- `PUT /role-permission/update-role` - `rbac_test.go`
- `PUT /user-role/update-user-roles` - `rbac_test.go`

### ⚠️ Chưa có test
- `POST /role/insert-many`
- `PUT /role/update-one`
- `PUT /role/update-by-id/:id`
- `DELETE /role/delete-by-id/:id`
- `POST /role-permission/insert-one`
- `GET /role-permission/find`
- `POST /user-role/insert-one`
- `GET /user-role/find`

---

## 🏢 Organization Routes

### ✅ Đã có test
- `GET /organization/find` - `crud_operations_test.go` (gián tiếp qua GetRootOrganizationID)

### ⚠️ Chưa có test
- `POST /organization/insert-one`
- `PUT /organization/update-by-id/:id`
- `DELETE /organization/delete-by-id/:id`
- Tất cả CRUD operations khác

---

## 🤖 Agent Routes

### ✅ Đã có test
- `POST /agent/check-in/:id` - `agent_test.go`
- `POST /agent/check-out/:id` - `agent_test.go`

### ⚠️ Chưa có test
- `POST /agent/insert-one`
- `GET /agent/find`
- `PUT /agent/update-by-id/:id`
- `DELETE /agent/delete-by-id/:id`

---

## 📘 Facebook Routes

### ✅ Đã có test
- `GET /access-token/find` - `facebook_test.go`
- `POST /access-token/insert-one` - `facebook_test.go`

### ⚠️ Chưa có test
- `GET /facebook/page/find` - CRUD cơ bản
- `GET /facebook/page/find-by-page-id/:id` - **Endpoint đặc biệt**
- `PUT /facebook/page/update-token` - **Endpoint đặc biệt**
- `GET /facebook/post/find` - CRUD cơ bản
- `GET /facebook/post/find-by-post-id/:id` - **Endpoint đặc biệt**
- `GET /facebook/conversation/find` - CRUD cơ bản
- `GET /facebook/conversation/sort-by-api-update` - **Endpoint đặc biệt**
- `GET /facebook/message/find` - CRUD cơ bản
- `POST /facebook/message/upsert-messages` - **Endpoint đặc biệt**
- `GET /facebook/message-item/find` - CRUD cơ bản
- `GET /facebook/message-item/find-by-conversation/:conversationId` - **Endpoint đặc biệt**
- `GET /facebook/message-item/find-by-message-id/:messageId` - **Endpoint đặc biệt**

---

## 🥞 Pancake Routes

### ✅ Đã có test
- `GET /pancake/order/find` - `pancake_test.go` (một phần)

### ⚠️ Chưa có test
- `POST /pancake/order/insert-one`
- `PUT /pancake/order/update-by-id/:id`
- `DELETE /pancake/order/delete-by-id/:id`

---

## 👤 Customer Routes

### ✅ Đã có test
- `POST /fb-customer/insert-one` - `organization_data_access_test.go` ✅ **MỚI**
- `GET /fb-customer/find` - `organization_data_access_test.go` ✅ **MỚI**

### ⚠️ Chưa có test
- `POST /pc-pos-customer/insert-one`
- `GET /pc-pos-customer/find`
- `POST /customer/insert-one` (deprecated)
- `GET /customer/find` (deprecated)

---

## 🏪 Pancake POS Routes

### ⚠️ Chưa có test
- `POST /pancake-pos/shop/insert-one`
- `GET /pancake-pos/shop/find`
- `POST /pancake-pos/warehouse/insert-one`
- `GET /pancake-pos/warehouse/find`
- `POST /pancake-pos/product/insert-one`
- `GET /pancake-pos/product/find`
- `POST /pancake-pos/variation/insert-one`
- `GET /pancake-pos/variation/find`
- `POST /pancake-pos/category/insert-one`
- `GET /pancake-pos/category/find`
- `POST /pancake-pos/order/insert-one`
- `GET /pancake-pos/order/find`

---

## 📧 Notification Routes

### ✅ Đã có test (MỚI)
- `POST /notification/sender/insert-one` - `notification_test.go` ✅ **MỚI**
- `GET /notification/sender/find` - `notification_test.go` ✅ **MỚI**
- `POST /notification/channel/insert-one` - `notification_test.go` ✅ **MỚI**
- `GET /notification/channel/find` - `notification_test.go` ✅ **MỚI**
- `POST /notification/template/insert-one` - `notification_test.go` ✅ **MỚI**
- `GET /notification/template/find` - `notification_test.go` ✅ **MỚI**
- `POST /notification/routing/insert-one` - `notification_test.go` ✅ **MỚI**
- `GET /notification/routing/find` - `notification_test.go` ✅ **MỚI**
- `GET /notification/history/find` - `notification_test.go` ✅ **MỚI**
- `POST /notification/trigger` - `notification_test.go` ✅ **MỚI**
- `GET /notification/track/open/:historyId` - `notification_test.go` ✅ **MỚI**
- `GET /notification/track/:historyId/:ctaIndex` - `notification_test.go` ✅ **MỚI**
- `GET /notification/confirm/:historyId` - `notification_test.go` ✅ **MỚI**

### ⚠️ Chưa có test chi tiết
- Update/Delete operations cho sender, channel, template, routing
- Test với organization context (organizationId tự động gán/filter)

---

## 🔧 Admin Routes

### ✅ Đã có test
- `POST /admin/user/block` - `admin_test.go`
- `POST /admin/user/unblock` - `admin_test.go`
- `POST /admin/user/role` - `admin_test.go`

### ⚠️ Chưa có test
- `POST /admin/user/set-administrator/:id`
- `POST /admin/sync-administrator-permissions`

---

## 🚀 Init Routes

### ✅ Đã có test (gián tiếp)
- `POST /init/all` - `initTestData()` function

### ⚠️ Chưa có test riêng
- `GET /init/status`
- `POST /init/organization`
- `POST /init/permissions`
- `POST /init/roles`
- `POST /init/admin-user`
- `POST /init/set-administrator/:id`

---

## 🏥 System Routes

### ✅ Đã có test
- `GET /system/health` - `health_test.go`

---

## 📋 CRUD Operations Coverage

### ✅ Đã test với organization context
- `POST /fb-customer/insert-one` - Tự động gán organizationId ✅
- `GET /fb-customer/find` - Tự động filter theo organizationId ✅
- `POST /notification/channel/insert-one` - Tự động gán organizationId ✅
- `GET /notification/channel/find` - Tự động filter theo organizationId ✅

### ⚠️ Chưa test với organization context
- Tất cả các collections khác có organizationId:
  - `FbCustomer`, `PcPosCustomer`, `PcPosOrder`, `PcPosShop`, `PcPosProduct`, `PcPosWarehouse`
  - `FbPage`, `FbPost`, `FbConversation`, `FbMessage`
  - `PcPosCategory`, `PcPosVariation`, `FbMessageItem`
  - `AccessTokens`, `Customer`
  - `NotificationSender`, `NotificationTemplate`, `NotificationRouting`

---

## 🎯 Priority Test Cases Cần Bổ Sung

### Priority 1 - High (Quan trọng)
1. ✅ **Notification CRUD với organization context** - ĐÃ TẠO
2. ⚠️ **Facebook endpoints đặc biệt** (find-by-page-id, update-token, upsert-messages, etc.)
3. ⚠️ **Test scope permissions** (Scope 0 vs Scope 1)
4. ⚠️ **Test inverse parent lookup** (xem dữ liệu cấp trên)

### Priority 2 - Medium
5. ⚠️ **Pancake POS CRUD operations** với organization context
6. ⚠️ **Admin endpoints** (set-administrator, sync-permissions)
7. ⚠️ **Agent CRUD operations**

### Priority 3 - Low
8. ⚠️ **Init endpoints** riêng lẻ
9. ⚠️ **CRUD operations** cho các collections ít dùng

---

## ✅ Test Files Hiện Có

1. `auth_test.go` - Auth flow, login, logout, profile
2. `auth_additional_test.go` - Additional auth tests
3. `rbac_test.go` - Role, Permission, UserRole APIs
4. `crud_operations_test.go` - Basic CRUD operations
5. `admin_test.go` - Admin operations (block, unblock, set role)
6. `agent_test.go` - Agent check-in/check-out
7. `facebook_test.go` - Facebook integration APIs (đã cập nhật với endpoints đặc biệt)
8. `pancake_test.go` - Pancake APIs
9. `health_test.go` - Health check
10. `error_handling_test.go` - Error handling
11. `organization_data_access_test.go` - Organization data access cơ bản
12. `organization_ownership_test.go` - Organization ownership scenarios cơ bản
13. `organization_ownership_full_test.go` - **MỚI** - Test đầy đủ organization ownership (10 test cases)
14. `scope_permissions_test.go` - **MỚI** - Test chi tiết Scope 0 vs Scope 1
15. `notification_test.go` - Notification APIs đầy đủ

---

## 📦 Unit Tests (trong api/)

| Package | File | Nội dung |
|---------|------|----------|
| crm/service | `service.crm.snapshot_test.go` | BuildCurrentMetricsSnapshot, BuildSnapshotForNewCustomer |
| report/layer3 | `layer3_test.go` | DeriveFromNested, DeriveFromMap (First, Engaged, nil) |

---

## 📝 Ghi Chú

- ✅ = Đã có test
- ⚠️ = Chưa có test hoặc test chưa đầy đủ
- ❌ = Không cần test (deprecated hoặc internal)

### Organization Context
Tất cả các endpoints CRUD cho collections có `organizationId` sẽ:
- Tự động gán `organizationId` khi insert/upsert (nếu có header `X-Active-Role-ID`)
- Tự động filter theo `organizationId` khi query (bao gồm parent organizations)
- Validate quyền truy cập khi update/delete

### Test Coverage Goal
- **Target**: 80%+ endpoint coverage
- **Current**: ~60% (ước tính)
- **Focus**: Test các endpoints đặc biệt và organization context

