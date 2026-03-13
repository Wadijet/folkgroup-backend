# Skill: Review Domain Logic

**Mục đích:** Review logic nghiệp vụ trong Service.

**Cách dùng:** Khi review hoặc debug logic.

**Bước:**
1. Logic nằm trong Service, không trong Handler
2. Kiểm tra OwnerOrganizationID / filter org
3. Validation business rules
4. Error handling với ConvertMongoError
5. Tham chiếu `docs/02-architecture/business-logic/`
