# Rà Soát Field Description Cho Các Model

**Mục đích:** Rà soát các model trong hệ thống để xác định model nào cần thêm field `Description` để người dùng hiểu được mục đích sử dụng.

---

## 📊 Tổng Quan

### **Đã Có Description/Describe:**
1. ✅ **NotificationChannel** - `Description` (đã thêm)
2. ✅ **NotificationTemplate** - `Description` (đã thêm)
3. ✅ **NotificationRoutingRule** - `Description` (đã thêm)
4. ✅ **NotificationChannelSender** - `Description` (đã thêm)
5. ✅ **OrganizationShare** - `Description` (đã thêm)
6. ✅ **Role** - `Describe` (đã có sẵn)
7. ✅ **Permission** - `Describe` (đã có sẵn)
8. ✅ **Agent** - `Describe` (đã có sẵn)
9. ✅ **AccessToken** - `Describe` (đã có sẵn)
10. ✅ **AuthLog** - `Describe` (đã có sẵn)

---

## 🔍 Các Model Cần Xem Xét

### **1. CTALibrary - ⚠️ NÊN THÊM**

**Lý do:**
- User tạo CTA templates để reuse
- Cần mô tả mục đích sử dụng của CTA
- Giúp người dùng hiểu khi nào dùng CTA nào

**Đề xuất:**
```go
type CTALibrary struct {
    // ... existing fields ...
    Description string `json:"description,omitempty" bson:"description,omitempty"` // Mô tả về CTA để người dùng hiểu được mục đích sử dụng
    // ... other fields ...
}
```

**Ví dụ sử dụng:**
- "CTA để xem chi tiết đơn hàng, dùng trong notification order_created"
- "CTA để phản hồi tin nhắn, dùng trong notification conversation_unreplied"

---

### **2. Organization - ⚠️ CÓ THỂ THÊM (Tùy chọn)**

**Lý do:**
- User tạo organizations
- Có thể cần mô tả mục đích của tổ chức
- Nhưng Name và Code đã đủ mô tả trong nhiều trường hợp

**Đề xuất:**
- **Không bắt buộc** - Name và Code đã đủ mô tả
- Nếu cần, có thể thêm `Description` optional

---

## ❌ Các Model KHÔNG CẦN Description

### **Dữ Liệu Nghiệp Vụ (Business Data):**
- ❌ **Customer, FbCustomer, PcPosCustomer** - Dữ liệu khách hàng, không phải config
- ❌ **PcOrder, PcPosOrder** - Đơn hàng, không cần mô tả
- ❌ **FbPage, FbConversation, FbMessage** - Dữ liệu từ Facebook, không phải config
- ❌ **PcPosProduct, PcPosCategory, PcPosVariation** - Sản phẩm, không cần mô tả
- ❌ **PcPosShop, PcPosWarehouse** - Cửa hàng, kho hàng, không cần mô tả

### **Dữ Liệu Hệ Thống (System Data):**
- ❌ **DeliveryQueueItem, DeliveryHistory** - Dữ liệu hệ thống, không phải config
- ❌ **CTATracking** - Tracking data, không cần mô tả
- ❌ **User** - Thông tin người dùng, không cần mô tả

### **Quan Hệ (Relationships):**
- ❌ **UserRole** - Quan hệ user-role, không cần mô tả
- ❌ **RolePermission** - Quan hệ role-permission, không cần mô tả

---

## ✅ Kết Luận

### **Model Cần Thêm Description:**

1. **CTALibrary** - ⚠️ **NÊN THÊM**
   - User tạo CTA templates
   - Cần mô tả mục đích sử dụng
   - Giúp người dùng hiểu khi nào dùng CTA nào

2. **Organization** - ⚠️ **TÙY CHỌN**
   - Có thể thêm nếu cần mô tả chi tiết
   - Nhưng Name và Code thường đã đủ

---

## 📝 Đề Xuất Implementation

### **Ưu Tiên 1: CTALibrary**

Thêm field `Description` vào model `CTALibrary` vì:
- User tạo và quản lý CTA templates
- Cần mô tả để hiểu mục đích sử dụng
- Giúp tái sử dụng CTA hiệu quả hơn

### **Ưu Tiên 2: Organization (Tùy chọn)**

Có thể thêm nếu cần, nhưng không bắt buộc vì:
- Name và Code thường đã đủ mô tả
- Có thể thêm sau nếu có yêu cầu cụ thể

---

**Tài liệu này rà soát các model cần thêm field Description để người dùng hiểu được mục đích sử dụng.**
