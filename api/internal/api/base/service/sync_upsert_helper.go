package basesvc

import (
	"context"
	"errors"
	"reflect"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"meta_commerce/internal/api/events"
	"meta_commerce/internal/common"
	"meta_commerce/internal/utility"
)

// getUpdatedAtFromStructSourceField lấy Unix ms từ struct theo sourceDataField (posData/panCakeData).updated_at.
// So sánh cùng ở vị trí nguồn, không phụ thuộc field trích xuất (posUpdatedAt/panCakeUpdatedAt).
// Lưu ý: Nguồn lưu updated_at dạng string (ISO) hoặc primitive.DateTime; ParseTimestampFromMap xử lý cả hai.
func getUpdatedAtFromStructSourceField(data interface{}, sourceDataField string) int64 {
	if data == nil || sourceDataField == "" {
		return 0
	}
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return 0
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("bson")
		if tag == "" || tag == "-" {
			continue
		}
		key := strings.TrimSpace(strings.Split(tag, ",")[0])
		if key != sourceDataField {
			continue
		}
		val := v.Field(i)
		if val.Kind() != reflect.Map {
			return 0
		}
		if !val.CanInterface() {
			return 0
		}
		m, ok := val.Interface().(map[string]interface{})
		if !ok {
			return 0
		}
		return utility.ParseTimestampFromMap(m, "updated_at")
	}
	return 0
}

// ParseUpdatedAtFromSet lấy Unix ms từ updateData.Set theo sourceDataField (posData hoặc panCakeData) và key updated_at.
func ParseUpdatedAtFromSet(set map[string]interface{}, sourceDataField string) int64 {
	if set == nil {
		return 0
	}
	var m map[string]interface{}
	if data, ok := set[sourceDataField].(map[string]interface{}); ok {
		m = data
	}
	return utility.ParseTimestampFromMap(m, "updated_at")
}

// DoSyncUpsert thực hiện upsert có điều kiện: chỉ ghi khi dữ liệu mới hơn (updatedAtField) hoặc document chưa tồn tại.
// Sau khi xử lý phần đặc biệt (so sánh updated_at), gọi UpsertWithPreparedData để tránh lặp code.
func DoSyncUpsert[T any](ctx context.Context, svc *BaseServiceMongoImpl[T], filter interface{}, data interface{}, sourceDataField, updatedAtField string) (T, bool, error) {
	var zero T
	updateData, err := ToUpdateData(data)
	if err != nil {
		return zero, false, common.ErrInvalidFormat
	}
	newUpdatedAt := ParseUpdatedAtFromSet(updateData.Set, sourceDataField)

	var existing T
	err = svc.Collection().FindOne(ctx, filter).Decode(&existing)
	isExisting := (err == nil)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return zero, false, common.ConvertMongoError(err)
	}
	// Phần đặc biệt sync: skip nếu dữ liệu hiện tại mới hơn hoặc bằng.
	// So sánh cùng ở vị trí nguồn (posData/panCakeData.updated_at) để tránh lỗi do trích xuất.
	if isExisting && newUpdatedAt > 0 {
		existingUpdatedAt := getUpdatedAtFromStructSourceField(existing, sourceDataField)
		if existingUpdatedAt >= newUpdatedAt {
			return zero, true, nil
		}
	}

	if err := prepareUpsertUpdateData(ctx, zero, updateData, existing, isExisting); err != nil {
		return zero, false, err
	}
	if newUpdatedAt > 0 {
		updateData.Set[updatedAtField] = newUpdatedAt
	}

	condFilter := BuildSyncUpsertFilter(filter, updatedAtField, newUpdatedAt)
	var prevDoc interface{}
	if isExisting {
		prevDoc = existing
	}
	result, err := svc.UpsertWithPreparedData(ctx, condFilter, updateData, prevDoc)
	if err != nil {
		// Sync: khi duplicate key (race), thử update không upsert; nếu không match thì skip
		if mongo.IsDuplicateKeyError(err) {
			updateDoc := bson.M{"$set": updateData.Set}
			if updateData.SetOnInsert != nil {
				updateDoc["$setOnInsert"] = updateData.SetOnInsert
			}
			if updateData.Unset != nil {
				updateDoc["$unset"] = updateData.Unset
			}
			res, retryErr := svc.Collection().UpdateOne(ctx, condFilter, updateDoc)
			if retryErr != nil {
				return zero, false, common.ConvertMongoError(retryErr)
			}
			if res.MatchedCount == 0 && res.ModifiedCount == 0 {
				return zero, true, nil
			}
			var updated T
			_ = svc.Collection().FindOne(ctx, filter).Decode(&updated)
			events.EmitDataChanged(ctx, events.DataChangeEvent{
				CollectionName: svc.Collection().Name(),
				Operation:       events.OpUpdate,
				Document:        updated,
			})
			return updated, false, nil
		}
		return zero, false, err
	}
	return result, false, nil
}

// BuildSyncUpsertFilter tạo filter có điều kiện updated_at cho sync-upsert.
// Chỉ match khi: document chưa có updatedAtField HOẶC updatedAtField < newUpdatedAt.
// Khi newUpdatedAt==0 thì không thêm điều kiện (luôn ghi, tương đương upsert thường).
func BuildSyncUpsertFilter(baseFilter interface{}, updatedAtField string, newUpdatedAt int64) bson.M {
	f := bson.M{}
	if m, ok := baseFilter.(map[string]interface{}); ok {
		for k, v := range m {
			f[k] = v
		}
	} else if d, ok := baseFilter.(bson.D); ok {
		for _, e := range d {
			f[e.Key] = e.Value
		}
	}
	if newUpdatedAt > 0 {
		f["$or"] = []bson.M{
			{updatedAtField: bson.M{"$lt": newUpdatedAt}},
			{updatedAtField: bson.M{"$exists": false}},
			{updatedAtField: nil},
		}
	}
	return f
}
