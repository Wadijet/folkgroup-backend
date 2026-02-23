// package basesvc cung cấp các service cơ bản cho việc tương tác với MongoDB
package basesvc

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	basemodels "meta_commerce/internal/api/base/models"
	"meta_commerce/internal/api/events"
	"meta_commerce/internal/common"
	"meta_commerce/internal/utility"
)

// isAdminFromContextFunc được gán từ auth domain (initsvc) để tránh import cycle services -> auth.
var isAdminFromContextFunc func(context.Context) (bool, error)

// SetIsAdminFromContextFunc đăng ký hàm kiểm tra user trong context có phải Administrator.
// Gọi từ initsvc.NewInitService hoặc nơi khởi tạo app (auth đã load).
func SetIsAdminFromContextFunc(fn func(context.Context) (bool, error)) {
	isAdminFromContextFunc = fn
}

// UpdateData định nghĩa kiểu dữ liệu cho partial update
type UpdateData struct {
	Set         map[string]interface{} `bson:"$set,omitempty"`         // Các trường cần update
	SetOnInsert map[string]interface{} `bson:"$setOnInsert,omitempty"` // Các trường chỉ set khi insert (upsert tạo mới)
	Unset       map[string]interface{} `bson:"$unset,omitempty"`       // Các trường cần xóa
	Push        map[string]interface{} `bson:"$push,omitempty"`        // Các trường cần thêm vào array
	AddToSet    map[string]interface{} `bson:"$addToSet,omitempty"`    // Các trường cần thêm vào set
}

// ToUpdateData chuyển đổi interface{} thành UpdateData
func ToUpdateData(data interface{}) (*UpdateData, error) {
	// Nếu data đã là UpdateData, return luôn
	if update, ok := data.(*UpdateData); ok {
		return update, nil
	}

	// Nếu data là UpdateData (không phải pointer), chuyển đổi thành pointer
	if update, ok := data.(UpdateData); ok {
		return &update, nil
	}

	// Nếu data là []byte (BSON raw), unmarshal trực tiếp
	if rawData, ok := data.([]byte); ok {
		update := &UpdateData{}
		if err := bson.Unmarshal(bson.Raw(rawData), update); err != nil {
			return nil, err
		}
		return update, nil
	}

	// Chuyển data thành map
	dataMap, err := utility.ToMap(data)
	if err != nil {
		return nil, err
	}

	// Nếu data có sẵn các operator MongoDB ($set, $unset, etc)
	// Xây dựng UpdateData từ map trực tiếp
	if _, hasSet := dataMap["$set"]; hasSet {
		update := &UpdateData{}
		if setVal, ok := dataMap["$set"].(map[string]interface{}); ok {
			update.Set = setVal
		}
		if unsetVal, ok := dataMap["$unset"].(map[string]interface{}); ok {
			update.Unset = unsetVal
		}
		if setOnInsertVal, ok := dataMap["$setOnInsert"].(map[string]interface{}); ok {
			update.SetOnInsert = setOnInsertVal
		}
		if pushVal, ok := dataMap["$push"].(map[string]interface{}); ok {
			update.Push = pushVal
		}
		if addToSetVal, ok := dataMap["$addToSet"].(map[string]interface{}); ok {
			update.AddToSet = addToSetVal
		}
		return update, nil
	}

	// Nếu data là map thường, wrap trong $set
	return &UpdateData{
		Set: dataMap,
	}, nil
}

// ====================================
// INTERFACE VÀ STRUCT
// ====================================

// BaseServiceMongo định nghĩa interface chứa các phương thức cơ bản cho việc tương tác với MongoDB
// Type Parameters:
//   - Model: Kiểu dữ liệu của model
type BaseServiceMongo[Model any] interface {
	// NHÓM 1: CÁC HÀM CHUẨN MONGODB DRIVER
	// ====================================

	// 1.1 Thao tác Insert
	InsertOne(ctx context.Context, data Model) (Model, error)
	InsertMany(ctx context.Context, data []Model) ([]Model, error)

	// 1.2 Thao tác Find
	FindOne(ctx context.Context, filter interface{}, opts *options.FindOneOptions) (Model, error)
	Find(ctx context.Context, filter interface{}, opts *options.FindOptions) ([]Model, error)

	// 1.3 Thao tác Update
	UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts *options.UpdateOptions) (Model, error)
	UpdateMany(ctx context.Context, filter interface{}, update interface{}, opts *options.UpdateOptions) (int64, error)

	// 1.4 Thao tác Delete
	DeleteOne(ctx context.Context, filter interface{}) error
	DeleteMany(ctx context.Context, filter interface{}) (int64, error)

	// 1.5 Thao tác Atomic
	FindOneAndUpdate(ctx context.Context, filter interface{}, update interface{}, opts *options.FindOneAndUpdateOptions) (Model, error)
	FindOneAndDelete(ctx context.Context, filter interface{}, opts *options.FindOneAndDeleteOptions) (Model, error)

	// 1.6 Các thao tác khác
	CountDocuments(ctx context.Context, filter interface{}) (int64, error)
	Distinct(ctx context.Context, fieldName string, filter interface{}) ([]interface{}, error)

	// NHÓM 2: CÁC HÀM TIỆN ÍCH MỞ RỘNG
	// ================================

	// 2.1 Các hàm Find mở rộng
	FindOneById(ctx context.Context, id primitive.ObjectID) (Model, error)
	FindManyByIds(ctx context.Context, ids []primitive.ObjectID) ([]Model, error)
	FindWithPagination(ctx context.Context, filter interface{}, page, limit int64, opts *options.FindOptions) (*basemodels.PaginateResult[Model], error)

	// 2.2 Các hàm Update/Delete mở rộng
	UpdateById(ctx context.Context, id primitive.ObjectID, data interface{}) (Model, error)
	DeleteById(ctx context.Context, id primitive.ObjectID) error

	// 2.3 Các hàm Upsert tiện ích
	Upsert(ctx context.Context, filter interface{}, data interface{}) (Model, error)
	UpsertMany(ctx context.Context, filter interface{}, data []Model) ([]Model, error)

	// 2.4 Các hàm kiểm tra
	DocumentExists(ctx context.Context, filter interface{}) (bool, error)
}

// BaseServiceMongoImpl định nghĩa struct triển khai các phương thức cơ bản cho service
// Type Parameters:
//   - Model: Kiểu dữ liệu của model
type BaseServiceMongoImpl[T any] struct {
	collection *mongo.Collection // Collection MongoDB
}

