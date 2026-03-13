# Pipeline chuẩn cho docs + .cursor

**Prompt quy trình (chạy lần lượt):** `prompts/prompt-quy-trinh-refactor-docs.md`

## Phase 1 — Workspace Docs
1. `workspace/docs/docs-recreate.md`
2. `workspace/docs/docs-recreate-review.md`
3. `workspace/docs/docs-refactor.md`
4. `workspace/docs/docs-reindex.md`
5. `workspace/docs/docs-reindex-review.md`

## Phase 2 — Workspace Cursor
6. `workspace/cursor/cursor-create.md`
7. `workspace/cursor/cursor-create-review.md`

## Phase 3 — Backend Repo Docs
8. `repo-backend/docs/docs-recreate.md`
9. `repo-backend/docs/docs-recreate-review.md`
10. `repo-backend/docs/docs-refactor.md`
11. `repo-backend/docs/docs-reindex.md`
12. `repo-backend/docs/docs-reindex-review.md`

## Phase 4 — Backend Repo Cursor
13. `repo-backend/cursor/cursor-create.md`
14. `repo-backend/cursor/cursor-create-review.md`

## Phase 5 — Maintenance
15. `maintenance/docs-sync-with-code.md`
16. `maintenance/docs-build-module-map.md`
17. `maintenance/docs-build-api-overview.md`
18. `maintenance/docs-audit.md`
19. `maintenance/cursor-audit.md`
