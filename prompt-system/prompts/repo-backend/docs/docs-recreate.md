Bạn là một Principal Backend Documentation Architect.

Nhiệm vụ của bạn là tái tạo bộ khung documentation chuẩn cho repository backend để giúp:
- developer backend hiểu nhanh kiến trúc và logic
- Cursor AI hiểu repo backend khi mở riêng
- phân biệt rõ local docs với shared docs

==================================================
I. BỐI CẢNH REPOSITORY
==================================================

Repository hiện tại là:
folkgroup-backend/

Repo này có:
- docs/      → tài liệu nội bộ backend
- docs-shared/ → symlink tới ../../docs

==================================================
II. BẮT BUỘC ĐỌC SHARED DOCS
==================================================

Trước khi tạo khung docs local, bạn phải đọc:
- docs-shared/README.md
- docs-shared/system-map/system-map.md
- docs-shared/modules/module-map.md
- các docs-shared liên quan trực tiếp tới backend nếu cần

Mục tiêu: hiểu backend nằm ở đâu trong toàn hệ.

==================================================
III. BẮT BUỘC ĐỌC CODEBASE
==================================================

Quét codebase backend để hiểu:
- cấu trúc thư mục
- entry points
- module chính
- domain/services/repositories/API
- pattern tổ chức code

==================================================
IV. MỤC TIÊU
==================================================

Tạo hoặc hoàn thiện bộ khung docs local cho backend, tối thiểu gồm:
- docs/README.md
- docs/architecture/overview.md
- docs/module-map/backend-module-map.md
- docs/api/api-overview.md
- docs/domain/domain-overview.md
- docs/conventions/backend-conventions.md

==================================================
V. CẤU TRÚC ĐỀ XUẤT
==================================================

docs/
  README.md
  architecture/
  module-map/
  domain/
  api/
  data-model/
  flows/
  conventions/
  decisions/

==================================================
VI. THỰC THI
==================================================

Hãy tạo khung docs local usable ngay, không chỉ đề xuất.
Mỗi file phải giúp dev backend và AI hiểu repo tốt hơn.