// NewBaseServiceMongo tạo mới một BaseServiceImpl
// Parameters:
//   - collection: Collection MongoDB
//
// Returns:
//   - *BaseServiceImpl[T]: Instance mới của BaseServiceImpl
func NewBaseServiceMongo[T any](collection *mongo.Collection) *BaseServiceMongoImpl[T] {
	return &BaseServiceMongoImpl[T]{
		collection: collection,
	}
}

// Collection trả về collection MongoDB (dùng bởi subpackage như deliverysvc, ctasvc khi cần truy cập trực tiếp)
func (s *BaseServiceMongoImpl[T]) Collection() *mongo.Collection {
	return s.collection
}

// ====================================
// NHÓM 1: CÁC HÀM CHUẨN MONGODB DRIVER
// ====================================

// 1.1 Thao tác Insert
// -------------------

// InsertOne tạo mới một bản ghi trong database
func (s *BaseServiceMongoImpl[T]) InsertOne(ctx context.Context, data T) (T, error) {
	var zero T

	// ✅ Validate system data protection
	if err := validateSystemDataInsert(ctx, data); err != nil {
		return zero, err
	}

	// Áp dụng default từ struct tag (chỉ set field đang zero)
	applyInsertDefaultsToModel(&data)

	// Chuyển data thành map để thêm timestamps
	dataMap, err := utility.ToMap(data)
	if err != nil {
		return zero, common.ErrInvalidFormat
	}

	// Loại bỏ các field empty string để sparse unique index hoạt động đúng
	// Sparse index chỉ bỏ qua null/không tồn tại, không bỏ qua empty string
	// Nếu có nhiều document với empty string, sẽ bị duplicate key error
	for key, value := range dataMap {
		if strValue, ok := value.(string); ok && strValue == "" {
			// Xóa field empty string để sparse index bỏ qua nó
			delete(dataMap, key)
		}
	}

	// Thêm timestamps
	now := time.Now().UnixMilli()
	dataMap["createdAt"] = now
	dataMap["updatedAt"] = now

	result, err := s.collection.InsertOne(ctx, dataMap)
	if err != nil {
		return zero, common.ConvertMongoError(err)
	}

	// Lấy lại document vừa tạo
	var created T
	err = s.collection.FindOne(ctx, bson.M{"_id": result.InsertedID}).Decode(&created)
	if err != nil {
		return zero, common.ConvertMongoError(err)
	}

	events.EmitDataChanged(ctx, events.DataChangeEvent{
		CollectionName: s.collection.Name(),
		Operation:      events.OpInsert,
		Document:       created,
	})
	return created, nil
}

// InsertMany tạo nhiều bản ghi trong database
func (s *BaseServiceMongoImpl[T]) InsertMany(ctx context.Context, data []T) ([]T, error) {
	// ✅ Validate system data protection cho từng item
	for _, item := range data {
		if err := validateSystemDataInsert(ctx, item); err != nil {
			return nil, err
		}
	}

	var documents []interface{}
	now := time.Now().UnixMilli()

	for i := range data {
		applyInsertDefaultsToModel(&data[i])
	}
	for _, item := range data {
		dataMap, err := utility.ToMap(item)
		if err != nil {
			return nil, common.ErrInvalidFormat
		}
		dataMap["createdAt"] = now
		dataMap["updatedAt"] = now
		documents = append(documents, dataMap)
	}

	result, err := s.collection.InsertMany(ctx, documents)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}

	// Lấy lại các documents vừa tạo
	var created []T
	filter := bson.M{"_id": bson.M{"$in": result.InsertedIDs}}
	cursor, err := s.collection.Find(ctx, filter)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}

	err = cursor.All(ctx, &created)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}

	for i := range created {
		events.EmitDataChanged(ctx, events.DataChangeEvent{
			CollectionName: s.collection.Name(),
			Operation:      events.OpInsert,
			Document:       created[i],
		})
	}
	return created, nil
}

// 1.2 Thao tác Find
// ----------------

// FindOne tìm một document theo điều kiện lọc
func (s *BaseServiceMongoImpl[T]) FindOne(ctx context.Context, filter interface{}, opts *options.FindOneOptions) (T, error) {
	var zero T
	var result T

	if filter == nil {
		filter = bson.D{}
	}

	if opts == nil {
		opts = options.FindOne()
	}

	findResult := s.collection.FindOne(ctx, filter, opts)
	if err := findResult.Err(); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return zero, common.ErrNotFound
		}
		return zero, common.ConvertMongoError(err)
	}

	if err := findResult.Decode(&result); err != nil {
		// Kiểm tra xem có phải lỗi không tìm thấy document không
		if errors.Is(err, mongo.ErrNoDocuments) {
			return zero, common.ErrNotFound
		}
		// Lỗi decode BSON thường là lỗi format/validation, không phải lỗi MongoDB command
		// Xử lý như lỗi format để tránh trả về DB02
		return zero, common.NewError(
			common.ErrCodeValidationFormat,
			"Lỗi định dạng dữ liệu khi decode từ MongoDB",
			common.StatusBadRequest,
			err,
		)
	}

	return result, nil
}

// Find tìm tất cả bản ghi theo điều kiện lọc
func (s *BaseServiceMongoImpl[T]) Find(ctx context.Context, filter interface{}, opts *options.FindOptions) ([]T, error) {
	// Xử lý filter rỗng hoặc nil
	if filter == nil {
		filter = bson.D{}
	} else {
		// Kiểm tra nếu filter là map rỗng, chuyển thành bson.D{}
		if filterMap, ok := filter.(map[string]interface{}); ok && len(filterMap) == 0 {
			filter = bson.D{}
		}
	}

	if opts == nil {
		opts = options.Find()
	}

	cursor, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	var results []T
	if err = cursor.All(ctx, &results); err != nil {
		return nil, common.ConvertMongoError(err)
	}

	// Đảm bảo luôn trả về mảng, không phải nil
	if results == nil {
		results = []T{}
	}

	return results, nil
}

// 1.3 Thao tác Update
// ------------------

