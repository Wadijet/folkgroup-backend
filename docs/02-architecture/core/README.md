# Kiến Trúc Cốt Lõi (Core Architecture)

Thư mục này chứa các tài liệu về kiến trúc cốt lõi của hệ thống, bao gồm:

## 📚 Tài Liệu

### Tổng Quan
- **[tong-quan.md](./tong-quan.md)** - Tổng quan kiến trúc hệ thống

### Authentication & Authorization
- **[authentication.md](./authentication.md)** - Luồng xác thực
- **[rbac.md](./rbac.md)** - Hệ thống phân quyền (Role-Based Access Control)
- **[firebase-auth-voi-database.md](./firebase-auth-voi-database.md)** - Firebase Authentication với Database
- **[multi-provider-authentication.md](./multi-provider-authentication.md)** - Xác thực đa nhà cung cấp
- **[user-identifiers.md](./user-identifiers.md)** - Định danh người dùng
- **[xu-ly-trung-lap-tai-khoan.md](./xu-ly-trung-lap-tai-khoan.md)** - Xử lý trùng lặp tài khoản

### Database & Organization
- **[database.md](./database.md)** - Cấu trúc database
- **[organization.md](./organization.md)** - Cấu trúc tổ chức

### Intelligence Pipeline
- **[activity-framework.md](./activity-framework.md)** - Activity Framework — event stream, snapshot
- **[learning-engine.md](./learning-engine.md)** - Learning Engine (Decision Brain) — learning memory layer
- **[rule-intelligence.md](./rule-intelligence.md)** - Rule Intelligence — đề xuất kiến trúc module biến đổi pipeline
- **[ads-metrics-pipeline.md](./ads-metrics-pipeline.md)** - Ads Metrics Pipeline — raw → layer1/2/3 → flag → action (trích từ codebase)

## 🎯 Mục Đích

Các tài liệu trong thư mục này mô tả các thành phần cốt lõi và nền tảng của hệ thống, là nền tảng để hiểu các phần khác của kiến trúc.
