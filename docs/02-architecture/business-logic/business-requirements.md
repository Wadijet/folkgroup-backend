# Yêu Cầu Bài Toán: Phân Quyền Dữ Liệu Theo Tổ Chức Dạng Cây

**Mục đích:** Mô tả chi tiết và đầy đủ yêu cầu bài toán về phân quyền dữ liệu trong hệ thống có cấu trúc tổ chức dạng cây.

---

## 📋 Tổng Quan

Hệ thống cần quản lý và phân quyền dữ liệu trong môi trường **multi-tenant** với cấu trúc tổ chức dạng cây (hierarchical organization structure). Mỗi dòng dữ liệu thuộc về một tổ chức cụ thể, và quyền truy cập được tính toán dựa trên vị trí của user trong cây tổ chức.

---

## 🏗️ Cấu Trúc Tổ Chức

### **Mô Hình Dữ Liệu**

Hệ thống sử dụng cấu trúc tổ chức dạng cây với các cấp độ:

```
System (Level -1) - Root, không thể xóa
└── Group (Level 0) - Tập đoàn
    └── Company (Level 1) - Công ty
        └── Department (Level 2) - Phòng ban
            └── Division (Level 3) - Bộ phận
                └── Team (Level 4+) - Team
```

### **Ví Dụ Cấu Trúc Thực Tế**

```
System (Level -1, Path: "/system")
└── Tập Đoàn ABC (Level 0, Path: "/system/abc_group")
    ├── Công Ty Miền Bắc (Level 1, Path: "/system/abc_group/north_company")
    │   ├── Phòng Kinh Doanh (Level 2, Path: "/system/abc_group/north_company/sales_dept")
    │   │   ├── Team Bán Hàng A (Level 3, Path: "/system/abc_group/north_company/sales_dept/team_a")
    │   │   └── Team Bán Hàng B (Level 3, Path: "/system/abc_group/north_company/sales_dept/team_b")
    │   └── Phòng Marketing (Level 2, Path: "/system/abc_group/north_company/marketing_dept")
    └── Công Ty Miền Nam (Level 1, Path: "/system/abc_group/south_company")
        └── Phòng Kỹ Thuật (Level 2, Path: "/system/abc_group/south_company/tech_dept")
```

### **Đặc Điểm Cấu Trúc**

- **Parent-Child Relationship:** Mỗi tổ chức có thể có nhiều children, nhưng chỉ có một parent
- **Path:** Đường dẫn đầy đủ từ root đến tổ chức (dùng để query nhanh children)
- **Level:** Cấp độ trong cây (dùng để phân biệt loại tổ chức)

---

## 📦 Quản Lý Dữ Liệu

### **1. Mỗi Dòng Dữ Liệu Thuộc Về Một Tổ Chức**

Mọi document trong database có field `organizationId`:

```go
type Order struct {
    ID             primitive.ObjectID
    OrganizationID primitive.ObjectID  // ✅ Dòng dữ liệu thuộc tổ chức nào
    CustomerName   string
    TotalAmount   float64
    // ... các trường khác
}
```

**Quy tắc:**
- ✅ Mỗi dòng dữ liệu **PHẢI** có `organizationId`
- ✅ `organizationId` được tự động gán khi tạo mới (từ active organization context)
- ✅ Không cho phép update `organizationId` trực tiếp (bảo mật)

### **2. Hai Loại Dữ Liệu**

Hệ thống cần hỗ trợ **cả 2 loại dữ liệu**:

#### **A. Dữ Liệu Riêng (Private Data)**
- **Thuộc về:** Team/Division level (Level 3+)
- **Đặc điểm:**
  - Chỉ team đó sở hữu và quản lý
  - Các teams khác không thấy (trừ khi có Scope 1 ở parent level)
  - Ví dụ: Khách hàng riêng của Team A, không chia sẻ với Team B

**Ví dụ:**
```
Customer "XYZ Ltd" (riêng Team A):
- organizationId: team_a (Level 3)
- Chỉ Team A thấy được
- Team B không thấy (trừ manager có Scope 1 ở sales_dept)
```

#### **B. Dữ Liệu Chung (Shared Data)**
- **Thuộc về:** Company/Department level (Level 1-2)
- **Đặc điểm:**
  - Nhiều teams cùng sở hữu và đóng góp
  - Tất cả teams trong parent organization đều thấy được
  - Mỗi team có thể thêm activities/notes riêng

**Ví dụ:**
```
Customer "ABC Corp" (chung cho cả Sales Department):
- organizationId: sales_dept (Level 2)
- Team A thấy được ✅ (vì sales_dept là parent của team_a)
- Team B thấy được ✅ (vì sales_dept là parent của team_b)
- Cả 2 teams có thể thêm notes/activities
```

---

## 🔐 Yêu Cầu Phân Quyền

### **1. Nguyên Tắc Cơ Bản**