// UpdateOne cập nhật một document
func (s *BaseServiceMongoImpl[T]) UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts *options.UpdateOptions) (T, error) {
	var zero T

	if filter == nil {
		filter = bson.D{}
	}

	if opts == nil {
		opts = options.Update().SetUpsert(false)
	}

	// ✅ Lấy document hiện tại để kiểm tra IsSystem
	var existing T
	err := s.collection.FindOne(ctx, filter).Decode(&existing)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return zero, common.ErrNotFound
		}
		return zero, common.ConvertMongoError(err)
	}

	// Chuyển update thành UpdateData
	updateData, err := ToUpdateData(update)
	if err != nil {
		return zero, common.ErrInvalidFormat
	}

	// ✅ Validate system data protection
	if err := validateSystemDataUpdate(ctx, existing, updateData); err != nil {
		return zero, err
	}

	// Thêm updatedAt vào $set
	if updateData.Set == nil {
		updateData.Set = make(map[string]interface{})
	}
	updateData.Set["updatedAt"] = time.Now().UnixMilli()

	result, err := s.collection.UpdateOne(ctx, filter, updateData, opts)
	if err != nil {
		return zero, common.ConvertMongoError(err)
	}

	if result.ModifiedCount == 0 && result.UpsertedCount == 0 {
		return zero, common.ErrNotFound
	}

	// Lấy lại document đã update
	var updated T
	if result.UpsertedID != nil {
		err = s.collection.FindOne(ctx, bson.M{"_id": result.UpsertedID}).Decode(&updated)
	} else {
		err = s.collection.FindOne(ctx, filter).Decode(&updated)
	}
	if err != nil {
		return zero, common.ConvertMongoError(err)
	}

	events.EmitDataChanged(ctx, events.DataChangeEvent{
		CollectionName: s.collection.Name(),
		Operation:      events.OpUpdate,
		Document:       updated,
	})
	return updated, nil
}

// UpdateMany cập nhật nhiều document
func (s *BaseServiceMongoImpl[T]) UpdateMany(ctx context.Context, filter interface{}, update interface{}, opts *options.UpdateOptions) (int64, error) {
	if filter == nil {
		filter = bson.D{}
	}

	if opts == nil {
		opts = options.Update().SetUpsert(false)
	}

	// ✅ Kiểm tra tất cả documents match filter có IsSystem không
	// Lấy tất cả documents sẽ bị update
	cursor, err := s.collection.Find(ctx, filter)
	if err != nil {
		return 0, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	var existingDocs []T
	if err := cursor.All(ctx, &existingDocs); err != nil {
		return 0, common.ConvertMongoError(err)
	}

	// Chuyển update thành UpdateData
	updateData, err := ToUpdateData(update)
	if err != nil {
		return 0, common.ErrInvalidFormat
	}

	// ✅ Validate system data protection cho từng document
	for _, existing := range existingDocs {
		if err := validateSystemDataUpdate(ctx, existing, updateData); err != nil {
			return 0, err
		}
	}

	// Thêm updatedAt vào $set
	if updateData.Set == nil {
		updateData.Set = make(map[string]interface{})
	}
	updateData.Set["updatedAt"] = time.Now().UnixMilli()

	result, err := s.collection.UpdateMany(ctx, filter, updateData, opts)
	if err != nil {
		return 0, common.ConvertMongoError(err)
	}

	return result.ModifiedCount, nil
}

// 1.4 Thao tác Delete
// ------------------

// DeleteOne xóa một document
func (s *BaseServiceMongoImpl[T]) DeleteOne(ctx context.Context, filter interface{}) error {
	if filter == nil {
		filter = bson.D{}
	}

	// ✅ Lấy document cần xóa để kiểm tra IsSystem
	var existing T
	err := s.collection.FindOne(ctx, filter).Decode(&existing)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return common.ErrNotFound
		}
		return common.ConvertMongoError(err)
	}

	// ✅ Validate system data protection
	if err := validateSystemDataDelete(ctx, existing); err != nil {
		return err
	}

	// ✅ Validate relationships từ struct tag
	if err := validateRelationshipsDelete(ctx, existing); err != nil {
		return err
	}

	result, err := s.collection.DeleteOne(ctx, filter)
	if err != nil {
		return common.ConvertMongoError(err)
	}

	if result.DeletedCount == 0 {
		return common.ErrNotFound
	}

	events.EmitDataChanged(ctx, events.DataChangeEvent{
		CollectionName: s.collection.Name(),
		Operation:      events.OpDelete,
		Document:       existing,
	})
	return nil
}

// DeleteMany xóa nhiều document
func (s *BaseServiceMongoImpl[T]) DeleteMany(ctx context.Context, filter interface{}) (int64, error) {
	if filter == nil {
		filter = bson.D{}
	}

	// ✅ Kiểm tra tất cả documents match filter có IsSystem không
	cursor, err := s.collection.Find(ctx, filter)
	if err != nil {
		return 0, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	var existingDocs []T
	if err := cursor.All(ctx, &existingDocs); err != nil {
		return 0, common.ConvertMongoError(err)
	}

	// ✅ Validate system data protection và relationships cho từng document
	for _, existing := range existingDocs {
		if err := validateSystemDataDelete(ctx, existing); err != nil {
			return 0, err
		}
		if err := validateRelationshipsDelete(ctx, existing); err != nil {
			return 0, err
		}
	}

	result, err := s.collection.DeleteMany(ctx, filter)
	if err != nil {
		return 0, common.ConvertMongoError(err)
	}

	return result.DeletedCount, nil
}

// 1.5 Thao tác Atomic
// ------------------

// FindOneAndUpdate tìm và cập nhật một document
func (s *BaseServiceMongoImpl[T]) FindOneAndUpdate(ctx context.Context, filter interface{}, update interface{}, opts *options.FindOneAndUpdateOptions) (T, error) {
	var zero T

	if filter == nil {
		filter = bson.D{}
	}

	if opts == nil {
		opts = options.FindOneAndUpdate()
	}

	// ✅ Lấy document hiện tại để kiểm tra IsSystem (nếu có)
	var existing T
	err := s.collection.FindOne(ctx, filter).Decode(&existing)
	isExisting := err == nil
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		// Lỗi khác ngoài không tìm thấy
		return zero, common.ConvertMongoError(err)
	}

	// Chuyển update thành UpdateData để validate
	updateData, err := ToUpdateData(update)
	if err != nil {
		return zero, common.ErrInvalidFormat
	}

	if isExisting {
		// Document tồn tại, kiểm tra IsSystem
		if err := validateSystemDataUpdate(ctx, existing, updateData); err != nil {
			return zero, err
		}
	} else {
		// Document không tồn tại, sẽ tạo mới, validate insert
		// Kiểm tra IsSystem trong updateData
		if updateData.Set != nil {
			if isSystem, ok := updateData.Set["isSystem"].(bool); ok && isSystem {
				// Nếu context cho phép insert system data (quá trình init), bỏ qua validation
				if !isSystemDataInsertAllowed(ctx) {
					return zero, common.NewError(
						common.ErrCodeBusinessOperation,
						"Không thể tạo dữ liệu với IsSystem = true. Chỉ hệ thống mới có thể tạo dữ liệu system",
						common.StatusForbidden,
						nil,
					)
				}
			}
		}
	}

	// Thêm updatedAt vào updateData
	if updateData.Set == nil {
		updateData.Set = make(map[string]interface{})
	}
	updateData.Set["updatedAt"] = time.Now().UnixMilli()

	var result T
	err = s.collection.FindOneAndUpdate(ctx, filter, updateData, opts).Decode(&result)
	if err != nil {
		return zero, common.ConvertMongoError(err)
	}

	// ✅ Nếu là upsert (tạo mới), validate insert
	if !isExisting {
		// Document mới được tạo, validate insert
		if err := validateSystemDataInsert(ctx, result); err != nil {
			// Nếu validation fail, cần rollback (xóa document vừa tạo)
			if id, ok := getIDFromModel(result); ok {
				s.collection.DeleteOne(ctx, bson.M{"_id": id})
			}
			return zero, err
		}
	}

	op := events.OpUpdate
	if !isExisting {
		op = events.OpUpsert
	}
	events.EmitDataChanged(ctx, events.DataChangeEvent{
		CollectionName: s.collection.Name(),
		Operation:      op,
		Document:       result,
	})
	return result, nil
}

