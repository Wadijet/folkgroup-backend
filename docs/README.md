# 📚 Tài Liệu Hệ Thống FolkForm Backend

Chào mừng đến với tài liệu hệ thống FolkForm Backend. Tài liệu này được tổ chức theo cấu trúc logic để giúp developer và Cursor AI dễ dàng tìm kiếm và sử dụng.

---

## 📋 Index Summary — Tìm nhanh

| Index | File | Mục đích |
|-------|------|----------|
| **AI Commerce OS** | [../docs-shared/architecture/ai-commerce-os-overview.md](../docs-shared/architecture/ai-commerce-os-overview.md) | Vision — chúng ta đang làm gì (đọc đầu) |
| **Architecture** | [architecture/overview.md](architecture/overview.md) | Layers, flow request |
| **Module Map** | [module-map/backend-module-map.md](module-map/backend-module-map.md) | Module → code, router |
| **Domain** | [domain/domain-overview.md](domain/domain-overview.md) | Domain logic |
| **API** | [api/api-overview.md](api/api-overview.md) | API surface |
| **Conventions** | [conventions/backend-conventions.md](conventions/backend-conventions.md) | Quy ước backend |

---

## 🤖 Cursor AI — Thứ Tự Đọc (Repo Mode)

Khi mở riêng repo backend, Cursor nên đọc theo thứ tự:

1. **docs/README.md** (file này) — Entry point, biết repo làm gì
2. **[docs-shared/architecture/ai-commerce-os-overview.md](../docs-shared/architecture/ai-commerce-os-overview.md)** — Vision — chúng ta đang làm gì
3. **[architecture/overview.md](architecture/overview.md)** — Kiến trúc layers
4. **[module-map/backend-module-map.md](module-map/backend-module-map.md)** — Map module → code, biết sửa ở đâu
5. **[domain/domain-overview.md](domain/domain-overview.md)** — Domain logic
6. **[api/api-overview.md](api/api-overview.md)** — API surface
7. **docs theo task** — 05-development/, 02-architecture/core/, 01-getting-started/
8. **docs-shared/** — Khi cần context hệ thống, API contract, module design cross-repo

**Khi task chạm repo khác:** Đọc `docs-shared/architecture/ai-commerce-os-overview.md`, `docs-shared/system-map/system-map.md` và `docs-shared/modules/module-map.md` trước.

---

## 📂 Local Docs vs Shared Docs

| Loại | Vị trí | Nội dung |
|------|--------|----------|
| **Backend local** | `docs/` (đây) | Kiến trúc nội bộ, handler/service pattern, conventions, development guide |
| **Shared** | `docs-shared/` (junction → workspace docs) | API contract, system map, module design dùng chung, ai-context |

**Quy tắc:** Tài liệu chỉ backend → `docs/`. Tài liệu cross-repo (API contract, design) → `docs-shared/`. Xem [doc-ownership](../docs-shared/doc-ownership.md) (khi junction đã thiết lập).

---

## 📑 Mục Lục

### 1. 🚀 Bắt Đầu (Getting Started)

- [Cài Đặt và Cấu Hình](01-getting-started/cai-dat.md) - Hướng dẫn cài đặt từ đầu
- [Cấu Hình Môi Trường](01-getting-started/cau-hinh.md) - Chi tiết về biến môi trường
- [Khởi Tạo Hệ Thống](01-getting-started/khoi-tao.md) - Quy trình khởi tạo hệ thống lần đầu

### 2. 🏗️ Kiến Trúc (Architecture)

- **[Tổng quan kiến trúc](architecture/overview.md)** — Entry point kiến trúc (layers, flow)
- **[Bản đồ module backend](module-map/backend-module-map.md)** — Map module → code, router (⭐ bắt đầu khi implement feature)
- [02-architecture/core/tong-quan.md](02-architecture/core/tong-quan.md) - Kiến trúc cốt lõi
- [02-architecture/core/activity-framework.md](02-architecture/core/activity-framework.md) - Activity framework (event backbone)
- [02-architecture/core/decision-brain.md](02-architecture/core/decision-brain.md) - Decision Brain (learning memory)

### 3. 🔌 API Reference

- **[API Overview](api/api-overview.md)** — Tổng quan module, endpoint (⭐ nhìn nhanh)
- **Chi tiết đầy đủ:** [docs-shared/ai-context/folkform/api-context.md](../docs-shared/ai-context/folkform/api-context.md)

### 4. 🚢 Triển Khai (Deployment)

- [Firebase Setup](04-deployment/firebase.md) - Cài đặt và cấu hình Firebase

### 5. 💻 Phát Triển (Development)

- [Quy Trình Refactor Docs](05-development/QUY_TRINH_REFACTOR_DOCS.md) - Quy trình AI refactor tài liệu
- [Quy Trình Refactor .cursor](05-development/QUY_TRINH_REFACTOR_CURSOR.md) - Refactor .cursor sau khi docs xong
- [Cấu Trúc Code](05-development/cau-truc-code.md) - Cấu trúc và tổ chức code

### 6. 🤖 AI Context & API Contract (Shared)

**📍 Canonical:** `docs-shared/ai-context/` (junction tới workspace docs)

- [FolkForm API Context](../docs-shared/ai-context/folkform/api-context.md) — Tài liệu chính về API (⭐ **BẮT ĐẦU TỪ ĐÂY** khi gọi/thêm endpoint)
- [AI Context README](../docs-shared/ai-context/README.md) — Hướng dẫn sử dụng
- [Notification System](../docs-shared/ai-context/folkform/notification-system.md) — Hệ thống notification

### 7. 📐 Quy Tắc Backend Cho AI (09-ai-context)

- [09-ai-context/README.md](09-ai-context/README.md) — Bảng quy tắc thiết kế
- [.cursor/rules/folkgroup-backend.mdc](../.cursor/rules/folkgroup-backend.mdc) — Cursor tự áp dụng

### 8. 📦 Tài Liệu Khác

- [data-model/](data-model/), [flows/](flows/), [decisions/](decisions/) — Khung tài liệu
- [08-archive/](08-archive/) — Tài liệu archive

## 🔍 Tìm Kiếm Nhanh

- **Architecture**: [architecture/overview.md](architecture/overview.md), [02-architecture/core/tong-quan.md](02-architecture/core/tong-quan.md), [02-architecture/core/activity-framework.md](02-architecture/core/activity-framework.md), [02-architecture/core/decision-brain.md](02-architecture/core/decision-brain.md)
- **Module map**: [module-map/backend-module-map.md](module-map/backend-module-map.md)
- **API**: [api/api-overview.md](api/api-overview.md), [docs-shared/ai-context/folkform/api-context.md](../docs-shared/ai-context/folkform/api-context.md)
- **Firebase**: [04-deployment/firebase.md](04-deployment/firebase.md)

## 📝 Ghi Chú

- Tất cả tài liệu được viết bằng **Tiếng Việt**
- Tài liệu được cập nhật thường xuyên, vui lòng kiểm tra phiên bản mới nhất
- Nếu có câu hỏi hoặc đề xuất, vui lòng tạo issue hoặc liên hệ team

## 🔄 Cập Nhật Gần Đây

- ✅ **2025-03-13**: Decision Brain — module learning memory (decision_cases), thiết kế + implement + docs
- ✅ **2025-03-13**: Activity Framework — CRM và Ads đã migrate xong (ActivityBase, LogActivity, RecordActivityForEntity). Agent chưa migrate sang ActivityBase. Cập nhật docs/02-architecture/core/activity-framework.md với trạng thái triển khai
- ✅ **2025-03-13**: Pipeline REPOSITORY-ONLY — sửa broken links (03-api, 02-architecture/systems); module map trỏ api-overview, docs-shared; architecture README cập nhật
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