User chỉ có thể truy cập dữ liệu của:
- ✅ Tổ chức mà role của user thuộc về
- ✅ Tổ chức con (children) nếu có Scope = 1
- ✅ Tổ chức cha (parents) - **CẦN XÁC ĐỊNH LOGIC** (hiện tại tự động thêm tất cả, có thể phá vỡ logic)

### **2. Scope của Permission**

Mỗi permission trong role có **scope**:

#### **Scope 0: Chỉ Tổ Chức Của Role**
```
User có Role A thuộc "Team Bán Hàng A"
→ Chỉ thấy dữ liệu có organizationId = "Team Bán Hàng A"
```

#### **Scope 1: Tổ Chức + Children**
```
User có Role A thuộc "Phòng Kinh Doanh" với Scope 1
→ Thấy dữ liệu của:
  - Phòng Kinh Doanh
  - Team Bán Hàng A (con)
  - Team Bán Hàng B (con)
```

### **3. Yêu Cầu Về Dữ Liệu Chung**

**Vấn đề:**
- 2 team sale (Team A và Team B) cùng cần xem khách hàng chung
- Nếu để dữ liệu ở cấp Team → Team khác không thấy
- Nếu để dữ liệu ở cấp Company → Nhân viên cấp thấp (Scope 0) không truy cập được

**Yêu cầu:**
- ✅ User ở cấp thấp (Team) cần thấy dữ liệu chung của cấp cao (Department/Company)
- ✅ Dữ liệu chung tự động visible cho tất cả children
- ✅ Không cần permission đặc biệt để xem dữ liệu chung
- ✅ Đơn giản, không cần đánh dấu `isShared` cho từng document

---

## 🎯 Use Cases Cụ Thể

### **Use Case 1: Nhân Viên Team (Scope 0)**

**User:** Nhân viên Team Bán Hàng A  
**Role:** Sales Staff (Scope 0, Permission: "order.read")  
**Organization:** Team Bán Hàng A (Level 3)

**Yêu cầu truy cập:**
- ✅ Thấy orders của Team Bán Hàng A (dữ liệu riêng)
- ✅ Thấy orders của Phòng Kinh Doanh (dữ liệu chung - parent)
- ✅ Thấy orders của Công Ty Miền Bắc (dữ liệu chung - parent)
- ✅ Thấy orders của Tập Đoàn ABC (dữ liệu chung - parent)
- ❌ KHÔNG thấy orders của Team Bán Hàng B (sibling - dữ liệu riêng)
- ❌ KHÔNG thấy orders của Phòng Marketing (sibling - dữ liệu riêng)

**Kết quả mong đợi:**
```
Allowed Organizations:
- Team Bán Hàng A (chính nó - Scope 0)
- Phòng Kinh Doanh (parent, Level 2 - dữ liệu chung)
- Công Ty Miền Bắc (parent, Level 1 - dữ liệu chung)
- Tập Đoàn ABC (parent, Level 0 - dữ liệu chung)
- System (root, Level -1 - dữ liệu chung)
```

---

### **Use Case 2: Trưởng Phòng (Scope 1)**

**User:** Trưởng Phòng Kinh Doanh  
**Role:** Department Manager (Scope 1, Permission: "order.read")  
**Organization:** Phòng Kinh Doanh (Level 2)

**Yêu cầu truy cập:**
- ✅ Thấy orders của Phòng Kinh Doanh (chính nó)
- ✅ Thấy orders của Team Bán Hàng A (child - Scope 1)
- ✅ Thấy orders của Team Bán Hàng B (child - Scope 1)
- ✅ Thấy orders của Công Ty Miền Bắc (parent - dữ liệu chung)
- ❌ KHÔNG thấy orders của Phòng Marketing (sibling - dữ liệu riêng)

**Kết quả mong đợi:**
```
Allowed Organizations:
- Phòng Kinh Doanh (chính nó - Scope 1)
- Team Bán Hàng A (child - Scope 1)
- Team Bán Hàng B (child - Scope 1)
- Công Ty Miền Bắc (parent, Level 1 - dữ liệu chung)
- Tập Đoàn ABC (parent, Level 0 - dữ liệu chung)
- System (root, Level -1 - dữ liệu chung)
```

---

### **Use Case 3: User Có Nhiều Roles**

**User:** Có 2 roles
- Role A: Team Bán Hàng A (Scope 0, Permission: "order.read")
- Role B: Phòng Marketing (Scope 1, Permission: "order.read")

**Yêu cầu truy cập:**
- ✅ Thấy orders của Team Bán Hàng A (từ Role A)
- ✅ Thấy orders của Phòng Marketing (từ Role B)
- ✅ Thấy orders của các teams con của Phòng Marketing (Scope 1)
- ✅ Thấy orders của các parent organizations của cả 2 orgs