// getIDFromModel lấy ID từ model bằng reflection
func getIDFromModel(data interface{}) (primitive.ObjectID, bool) {
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return primitive.NilObjectID, false
	}

	// Thử field ID
	field := v.FieldByName("ID")
	if !field.IsValid() {
		return primitive.NilObjectID, false
	}

	if field.Kind() == reflect.Interface {
		if id, ok := field.Interface().(primitive.ObjectID); ok {
			return id, true
		}
	}

	return primitive.NilObjectID, false
}

// FindOneAndDelete tìm và xóa một document
func (s *BaseServiceMongoImpl[T]) FindOneAndDelete(ctx context.Context, filter interface{}, opts *options.FindOneAndDeleteOptions) (T, error) {
	var zero T

	if filter == nil {
		filter = bson.D{}
	}

	if opts == nil {
		opts = options.FindOneAndDelete()
	}

	// ✅ Lấy document cần xóa để kiểm tra IsSystem
	var existing T
	err := s.collection.FindOne(ctx, filter).Decode(&existing)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return zero, common.ErrNotFound
		}
		return zero, common.ConvertMongoError(err)
	}

	// ✅ Validate system data protection
	if err := validateSystemDataDelete(ctx, existing); err != nil {
		return zero, err
	}

	// ✅ Validate relationships từ struct tag
	if err := validateRelationshipsDelete(ctx, existing); err != nil {
		return zero, err
	}

	var result T
	err = s.collection.FindOneAndDelete(ctx, filter, opts).Decode(&result)
	if err != nil {
		return zero, common.ConvertMongoError(err)
	}

	return result, nil
}

// 1.6 Các thao tác khác
// --------------------

// CountDocuments đếm số lượng document
func (s *BaseServiceMongoImpl[T]) CountDocuments(ctx context.Context, filter interface{}) (int64, error) {
	if filter == nil {
		filter = bson.D{}
	}

	count, err := s.collection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, common.ConvertMongoError(err)
	}

	return count, nil
}

// Distinct lấy danh sách các giá trị duy nhất
func (s *BaseServiceMongoImpl[T]) Distinct(ctx context.Context, fieldName string, filter interface{}) ([]interface{}, error) {
	if filter == nil {
		filter = bson.D{}
	}

	values, err := s.collection.Distinct(ctx, fieldName, filter)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}

	return values, nil
}

// ====================================
// NHÓM 2: CÁC HÀM TIỆN ÍCH MỞ RỘNG
// ====================================

// 2.1 Các hàm Find mở rộng
// -----------------------

// FindOneById tìm một document theo ObjectId
func (s *BaseServiceMongoImpl[T]) FindOneById(ctx context.Context, id primitive.ObjectID) (T, error) {
	var zero T
	filter := bson.M{"_id": id}
	err := s.collection.FindOne(ctx, filter).Decode(&zero)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return zero, common.ErrNotFound
		}
		return zero, common.ConvertMongoError(err)
	}
	return zero, nil
}

// FindManyByIds tìm nhiều document theo danh sách ID
func (s *BaseServiceMongoImpl[T]) FindManyByIds(ctx context.Context, ids []primitive.ObjectID) ([]T, error) {
	filter := bson.M{"_id": bson.M{"$in": ids}}
	cursor, err := s.collection.Find(ctx, filter)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	var results []T
	if err = cursor.All(ctx, &results); err != nil {
		return nil, common.ConvertMongoError(err)
	}

	return results, nil
}

// FindWithPagination tìm tất cả bản ghi với phân trang
func (s *BaseServiceMongoImpl[T]) FindWithPagination(ctx context.Context, filter interface{}, page, limit int64, opts *options.FindOptions) (*basemodels.PaginateResult[T], error) {
	if filter == nil {
		filter = bson.D{}
	}

	// Tạo options mới nếu chưa có
	if opts == nil {
		opts = options.Find()
	}

	// Ghi đè skip và limit cho phân trang
	// Đảm bảo page >= 1 và limit > 0 để tránh skip âm
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	skip := (page - 1) * limit
	// Đảm bảo skip >= 0 (phòng trường hợp edge case)
	if skip < 0 {
		skip = 0
	}
	opts.SetSkip(skip)
	opts.SetLimit(limit)

	// Lấy tổng số bản ghi
	total, err := s.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}

	// Lấy dữ liệu theo trang
	var items []T
	cursor, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &items); err != nil {
		return nil, common.ConvertMongoError(err)
	}

	// Tính tổng số trang
	// Nếu total = 0, thì totalPage = 0
	// Nếu total > 0, tính bằng công thức làm tròn lên: (total + limit - 1) / limit
	var totalPage int64
	if total == 0 {
		totalPage = 0
	} else {
		totalPage = (total + limit - 1) / limit
	}

	return &basemodels.PaginateResult[T]{
		Items:     items,
		Page:      page,
		Limit:     limit,
		ItemCount: int64(len(items)),
		Total:     total,
		TotalPage: totalPage,
	}, nil
}

// 2.2 Các hàm Update/Delete mở rộng
// --------------------------------

