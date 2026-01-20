# 📚 Tài Liệu Hệ Thống FolkForm Auth Backend

Chào mừng đến với tài liệu hệ thống FolkForm Auth Backend. Tài liệu này được tổ chức theo cấu trúc logic để giúp bạn dễ dàng tìm kiếm và sử dụng.

## 📑 Mục Lục

### 1. 🚀 Bắt Đầu (Getting Started)

Tài liệu dành cho người mới bắt đầu:

- [Cài Đặt và Cấu Hình](01-getting-started/cai-dat.md) - Hướng dẫn cài đặt từ đầu
- [Cấu Hình Môi Trường](01-getting-started/cau-hinh.md) - Chi tiết về biến môi trường
- [Khởi Tạo Hệ Thống](01-getting-started/khoi-tao.md) - Quy trình khởi tạo hệ thống lần đầu
- [Tài Liệu Ngắn Gọn](01-getting-started/tai-lieu-he-thong.md) - Tổng quan nhanh về hệ thống

### 2. 🏗️ Kiến Trúc (Architecture)

Tài liệu về kiến trúc và thiết kế hệ thống được tổ chức thành các danh mục:

- **[Core Architecture](02-architecture/core/)** - Kiến trúc cốt lõi (Authentication, RBAC, Database, Organization)
- **[Systems](02-architecture/systems/)** - Các hệ thống (Logging, Worker, Notification, AI & Content, Agent)
- **[Design Proposals](02-architecture/design/)** - Đề xuất thiết kế và proposal
- **[Analysis & Audits](02-architecture/analysis/)** - Phân tích, đánh giá và kiểm tra
- **[Refactoring](02-architecture/refactoring/)** - Tài liệu tái cấu trúc
- **[Business Logic](02-architecture/business-logic/)** - Logic nghiệp vụ và quy tắc xử lý
- **[Solutions](02-architecture/solutions/)** - Giải pháp kỹ thuật cụ thể
- **[Other](02-architecture/other/)** - Tài liệu hỗ trợ khác

**👉 Xem chi tiết:** [02-architecture/README.md](02-architecture/README.md)

### 3. 🔌 API Reference

Tài liệu về các API endpoints:

- [Authentication APIs](03-api/authentication.md) - API đăng nhập, đăng xuất
- [User Management APIs](03-api/user-management.md) - API quản lý người dùng
- [RBAC APIs](03-api/rbac.md) - API quản lý role và permission
- [Admin APIs](03-api/admin.md) - API quản trị hệ thống
- [Facebook Integration APIs](03-api/facebook.md) - API tích hợp Facebook
- [Pancake Integration APIs](03-api/pancake.md) - API tích hợp Pancake
- [Agent Management APIs](03-api/agent.md) - API quản lý agent

### 4. 🚢 Triển Khai (Deployment)

Hướng dẫn triển khai hệ thống:

- [Triển Khai Production](04-deployment/production.md) - Hướng dẫn deploy production
- [Cấu Hình Server](04-deployment/cau-hinh-server.md) - Cấu hình server
- [MongoDB Setup](04-deployment/mongodb.md) - Cài đặt và cấu hình MongoDB
- [Firebase Setup](04-deployment/firebase.md) - Cài đặt và cấu hình Firebase
- [Systemd Service](04-deployment/systemd.md) - Cấu hình systemd service

### 5. 💻 Phát Triển (Development)

Hướng dẫn cho developers:

- [Cấu Trúc Code](05-development/cau-truc-code.md) - Cấu trúc và tổ chức code
- [Thêm API Mới](05-development/them-api-moi.md) - Hướng dẫn thêm API endpoint
- [Thêm Service Mới](05-development/them-service-moi.md) - Hướng dẫn thêm service
- [Coding Standards](05-development/coding-standards.md) - Tiêu chuẩn code
- [Git Workflow](05-development/git-workflow.md) - Quy trình làm việc với Git

### 6. 🧪 Testing

Hướng dẫn testing:

- [Tổng Quan Testing](06-testing/tong-quan.md) - Tổng quan về testing
- [Chạy Test Suite](06-testing/chay-test.md) - Hướng dẫn chạy test
- [Viết Test Case](06-testing/viet-test.md) - Hướng dẫn viết test case
- [Test Reports](06-testing/bao-cao-test.md) - Xem và phân tích báo cáo test

### 7. 🔧 Xử Lý Sự Cố (Troubleshooting)

Hướng dẫn xử lý các vấn đề thường gặp:

- [Lỗi Thường Gặp](07-troubleshooting/loi-thuong-gap.md) - Các lỗi và cách xử lý
- [Debug Guide](07-troubleshooting/debug.md) - Hướng dẫn debug
- [Log Analysis](07-troubleshooting/phan-tich-log.md) - Phân tích log
- [Performance Issues](07-troubleshooting/performance.md) - Vấn đề hiệu năng

## 📖 Tài Liệu Tham Khảo

### Tài Liệu Kỹ Thuật (Architecture)

Tài liệu kiến trúc đã được tổ chức lại theo danh mục. Xem [02-architecture/README.md](02-architecture/README.md) để tìm tài liệu cụ thể:

- **Core:** [Firebase Authentication](02-architecture/core/firebase-auth-voi-database.md), [Multi-Provider Auth](02-architecture/core/multi-provider-authentication.md), [User Identifiers](02-architecture/core/user-identifiers.md)
- **Systems:** [Logging System](02-architecture/systems/logging-system-usage.md), [Content & AI](02-architecture/systems/content-strategy-os-backend-design.md)
- **Analysis:** [Project Review](02-architecture/analysis/comprehensive-project-review.md), [Code Audits](02-architecture/analysis/)

### Tài Liệu Deployment

- [Hướng Dẫn Cài Đặt Firebase](04-deployment/huong-dan-cai-dat-firebase.md)
- [Hướng Dẫn Đăng Ký Firebase](04-deployment/huong-dan-dang-ky-firebase.md)

### Tài Liệu Testing

- [Hướng Dẫn Lấy Firebase Token cho Test](06-testing/huong-dan-lay-firebase-token-cho-test.md)

### 9. 🤖 AI Context Documentation

**📍 Tài liệu AI Context đã được di chuyển lên workspace-level**

Tài liệu context chi tiết, đầy đủ để cung cấp cho AI assistants (ChatGPT, Claude, Cursor AI, v.v.) để xây dựng frontend application:

**Vị trí mới:** `../../docs/ai-context/` (Workspace-level)

- [AI Context README](../../docs/ai-context/README.md) - Hướng dẫn sử dụng tài liệu AI context
- [FolkForm API Context](../../docs/ai-context/folkform-api-context.md) - Tài liệu chính về API (⭐ **BẮT ĐẦU TỪ ĐÂY**)
- [TypeScript Types & Interfaces](../../docs/ai-context/types-and-interfaces.md) - Tất cả TypeScript types
- [Frontend Implementation Guide](../../docs/ai-context/frontend-implementation-guide.md) - Hướng dẫn implementation
- [Code Examples](../../docs/ai-context/examples.md) - Ví dụ code cho React, Vue, Angular, Vanilla JS

### Tài Liệu Archive

Các tài liệu phân tích và báo cáo cũ được lưu trong [08-archive/](08-archive/) để tham khảo.

## 🔍 Tìm Kiếm Nhanh

### Theo Chủ Đề

- **Authentication**: [Authentication Flow](02-architecture/core/authentication.md), [Firebase Auth](02-architecture/core/firebase-auth-voi-database.md)
- **RBAC**: [RBAC System](02-architecture/core/rbac.md), [RBAC APIs](03-api/rbac.md)
- **Firebase**: [Firebase Setup](04-deployment/firebase.md), [Firebase Auth](02-architecture/core/firebase-auth-voi-database.md)
- **Testing**: [Testing Guide](06-testing/tong-quan.md), [README_TEST.md](../README_TEST.md)
- **Deployment**: [Production Deployment](04-deployment/production.md), [MongoDB Setup](04-deployment/mongodb.md)
- **Architecture**: Xem [02-architecture/README.md](02-architecture/README.md) để tìm tài liệu theo danh mục

### Theo Vai Trò

- **Developer Mới**: Bắt đầu với [Tài Liệu Ngắn Gọn](01-getting-started/tai-lieu-he-thong.md) và [Cài Đặt](01-getting-started/cai-dat.md)
- **Backend Developer**: Xem [Kiến Trúc](02-architecture/), [API Reference](03-api/), và [Development Guide](05-development/)
- **Frontend Developer**: Xem [AI Context Documentation](../../docs/ai-context/) - Tài liệu đầy đủ để xây dựng frontend
- **DevOps**: Xem [Deployment](04-deployment/) và [Troubleshooting](07-troubleshooting/)
- **QA/Tester**: Xem [Testing Guide](06-testing/)

## 📝 Ghi Chú

- Tất cả tài liệu được viết bằng **Tiếng Việt**
- Tài liệu được cập nhật thường xuyên, vui lòng kiểm tra phiên bản mới nhất
- Nếu có câu hỏi hoặc đề xuất, vui lòng tạo issue hoặc liên hệ team

## 🔄 Cập Nhật Gần Đây

- ✅ **2025-01-20**: Tổ chức lại 67 files trong 02-architecture/ thành 8 thư mục con theo chủ đề
- ✅ **2025-01-20**: Tạo README.md cho mỗi thư mục con để dễ điều hướng
- ✅ **2025-01-20**: Di chuyển analysis/ và solutions/ vào cấu trúc 02-architecture/
- ✅ **2025-01-20**: Gộp các file trùng lặp và outdated - giảm từ ~76 files xuống còn 60 files
- ✅ Tổ chức lại hệ thống tài liệu theo cấu trúc chuẩn
- ✅ Tạo README.md chính và docs/README.md
- ✅ Tạo đầy đủ tài liệu API Reference (7 files)
- ✅ Tạo đầy đủ tài liệu Deployment (5 files)
- ✅ Tạo đầy đủ tài liệu Development (5 files)
- ✅ Tạo đầy đủ tài liệu Testing (4 files)
- ✅ Tạo đầy đủ tài liệu Troubleshooting (4 files)
- ✅ Tạo thư mục AI Context Documentation (5 files) cho frontend development

---

**Lưu ý**: Tất cả tài liệu mới đều nằm trong các thư mục con được tổ chức (01-getting-started, 02-architecture, v.v.). Các tài liệu cũ trong thư mục gốc vẫn được giữ lại để tham khảo.

