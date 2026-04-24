# Quy hoạch đặt tên collection domain Order

Tài liệu này chuẩn hóa naming cho collection thuộc domain Order theo hướng **domain trước, lớp sau** để các bảng liên quan nằm gần nhau khi sắp xếp theo alphabet trong Mongo UI.

Ma trận rename toàn hệ và mô tả mục đích từng collection: `docs/05-development/COLLECTION_RENAME_MATRIX.md`.

## 1) Quy ước áp dụng

Định dạng:

`order_<layer>_<entity>`

Trong đó:

- `src`: mirror/raw từ nguồn ngoài.
- `core`: canonical của domain.
- `job`: hàng đợi xử lý bất đồng bộ.
- `run`: lịch sử chạy (append-only).
- `rm`: read model/snapshot phục vụ query nhanh.

## 2) Mapping đã áp dụng trong code

- `pc_pos_orders` -> `order_src_pcpos_orders`
- `manual_pos_orders` -> `order_src_manual_orders`
- `order_canonical` -> `order_core_records`
- `pc_pos_products` -> `order_src_pcpos_products`
- `manual_pos_products` -> `order_src_manual_products`
- `pc_pos_variations` -> `order_src_pcpos_variations`
- `manual_pos_variations` -> `order_src_manual_variations`
- `pc_pos_categories` -> `order_src_pcpos_categories`
- `manual_pos_categories` -> `order_src_manual_categories`
- `manual_pos_customers` -> `order_src_manual_customers`
- `manual_pos_shops` -> `order_src_manual_shops`
- `manual_pos_warehouses` -> `order_src_manual_warehouses`
- `order_intel_compute` -> `order_job_intel`
- `order_intel_runs` -> `order_run_intel`
- `order_intel_snapshots` -> `order_rm_intel`

Phạm vi đã cập nhật:

- Khai báo tên collection tại `api/cmd/server/init.go`.
- Danh sách đăng ký collection tại `api/cmd/server/init.registry.go`.
- Registry identity cho enrich 4 lớp tại `api/internal/utility/identity/registry.go`.
- Một số policy/logic có dùng literal tên cũ (emit policy, command center, period timestamp) để tương thích chuyển tiếp.

## 3) Lý do chọn domain-first

- Các collection của Order đứng gần nhau, dễ lọc và vận hành.
- Không trộn lẫn các domain khác khi nhìn theo prefix.
- Dễ mở rộng thêm nguồn mới như `order_src_manual_orders`, `order_src_marketplace_orders`.

## 4) Kế hoạch migrate dữ liệu production (khuyến nghị)

1. Tạo collection mới theo tên chuẩn.
2. Backfill dữ liệu từ collection cũ sang collection mới (giữ nguyên `_id` nếu chiến lược cho phép).
3. Build lại index cho collection mới theo model hiện tại.
4. Chạy dual-read/đối soát số lượng + checksum theo từng collection.
5. Cutover ứng dụng sang tên mới (phiên bản code hiện tại).
6. Giữ collection cũ ở chế độ chỉ đọc trong một cửa sổ an toàn trước khi dọn dẹp.

## 5) Nguyên tắc cho collection mới của Order

- Nguồn mới: `order_src_<nguon>_<entity>`.
- Canonical: luôn vào `order_core_records`.
- Queue xử lý: `order_job_<purpose>`.
- Lịch sử chạy: `order_run_<purpose>`.
- Read model: `order_rm_<purpose>`.

Ví dụ nguồn nhập tay độc lập:

- `order_src_manual_orders` (mirror nguồn nhập tay).
- `order_core_records` (canonical hợp nhất theo chuẩn domain Order).

## 6) API cho nguồn manual (đã mở trong backend)

- CRUD mirror manual theo nhóm Order:
  - `/api/v1/manual-pos/order`
  - `/api/v1/manual-pos/product`
  - `/api/v1/manual-pos/variation`
  - `/api/v1/manual-pos/category`
  - `/api/v1/manual-pos/customer`
  - `/api/v1/manual-pos/shop`
  - `/api/v1/manual-pos/warehouse`
- CIO ingest cho manual:
  - Domain `manual_order`, `manual_pos_product`, `manual_pos_variation`, `manual_pos_category`, `manual_pos_customer`, `manual_pos_shop`, `manual_pos_warehouse`
  - Alias ngắn tương ứng: `m_order`, `m_pos_product`, `m_pos_variation`, `m_pos_category`, `m_pos_customer`, `m_pos_shop`, `m_pos_warehouse`