// UpdateById cập nhật một document theo ObjectId
// Parameters:
//   - ctx: Context cho việc hủy bỏ hoặc timeout
//   - id: ObjectId của document cần cập nhật
//   - data: Dữ liệu cần cập nhật (có thể là T hoặc BsonWrapper)
//
// Returns:
//   - T: Document đã được cập nhật
//   - error: Lỗi nếu có
func (s *BaseServiceMongoImpl[T]) UpdateById(ctx context.Context, id primitive.ObjectID, data interface{}) (T, error) {
	var zero T
	filter := bson.M{"_id": id}

	// ✅ Lấy document hiện tại để kiểm tra IsSystem
	var existing T
	err := s.collection.FindOne(ctx, filter).Decode(&existing)
	if err != nil {
		return zero, common.ConvertMongoError(err)
	}

	// Chuyển data thành UpdateData
	updateData, err := ToUpdateData(data)
	if err != nil {
		return zero, common.ErrInvalidFormat
	}

	// ✅ Validate system data protection
	if err := validateSystemDataUpdate(ctx, existing, updateData); err != nil {
		return zero, err
	}

	// Thêm updatedAt vào $set
	if updateData.Set == nil {
		updateData.Set = make(map[string]interface{})
	}
	updateData.Set["updatedAt"] = time.Now().UnixMilli()

	// Tạo options cho update
	opts := options.Update().SetUpsert(false)

	// Thực hiện update
	result, err := s.collection.UpdateOne(ctx, filter, updateData, opts)
	if err != nil {
		return zero, common.ConvertMongoError(err)
	}

	if result.ModifiedCount == 0 {
		return zero, common.ErrNotFound
	}

	// Lấy lại document đã update
	var updated T
	err = s.collection.FindOne(ctx, filter).Decode(&updated)
	if err != nil {
		return zero, common.ConvertMongoError(err)
	}

	events.EmitDataChanged(ctx, events.DataChangeEvent{
		CollectionName: s.collection.Name(),
		Operation:      events.OpUpdate,
		Document:       updated,
	})
	return updated, nil
}

// DeleteById xóa một document theo ObjectId
// Parameters:
//   - ctx: Context cho việc hủy bỏ hoặc timeout
//   - id: ObjectId của document cần xóa
//
// Returns:
//   - error: Lỗi nếu có
func (s *BaseServiceMongoImpl[T]) DeleteById(ctx context.Context, id primitive.ObjectID) error {
	// ✅ Lấy document cần xóa để kiểm tra IsSystem
	var existing T
	err := s.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&existing)
	if err != nil {
		return common.ConvertMongoError(err)
	}

	// ✅ Validate system data protection
	if err := validateSystemDataDelete(ctx, existing); err != nil {
		return err
	}

	// ✅ Validate relationships từ struct tag
	if err := validateRelationshipsDelete(ctx, existing); err != nil {
		return err
	}

	filter := bson.M{"_id": id}
	result, err := s.collection.DeleteOne(ctx, filter)
	if err != nil {
		return common.ConvertMongoError(err)
	}

	if result.DeletedCount == 0 {
		return common.ErrNotFound
	}

	events.EmitDataChanged(ctx, events.DataChangeEvent{
		CollectionName: s.collection.Name(),
		Operation:      events.OpDelete,
		Document:       existing,
	})
	return nil
}

// 2.3 Các hàm Upsert tiện ích
// --------------------------

