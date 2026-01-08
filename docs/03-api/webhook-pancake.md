# Webhook Integration - Pancake & Pancake POS

## 📚 Tổng quan

Tài liệu này hướng dẫn cách cấu hình và sử dụng webhook từ **Pancake** và **Pancake POS** để nhận dữ liệu real-time vào hệ thống.

**Tài liệu tham khảo:**
- [Pancake API Docs](https://api-docs.pancake.vn/)
- [Pancake Webhooks Docs](https://docs.pancake.biz/pancake/st-f12/st-p2?lang=en)

---

## 🔗 Endpoints Webhook

Hệ thống cung cấp 2 endpoints để nhận webhook:

### 1. Pancake Webhook
```
POST /api/v1/pancake/webhook
```

**Mục đích:** Nhận webhook từ Pancake về các events như:
- `conversation_updated` - Cuộc hội thoại được cập nhật
- `message_received` - Nhận tin nhắn mới
- `order_created` - Đơn hàng mới được tạo
- `order_updated` - Đơn hàng được cập nhật
- `customer_updated` - Khách hàng được cập nhật

### 2. Pancake POS Webhook
```
POST /api/v1/pancake-pos/webhook
```

**Mục đích:** Nhận webhook từ Pancake POS về các events như:
- `order_created` - Đơn hàng mới được tạo
- `order_updated` - Đơn hàng được cập nhật
- `order_status_changed` - Trạng thái đơn hàng thay đổi
- `product_created/updated` - Sản phẩm được tạo/cập nhật
- `customer_created/updated` - Khách hàng được tạo/cập nhật
- `inventory_updated` - Tồn kho được cập nhật

---

## ⚙️ Cấu hình Webhook trên Pancake

### Bước 1: Truy cập Cấu hình Webhook

1. Đăng nhập vào tài khoản **Pancake** của bạn
2. Điều hướng đến phần **Cấu hình**
3. Chọn mục **Webhook/API** trong phần **Nâng cao**

### Bước 2: Lấy API Key

- Tại trang **Webhook/API**, bạn sẽ thấy **API Key**
- Sao chép giá trị này để sử dụng trong quá trình tích hợp

### Bước 3: Cấu hình Webhook URL

1. Trong phần **Webhook URL**, nhập địa chỉ URL của hệ thống:
   ```
   https://yourdomain.com/api/v1/pancake/webhook
   ```

2. Chọn loại dữ liệu bạn muốn nhận qua Webhook:
   - ✅ Đơn hàng (Orders)
   - ✅ Khách hàng (Customers)
   - ✅ Cuộc hội thoại (Conversations)
   - ✅ Tin nhắn (Messages)
   - etc.

3. Nhập email để nhận thông báo lỗi (nếu có)

4. Thêm các **Request Headers** cần thiết (nếu có):
   - **Key**: `X-API-Key` (hoặc tên header tùy chỉnh)
   - **Value**: API Key của bạn

### Bước 4: Lưu cấu hình

- Nhấn **Lưu** để áp dụng các thay đổi

---

## ⚙️ Cấu hình Webhook trên Pancake POS

### Bước 1: Truy cập Cấu hình Webhook

1. Đăng nhập vào tài khoản **Pancake POS** của bạn
2. Điều hướng đến phần **Cấu hình**
3. Chọn mục **Kết nối bên thứ 3** trong phần **Nâng cao**
4. Chọn **Webhook/API**

### Bước 2: Lấy API Key

- Tại trang **Webhook/API**, bạn sẽ thấy **API Key**
- Sao chép giá trị này để sử dụng trong quá trình tích hợp

### Bước 3: Cấu hình Webhook URL

1. Trong phần **Webhook URL**, nhập địa chỉ URL của hệ thống:
   ```
   https://yourdomain.com/api/v1/pancake-pos/webhook
   ```

2. Chọn loại dữ liệu bạn muốn nhận qua Webhook:
   - ✅ Đơn hàng (Orders)
   - ✅ Khách hàng (Customers)
   - ✅ Sản phẩm (Products)
   - ✅ Tồn kho (Inventory)
   - etc.

3. Nhập email để nhận thông báo lỗi (nếu có)

4. Thêm các **Request Headers** cần thiết (nếu có):
   - **Key**: `X-API-Key` (hoặc tên header tùy chỉnh)
   - **Value**: API Key của bạn

### Bước 4: Lưu cấu hình

- Nhấn **Lưu** để áp dụng các thay đổi

---

## 📋 Format dữ liệu Webhook

### Pancake Webhook Payload

```json
{
  "payload": {
    "eventType": "conversation_updated",
    "pageId": "123456789",
    "data": {
      // Dữ liệu chi tiết của event
    },
    "timestamp": 1234567890
  },
  "signature": "optional_signature"
}
```

### Pancake POS Webhook Payload

```json
{
  "payload": {
    "eventType": "order_created",
    "shopId": 123,
    "data": {
      // Dữ liệu chi tiết của event
    },
    "timestamp": 1234567890
  },
  "signature": "optional_signature"
}
```

---

## 🔒 Bảo mật Webhook

### Xác thực bằng API Key

Pancake và Pancake POS có thể gửi API Key trong:
- **Query Parameter**: `?api_key=YOUR_API_KEY`
- **Request Header**: `X-API-Key: YOUR_API_KEY`

**Lưu ý:** Hiện tại endpoint webhook chưa verify API Key. Cần implement verification trong tương lai.

### Xác thực bằng Signature (nếu có)

Nếu Pancake hỗ trợ signature verification:
- Verify signature từ request body
- Sử dụng secret key để verify HMAC signature

---

## 📝 Response Format

Endpoint webhook trả về response theo format chuẩn:

### Success Response (200 OK)

```json
{
  "code": 200,
  "message": "Webhook đã được nhận và xử lý thành công",
  "data": {
    "eventType": "order_created",
    "pageId": "123456789"
  },
  "status": "success"
}
```

### Error Response (400 Bad Request)

```json
{
  "code": "VAL_002",
  "message": "Dữ liệu gửi lên không đúng định dạng JSON",
  "status": "error"
}
```

---

## 🧪 Testing Webhook

### Test với cURL

**Pancake Webhook:**
```bash
curl -X POST https://yourdomain.com/api/v1/pancake/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "payload": {
      "eventType": "conversation_updated",
      "pageId": "123456789",
      "data": {},
      "timestamp": 1234567890
    }
  }'
```

**Pancake POS Webhook:**
```bash
curl -X POST https://yourdomain.com/api/v1/pancake-pos/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "payload": {
      "eventType": "order_created",
      "shopId": 123,
      "data": {},
      "timestamp": 1234567890
    }
  }'
```

---

## 📊 Xử lý Webhook Events

### Pancake Events

| Event Type | Mô tả | Xử lý |
|------------|-------|-------|
| `conversation_updated` | Cuộc hội thoại được cập nhật | Lưu vào `fb_conversation` collection |
| `message_received` | Nhận tin nhắn mới | Lưu vào `fb_message` collection |
| `order_created` | Đơn hàng mới được tạo | Lưu vào `pc_order` collection |
| `order_updated` | Đơn hàng được cập nhật | Cập nhật `pc_order` collection |
| `customer_updated` | Khách hàng được cập nhật | Cập nhật `fb_customer` collection |

### Pancake POS Events

| Event Type | Mô tả | Xử lý |
|------------|-------|-------|
| `order_created` | Đơn hàng mới được tạo | Lưu vào `pc_pos_order` collection |
| `order_updated` | Đơn hàng được cập nhật | Cập nhật `pc_pos_order` collection |
| `order_status_changed` | Trạng thái đơn hàng thay đổi | Cập nhật status + trigger notification |
| `product_created` | Sản phẩm mới được tạo | Lưu vào `pc_pos_product` collection |
| `product_updated` | Sản phẩm được cập nhật | Cập nhật `pc_pos_product` collection |
| `customer_created` | Khách hàng mới được tạo | Lưu vào `pc_pos_customer` collection |
| `customer_updated` | Khách hàng được cập nhật | Cập nhật `pc_pos_customer` collection |
| `inventory_updated` | Tồn kho được cập nhật | Cập nhật inventory trong `pc_pos_variation` |

---

## ⚠️ Lưu ý quan trọng

1. **Bảo mật Endpoint:**
   - Đảm bảo endpoint webhook được bảo mật (HTTPS)
   - Implement API Key verification (TODO)
   - Implement signature verification nếu Pancake hỗ trợ (TODO)

2. **Xử lý Lỗi:**
   - Endpoint luôn trả về 200 OK để Pancake không retry
   - Log tất cả errors để debug
   - Có thể implement queue để xử lý async

3. **Performance:**
   - Xử lý webhook nhanh chóng (< 5 giây)
   - Tránh blocking operations
   - Sử dụng background workers nếu cần

4. **Monitoring:**
   - Monitor số lượng webhook nhận được
   - Track errors và retries
   - Alert khi webhook không hoạt động

---

## 🔄 Workflow xử lý Webhook

```
1. Pancake/Pancake POS gửi webhook → POST /api/v1/pancake/webhook
2. Handler nhận và parse request body
3. Validate dữ liệu (eventType, pageId/shopId)
4. Verify API Key/Signature (TODO)
5. Log webhook received
6. Xử lý dựa trên eventType:
   - Lưu vào database
   - Trigger notification
   - Đồng bộ dữ liệu
7. Trả về 200 OK
```

---

## 📚 Tài liệu tham khảo

- [Pancake API Documentation](https://api-docs.pancake.vn/)
- [Pancake Webhooks Documentation](https://docs.pancake.biz/pancake/st-f12/st-p2?lang=en)
- [Pancake POS API Documentation](docs-shared/ai-context/pancake-pos/api-context.md)

---

## 🐛 Troubleshooting

### Webhook không nhận được

1. Kiểm tra URL webhook có đúng không
2. Kiểm tra server có accessible từ internet không
3. Kiểm tra firewall/security groups
4. Kiểm tra logs để xem có request đến không

### Webhook nhận được nhưng lỗi

1. Kiểm tra format dữ liệu có đúng không
2. Kiểm tra logs để xem lỗi cụ thể
3. Kiểm tra database connection
4. Kiểm tra validation errors

### Webhook chậm

1. Kiểm tra performance của endpoint
2. Kiểm tra database queries
3. Cân nhắc sử dụng background workers
4. Optimize code xử lý webhook
