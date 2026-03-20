# Rà Soát Tài Liệu vs Codebase (2026-03-19)

**Mục đích:** Đối chiếu tài liệu vision/phương án với codebase sau khi đổi tên module (decision→ai-decision+learning, approval+delivery→executor).

---

## 1. Tóm Tắt Thay Đổi Đã Thực Hiện (Code)

| Cũ | Mới |
|----|-----|
| `api/internal/api/decision/` | `api/internal/api/aidecision/` + `api/internal/api/learning/` |
| `api/internal/api/approval/` | Gộp vào `api/internal/api/executor/` |
| `api/internal/api/delivery/router` | Xóa — handler dùng nội bộ bởi executor |
| `/decision/execute` | `/ai-decision/execute` |
| `/decision/cases` | `/learning/cases` |
| `/approval/actions/*` | `/executor/actions/*` |
| `/delivery/send`, `/delivery/execute`, `/delivery/history` | `/executor/send`, `/executor/execute`, `/executor/history` |

---

## 2. PHUONG_AN_TRIEN_KHAI_AI_DECISION_VA_LEARNING.md — Cần Cập Nhật

### 2.1 Section 0 — Trạng Thái Triển Khai

| Hạng mục | Trong doc | Thực tế code |
|----------|-----------|--------------|
| **Decision Brain** | decision_cases, BuildDecisionCaseFromAction | ✅ `learning/` — LearningCase, BuildLearningCaseFromAction, collection `decision_cases` |
| **Delivery Gate** | ❌ POST /delivery/execute nhận trực tiếp | ✅ Đã có — POST /executor/execute, gate trong handler.delivery.execute (DELIVERY_ALLOW_DIRECT_USE deprecate) |

**Đề xuất:** Cập nhật ngày rà soát 2026-03-19, sửa Delivery Gate → ✅ Có (qua executor).

### 2.2 Section 2.1 — Cấu Trúc Hiện Tại

**Trong doc:**
```
api/internal/api/approval/   # API layer
├── handler/
└── router/
```

**Thực tế:** `api/internal/api/approval/` đã xóa. Thay bằng:
```
api/internal/api/executor/   # Approval Gate + Execution
├── handler/handler.executor.action.go
└── router/routes.go
```

**Đề xuất:** Sửa §2.1 — bỏ approval API layer, thêm executor.

### 2.3 Section 6 — Cấu Trúc Thư Mục Đề Xuất

**Trong doc:**
```
api/internal/api/
├── decision/     # service.decision.engine, builder, cix
├── approval/     # handler, router, models
└── delivery/     # handler
```

**Thực tế:**
```
api/internal/api/
├── aidecision/   # service.aidecision.engine, cix — AI Decision
├── learning/     # service.learning.builder, case — Learning engine
├── executor/    # handler.executor.action, router — Approval Gate + Execution
└── delivery/    # handler (nội bộ, không router)
```

**Đề xuất:** Cập nhật §6 theo cấu trúc mới.

### 2.4 Section 8 — Rà Soát Đối Chiếu Codebase

| Thành phần | Trong doc | Thực tế |
|------------|-----------|---------|
| BuildDecisionCaseFromAction | service.decision.builder.go | service.learning.builder.go — BuildLearningCaseFromAction |
| CreateDecisionCase | service.decision.case.go | service.learning.case.go — CreateLearningCaseFromAction |
| DecisionEngineService.Execute | service.decision.engine.go | service.aidecision.engine.go |
| ReceiveCixPayload | service.decision.cix.go | service.aidecision.cix.go |
| BuildDecisionCase khi Reject | handler approval | handler.executor.action.go — learningsvc.CreateLearningCaseFromAction |
| Domain cix Executor | ❌ Chưa có | ✅ executors/cix/ |
| Delivery HandleExecute | handler.delivery.execute.go | Vẫn có, mount qua /executor/execute |

**Đề xuất:** Cập nhật §8.1, §8.2, §8.4 — đổi tên file/vị trí, đánh dấu ReceiveCixPayload ✅, Domain cix Executor ✅, BuildLearningCase khi Reject ✅.

### 2.5 Section 8.2 — Chưa Có / Chưa Đúng

Các mục sau **đã có** trong code, cần chuyển sang §8.1:
- BuildDecisionCaseFromAction khi Reject → handler.executor.action gọi learningsvc
- ReceiveCixPayload → aidecisionsvc.ReceiveCixPayload
- Domain cix Executor → executors/cix/

---

## 3. DE_XUAT_SUA_CODE_THEO_VISION.md — Cần Cập Nhật

| Vị trí | Trong doc | Thực tế |
|--------|-----------|---------|
| §3.1 BuildDecisionCase khi Reject | handler.approval.action.go | handler.executor.action.go |
| §3.1 ReceiveCixPayload | service.decision.cix.go | service.aidecision.cix.go |
| §3.1 AI Decision Engine Execute | service.decision.engine.go | service.aidecision.engine.go |
| §3.2 Phase 1 — model.approval.config | api/internal/api/approval/models/ | approval đã xóa — đặt trong executor hoặc internal/approval |

**Đề xuất:** Sửa đường dẫn file trong §3.1, §3.2.

---

## 4. Các File Khác

| File | Trạng thái |
|------|------------|
| backend-module-map.md | ✅ Đã cập nhật (executor, ai-decision, learning) |
| api-overview.md | ✅ Đã cập nhật (/learning/cases) |
| KE_HOACH_DOI_TEN_MODULE.md | Tài liệu kế hoạch — giữ làm tham chiếu |
| HUONG_DAN_DOI_TEN_MODULE_CHI_TIET.md | Tài liệu hướng dẫn — đã thực hiện |
| api/internal/approval/README.md | ⚠️ Dòng 15: API /approval/actions/* → nên đổi /executor/actions/* |

---

## 5. Checklist Cập Nhật Doc

- [x] PHUONG_AN_TRIEN_KHAI_AI_DECISION_VA_LEARNING.md §0 — Trạng thái, Delivery Gate
- [x] PHUONG_AN_TRIEN_KHAI_AI_DECISION_VA_LEARNING.md §2.1 — Cấu trúc (executor thay approval)
- [x] PHUONG_AN_TRIEN_KHAI_AI_DECISION_VA_LEARNING.md §6 — Cấu trúc thư mục (aidecision, learning, executor)
- [x] PHUONG_AN_TRIEN_KHAI_AI_DECISION_VA_LEARNING.md §8 — Rà soát (đường dẫn mới, trạng thái đã có)
- [x] DE_XUAT_SUA_CODE_THEO_VISION.md §3 — Đường dẫn file
- [x] api/internal/approval/README.md — API path /executor/actions/*

---

## Changelog

- 2026-03-19: Tạo báo cáo rà soát docs vs code sau đổi tên module