// Upsert thực hiện thao tác update nếu tồn tại, insert nếu chưa tồn tại
func (s *BaseServiceMongoImpl[T]) Upsert(ctx context.Context, filter interface{}, data interface{}) (T, error) {
	var zero T

	logrus.WithFields(logrus.Fields{
		"collection": s.collection.Name(),
		"filter":     filter,
	}).Debug("Upsert: Bắt đầu upsert")

	// ✅ Kiểm tra document hiện tại (nếu có) để validate update
	var existing T
	err := s.collection.FindOne(ctx, filter).Decode(&existing)
	isExisting := err == nil
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return zero, common.ConvertMongoError(err)
	}

	// Chuyển data thành UpdateData
	updateData, err := ToUpdateData(data)
	if err != nil {
		logrus.WithError(err).Error("Upsert: Lỗi chuyển đổi data thành UpdateData")
		return zero, common.ErrInvalidFormat
	}

	// ✅ Validate system data protection
	if isExisting {
		// Document tồn tại, validate update
		if err := validateSystemDataUpdate(ctx, existing, updateData); err != nil {
			return zero, err
		}
	} else {
		// Document không tồn tại, sẽ tạo mới, validate insert
		// Cần tạo model từ updateData để validate
		// Tạm thời validate qua updateData.Set
		if updateData.Set != nil {
			if isSystem, ok := updateData.Set["isSystem"].(bool); ok && isSystem {
				// Nếu context cho phép insert system data (quá trình init), bỏ qua validation
				if !isSystemDataInsertAllowed(ctx) {
					return zero, common.NewError(
						common.ErrCodeBusinessOperation,
						"Không thể tạo dữ liệu với IsSystem = true. Chỉ hệ thống mới có thể tạo dữ liệu system",
						common.StatusForbidden,
						nil,
					)
				}
			}
		}
	}

	// Thêm timestamps
	now := time.Now().UnixMilli()
	if updateData.Set == nil {
		updateData.Set = make(map[string]interface{})
	}
	updateData.Set["updatedAt"] = now
	updateData.Set["createdAt"] = now

	// Khi tạo mới: áp dụng default từ struct tag (chỉ set khi insert, qua $setOnInsert)
	if !isExisting {
		defaults := getInsertDefaultsFromModelType(reflect.TypeOf(zero))
		if len(defaults) > 0 {
			if updateData.SetOnInsert == nil {
				updateData.SetOnInsert = make(map[string]interface{})
			}
			for k, v := range defaults {
				if _, inSet := updateData.Set[k]; !inSet {
					updateData.SetOnInsert[k] = v
				}
			}
		}
	}

	// Xử lý các field empty string cho sparse unique index
	// Sparse index chỉ bỏ qua null/không tồn tại, không bỏ qua empty string
	// Nếu có nhiều document với empty string, sẽ bị duplicate key error
	// Với email và phone: nếu empty string hoặc không có trong $set, cần dùng $unset để xóa field hoàn toàn
	// (không chỉ xóa khỏi $set) để sparse index bỏ qua nó
	removedFromSet := []string{}
	unsetFields := []string{}
	if updateData.Unset == nil {
		updateData.Unset = make(map[string]interface{})
	}

	// Xử lý các field empty string trong $set
	for key, value := range updateData.Set {
		if strValue, ok := value.(string); ok && strValue == "" {
			// Chỉ xử lý email và phone empty string (các field có sparse unique index)
			if key == "email" || key == "phone" {
				// Xóa khỏi $set
				delete(updateData.Set, key)
				removedFromSet = append(removedFromSet, key)
				// Thêm vào $unset để đảm bảo field bị xóa hoàn toàn (không phải null)
				// Điều này quan trọng khi upsert tạo document mới
				updateData.Unset[key] = ""
				unsetFields = append(unsetFields, key)
			}
		}
	}

	// Quan trọng: Kiểm tra xem email và phone có trong $set không
	// Nếu không có, cần thêm vào $unset để đảm bảo khi upsert tạo document mới,
	// các field này sẽ không tồn tại (không phải null)
	// Điều này tránh duplicate key error với sparse unique index
	// Lưu ý: Khi upsert tạo document mới, MongoDB có thể tự động set field = null
	// nếu field không có trong $set. Do đó, cần đảm bảo $unset được áp dụng
	sparseFields := []string{"email", "phone"}
	for _, field := range sparseFields {
		if _, exists := updateData.Set[field]; !exists {
			// Field không có trong $set, thêm vào $unset để đảm bảo không tồn tại
			// Khi upsert tạo document mới, $unset sẽ xóa field này
			if _, alreadyUnset := updateData.Unset[field]; !alreadyUnset {
				updateData.Unset[field] = ""
				unsetFields = append(unsetFields, field)
			}
		}
	}

	// Nếu có lỗi duplicate key do document cũ có phone/email = null,
	// cần xử lý bằng cách tìm và xóa field đó từ document cũ
	// Nhưng điều này sẽ được xử lý trong error handling

	if len(removedFromSet) > 0 || len(unsetFields) > 0 {
		logrus.WithFields(logrus.Fields{
			"removed_from_set": removedFromSet,
			"unset_fields":     unsetFields,
		}).Debug("Upsert: Đã xử lý các field sparse unique index (email/phone) để tránh lỗi duplicate key")
	}

	logrus.WithFields(logrus.Fields{
		"update_data_keys": getMapKeys(updateData.Set),
		"unset_keys":       getMapKeys(updateData.Unset),
		"removed_from_set": removedFromSet,
		"unset_fields":     unsetFields,
	}).Debug("Upsert: Dữ liệu update sau khi xử lý")

	// Tạo options cho upsert với sort để đảm bảo chỉ update một document
	opts := options.FindOneAndUpdate().
		SetUpsert(true).
		SetReturnDocument(options.After).
		SetSort(bson.D{{Key: "_id", Value: 1}}) // Sắp xếp theo _id để đảm bảo tính nhất quán

	// Thực hiện upsert và lấy document sau khi update
	var upserted T
	err = s.collection.FindOneAndUpdate(ctx, filter, updateData, opts).Decode(&upserted)
	if err != nil {
		// Kiểm tra xem có phải lỗi duplicate key với phone/email = null không
		// Nếu có, cần xóa field đó từ document cũ và thử lại
		if mongoErr, ok := err.(mongo.WriteException); ok {
			for _, writeErr := range mongoErr.WriteErrors {
				if writeErr.Code == 11000 { // Duplicate key error
					// Kiểm tra xem có phải lỗi với phone hoặc email = null không
					errMsg := writeErr.Message
					if (strings.Contains(errMsg, "phone") || strings.Contains(errMsg, "email")) &&
						strings.Contains(errMsg, "null") {
						logrus.WithFields(logrus.Fields{
							"error": errMsg,
						}).Warn("Upsert: Phát hiện lỗi duplicate key với phone/email = null, thử xóa field từ document cũ")

						// Tìm document cũ có phone/email = null và xóa field đó
						// Sử dụng filter để tìm document có field = null
						var fieldToClean string
						if strings.Contains(errMsg, "phone") {
							fieldToClean = "phone"
						} else if strings.Contains(errMsg, "email") {
							fieldToClean = "email"
						}

						if fieldToClean != "" {
							// Tìm và xóa field từ document cũ
							cleanFilter := bson.M{fieldToClean: nil}
							cleanUpdate := bson.M{"$unset": bson.M{fieldToClean: ""}}
							_, cleanErr := s.collection.UpdateMany(ctx, cleanFilter, cleanUpdate)
							if cleanErr == nil {
								logrus.WithFields(logrus.Fields{
									"field": fieldToClean,
								}).Info("Upsert: Đã xóa field từ document cũ, thử lại upsert")

								// Thử lại upsert
								err = s.collection.FindOneAndUpdate(ctx, filter, updateData, opts).Decode(&upserted)
								if err == nil {
									logrus.Debug("Upsert: Upsert thành công sau khi xóa field từ document cũ")
									events.EmitDataChanged(ctx, events.DataChangeEvent{
										CollectionName: s.collection.Name(),
										Operation:      events.OpUpsert,
										Document:       upserted,
									})
									return upserted, nil
								}
							}
						}
					}
				}
			}
		}

		logrus.WithFields(logrus.Fields{
			"filter":      filter,
			"update_data": updateData.Set,
			"error":       err.Error(),
		}).Error("Upsert: Lỗi khi thực hiện FindOneAndUpdate")
		return zero, common.ConvertMongoError(err)
	}

	// Log thành công (không log ID vì generic type khó lấy)
	logrus.WithFields(logrus.Fields{
		"collection": s.collection.Name(),
	}).Debug("Upsert: Upsert thành công")

	events.EmitDataChanged(ctx, events.DataChangeEvent{
		CollectionName: s.collection.Name(),
		Operation:      events.OpUpsert,
		Document:       upserted,
	})
	return upserted, nil
}

// applyInsertDefaultsToModel áp dụng giá trị default từ struct tag lên model (chỉ set field đang zero).
// Dùng cho InsertOne/InsertMany để document tạo mới có đủ field có tag default.
// ptr phải là con trỏ tới struct (ví dụ &data).
func applyInsertDefaultsToModel(ptr interface{}) {
	if ptr == nil {
		return
	}
	v := reflect.ValueOf(ptr)
	if v.Kind() != reflect.Ptr {
		return
	}
	struc := v.Elem()
	if struc.Kind() != reflect.Struct {
		return
	}
	rt := struc.Type()
	defaults := getInsertDefaultsFromModelType(rt)
	if len(defaults) == 0 {
		return
	}
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		bsonTag := f.Tag.Get("bson")
		if bsonTag == "" || bsonTag == "-" {
			continue
		}
		bsonKey := strings.TrimSpace(strings.Split(bsonTag, ",")[0])
		if bsonKey == "" {
			continue
		}
		defaultVal, ok := defaults[bsonKey]
		if !ok {
			continue
		}
		fieldVal := struc.Field(i)
		if !fieldVal.CanSet() || !fieldVal.IsZero() {
			continue
		}
		rv := reflect.ValueOf(defaultVal)
		if rv.Type().AssignableTo(fieldVal.Type()) {
			fieldVal.Set(rv)
		} else if rv.Type().ConvertibleTo(fieldVal.Type()) {
			fieldVal.Set(rv.Convert(fieldVal.Type()))
		}
	}
}

