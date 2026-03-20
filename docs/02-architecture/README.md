# 🏗️ Kiến Trúc Hệ Thống (Architecture)

Thư mục này chứa tất cả tài liệu về kiến trúc, thiết kế và phân tích hệ thống. Tài liệu được tổ chức theo các danh mục chính để dễ dàng tìm kiếm và tham khảo.

## 📑 Cấu Trúc Thư Mục

### 🔷 Vision — Định Hướng Chiến Lược
Tài liệu vision về AI Commerce OS Platform L1 (nguồn: **docs-shared**):

- **Tổng quan:** AI Commerce OS — toàn bộ hệ Platform L1 (Content OS → Ads → CIO → Customer → Order → Decision Brain)
- Phần 1: Customer Intelligence Core — Unified Profile
- Phần 2: AI Application Layer — Bán hàng & Chăm sóc
- Phần 3: Customer Interaction Orchestrator (CIO)

**👉 Nguồn duy nhất:** [docs-shared/architecture/vision/](../../../docs-shared/architecture/vision/) · [vision/README.md](./vision/README.md) (redirect)

### 🔷 [Core](./core/) - Kiến Trúc Cốt Lõi
Tài liệu về các thành phần cốt lõi của hệ thống:
- Tổng quan kiến trúc
- Authentication & Authorization
- Database & Organization

**👉 Xem chi tiết:** [core/README.md](./core/README.md)

### 🔷 [Systems](./systems/) - Hệ Thống
Tài liệu về các hệ thống và module chính:
- Logging System
- Worker System
- Notification System
- AI & Content System
- Agent System

**👉 Xem chi tiết:** [systems/README.md](./systems/README.md)

### 🔷 [Design](./design/) - Đề Xuất Thiết Kế
Các đề xuất thiết kế và proposal:
- System Design Proposals
- Architecture Proposals
- Code Improvement Proposals

**👉 Xem chi tiết:** [design/README.md](./design/README.md)

### 🔷 [Analysis](./analysis/) - Phân Tích & Kiểm Tra
Các tài liệu phân tích, đánh giá và kiểm tra:
- Project Reviews
- Code Audits
- Code Analysis
- Safety & Crash Analysis

**👉 Xem chi tiết:** [analysis/README.md](./analysis/README.md)

### 🔷 [Refactoring](./refactoring/) - Tái Cấu Trúc
Tài liệu về các hoạt động tái cấu trúc:
- Business Logic Refactoring
- Layer Separation
- Code Refactoring

**👉 Xem chi tiết:** [refactoring/README.md](./refactoring/README.md)

### 🔷 [Business Logic](./business-logic/) - Logic Nghiệp Vụ
Tài liệu về logic nghiệp vụ và quy tắc xử lý:
- Business Requirements
- Data Sharing & Authorization
- Organization Management
- Relationship & Protection

**👉 Xem chi tiết:** [business-logic/README.md](./business-logic/README.md)

### 🔷 [Solutions](./solutions/) - Giải Pháp
Các giải pháp kỹ thuật cụ thể:
- Extract Tag Specification
- Và các giải pháp khác

**👉 Xem chi tiết:** [solutions/README.md](./solutions/README.md)

### 🔷 [Other](./other/) - Khác
Các tài liệu hỗ trợ và hướng dẫn:
- Workspace & Git Setup
- AI Context
- Services Summary

**👉 Xem chi tiết:** [other/README.md](./other/README.md)

## 🚀 Bắt Đầu Nhanh

### Cho Người Mới
1. Bắt đầu với [Core Architecture](./core/tong-quan.md) để hiểu tổng quan hệ thống
2. Xem [Authentication](./core/authentication.md) và [RBAC](./core/rbac.md) để hiểu bảo mật
3. Đọc [Database Schema](./core/database.md) để hiểu cấu trúc dữ liệu

### Cho Developer
1. Xem [Layer Separation Principles](./refactoring/layer-separation-principles.md) để hiểu cấu trúc code
2. Đọc [Business Logic](./business-logic/) để hiểu logic nghiệp vụ
3. Tham khảo [Design Proposals](./design/) để biết hướng phát triển

### Cho Architect
1. Xem [Comprehensive Project Review](./analysis/comprehensive-project-review.md)
2. Đọc các [Design Proposals](./design/) để hiểu quyết định thiết kế
3. Tham khảo [Analysis](./analysis/) để xem các đánh giá và kiểm tra

## 🔍 Tìm Kiếm Theo Chủ Đề

### Authentication & Security
- [Authentication Flow](./core/authentication.md)
- [RBAC System](./core/rbac.md)
- [Firebase Auth](./core/firebase-auth-voi-database.md)
- [Multi-Provider Auth](./core/multi-provider-authentication.md)

### Database & Data
- [Database Schema](./core/database.md)
- [Organization Structure](./core/organization.md)
- [Data Authorization](./business-logic/organization-data-authorization.md)

### Systems
- [Logging System](./systems/logging-system-usage.md)
- [Worker System](./systems/worker-system.md)
- [Notification System](./systems/notification-processing-rules.md)
- [Content & AI System](./systems/content-strategy-os-backend-design.md)

### Code Quality
- [Project Review](./analysis/comprehensive-project-review.md)
- [Code Audits](./analysis/)
- [Refactoring](./refactoring/)

## 📝 Ghi Chú

- Tất cả tài liệu được viết bằng **Tiếng Việt**
- Cấu trúc này được tổ chức lại vào **2025-01-20** để dễ dàng quản lý và tìm kiếm
- Mỗi thư mục con đều có README.md riêng với mô tả chi tiết

## 🔄 Cập Nhật Gần Đây

- ✅ **2025-01-20**: Tổ chức lại 67 files thành 8 thư mục con theo chủ đề
- ✅ **2025-01-20**: Tạo README.md cho mỗi thư mục con
- ✅ **2025-01-20**: Di chuyển analysis/ và solutions/ vào cấu trúc mới
- ✅ **2025-01-20**: Gộp các file trùng lặp và outdated:
  - Gộp log-filter-system (2 files → 1 file)
  - Gộp notification-domain-severity (4 files → 1 file)
  - Gộp project-review (2 files → 1 file)
  - Gộp CRUD override audits (4 files → 1 file)
  - Gộp panic safety (5 files → 1 file)
  - Gộp Content & AI (2 files → 1 file)
  - Gộp custom endpoints (2 files → 1 file)
- ✅ **Kết quả**: Giảm từ ~76 files xuống còn 60 files (không tính README)
