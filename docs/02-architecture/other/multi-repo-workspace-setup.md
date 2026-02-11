# Mô Hình Làm Việc Với 3 Git Repositories - AI Context Setup

## 📋 Tổng Quan

Khi bạn có **3 git repositories riêng biệt** nhưng muốn AI hiểu được **toàn bộ context**, cần setup workspace đúng cách:

- `ff_be_auth` - Backend API
- `agent_pancake` - Sync Agent Service  
- `folk_form` - Frontend Application

## 🎯 Mô Hình Đề Xuất: Workspace với Multiple Repos

### Cấu Trúc Workspace

```
folkform-workspace/              # Workspace root (KHÔNG có .git)
├── .cursor/                     # Cursor workspace config (nếu dùng)
├── README.md                    # Workspace documentation
│
├── ff_be_auth/                  # Git repo 1
│   ├── .git/                    # Git riêng của ff_be_auth
│   ├── go.work                  # Go workspace (nếu có)
│   ├── api/
│   ├── api-tests/
│   └── docs/
│
├── agent_pancake/               # Git repo 2
│   ├── .git/                    # Git riêng của agent_pancake
│   ├── go.mod
│   └── ...
│
└── folk_form/                   # Git repo 3
    ├── .git/                    # Git riêng của folk_form
    ├── package.json
    └── ...
```

## 🚀 Cách Setup

### Bước 1: Tạo Workspace Root

```bash
# Tạo thư mục workspace
mkdir folkform-workspace
cd folkform-workspace

# Tạo README cho workspace
cat > README.md << 'EOF'
# FolkForm Workspace

Workspace chứa 3 repositories:
- ff_be_auth: Backend API
- agent_pancake: Sync Agent Service
- folk_form: Frontend Application

## Setup

```bash
# Clone tất cả repos
git clone https://github.com/Wadijet/ff_be_auth.git
git clone https://github.com/Wadijet/agent_pancake.git
git clone https://github.com/Wadijet/folk_form.git
```
EOF
```

### Bước 2: Clone Tất Cả Repositories

```bash
# Clone từng repo vào workspace
git clone https://github.com/Wadijet/ff_be_auth.git
git clone https://github.com/Wadijet/agent_pancake.git
git clone https://github.com/Wadijet/folk_form.git
```

### Bước 3: Mở Workspace trong Cursor/VS Code

**Option 1: Mở thư mục workspace root**
```bash
# Trong Cursor/VS Code
File → Open Folder → Chọn folkform-workspace/
```

**Option 2: Tạo workspace file (khuyến nghị)**

Tạo file `folkform.code-workspace`:

```json
{
  "folders": [
    {
      "name": "ff_be_auth",
      "path": "./ff_be_auth"
    },
    {
      "name": "agent_pancake",
      "path": "./agent_pancake"
    },
    {
      "name": "folk_form",
      "path": "./folk_form"
    }
  ],
  "settings": {
    "files.exclude": {
      "**/.git": false
    },
    "search.exclude": {
      "**/node_modules": true,
      "**/vendor": true,
      "**/.next": true
    }
  }
}
```

Mở workspace:
```bash
# Trong Cursor/VS Code
File → Open Workspace from File → Chọn folkform.code-workspace
```

## ✅ Lợi Ích Với AI

### AI Có Thể Đọc Tất Cả

```
folkform-workspace/
├── ff_be_auth/              ← AI đọc được
│   ├── api/internal/api/        ← Backend services
│   └── docs/               ← Backend docs
│
├── agent_pancake/           ← AI đọc được
│   └── app/                ← Sync logic
│
└── folk_form/               ← AI đọc được
    ├── src/                ← Frontend code
    └── components/         ← React components
```

### Ví Dụ Tương Tác Với AI

**Scenario 1: Tích hợp Frontend với Backend**

```
Bạn: "Tạo API endpoint để frontend lấy danh sách conversations"

AI: ✅ Đọc được:
    - ff_be_auth/api/internal/api/handler/ (hiểu pattern handler)
    - ff_be_auth/api/internal/api/services/ (hiểu services)
    - folk_form/src/services/ (hiểu cách frontend gọi API)
    
    ✅ Suggest:
    - Handler endpoint trong ff_be_auth
    - Service method
    - Frontend API client call
    - Types/interfaces cho cả 2
```

**Scenario 2: Agent sync data về backend**

```
Bạn: "Agent cần sync conversations về backend"

AI: ✅ Đọc được:
    - agent_pancake/app/ (hiểu sync logic)
    - ff_be_auth/api/internal/api/handler/ (hiểu API endpoints)
    - ff_be_auth/api/internal/api/models/ (hiểu data models)
    
    ✅ Suggest:
    - Code trong agent_pancake để gọi API
    - Endpoint trong ff_be_auth để nhận data
    - Data transformation giữa 2 bên
```

## 🔧 Workflow Hàng Ngày

### Làm Việc Với Nhiều Repos

```bash
# Terminal 1: Backend
cd folkform-workspace/ff_be_auth
git checkout -b feature/new-endpoint
# ... code changes ...

# Terminal 2: Agent
cd folkform-workspace/agent_pancake
git checkout -b feature/sync-conversations
# ... code changes ...

# Terminal 3: Frontend
cd folkform-workspace/folk_form
git checkout -b feature/conversation-list
# ... code changes ...
```

### Commit Riêng Từng Repo

