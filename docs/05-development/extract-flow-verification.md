# Kiểm tra luồng Extract (struct tag `extract`)

## Kết luận: **Luồng extract đã chạy hoàn toàn** khi dữ liệu đi qua `ToMap`.

---

## 1. Luồng Order (webhook)

```
handleOrderEvent(ctx, payload)
  → order := PcPosOrder{PosData: orderData}
  → pcPosOrderService.Upsert(ctx, bson.M{"orderId": orderId}, order)
       ↓
BaseServiceMongoImpl.Upsert(ctx, filter, data)
  → updateData, err := ToUpdateData(data)   // data = PcPosOrder (value)
       ↓
ToUpdateData(data)
  → data không phải *UpdateData / UpdateData / []byte
  → dataMap, err := utility.ToMap(data)     // ← ENTRY EXTRACT
       ↓
utility.ToMap(s)
  → s là value struct → tạo ptrVal = &copy, toMarshal = ptrVal
  → ExtractDataIfExists(toMarshal)          // ← EXTRACT CHẠY Ở ĐÂY (pointer)
  → bson.Marshal(ptrVal.Elem()) → map
  → return dataMap (đã có đủ field flatten)
       ↓
ToUpdateData return &UpdateData{Set: dataMap}
  → Upsert dùng updateData.Set cho $set → FindOneAndUpdate
```

- **Extract có chạy:** Có. `ToMap(data)` nhận value → tạo pointer bên trong → gọi `ExtractDataIfExists(pointer)` → struct được điền đủ field → marshal ra map → `$set` nhận map đã có flatten.
- **OrderId từ posData dạng `{"$numberLong": "3973"}`:** Được unwrap trong `applyConverter` (đã thêm unwrap Extended JSON) nên extract vẫn thành công.

---

## 2. Luồng InsertOne (API tạo order/customer)

```
Handler InsertOne / Create
  → model, _ := TransformCreateInputToModel(&input)  // model chỉ có PosData
  → service.InsertOne(ctx, *model)
       ↓
BaseServiceMongoImpl.InsertOne(ctx, data)
  → dataMap, err := utility.ToMap(data)     // data = PcPosOrder (value)
       ↓
utility.ToMap(s)
  → ptrVal = &copy, toMarshal = ptrVal
  → ExtractDataIfExists(toMarshal)          // ← EXTRACT CHẠY
  → return dataMap (đã flatten)
  → InsertOne(ctx, dataMap)
```

- **Extract có chạy:** Có. Cùng cơ chế: value vào `ToMap` → tạo pointer → extract → map có đủ field → insert đúng.

---

## 3. Điểm quan trọng trong code

### 3.1 ExtractDataIfExists chỉ chạy khi nhận **pointer**

- `data.extract.go` (khoảng dòng 195–199): Nếu nhận **value** (không phải pointer) thì `return nil` ngay, **không** extract.
- Trong `ToMap`: luôn chuyển value thành pointer rồi mới gọi `ExtractDataIfExists(toMarshal)` nên extract luôn chạy khi đi qua ToMap.

### 3.2 ToUpdateData chỉ gọi ToMap khi data là struct thường

- `service.base.mongo.go` (khoảng dòng 42–94): Nếu `data` là `*UpdateData` / `UpdateData` / `[]byte` thì **không** gọi `ToMap` → không có extract.
- Webhook order truyền `PcPosOrder` (struct) → không rơi vào các case trên → gọi `utility.ToMap(data)` → extract chạy.

### 3.3 Khi extract lỗi

- Nếu một field **required** extract lỗi → `ExtractDataIfExists` return error → `ToMap` return error → `ToUpdateData` / `Upsert` fail → webhook/API trả lỗi, **không** ghi document thiếu field.
- Field **optional** lỗi chỉ bỏ qua field đó, các field khác vẫn extract.

---

## 4. Luồng chưa chạy extract (customer webhook hiện tại)

```
handleCustomerEvent(ctx, payload)
  → FindOneAndUpdate với $set: { posData, shopId, posUpdatedAt, updatedAt }
  → Không gọi ToMap / Upsert → không có extract
```

- Customer webhook **không** đi qua `ToMap` nên **không** chạy extract → document customer chỉ có vài field ở root. Muốn đủ field flatten cần đổi customer webhook sang dùng `Upsert` (giống order).

---

## 5. Tóm tắt

| Luồng | Có gọi ToMap? | Extract chạy? |
|-------|----------------|----------------|
| Webhook order (Upsert) | Có (trong ToUpdateData) | Có |
| API InsertOne (order/customer) | Có | Có |
| API UpdateOne (raw update) | Không (update là bson.M) | Không |
| Webhook customer (hiện tại) | Không | Không |

**Kết luận:** Luồng extract đã chạy hoàn toàn cho mọi chỗ đi qua `ToMap` (order webhook + InsertOne). Customer webhook chưa dùng Upsert nên chưa có extract; muốn đủ field thì cần chuyển customer sang Upsert.
