Bạn là một Principal System Architect và Documentation Architect.

Nhiệm vụ của bạn là tái tạo bộ khung documentation chuẩn cho toàn workspace để giúp:
- con người hiểu nhanh toàn bộ hệ thống
- Cursor AI hiểu kiến trúc workspace
- các repo giao tiếp thông qua docs

Bạn phải thực hiện thật trên file system, không chỉ đề xuất.

==================================================
I. BỐI CẢNH WORKSPACE
==================================================

Workspace có các repository chính:
- folkgroup-backend/
- folkgroup-frontend/
- folkgroup-agent/

Ngoài ra có:
- docs/
- scripts/

`docs/` là source of truth cho tài liệu dùng chung.
Mỗi repo có:
- repo/docs/
- repo/docs-shared/ → symlink tới ../../docs

==================================================
II. MỤC TIÊU
==================================================

Bạn cần tạo hoặc hoàn thiện bộ khung documentation cho workspace docs, bao gồm tối thiểu:
1. docs/README.md
2. docs/system-map/system-map.md
3. docs/modules/module-map.md
4. README cho từng nhánh chính

Ba mục tiêu lớn:
- tạo entry point rõ ràng
- tạo cấu trúc docs chuẩn
- tạo navigation cho AI và developer

==================================================
III. CÁCH LÀM
==================================================

Bắt đầu bằng:
1. quét toàn bộ docs hiện có trong workspace
2. xác định file nào đã có thể tái sử dụng
3. nếu chưa có thì tạo mới
4. tạo khung thư mục chuẩn và các README/index cần thiết

==================================================
IV. CẤU TRÚC ĐỀ XUẤT
==================================================

Thiết kế lại hoặc hoàn thiện cấu trúc docs theo hướng:

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

Mỗi thư mục nên có README.md làm index.

==================================================
V. NỘI DUNG CẦN TẠO
==================================================

1. docs/README.md
- overview
- docs structure
- navigation links
- hướng dẫn cho developer
- hướng dẫn cho Cursor AI

2. docs/system-map/system-map.md
- mô tả kiến trúc tổng thể
- repo boundaries
- luồng tương tác giữa backend / frontend / agent
- docs đóng vai trò communication protocol

3. docs/modules/module-map.md
- danh sách module chính
- mục đích
- repo sở hữu logic chính
- repo nào sử dụng module
- docs liên quan

4. README cho các nhánh:
- docs/architecture/README.md
- docs/modules/README.md
- docs/api-contracts/README.md
- docs/tasks/README.md
- docs/decision-logs/README.md

==================================================
VI. NGUYÊN TẮC VIẾT TÀI LIỆU
==================================================

Tài liệu phải:
- rõ ràng
- có heading hợp lý
- có navigation
- có cross-link khi cần
- không viết lan man
- AI-friendly: mỗi file có một mục đích tương đối rõ

==================================================
VII. THỰC THI
==================================================

Hãy thực hiện thật:
- tạo các thư mục còn thiếu
- tạo các file còn thiếu
- nâng cấp các file đã có
- ưu tiên entry point, system map, module map và index

==================================================
VIII. MỤC TIÊU CUỐI
==================================================

Sau khi hoàn thành:
- developer mới có thể hiểu hệ thống nhanh
- Cursor AI có entry point rõ ràng
- docs trở thành communication protocol giữa các repo
