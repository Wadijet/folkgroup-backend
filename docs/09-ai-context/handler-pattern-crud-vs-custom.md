# Handler: CRUD chuẩn vs custom handler

Tóm tắt nhanh cho AI/người đọc: khi nào dùng CRUD BaseHandler, khi nào thêm handler method + route riêng.

---

## CRUD chuẩn (registerCRUDRoutes)

- **`:id` trong URL = document _id** (FindOneById, UpdateById, DeleteById đều dùng _id).
- Route: `/find-by-id/:id`, `/update-by-id/:id`, `/delete-by-id/:id`.
- Dùng khi: resource được định danh bằng document _id, thao tác đúng Insert/Find/Update/Delete theo _id.

---

## Custom handler + route riêng

Dùng khi **một trong hai**:

1. **Id trong URL không phải document _id**  
   Ví dụ: PageID, PostID, agentId, **organization id** (ownerOrganizationId), conversationId.  
   → Cần handler riêng: lấy param → gọi service theo key đó (FindOneByPageID, GetByOwnerOrganizationID, …).

2. **Thao tác không phải CRUD đơn giản**  
   Ví dụ: commit draft, approve/reject, upsert theo parent id, “update-data” theo agentId.  
   → Cần handler riêng: parse body/params → gọi service action (CommitDraftNode, UpsertByOwnerOrganizationID, …).

Vẫn dùng **BaseHandler** cho: SafeHandler, HandleResponse, ParseRequestBody, validateUserHasAccessToOrg / validateOrganizationAccess (khi áp dụng được). Chỉ **không** dùng FindOneById/UpdateById/DeleteById cho những route này.

---

## Ví dụ trong codebase

| Module            | Route (tóm tắt)              | Id/param là gì     | Ghi chú                    |
|-------------------|-----------------------------|---------------------|----------------------------|
| FbPage            | find-by-page-id/:id         | PageID (external)    | Custom: FindOneByPageID     |
| FbPost            | find-by-post-id/:id         | PostID (external)   | Custom: FindOneByPostID    |
| FbMessageItem     | find-by-conversation/:id    | conversationId      | Custom: FindByConversationId |
| AgentConfig       | config/:agentId/update-data | agentId             | Custom: upsert theo agentId |
| DraftContentNode  | drafts/nodes/:id/commit     | document _id       | Custom: action commit      |
| OrganizationConfig | organization/:id/config    | organization id     | Custom: GetByOwnerOrganizationID, Upsert, Delete theo org id |

---

**Khi cần:** Thêm API mới → xem `docs/05-development/them-api-moi.md` và file này để chọn CRUD chuẩn hay custom handler.
