# Kế Hoạch Đổi Tên Module

**Ngày:** 2026-03-19

## Ánh xạ

| Cũ | Mới | API path cũ | API path mới |
|----|-----|-------------|--------------|
| AI Decision Engine (trong decision/) | **AI Decision** | /decision/execute | /ai-decision/execute |
| Decision Brain (trong decision/) | **Learning engine** | /decision/cases | /learning/cases |
| Approval + Delivery | **Executor** | /approval/*, /delivery/* | /executor/actions/*, /executor/send, /executor/execute, /executor/history |

## Cấu trúc mới

```
api/internal/api/
├── decision/          → XÓA (tách thành ai-decision + learning)
├── ai-decision/       MỚI — AI Decision (engine, cix, execute)
├── learning/          MỚI — Learning engine (cases, builder, CreateDecisionCase)
├── approval/          → GỘP vào executor
├── delivery/          → GỘP vào executor
└── executor/          MỚI — Approval Gate + Execution (actions, send, execute, history)
```

## Thứ tự thực hiện

1. Tạo learning/ — chuyển Decision Brain
2. Tạo ai-decision/ — chuyển AI Decision Engine
3. Tạo executor/ — gộp approval + delivery
4. Xóa decision/, approval/, delivery/
5. Cập nhật imports toàn codebase
6. Cập nhật docs
