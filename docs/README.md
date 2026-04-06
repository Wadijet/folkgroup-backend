# 📚 Tài Liệu Hệ Thống FolkForm Backend

Chào mừng đến với tài liệu hệ thống FolkForm Backend. Tài liệu này được tổ chức theo cấu trúc logic để giúp developer và Cursor AI dễ dàng tìm kiếm và sử dụng.

---

## 📋 Index Summary — Tìm nhanh

| Index | File | Mục đích |
|-------|------|----------|
| **AI Commerce OS** | [docs-shared/architecture/vision/00 - ai-commerce-os-platform-l1.md](../docs-shared/architecture/vision/00%20-%20ai-commerce-os-platform-l1.md) | Vision Platform L1 — toàn bộ hệ (đọc đầu) |
| **Architecture** | [architecture/overview.md](architecture/overview.md) | Layers, flow request |
| **Module Map** | [module-map/backend-module-map.md](module-map/backend-module-map.md) | Module → code, router |
| **Cơ cấu module (AID + queue miền)** | [module-map/co-cau-module-aid-va-domain-queue.md](module-map/co-cau-module-aid-va-domain-queue.md) | Chốt nhóm A–F, `decision_events_queue`, `EventSource`, checklist thêm luồng |
| **Domain** | [domain/domain-overview.md](domain/domain-overview.md) | Domain logic |
| **API** | [api/api-overview.md](api/api-overview.md) | API surface |
| **Conventions** | [conventions/backend-conventions.md](conventions/backend-conventions.md) | Quy ước backend |
| **Luồng Ingress → Merge → Intel** | [05-development/KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md](05-development/KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md) | Khung CIO / Domain / AID; chuẩn tách lớp intel (mục 1.1) |
| **Khuôn module Intelligence** | [05-development/KHUNG_KHUON_MODULE_INTELLIGENCE.md](05-development/KHUNG_KHUON_MODULE_INTELLIGENCE.md) | Raw / layer1–3 / flag; lưu A–B; lịch sử & snapshot theo thời điểm (tham chiếu CRM) |
| **Đặt tên hệ thống** | [uid-field-naming.md](../docs-shared/architecture/data-contract/uid-field-naming.md) | Module, collection, `uid`, worker, route; khớp `uid.go` + `global.vars` |

---

## 🤖 Cursor AI — Thứ Tự Đọc (Repo Mode)

Khi mở riêng repo backend, Cursor nên đọc theo thứ tự:

