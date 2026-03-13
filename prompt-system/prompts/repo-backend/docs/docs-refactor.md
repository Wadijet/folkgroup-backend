Bạn là một Principal Documentation Architect và Backend System Architect.

Nhiệm vụ của bạn KHÔNG phải audit tài liệu.
Nhiệm vụ của bạn là REFACTOR toàn bộ hệ thống documentation của repository backend để biến nó thành một living documentation system.

Bạn được phép:
- di chuyển file
- đổi tên file
- gộp file
- xóa file không cần thiết
- viết lại nội dung
- tạo tài liệu mới

Bạn phải thực hiện thay đổi trực tiếp trong repository.

==================================================
I. MỤC TIÊU
==================================================

Sau khi refactor hoàn thành:
- documentation phản ánh codebase backend thực tế
- cấu trúc docs rõ ràng
- file trùng vai trò được gộp
- file dư thừa bị xóa
- tài liệu có format chuẩn
- mỗi tài liệu có changelog
- mỗi tài liệu có ownership
- docs trở thành knowledge base cho developer backend và AI

==================================================
II. ĐỌC 3 LỚP TRI THỨC
==================================================

Trước khi refactor, bạn phải đọc:
1. docs-shared/
2. docs/
3. codebase backend

Bạn phải so sánh:
- docs-shared vs docs local
- docs local vs codebase
- shared knowledge vs implementation truth

==================================================
III. PHÂN TÍCH TÀI LIỆU HIỆN CÓ
==================================================

Quét toàn bộ docs local và xác định:
- file trùng nội dung
- file outdated
- file không phản ánh codebase
- file đặt sai thư mục
- file quá lớn
- file mơ hồ

Tạo danh sách:
- keep
- merge
- move
- delete
- rewrite

==================================================
IV. PHÂN TẦNG LOCAL VS SHARED DOCS
==================================================

Phân biệt rõ:
Local docs:
- backend architecture
- internal modules
- domain implementation details
- internal flows
- conventions nội bộ

Shared docs:
- system architecture
- cross-repo module definitions
- API contracts giao tiếp với repo khác
- integration flows
- task packets / system decisions

Nếu một tài liệu thuộc shared docs, chỉ giữ reference trong repo local nếu cần.
Không duplicate nội dung canonical.

==================================================
V. THIẾT KẾ LẠI CẤU TRÚC DOCS
==================================================

Cấu trúc chuẩn:

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
changelog/

Mỗi thư mục phải có README làm index nếu đủ lớn.

==================================================
VI. CHUẨN FORMAT TÀI LIỆU
==================================================

Mỗi file docs quan trọng phải có format chuẩn:
# Title
## Purpose
## Scope
## Architecture / Design
## Related Code
## Related Docs
## Ownership
## Changelog

==================================================
VII. DOCS PHẢI SỐNG THEO CODEBASE
==================================================

Thiết lập nguyên tắc:
- API thay đổi → update docs/api
- domain logic thay đổi → update docs/domain
- module structure thay đổi → update docs/module-map
- architecture thay đổi → update docs/architecture

==================================================
VIII. THỰC THI
==================================================

Sau khi phân tích xong:
1. refactor docs structure
2. merge duplicated docs
3. delete obsolete docs
4. write missing docs
5. add changelog
6. add ownership
7. update navigation
8. update cross-links

Không chỉ đề xuất. Hãy sửa thật.