// getInsertDefaultsFromModelType đọc struct tag default trên model và trả về map[bsonKey]giá trị mặc định.
// Dùng cho Insert (applyInsertDefaultsToModel) và Upsert ($setOnInsert).
// Hỗ trợ: bool (true/false), int, int64, string.
func getInsertDefaultsFromModelType(rt reflect.Type) map[string]interface{} {
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	if rt.Kind() != reflect.Struct {
		return nil
	}
	out := make(map[string]interface{})
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		defaultStr := f.Tag.Get("default")
		if defaultStr == "" {
			continue
		}
		bsonTag := f.Tag.Get("bson")
		if bsonTag == "" || bsonTag == "-" {
			continue
		}
		bsonKey := strings.TrimSpace(strings.Split(bsonTag, ",")[0])
		if bsonKey == "" {
			continue
		}
		val := parseDefaultValue(defaultStr, f.Type)
		if val != nil {
			out[bsonKey] = val
		}
	}
	return out
}

// parseDefaultValue chuyển chuỗi default tag sang giá trị đúng kiểu (bool, int, int64, string).
func parseDefaultValue(s string, t reflect.Type) interface{} {
	switch t.Kind() {
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return false
		}
		return b
	case reflect.Int, reflect.Int32:
		n, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return int32(0)
		}
		return int32(n)
	case reflect.Int64:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return int64(0)
		}
		return n
	case reflect.String:
		return s
	default:
		return nil
	}
}

// getMapKeys lấy danh sách keys từ map
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// UpsertMany thực hiện thao tác upsert cho nhiều document
func (s *BaseServiceMongoImpl[T]) UpsertMany(ctx context.Context, filter interface{}, data []T) ([]T, error) {
	if len(data) == 0 {
		return []T{}, nil
	}

	// ✅ Validate system data protection cho từng item (insert case)
	for _, item := range data {
		if err := validateSystemDataInsert(ctx, item); err != nil {
			return nil, err
		}
	}

	// ✅ Kiểm tra documents hiện tại (nếu có) để validate update
	// Với UpsertMany khó validate chính xác vì không biết document nào match với item nào
	// Tạm thời chỉ validate insert, và nếu có document system thì sẽ bị chặn ở UpdateMany
	// Note: UpsertMany thường dùng cho bulk import, ít khi dùng để update system data

	// Tạo các models cho bulk write
	var models []mongo.WriteModel
	now := time.Now().UnixMilli()

	for _, item := range data {
		// Chuyển data thành map (BSON marshal có thể đưa cả zero value nếu model không omitempty)
		dataMap, err := utility.ToMap(item)
		if err != nil {
			return nil, common.ErrInvalidFormat
		}

		// Chỉ đưa field non-zero vào $set (partial update, tránh ghi đè zero value không mong muốn)
		setMap := make(map[string]interface{})
		for k, v := range dataMap {
			if rv := reflect.ValueOf(v); rv.IsValid() && !rv.IsZero() {
				setMap[k] = v
			}
		}
		setMap["updatedAt"] = now

		// Tạo upsert model
		upsertModel := mongo.NewUpdateOneModel().
			SetFilter(filter).
			SetUpdate(bson.M{"$set": setMap}).
			SetUpsert(true)

		models = append(models, upsertModel)
	}

	// Thực hiện bulk write
	opts := options.BulkWrite().SetOrdered(false) // SetOrdered(false) để thực hiện song song
	result, err := s.collection.BulkWrite(ctx, models, opts)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}

	// Lấy lại các documents sau khi upsert
	var upserted []T
	if result.UpsertedCount > 0 {
		// Nếu có documents mới được tạo
		var upsertedIDs []primitive.ObjectID
		for _, id := range result.UpsertedIDs {
			if objectID, ok := id.(primitive.ObjectID); ok {
				upsertedIDs = append(upsertedIDs, objectID)
			}
		}

		if len(upsertedIDs) > 0 {
			cursor, err := s.collection.Find(ctx, bson.M{"_id": bson.M{"$in": upsertedIDs}})
			if err != nil {
				return nil, common.ConvertMongoError(err)
			}
			defer cursor.Close(ctx)

			if err = cursor.All(ctx, &upserted); err != nil {
				return nil, common.ConvertMongoError(err)
			}
		}
	}

	// Lấy các documents đã được update
	if result.ModifiedCount > 0 {
		cursor, err := s.collection.Find(ctx, filter)
		if err != nil {
			return nil, common.ConvertMongoError(err)
		}
		defer cursor.Close(ctx)

		var updated []T
		if err = cursor.All(ctx, &updated); err != nil {
			return nil, common.ConvertMongoError(err)
		}

		// Kết hợp cả documents mới và documents đã update
		upserted = append(upserted, updated...)
	}

	for i := range upserted {
		events.EmitDataChanged(ctx, events.DataChangeEvent{
			CollectionName: s.collection.Name(),
			Operation:      events.OpUpsert,
			Document:       upserted[i],
		})
	}
	return upserted, nil
}

// 2.4 Các hàm kiểm tra
// -------------------

// DocumentExists kiểm tra xem một document có tồn tại không
func (s *BaseServiceMongoImpl[T]) DocumentExists(ctx context.Context, filter interface{}) (bool, error) {
	if filter == nil {
		filter = bson.D{}
	}

	count, err := s.collection.CountDocuments(ctx, filter)
	if err != nil {
		return false, common.ConvertMongoError(err)
	}

	return count > 0, nil
}

// ====================================
// BẢO VỆ DỮ LIỆU HỆ THỐNG (IsSystem)
// ====================================

// Context key type để đánh dấu quá trình init cho phép tạo system data
// Lưu ý: Key này là private và chỉ được sử dụng nội bộ trong package basesvc
type systemDataContextKey string

const allowSystemDataInsertKey systemDataContextKey = "allow_system_data_insert"

// WithSystemDataInsertAllowed tạo context cho phép insert system data (dùng trong quá trình init).
// Được gọi từ package initsvc khi chạy init; không dùng từ API thông thường.
func WithSystemDataInsertAllowed(ctx context.Context) context.Context {
	return context.WithValue(ctx, allowSystemDataInsertKey, true)
}

// isSystemDataInsertAllowed kiểm tra xem context có cho phép insert system data không
func isSystemDataInsertAllowed(ctx context.Context) bool {
	allowed, ok := ctx.Value(allowSystemDataInsertKey).(bool)
	return ok && allowed
}

// getIsSystemValue lấy giá trị IsSystem từ model bằng reflection
func getIsSystemValue(data interface{}) (bool, bool) {
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return false, false
	}

	field := v.FieldByName("IsSystem")
	if !field.IsValid() || !field.CanInterface() {
		return false, false
	}

	if field.Kind() == reflect.Bool {
		return field.Bool(), true
	}

	return false, false
}

// setIsSystemValue set giá trị IsSystem cho model bằng reflection
func setIsSystemValue(data interface{}, value bool) {
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}

	field := v.FieldByName("IsSystem")
	if !field.IsValid() || !field.CanSet() {
		return
	}

	if field.Kind() == reflect.Bool {
		field.SetBool(value)
	}
}

