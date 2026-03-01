// Package events cung cấp cơ chế event trung tâm khi dữ liệu thay đổi qua CRUD.
// Các service CRUD không cần override từng method — BaseServiceMongoImpl tự động phát event.
// Logic phản ứng (report MarkDirty, cache invalidation, ...) đăng ký qua OnDataChanged.
package events

import (
	"context"
	"reflect"
	"sync"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// OpInsert, OpUpdate, OpUpsert, OpDelete là các loại thao tác CRUD.
const (
	OpInsert = "insert"
	OpUpdate = "update"
	OpUpsert = "upsert"
	OpDelete = "delete"
)

// DataChangeEvent mô tả sự kiện thay đổi dữ liệu.
// Document là bản ghi sau khi thay đổi (nil nếu delete).
// PreviousDocument có khi Operation = update (từ UpdateOne); nil cho insert/upsert/delete.
type DataChangeEvent struct {
	CollectionName    string
	Operation         string
	Document          interface{}
	PreviousDocument  interface{} // Document trước khi update; dùng để so sánh skip MarkDirty/Merge
}

// DataChangeHandler xử lý sự kiện thay đổi dữ liệu.
type DataChangeHandler func(ctx context.Context, e DataChangeEvent)

var (
	handlers   []DataChangeHandler
	handlersMu sync.RWMutex
)

// OnDataChanged đăng ký handler. Gọi khi init (ví dụ từ report package).
func OnDataChanged(h DataChangeHandler) {
	handlersMu.Lock()
	defer handlersMu.Unlock()
	handlers = append(handlers, h)
}

// EmitDataChanged phát sự kiện. Gọi từ BaseServiceMongoImpl sau mỗi CRUD thành công.
// Mỗi handler chạy trong goroutine riêng, panic được recover để không ảnh hưởng handler khác.
func EmitDataChanged(ctx context.Context, e DataChangeEvent) {
	handlersMu.RLock()
	list := make([]DataChangeHandler, len(handlers))
	copy(list, handlers)
	handlersMu.RUnlock()

	for _, h := range list {
		go func(fn DataChangeHandler) {
			defer func() {
				if r := recover(); r != nil {
					// Log panic nhưng không làm sập app
					// Logger có thể chưa init khi event chạy sớm
					_ = r
				}
			}()
			fn(ctx, e)
		}(h)
	}
}

// GetOwnerOrganizationIDFromDocument lấy ownerOrganizationId từ document (dùng reflection).
// Trả về zero ObjectID nếu document không có field OwnerOrganizationID.
func GetOwnerOrganizationIDFromDocument(doc interface{}) primitive.ObjectID {
	if doc == nil {
		return primitive.NilObjectID
	}
	val := reflect.ValueOf(doc)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return primitive.NilObjectID
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return primitive.NilObjectID
	}
	f := val.FieldByName("OwnerOrganizationID")
	if !f.IsValid() {
		return primitive.NilObjectID
	}
	switch f.Kind() {
	case reflect.Array, reflect.Struct:
		// primitive.ObjectID là [12]byte
		if obj, ok := f.Interface().(primitive.ObjectID); ok {
			return obj
		}
		return primitive.NilObjectID
	case reflect.Ptr:
		if f.IsNil() {
			return primitive.NilObjectID
		}
		if ptr, ok := f.Interface().(*primitive.ObjectID); ok && ptr != nil {
			return *ptr
		}
		if obj, ok := f.Elem().Interface().(primitive.ObjectID); ok {
			return obj
		}
		return primitive.NilObjectID
	default:
		return primitive.NilObjectID
	}
}

// GetPeriodTimestamp lấy timestamp quyết định period cho report (theo collection).
// Dùng để so sánh prev vs new — nếu bằng nhau thì skip MarkDirty.
func GetPeriodTimestamp(doc interface{}, collectionName string) int64 {
	if doc == nil {
		return 0
	}
	switch collectionName {
	case "pc_pos_orders":
		ts := GetInt64Field(doc, "PosCreatedAt")
		if ts == 0 {
			ts = GetInt64Field(doc, "InsertedAt")
		}
		if ts == 0 {
			ts = GetInt64Field(doc, "CreatedAt")
		}
		if ts > 1e12 {
			ts = ts / 1000
		}
		return ts
	case "pc_pos_customers":
		ts := GetInt64Field(doc, "UpdatedAt")
		if ts == 0 {
			ts = GetInt64Field(doc, "LastOrderAt")
		}
		if ts == 0 {
			ts = GetInt64Field(doc, "CreatedAt")
		}
		if ts > 1e12 {
			ts = ts / 1000
		}
		return ts
	case "crm_activity_history":
		ts := GetInt64Field(doc, "ActivityAt")
		if ts == 0 {
			ts = GetInt64Field(doc, "CreatedAt")
		}
		if ts > 1e12 {
			ts = ts / 1000
		}
		return ts
	}
	return 0
}

// GetInt64Field lấy giá trị int64 của field từ document (dùng reflection).
// Dùng để lấy posCreatedAt, insertedAt, createdAt cho report period.
func GetInt64Field(doc interface{}, fieldName string) int64 {
	if doc == nil {
		return 0
	}
	val := reflect.ValueOf(doc)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return 0
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return 0
	}
	f := val.FieldByName(fieldName)
	if !f.IsValid() || !f.CanInterface() {
		return 0
	}
	switch f.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return f.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(f.Uint())
	default:
		return 0
	}
}
