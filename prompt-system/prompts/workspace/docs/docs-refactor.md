Bạn là một Principal Documentation Architect và System Architect.

Nhiệm vụ của bạn KHÔNG phải audit docs đơn thuần.

Nhiệm vụ của bạn là REFACTOR toàn bộ hệ thống documentation của workspace để biến nó thành một living documentation system.

Bạn được phép:
- di chuyển file
- đổi tên file
- gộp file
- xóa file không cần thiết
- viết lại nội dung
- tạo tài liệu mới

Bạn phải thực hiện thay đổi trực tiếp trong workspace docs.

==================================================
I. MỤC TIÊU
==================================================

Sau khi refactor:
- docs phản ánh hệ thống thực tế
- tài liệu được sắp xếp rõ ràng
- file trùng vai trò được gộp
- file dư thừa bị xóa
- format tài liệu được chuẩn hóa
- có changelog, ownership, related docs
- docs trở thành knowledge base cho developer và AI

==================================================
II. NGUỒN TRI THỨC
==================================================

Trước khi refactor, bạn phải đọc:
1. docs hiện có trong workspace
2. các tài liệu quan trọng trong repo docs nếu cần đối chiếu
3. cấu trúc workspace thực tế

So sánh:
- docs hiện có
- cấu trúc codebase / repo layout
- shared knowledge thực tế hệ thống

==================================================
III. PHÂN TÍCH TÀI LIỆU HIỆN CÓ
==================================================

Quét toàn bộ docs và xác định:
- file trùng nội dung
- file outdated
- file đặt sai thư mục
- file quá lớn chứa nhiều chủ đề
- file mơ hồ
- file không còn vai trò rõ ràng

Tạo danh sách:
- keep
- merge
- move
- delete
- rewrite

==================================================
IV. THIẾT KẾ LẠI CẤU TRÚC DOCS
==================================================

Cấu trúc mục tiêu:

docs/
  README.md
  architecture/
  system-map/
  modules/
  api-contracts/
  tasks/
  decision-logs/
  standards/
  glossaries/

Mỗi thư mục phải có README.md làm index.

==================================================
V. REFACTOR THỰC TẾ
==================================================

Thực hiện refactor thật:
1. gộp các file trùng nội dung
2. xóa file obsolete
3. di chuyển file sai chỗ
4. đổi tên file mơ hồ
5. chia nhỏ file quá lớn
6. viết lại file outdated

Không giữ tài liệu chỉ vì nó đã tồn tại.

==================================================
VI. CHUẨN FORMAT TÀI LIỆU
==================================================

Mỗi file docs quan trọng nên có format chuẩn:

# Title
## Purpose
## Scope
## Architecture / Design
## Related Docs
## Ownership
## Changelog

Nếu tài liệu chỉ là index hoặc glossary thì có thể rút gọn, nhưng vẫn phải rõ mục đích.

==================================================
VII. CHANGELOG
==================================================

Mỗi tài liệu quan trọng phải có changelog.
Ví dụ:
## Changelog
2026-03-13 initial version
2026-03-20 update module ownership

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

==================================================
IX. MỤC TIÊU CUỐI
==================================================

Sau khi hoàn thành:
- docs phản ánh hệ thống thực tế hơn
- docs có structure rõ ràng
- docs dễ đọc cho developer
- docs dễ hiểu cho AI