// validateSystemDataInsert kiểm tra và bảo vệ khi insert dữ liệu system
func validateSystemDataInsert(ctx context.Context, data interface{}) error {
	isSystem, hasField := getIsSystemValue(data)
	if !hasField {
		return nil // Model không có field IsSystem, không cần validate
	}

	// Nếu context cho phép insert system data (quá trình init), bỏ qua validation
	if isSystemDataInsertAllowed(ctx) {
		return nil // Cho phép insert system data trong quá trình init
	}

	// Không cho phép user tạo dữ liệu với IsSystem = true
	if isSystem {
		return common.NewError(
			common.ErrCodeBusinessOperation,
			"Không thể tạo dữ liệu với IsSystem = true. Chỉ hệ thống mới có thể tạo dữ liệu system",
			common.StatusForbidden,
			nil,
		)
	}

	// Đảm bảo IsSystem = false khi tạo mới
	setIsSystemValue(data, false)
	return nil
}

// validateSystemDataDelete kiểm tra và bảo vệ khi xóa dữ liệu system
func validateSystemDataDelete(ctx context.Context, data interface{}) error {
	isSystem, hasField := getIsSystemValue(data)
	if !hasField {
		return nil // Model không có field IsSystem, không cần validate
	}

	// Không cho phép xóa dữ liệu system (kể cả admin)
	if isSystem {
		// Lấy tên model để hiển thị trong error message
		modelType := reflect.TypeOf(data)
		if modelType.Kind() == reflect.Ptr {
			modelType = modelType.Elem()
		}
		modelName := modelType.Name()

		return common.NewError(
			common.ErrCodeBusinessOperation,
			fmt.Sprintf("Không thể xóa %s vì đây là dữ liệu hệ thống mặc định", modelName),
			common.StatusForbidden,
			nil,
		)
	}

	return nil
}

// validateRelationshipsDelete kiểm tra các quan hệ được định nghĩa trong struct tag trước khi xóa
// Tự động đọc struct tag `relationship` và kiểm tra xem có record nào đang tham chiếu tới record này không
func validateRelationshipsDelete(ctx context.Context, data interface{}) error {
	// Lấy type của struct
	modelType := reflect.TypeOf(data)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	// Parse relationship tags từ struct
	relationships := ParseRelationshipTag(modelType)
	if len(relationships) == 0 {
		return nil // Không có relationship tag, không cần kiểm tra
	}

	// Lấy ID từ record
	recordID, ok := getIDFromModel(data)
	if !ok {
		// Nếu không có ID, không thể kiểm tra quan hệ
		// Có thể là record mới chưa có ID, bỏ qua
		return nil
	}

	// Chuyển đổi sang RelationshipCheck để sử dụng hàm có sẵn
	checks := make([]RelationshipCheck, 0, len(relationships))
	for _, rel := range relationships {
		// Bỏ qua nếu có cascade flag (sẽ xóa cascade)
		if rel.Cascade {
			continue
		}

		checks = append(checks, RelationshipCheck{
			CollectionName: rel.CollectionName,
			FieldName:      rel.FieldName,
			ErrorMessage:   rel.ErrorMessage,
			Optional:       rel.Optional,
		})
	}

	if len(checks) > 0 {
		return CheckRelationshipExists(ctx, recordID, checks)
	}

	return nil
}

// validateSystemDataUpdate kiểm tra và bảo vệ khi update dữ liệu system
// Cho phép admin sửa một số field nhất định (IsActive, config fields)
// Không cho phép sửa các field quan trọng (IsSystem, Name, EventType, ChannelType, etc.)
func validateSystemDataUpdate(ctx context.Context, existingData interface{}, update *UpdateData) error {
	isSystem, hasField := getIsSystemValue(existingData)
	if !hasField {
		// Model không có field IsSystem, nhưng vẫn cần check nếu user cố set IsSystem = true
		if update.Set != nil {
			if isSystemVal, ok := update.Set["isSystem"].(bool); ok && isSystemVal {
				return common.NewError(
					common.ErrCodeBusinessOperation,
					"Không thể set IsSystem = true. Chỉ hệ thống mới có thể tạo dữ liệu system",
					common.StatusForbidden,
					nil,
				)
			}
			delete(update.Set, "isSystem")
		}
		return nil
	}

	// Kiểm tra user có phải admin không (callback đăng ký từ auth domain)
	var isAdmin bool
	if isAdminFromContextFunc != nil {
		isAdmin, _ = isAdminFromContextFunc(ctx)
	}

	if isSystem {
		// Dữ liệu system: chỉ admin mới được sửa
		if !isAdmin {
			return common.NewError(
				common.ErrCodeBusinessOperation,
				"Chỉ Administrator mới có thể sửa dữ liệu hệ thống",
				common.StatusForbidden,
				nil,
			)
		}

		// ✅ Admin có quyền sửa TẤT CẢ các field của dữ liệu hệ thống, kể cả protected fields và isSystem
		// Không cần chặn bất kỳ field nào vì admin là người quản trị hệ thống
		// Logic cũ đã được bỏ để cho phép admin linh hoạt quản lý dữ liệu hệ thống
	} else {
		// Dữ liệu không phải system: không cho phép set IsSystem = true
		if update.Set != nil {
			if isSystemVal, ok := update.Set["isSystem"].(bool); ok && isSystemVal {
				return common.NewError(
					common.ErrCodeBusinessOperation,
					"Không thể set IsSystem = true. Chỉ hệ thống mới có thể tạo dữ liệu system",
					common.StatusForbidden,
					nil,
				)
			}
			delete(update.Set, "isSystem")
		}
	}

	return nil
}

// getProtectedFieldsForModel trả về danh sách các field không được sửa của dữ liệu system
// Tùy theo từng model, có các field quan trọng khác nhau
// LƯU Ý: Hàm này hiện tại không được sử dụng vì admin có quyền sửa tất cả các field của dữ liệu hệ thống
// Giữ lại để có thể sử dụng trong tương lai nếu cần áp dụng logic bảo vệ field cho non-admin users
func getProtectedFieldsForModel(modelName string) []string {
	// Các field chung không được sửa cho tất cả model có IsSystem
	commonProtected := []string{"name"}

	// Các field đặc thù theo model
	switch modelName {
	case "NotificationChannelSender":
		return append(commonProtected, "channelType")
	case "NotificationTemplate":
		return append(commonProtected, "eventType", "channelType")
	case "NotificationChannel":
		return append(commonProtected, "channelType")
	case "NotificationRoutingRule":
		return append(commonProtected, "eventType")
	default:
		return commonProtected
	}
}