```bash
# Commit trong từng repo riêng
cd ff_be_auth && git commit -m "feat: add conversation endpoint"
cd ../agent_pancake && git commit -m "feat: sync conversations"
cd ../folk_form && git commit -m "feat: display conversations"
```

### Hoặc Commit Đồng Bộ (Nếu cần)

```bash
# Script để commit tất cả (nếu thay đổi liên quan)
#!/bin/bash
cd ff_be_auth && git add . && git commit -m "feat: backend changes"
cd ../agent_pancake && git add . && git commit -m "feat: agent changes"
cd ../folk_form && git add . && git commit -m "feat: frontend changes"
```

## 📝 Best Practices

### 1. Naming Convention Nhất Quán

```
✅ Tốt:
- ff_be_auth/api/internal/api/services/service.fb.conversation.go
- agent_pancake/app/sync/conversation_sync.go
- folk_form/src/services/conversationService.ts

❌ Không tốt:
- backend/services/conversation.go
- agent/sync.go
- frontend/api.ts
```

### 2. Documentation Cross-Repo

Tạo file `docs/CROSS_REPO.md` trong workspace root:

```markdown
# Cross-Repository Documentation

## API Endpoints (ff_be_auth)
- `/api/v1/facebook/conversation/find` - Lấy conversations

## Agent Sync (agent_pancake)
- Sync conversations từ Pancake → ff_be_auth

## Frontend Usage (folk_form)
- `conversationService.getList()` - Gọi API lấy conversations
```

### 3. Shared Types/Interfaces

Nếu có types dùng chung, đặt ở:

```
folkform-workspace/
└── shared/
    ├── types/
    │   ├── conversation.ts
    │   └── customer.ts
    └── README.md
```

### 4. Workspace README

Luôn có README ở root workspace giải thích:
- Cấu trúc repos
- Cách setup
- Mối quan hệ giữa repos
- Workflow

## 🎯 So Sánh Với Monorepo

| Tiêu chí | Workspace Multi-Repo | Monorepo |
|----------|---------------------|----------|
| **AI Context** | ✅ Đầy đủ (nếu setup đúng) | ✅ Đầy đủ |
| **Git History** | ✅ Riêng từng repo | ✅ Tập trung |
| **Permissions** | ✅ Riêng từng repo | ⚠️ Chung |
| **Clone** | ⚠️ Phải clone 3 lần | ✅ Clone 1 lần |
| **Atomic Commits** | ❌ Không thể | ✅ Có thể |
| **Team Independence** | ✅ Hoàn toàn | ⚠️ Phụ thuộc |
| **CI/CD** | ⚠️ 3 pipelines | ✅ 1 pipeline |

## ⚠️ Lưu Ý Quan Trọng

### 1. AI Chỉ Đọc Được Nếu Mở Đúng Workspace

```
❌ SAI: Mở từng repo riêng
File → Open Folder → ff_be_auth/
→ AI chỉ thấy ff_be_auth, không thấy agent_pancake và folk_form

✅ ĐÚNG: Mở workspace root
File → Open Folder → folkform-workspace/
→ AI thấy tất cả 3 repos
```

### 2. Git Operations Vẫn Riêng

```bash
# Mỗi repo vẫn có .git riêng
cd ff_be_auth && git status      # Chỉ thấy changes trong ff_be_auth
cd agent_pancake && git status   # Chỉ thấy changes trong agent_pancake
cd folk_form && git status        # Chỉ thấy changes trong folk_form
```

### 3. Go Workspace (Nếu cần)

Nếu `ff_be_auth` và `agent_pancake` đều là Go projects:

```bash
# Tạo go.work ở workspace root
cd folkform-workspace
go work init
go work use ./ff_be_auth/api
go work use ./agent_pancake
```

## 🚀 Quick Start Script

Tạo script `setup-workspace.sh`:

```bash
#!/bin/bash

WORKSPACE_DIR="folkform-workspace"

# Tạo workspace
mkdir -p $WORKSPACE_DIR
cd $WORKSPACE_DIR

# Clone repos
echo "Cloning ff_be_auth..."
git clone https://github.com/Wadijet/ff_be_auth.git

echo "Cloning agent_pancake..."
git clone https://github.com/Wadijet/agent_pancake.git

echo "Cloning folk_form..."
git clone https://github.com/Wadijet/folk_form.git

# Tạo workspace file
cat > folkform.code-workspace << 'EOF'
{
  "folders": [
    {"name": "ff_be_auth", "path": "./ff_be_auth"},
    {"name": "agent_pancake", "path": "./agent_pancake"},
    {"name": "folk_form", "path": "./folk_form"}
  ]
}
EOF

echo "✅ Workspace setup complete!"
echo "📂 Open folkform.code-workspace in Cursor/VS Code"
```

## 📚 Tài Liệu Tham Khảo

- [Git Strategy Workspace](./git-strategy-workspace.md)
- [AI Context Benefits](./ai-context-benefits.md)
- [Go Workspace Documentation](https://go.dev/doc/tutorial/workspaces)

## 🎯 Kết Luận

**Mô hình Workspace với Multiple Repos** cho phép:
- ✅ AI hiểu được toàn bộ context
- ✅ Giữ git history riêng cho từng repo
- ✅ Team độc lập làm việc
- ✅ Permissions riêng nếu cần

**Quan trọng nhất**: Luôn mở **workspace root** trong Cursor/VS Code để AI đọc được tất cả!


