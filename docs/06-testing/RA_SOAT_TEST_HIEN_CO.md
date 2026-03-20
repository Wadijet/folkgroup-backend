# Rà Soát Phần Test Hiện Có

Tài liệu rà soát tổng quan phần test đã có trong codebase (cập nhật: 2025-03).

---

## 1. Tổng Quan Số Liệu

| Thành phần | Số lượng | Ghi chú |
|------------|----------|---------|
| **Integration test files** | 22 file | `api-tests/cases/*.go` |
| **Test cases (sub-tests)** | ~160 | Theo test-coverage-details.md |
| **Unit test files** | 1 file | `service.crm.snapshot_test.go` |
| **Test utilities** | 4 file | http_client, test_fixtures, test_helper, get_firebase_token |
| **Scripts** | 25+ script | test.ps1, manage_server, test-*, activate-*, v.v. |

---

## 2. Đã Có

### 2.1 Integration tests (api-tests)

| Module | File | Endpoints / Nội dung |
|--------|------|---------------------|
| System | `health_test.go` | GET /system/health |
| Auth | `auth_test.go`, `auth_additional_test.go` | login, logout, profile |
| Admin | `admin_test.go`, `admin_full_test.go` | block, unblock, set role |
| RBAC | `rbac_test.go` | Role, Permission, UserRole |
| CRUD | `crud_operations_test.go` | Role, Permission, User CRUD |
| Agent | `agent_test.go` | Agent CRUD, check-in/check-out |
| Facebook | `facebook_test.go` | AccessToken, Page, Post, Conversation, Message |
| Pancake | `pancake_test.go` | Pancake order |
| Notification | `notification_test.go` | Sender, Channel, Template, Routing, Trigger, History |
| Organization | `organization_*_test.go` | Data access, ownership, sharing |
| Scope | `scope_permissions_test.go` | Scope 0 vs Scope 1 |
| Error | `error_handling_test.go` | Token invalid, API not found |
| Middleware | `middleware_test.go`, `endpoint_middleware_test.go` | Middleware |
| Content | `content_storage_test.go` | Content storage |
| AI | `ai_service_test.go` | AI service |

### 2.2 Unit tests (api)

| Package | File | Hàm test |
|---------|------|----------|
| crm/service | `service.crm.snapshot_test.go` | BuildCurrentMetricsSnapshot, BuildSnapshotForNewCustomer |
| report/layer3 | `layer3_test.go` | DeriveFromNested, DeriveFromMap (First, Engaged, nil cases) |

### 2.3 Scripts & tools

- `test.ps1` – Script chính: build server, start, run tests, báo cáo
- `-UnitOnly` – Chỉ chạy unit tests (không cần server)
- `-SkipServer` – Bỏ qua khởi động server
- `manage_server.ps1` – Start/stop/status server
- Report Markdown tự động trong `reports/`

### 2.4 Tài liệu

- `tong-quan.md`, `chay-test.md`, `viet-test.md`, `bao-cao-test.md`
- `endpoint-coverage-checklist.md`, `test-coverage-details.md`
- `DE_XUAT_CO_CHE_TEST_TU_DONG.md`

---

## 3. Gaps / Chưa Có

### 3.1 Endpoint coverage (~60%, target 80%)

- Role: insert-many, update, delete
- Organization: insert, update, delete
- Agent: insert, find, update, delete (chỉ có check-in/check-out)
- Facebook: nhiều endpoint đặc biệt (find-by-page-id, update-token, upsert-messages, v.v.)
- Pancake POS: toàn bộ shop, warehouse, product, variation, category, order
- Admin: set-administrator, sync-administrator-permissions
- Init: status, organization, permissions, roles, admin-user

### 3.2 Unit tests

- Đã có: crm/snapshot, report/layer3
- Thiếu: ruleintel/engine, ads/service, utility/data.extract

### 3.3 CI/CD

- Không có GitHub Actions / GitLab CI
- Không có pre-commit hook

### 3.4 Phụ thuộc integration

- Cần `TEST_FIREBASE_ID_TOKEN` cho hầu hết test
- Cần server + MongoDB
- Có thể skip khi không có token (nhưng nhiều test bị skip)

---

## 4. Chất Lượng

| Tiêu chí | Đánh giá |
|----------|----------|
| Helper/fixtures | Tốt – SetupTestWithAdminUser, CreateAdminUser, InitData |
| HTTP client | Tốt – SetToken, SetActiveRoleID, GET/POST/PUT/DELETE |
| Error handling | Skip khi thiếu token, không crash |
| Report | Tự động – Markdown với pass/fail, duration |
| Isolation | Mỗi test dùng user/role riêng, có cleanup |

---

## 5. Ưu Tiên Mở Rộng

1. **Unit tests** – Thêm report/layer3, utility (data.extract)
2. **UnitOnly trong test.ps1** – Đã thêm
3. **Smoke test** – Health test không cần token (đã có)
4. **Integration** – Bổ sung theo endpoint-coverage-checklist (Priority 1–2)
5. **CI** – GitHub Actions cho unit tests

---

## 6. Tài Liệu Liên Quan

- [Endpoint Coverage Checklist](endpoint-coverage-checklist.md)
- [Test Coverage Details](test-coverage-details.md)
- [Đề Xuất Cơ Chế Test Tự Động](DE_XUAT_CO_CHE_TEST_TU_DONG.md)
