// Package events cung cấp cơ chế event trung tâm khi dữ liệu thay đổi qua CRUD.
// Các service CRUD không cần override từng method — BaseServiceMongoImpl tự động phát event.
// Vision L1: chỉ hook aidecision đăng ký OnDataChanged → decision_events_queue; consumer gọi applyDatachangedSideEffects (một cửa: ingest / report / ads / refresh metrics).
package events

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
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
	case "meta_ad_insights": // global.MongoDB_ColNames.MetaAdInsights
		// dateStart là string YYYY-MM-DD — không dùng timestamp; hook dùng trực tiếp dateStart làm periodKey.
		return 0
	case "meta_campaigns", "meta_ad_accounts", "meta_adsets", "meta_ads":
		ts := GetInt64Field(doc, "UpdatedAt")
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

// GetStringField lấy giá trị string của field từ document (dùng reflection).
// Dùng để lấy dateStart từ meta_ad_insights.
func GetStringField(doc interface{}, fieldName string) string {
	if doc == nil {
		return ""
	}
	val := reflect.ValueOf(doc)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return ""
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return ""
	}
	f := val.FieldByName(fieldName)
	if !f.IsValid() || !f.CanInterface() {
		return ""
	}
	if f.Kind() == reflect.String {
		return f.String()
	}
	return ""
}

// GetNestedStringField lấy giá trị string từ document theo đường dẫn (struct field hoặc map key).
// Ví dụ: GetNestedStringField(doc, "posData", "ad_id") hoặc GetNestedStringField(doc, "panCakeData", "ad_ids").
// Hỗ trợ cả struct (FieldByName) và map (key lookup). Thử cả "PosData"/"posData" nếu cần.
func GetNestedStringField(doc interface{}, path ...string) string {
	if doc == nil || len(path) == 0 {
		return ""
	}
	v := getNestedValue(doc, path[0])
	if v == nil || len(path) == 1 {
		return toString(v)
	}
	for i := 1; i < len(path); i++ {
		v = getNestedValue(v, path[i])
		if v == nil {
			return ""
		}
	}
	return toString(v)
}

// GetNestedStringSlice lấy slice string từ document (ví dụ panCakeData.ad_ids).
// Trả về nil nếu không tìm thấy hoặc không phải array.
func GetNestedStringSlice(doc interface{}, path ...string) []string {
	if doc == nil || len(path) == 0 {
		return nil
	}
	v := getNestedValue(doc, path[0])
	for i := 1; i < len(path) && v != nil; i++ {
		v = getNestedValue(v, path[i])
	}
	if v == nil {
		return nil
	}
	return toSliceString(v)
}

func getNestedValue(doc interface{}, key string) interface{} {
	if doc == nil {
		return nil
	}
	// Thử map trước (bson.M, map[string]interface{})
	if m, ok := doc.(map[string]interface{}); ok {
		if v, ok := m[key]; ok {
			return v
		}
		if v, ok := m[toLowerFirst(key)]; ok {
			return v
		}
		return nil
	}
	// Struct qua reflection
	val := reflect.ValueOf(doc)
	if val.Kind() == reflect.Ptr && !val.IsNil() {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil
	}
	f := val.FieldByName(key)
	if !f.IsValid() {
		f = val.FieldByName(toUpperFirst(key))
	}
	if !f.IsValid() || !f.CanInterface() {
		return nil
	}
	return f.Interface()
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(x, 10)
	case int:
		return strconv.Itoa(x)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func toSliceString(v interface{}) []string {
	if v == nil {
		return nil
	}
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
		out := make([]string, 0, val.Len())
		for i := 0; i < val.Len(); i++ {
			out = append(out, toString(val.Index(i).Interface()))
		}
		return out
	}
	return nil
}

func toLowerFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func toUpperFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
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
