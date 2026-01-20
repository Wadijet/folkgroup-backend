# Quick Start: Setup Workspace với 3 Git Repos

## 🚀 Setup Nhanh (5 phút)

### Bước 1: Tạo và Clone Workspace

```bash
# Tạo workspace folder
mkdir folkform-workspace
cd folkform-workspace

# Clone cả 3 repos
git clone https://github.com/Wadijet/ff_be_auth.git
git clone https://github.com/Wadijet/agent_pancake.git
git clone https://github.com/Wadijet/folk_form.git
```

### Bước 2: Tạo Workspace File

Tạo file `folkform.code-workspace`:

```json
{
  "folders": [
    {"name": "ff_be_auth", "path": "./ff_be_auth"},
    {"name": "agent_pancake", "path": "./agent_pancake"},
    {"name": "folk_form", "path": "./folk_form"}
  ]
}
```

### Bước 3: Mở trong Cursor/VS Code

```
File → Open Workspace from File → Chọn folkform.code-workspace
```

## ✅ Kết Quả

Sau khi setup, AI sẽ thấy:

```
folkform-workspace/
├── ff_be_auth/          ← Backend API
├── agent_pancake/       ← Sync Agent
└── folk_form/           ← Frontend
```

**AI có thể đọc và hiểu tất cả 3 repos cùng lúc!** 🎉

## 📝 Lưu Ý Quan Trọng

⚠️ **Phải mở workspace root**, không mở từng repo riêng!

```
❌ SAI: File → Open Folder → ff_be_auth/
✅ ĐÚNG: File → Open Workspace → folkform.code-workspace
```

## 🔧 Workflow

```bash
# Làm việc với từng repo
cd ff_be_auth && git checkout -b feature/xxx
cd ../agent_pancake && git checkout -b feature/yyy
cd ../folk_form && git checkout -b feature/zzz

# Commit riêng từng repo
cd ff_be_auth && git commit -m "..."
cd ../agent_pancake && git commit -m "..."
cd ../folk_form && git commit -m "..."
```

## 📚 Chi Tiết

Xem tài liệu đầy đủ: [multi-repo-workspace-setup.md](./multi-repo-workspace-setup.md)