**Kết quả mong đợi:**
```
Allowed Organizations (hợp nhất):
- Team Bán Hàng A (từ Role A)
- Phòng Marketing (từ Role B)
- Team Marketing A (child của Role B - Scope 1)
- Team Marketing B (child của Role B - Scope 1)
- Tất cả parents của cả 2 orgs (dữ liệu chung)
```

---

## ⚠️ Vấn Đề Cần Giải Quyết

### **1. Logic Tự Động Thêm Parents**

**Hiện tại:** User tự động thấy dữ liệu của **TẤT CẢ** parent organizations.

**Vấn đề:**
- ❌ Vi phạm nguyên tắc "least privilege"
- ❌ Không phân biệt được dữ liệu riêng vs chung
- ❌ User ở cấp thấp có thể thấy dữ liệu nhạy cảm của cấp cao
- ❌ Phá vỡ logic Scope (Scope 0 vẫn thấy parents)

**Yêu cầu:**
- ✅ Cần phân biệt dữ liệu riêng vs chung
- ✅ Chỉ thêm parents nếu là dữ liệu chung
- ✅ Không thêm parents nếu là dữ liệu riêng

---

### **2. Phân Biệt Dữ Liệu Riêng vs Chung**

**Yêu cầu:**
- ✅ Dữ liệu ở cấp cao (Group/Company/Department - Level 0-2) → Dữ liệu chung
- ✅ Dữ liệu ở cấp thấp (Division/Team - Level 3+) → Dữ liệu riêng
- ✅ Dữ liệu chung tự động visible cho tất cả children
- ✅ Dữ liệu riêng chỉ visible cho organization đó và children (nếu Scope 1)

**Giải pháp đề xuất:**
- Dựa vào **Level** của organization để phân biệt
- Hoặc dựa vào **Type** của organization (group/company/department vs division/team)

---

## 📝 Tóm Tắt Yêu Cầu

### **Yêu Cầu Chức Năng:**

1. ✅ **Quản lý dữ liệu theo tổ chức:**
   - Mỗi dòng dữ liệu thuộc về một tổ chức (`organizationId`)
   - Tự động gán `organizationId` khi tạo mới
   - Không cho phép update `organizationId` trực tiếp

2. ✅ **Phân quyền theo scope:**
   - Scope 0: Chỉ tổ chức của role
   - Scope 1: Tổ chức + children

3. ✅ **Hỗ trợ dữ liệu riêng:**
   - Dữ liệu ở cấp thấp (Team/Division - Level 3+)
   - Chỉ organization đó và children (nếu Scope 1) thấy được

4. ✅ **Hỗ trợ dữ liệu chung:**
   - Dữ liệu ở cấp cao (Group/Company/Department - Level 0-2)
   - Tất cả children tự động thấy được
   - Không cần permission đặc biệt

5. ✅ **User ở cấp thấp thấy dữ liệu chung của cấp cao:**
   - User Team (Level 3) thấy dữ liệu của Department (Level 2)
   - User Team (Level 3) thấy dữ liệu của Company (Level 1)
   - User Team (Level 3) thấy dữ liệu của Group (Level 0)

### **Yêu Cầu Phi Chức Năng:**

1. ✅ **Đơn giản:**
   - Không cần field `isShared` cho từng document
   - Không cần permission đặc biệt để xem dữ liệu chung
   - Logic tự động, dễ hiểu

2. ✅ **Bảo mật:**
   - Tuân thủ nguyên tắc "least privilege"
   - User chỉ thấy dữ liệu được phép
   - Không thể bypass filter

3. ✅ **Hiệu năng:**
   - Filter tự động áp dụng cho mọi query
   - Có thể cache allowed organization IDs

4. ✅ **Dễ maintain:**
   - Logic rõ ràng, dễ debug
   - Tài liệu đầy đủ

---

## 🎯 Kết Luận

**Yêu cầu bài toán:**
- Hệ thống cần quản lý và phân quyền dữ liệu trong cấu trúc tổ chức dạng cây
- Hỗ trợ cả dữ liệu riêng (Team level) và dữ liệu chung (Department/Company level)
- User ở cấp thấp cần thấy dữ liệu chung của cấp cao
- Logic phải đơn giản, bảo mật, và dễ maintain

**Vấn đề cần giải quyết:**
- Logic tự động thêm TẤT CẢ parents phá vỡ phân quyền
- Cần phân biệt dữ liệu riêng vs chung
- Chỉ thêm parents nếu là dữ liệu chung (Level <= 2)

**Giải pháp đề xuất:**
- Level-Based Access: Chỉ thêm parents nếu parent có Level <= 2
- Hoặc Type-Based Access: Chỉ thêm parents nếu type là group/company/department

---

**Tài liệu này mô tả đầy đủ yêu cầu bài toán để làm cơ sở cho việc thiết kế và implement giải pháp.**
