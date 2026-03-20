# Hướng Dẫn Đổi Tên Module — Chi Tiết

**Ngày:** 2026-03-19  
**Phạm vi:** Tách decision → ai-decision + learning; gộp approval + delivery → executor.

---

## 1. Tổng Quan Thay Đổi

| Cũ | Mới | API Path |
|----|-----|----------|
| decision/ (AI Decision Engine) | ai-decision/ | /decision/execute → /ai-decision/execute |
| decision/ (Decision Brain) | learning/ | /decision/cases → /learning/cases |
| approval/ | executor/ | /approval/actions/* → /executor/actions/* |
| delivery/ | executor/ | /delivery/* → /executor/send, /executor/execute, /executor/history |

---

## 2. Phụ Thuộc Cần Cập Nhật

### 2.1 CreateDecisionCaseFromAction, BuildDecisionCaseFromCIOChoice

| File | Thay đổi |
|------|----------|
| `api/internal/api/ads/worker/worker.ads.execution.go` | decisionsvc → learningsvc |
| `api/internal/api/approval/handler/handler.approval.action.go` | decisionsvc → learningsvc |
| `api/internal/api/cio/service/service.cio.feedback.go` | decisionsvc → learningsvc |
| `api/internal/api/decision/service/service.decision.engine.go` | CreateDecisionCaseFromAction → learningsvc |

### 2.2 DecisionEngineService, ReceiveCixPayload

| File | Thay đổi |
|------|----------|
| `api/internal/api/cix/service/service.cix.analysis.go` | decisionsvc → aidecisionsvc |
| `api/internal/api/decision/handler/handler.decision.execute.go` | → chuyển sang ai-decision |

### 2.3 Approval API paths (ApprovePath, RejectPath)

| File | Path cũ | Path mới |
|------|---------|----------|
| `api/internal/api/ads/service/service.ads.propose.go` | /api/v1/approval/actions/approve | /api/v1/executor/actions/approve |
| `api/internal/api/cio/service/service.cio.touchpoint.go` | /api/v1/approval/actions/reject | /api/v1/executor/actions/reject |
| `api/internal/api/cio/service/service.cio.plan_execution.go` | /api/v1/approval/actions/reject | /api/v1/executor/actions/reject |
| `api/internal/api/decision/service/service.decision.engine.go` | /api/v1/approval/actions/approve, reject | /api/v1/executor/actions/approve, reject |

### 2.4 Delivery

| File | Thay đổi |
|------|----------|
| `api/internal/executors/cix/executor.go` | Gọi delivery → executor (nội bộ) |
| `api/internal/delivery/processor.go` | Giữ nguyên (xử lý queue) |

---

## 3. Thứ Tự Thực Hiện

### Phase 1: Tạo learning/ (1–2 giờ)
1. Tạo learning/models, service, builder, handler, dto, router
2. Cập nhật worker.ads.execution, handler.approval.action, service.cio.feedback
3. Cập nhật service.decision.engine (CreateDecisionCaseFromAction)
4. Đăng ký learning router
5. Xóa decision cases khỏi decision router

### Phase 2: Tạo ai-decision/ (1 giờ)
1. Tạo ai-decision/service (engine, cix), handler, router
2. Cập nhật service.cix.analysis
3. Đăng ký ai-decision router
4. Xóa execute khỏi decision router

### Phase 3: Tạo executor/ (2–3 giờ)
1. Tạo executor/ gộp approval handler + delivery handler
2. Cập nhật tất cả ApprovePath, RejectPath
3. Đăng ký executor router
4. Xóa approval, delivery router

### Phase 4: Dọn dẹp
1. Xóa decision/ (đã rỗng)
2. Xóa approval/, delivery/
3. Cập nhật initsvc (permissions nếu có)
4. Cập nhật docs, api-overview, backend-module-map

---

## 4. Rủi Ro

- **Frontend/Agent:** Nếu gọi /decision/*, /approval/*, /delivery/* trực tiếp → cần cập nhật URL
- **API tests:** Cần cập nhật path trong test
- **Deprecation:** Có thể thêm alias redirect từ path cũ → path mới (tạm thời)
