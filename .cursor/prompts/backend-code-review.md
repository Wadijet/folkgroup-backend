# Backend Code Review Prompt

Khi review code backend:

1. Handler chỉ parse/validate/gọi service/trả response?
2. Business logic nằm trong Service?
3. Response đúng format (code, message, data, status)?
4. Đã xử lý OwnerOrganizationID / filter theo org?
5. Error handling đầy đủ, không panic?
6. Comment Tiếng Việt cho public functions?