1. **docs/README.md** (file này) — Entry point, biết repo làm gì
2. **[docs-shared/architecture/vision/00 - ai-commerce-os-platform-l1.md](../docs-shared/architecture/vision/00%20-%20ai-commerce-os-platform-l1.md)** — Vision Platform L1 — toàn bộ hệ AI Commerce OS
3. **[architecture/overview.md](architecture/overview.md)** — Kiến trúc layers
4. **[module-map/backend-module-map.md](module-map/backend-module-map.md)** — Map module → code, biết sửa ở đâu
5. **[domain/domain-overview.md](domain/domain-overview.md)** — Domain logic
6. **[api/api-overview.md](api/api-overview.md)** — API surface
7. **docs theo task** — 05-development/, 02-architecture/core/, 01-getting-started/
8. **docs-shared/** — Khi cần context hệ thống, API contract, module design cross-repo

**Khi task chạm repo khác:** Đọc `docs-shared/architecture/vision/00 - ai-commerce-os-platform-l1.md`, `docs-shared/system-map/system-map.md` và `docs-shared/modules/module-map.md` trước.

---

## 📂 Local Docs vs Shared Docs

| Loại | Vị trí | Nội dung |
|------|--------|----------|
| **Backend local** | `docs/` (đây) | Kiến trúc nội bộ, handler/service pattern, conventions, development guide |
| **Shared** | `docs-shared/` (**một nguồn** — junction trỏ tới cùng cây thư mục với front/agent trên workspace) | API contract, data contract, vision, system map, module design, ai-context, envelope live (`opsTier`, …) |

**Quy tắc:** Tài liệu chỉ backend → `docs/`. Tài liệu **dùng chung nhiều repo** → chỉ sửa tại **`docs-shared`** (không copy nội dung shared vào `docs/`). Xem [doc-ownership](../docs-shared/doc-ownership.md) (khi junction đã thiết lập).

---

## 📑 Mục Lục

### 1. 🚀 Bắt Đầu (Getting Started)

- [Cài Đặt và Cấu Hình](01-getting-started/cai-dat.md) - Hướng dẫn cài đặt từ đầu
- [Cấu Hình Môi Trường](01-getting-started/cau-hinh.md) - Chi tiết về biến môi trường
- [Khởi Tạo Hệ Thống](01-getting-started/khoi-tao.md) - Quy trình khởi tạo hệ thống lần đầu

### 2. 🏗️ Kiến Trúc (Architecture)

- **[Tổng quan kiến trúc](architecture/overview.md)** — Entry point kiến trúc (layers, flow)
- **[Bản đồ module backend](module-map/backend-module-map.md)** — Map module → code, router (⭐ bắt đầu khi implement feature)
- **[Cơ cấu module — AID & queue miền](module-map/co-cau-module-aid-va-domain-queue.md)** — Event-driven: bus AID vs queue/worker domain, bảng `EventSource`
- [02-architecture/core/tong-quan.md](02-architecture/core/tong-quan.md) - Kiến trúc cốt lõi
- [docs-shared/architecture/vision/](../docs-shared/architecture/vision/) - Vision Platform L1, Customer Intelligence & AI Commerce (Phần 1, 2, 3)
- [02-architecture/core/activity-framework.md](02-architecture/core/activity-framework.md) - Activity framework (event backbone)
- [02-architecture/core/learning-engine.md](02-architecture/core/learning-engine.md) - Decision Brain (learning memory)

### 3. 🔌 API Reference

- **[API Overview](api/api-overview.md)** — Tổng quan module, endpoint (⭐ nhìn nhanh)
- **Chi tiết đầy đủ:** [docs-shared/ai-context/folkform/api-context.md](../docs-shared/ai-context/folkform/api-context.md)

### 4. 🚢 Triển Khai (Deployment)

- [Firebase Setup](04-deployment/firebase.md) - Cài đặt và cấu hình Firebase

### 5. 💻 Phát Triển (Development)

- **[Hướng dẫn Identity & Links](05-development/HUONG_DAN_IDENTITY_LINKS.md)** — Cách dùng uid, sourceIds, links; ưu tiên cấu trúc mới, fallback logic cũ
- **[Quy ước đặt tên hệ thống (shared)](../docs-shared/architecture/data-contract/uid-field-naming.md)** — `uid`, module/package, file Go, **collection Mongo**, worker, route, env; khớp `utility/uid.go` + `global.vars.go`
- [Quy Trình Refactor Docs](05-development/QUY_TRINH_REFACTOR_DOCS.md) - Quy trình AI refactor tài liệu
- [Quy Trình Refactor .cursor](05-development/QUY_TRINH_REFACTOR_CURSOR.md) - Refactor .cursor sau khi docs xong
- [Cấu Trúc Code](05-development/cau-truc-code.md) - Cấu trúc và tổ chức code
- **[Nguyên tắc CRUD → DataChanged → AI Decision](05-development/NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION.md)** — Một hook, một cửa side-effect; **hai luồng** (data change vs intelligence handoff); **đọc trước khi sửa luồng queue / ingest / sync**
- **[Khung luồng Ingress → Merge → Intel (CIO · Domain · AID)](05-development/KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md)** — Mẫu thống nhất cho các module; CRM làm tham chiếu; **chuẩn tách lớp intel** (mục 1.1); checklist mở rộng nguồn
- **[Khung khuôn module Intelligence](05-development/KHUNG_KHUON_MODULE_INTELLIGENCE.md)** — Nguyên tắc chung: hai trục layer (L1/L2 dữ liệu vs raw→layer3), lưu kết quả A/B, activity + point-in-time, checklist miền mới
- **[Phương án domain Order khớp khung](05-development/PHUONG_AN_DOMAIN_ORDER_KHOP_KHUNG_CIO_AID.md)** — Giai đoạn 0–3: ID/enrich, đa nguồn, registry AID; đối chiếu hiện trạng `pc_pos_orders` / `order_intel_compute`
- **[Trung tâm chỉ huy AI Decision — ý tưởng & backend data](05-development/THIET_KE_TRUNG_TAM_CHI_HUY_AI_DECISION.md)** — Màn vận hành (phễu/KPI/feed), `GET org-live/metrics`, hợp đồng JSON, **`opsTier` trên feed live** (v1.8)

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

- **Architecture**: [architecture/overview.md](architecture/overview.md), [02-architecture/core/tong-quan.md](02-architecture/core/tong-quan.md), [02-architecture/core/activity-framework.md](02-architecture/core/activity-framework.md), [02-architecture/core/learning-engine.md](02-architecture/core/learning-engine.md)
- **AI Commerce OS Vision**: [docs-shared/architecture/ai-commerce-os-overview.md](../docs-shared/architecture/ai-commerce-os-overview.md) — Platform L1 tổng quan
- **Rà soát triển khai**: [docs-shared/architecture/reviews/RA_SOAT_TRIEN_KHAI_VISION.md](../docs-shared/architecture/reviews/RA_SOAT_TRIEN_KHAI_VISION.md) — Snapshot đối chiếu vision ↔ code (canonical reviews/)
- **Customer Intelligence**: [docs-shared/architecture/vision/](../docs-shared/architecture/vision/) — Phần 1 (Unified Profile), Phần 2 (AI Application), Phần 3 (CIO)
- **Module map**: [module-map/backend-module-map.md](module-map/backend-module-map.md)
- **API**: [api/api-overview.md](api/api-overview.md), [docs-shared/ai-context/folkform/api-context.md](../docs-shared/ai-context/folkform/api-context.md)
- **Firebase**: [04-deployment/firebase.md](04-deployment/firebase.md)

## 📝 Ghi Chú

- Pipeline chỉnh **docs + `.cursor`** toàn hệ (workspace → backend → frontend → agent): `prompt-system/prompt_system_scripted/scripts/run-all.md` (workspace root).
- Tất cả tài liệu được viết bằng **Tiếng Việt**
- Tài liệu được cập nhật thường xuyên, vui lòng kiểm tra phiên bản mới nhất
- Nếu có câu hỏi hoặc đề xuất, vui lòng tạo issue hoặc liên hệ team

## 🔄 Cập Nhật Gần Đây

- ✅ **2026-04-07**: Thêm [KHUNG_KHUON_MODULE_INTELLIGENCE.md](05-development/KHUNG_KHUON_MODULE_INTELLIGENCE.md) — khuôn mẫu intelligence dùng chung (raw/layer/flag, persist, lịch sử theo `activityAt`).
- ✅ **2026-04-06 (tiếp 4):** [uid-field-naming.md](../docs-shared/architecture/data-contract/uid-field-naming.md) v1.4 — **§2.9.0** khung **3 chiều** (miền / vai trò persistence / thực thể) + thuật toán đặt tên; §2.9.2 đồng bộ theo khung.
- ✅ **2026-04-06 (tiếp 3):** [uid-field-naming.md](../docs-shared/architecture/data-contract/uid-field-naming.md) v1.3 — **§2.9.1** bảng tiền tố collection (`auth_`, `fb_`, `crm_`, `decision_`, …) theo `init.go`; quy tắc collection mới.
- ✅ **2026-04-06 (tiếp 2):** [uid-field-naming.md](../docs-shared/architecture/data-contract/uid-field-naming.md) v1.2 — thêm **§2.7–2.12:** module/package, file Go, **collection Mongo**, worker, route, env; [cau-truc-code](05-development/cau-truc-code.md) trỏ chéo.
- ✅ **2026-04-06 (tiếp):** [uid-field-naming.md](../docs-shared/architecture/data-contract/uid-field-naming.md) v1.1 — thống nhất **tiền tố** (bảng khớp `utility/uid.go`), **khóa `links`**, JSON/BSON, `eventType`/queue/collection, L1/L2 đặt tên; [unified-data-contract.md](../docs-shared/architecture/data-contract/unified-data-contract.md) v1.5 trỏ doc này; [backend-conventions](conventions/backend-conventions.md) + Cursor [data-contract.md](../.cursor/rules/data-contract.md) cập nhật liên kết.
- ✅ **2026-04-06**: [unified-data-contract.md](../docs-shared/architecture/data-contract/unified-data-contract.md) §1.7 + [identity-links-model.md](../docs-shared/architecture/data-contract/identity-links-model.md) §1.1 — **L1 mirror / L2 canonical**, bốn lớp field áp cả hai, links L1→L1 phục vụ merge sang L2. [HUONG_DAN_IDENTITY_LINKS.md](05-development/HUONG_DAN_IDENTITY_LINKS.md) mục 2.1; [KHUNG_LUONG_…](05-development/KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md) mục 1.1–1.2.
- ✅ **2026-04-02**: Thêm [PHUONG_AN_DOMAIN_ORDER_KHOP_KHUNG_CIO_AID.md](05-development/PHUONG_AN_DOMAIN_ORDER_KHOP_KHUNG_CIO_AID.md) — phương án chỉnh domain Order theo khung (giai đoạn 0–3).
- ✅ **2026-04-02**: Thêm [KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md](05-development/KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md) — khung luồng thống nhất CIO / Domain / AID (Pha A–B–C), tham chiếu CRM, checklist module khác.
- ✅ **2026-03-25**: Thêm [THIET_KE_TRUNG_TAM_CHI_HUY_AI_DECISION.md](05-development/THIET_KE_TRUNG_TAM_CHI_HUY_AI_DECISION.md) — thiết kế màn command center AI Decision và API metrics / WS aggregate (đề xuất).
- ✅ **2026-03-18**: Rà soát tài liệu — CIX, AI Decision Engine đã triển khai đầy đủ; luồng CIO→CIX→Decision→Executor→Delivery đã khép vòng. Cập nhật RASOAT_MODULE_KHUNG_XUONG, PHUONG_AN_CIX/CIO/DECISION_BRAIN, `docs-shared/architecture/reviews/RA_SOAT_TRIEN_KHAI_VISION.md`.
- ✅ **2026-03-24**: Thêm [NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION.md](05-development/NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION.md) — nguyên tắc hook / queue / side-effect một cửa; cập nhật `.cursor/rules`, module-map.
- ✅ **2026-03-23**: Pipeline `run-all.md` — reindex link RA_SOAT → `architecture/reviews/`; đồng bộ workspace module-map với `aidecision/` + `executor/`.
- ✅ **2026-03-18**: Phase 0 Nền — trace_id/correlation_id trong CioEvent; Action schema chuẩn (ExecutionActionInput); BANG_QUY_TAC trỏ Foundational docs. Xem [BANG_QUY_TAC_THIET_KE_HE_THONG.md](09-ai-context/BANG_QUY_TAC_THIET_KE_HE_THONG.md)
- ✅ **2026-03-17**: Identity + Links — Rà soát CRM: lookup customer theo uid/unifiedId, thêm links.customer.uid vào filter orders/conversations, response DTO có uid. Xem [HUONG_DAN_IDENTITY_LINKS.md](05-development/HUONG_DAN_IDENTITY_LINKS.md) mục 9.1
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

