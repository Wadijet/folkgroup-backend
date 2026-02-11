package utility

import (
	"fmt"
	"reflect"

	"go.mongodb.org/mongo-driver/bson"
)

// ****************************************************  Bson *******************************************
// Các thao tác Bson tùy chỉnh

// CustomBson dùng để thực hiện các thao tác bson tùy chỉnh
// như set, push, unset, v.v. bằng cách sử dụng các struct
// Điều này rất hữu ích khi cần tạo bản đồ bson từ struct
type CustomBson struct{}

// BsonWrapper chứa các thao tác bson cơ bản
// như $set, $push, $addToSet
// Nó rất hữu ích để chuyển đổi struct thành bson
type BsonWrapper struct {

	// Set sẽ đặt dữ liệu trong db
	// ví dụ - nếu cần đặt "name":"Jack", thì cần tạo một struct chứa trường name và gán struct đó vào trường này.
	// Sau khi mã hóa thành bson, nó sẽ như { $set : {name : "Jack"}} và điều này sẽ hữu ích trong truy vấn mongo
	Set interface{} `json:"$set,omitempty" bson:"$set,omitempty"`

	// Toán tử Unset xóa một trường cụ thể.
	// Nếu trường không tồn tại, thì Unset không làm gì cả
	// Nếu cần unset trường name thì chỉ cần tạo một struct chứa trường name và gán "" cho name.
	// Bây giờ để unset, gán struct đó vào trường Unset. Sau khi mã hóa, nó sẽ trở thành { $unset: { name: "" } }
	Unset interface{} `json:"$unset,omitempty" bson:"$unset,omitempty"`

	// Toán tử Push thêm một giá trị cụ thể vào một mảng.
	// Nếu trường không có trong tài liệu để cập nhật,
	// Push thêm trường mảng với giá trị là phần tử của nó.
	// Nếu trường không phải là một mảng, thao tác sẽ thất bại.
	Push interface{} `json:"$push,omitempty" bson:"$push,omitempty"`

	// Toán tử AddToSet thêm một giá trị vào một mảng trừ khi giá trị đã có, trong trường hợp đó AddToSet không làm gì với mảng đó.
	// Nếu sử dụng AddToSet trên một trường không có trong tài liệu để cập nhật,
	// AddToSet tạo trường mảng với giá trị cụ thể là phần tử của nó.
	AddToSet interface{} `json:"$addToSet,omitempty" bson:"$addToSet,omitempty"`
}

// ToMap chuyển đổi interface thành bản đồ.
// Nó nhận interface làm tham số và trả về bản đồ và lỗi nếu có
// Hook: Tự động extract data từ source fields (PanCakeData, FacebookData, ...) vào typed fields
// dựa trên tag extract trước khi convert sang map
func ToMap(s interface{}) (map[string]interface{}, error) {
	// Xử lý extract: Nếu là value, convert thành pointer để extract
	val := reflect.ValueOf(s)
	var toMarshal interface{} = s
	var ptrVal reflect.Value

	if val.Kind() != reflect.Ptr && val.Kind() == reflect.Struct {
		// Nếu là value struct, tạo pointer mới để extract
		ptrVal = reflect.New(val.Type())
		ptrVal.Elem().Set(val)
		toMarshal = ptrVal.Interface()
	}

	// Extract data từ source fields vào typed fields
	// Hook tại đây để tự động extract cho tất cả CRUD operations
	if err := ExtractDataIfExists(toMarshal); err != nil {
		return nil, fmt.Errorf("extract data failed: %w", err)
	}

	// Nếu đã tạo pointer tạm thời, marshal struct bên trong (không phải pointer wrapper)
	if ptrVal.IsValid() {
		toMarshal = ptrVal.Elem().Interface()
	}

	// Convert sang map như bình thường
	var stringInterfaceMap map[string]interface{}
	itr, err := bson.Marshal(toMarshal)
	if err != nil {
		return nil, fmt.Errorf("bson marshal failed: %w", err)
	}
	err = bson.Unmarshal(itr, &stringInterfaceMap)
	if err != nil {
		return nil, fmt.Errorf("bson unmarshal failed: %w", err)
	}
	return stringInterfaceMap, err
}

// Set tạo truy vấn để thay thế giá trị của một trường bằng giá trị cụ thể
// @params - dữ liệu cần đặt
// @returns - bản đồ truy vấn và lỗi nếu có
func (customBson *CustomBson) Set(data interface{}) (map[string]interface{}, error) {
	s := BsonWrapper{Set: data}
	return ToMap(s)
}

// Push tạo truy vấn để thêm một giá trị cụ thể vào một trường mảng
// @params - dữ liệu cần thêm
// @returns - bản đồ truy vấn và lỗi nếu có
func (customBson *CustomBson) Push(data interface{}) (map[string]interface{}, error) {
	s := BsonWrapper{Push: data}
	return ToMap(s)
}

// Unset tạo truy vấn để xóa một trường cụ thể
// @params - dữ liệu cần unset
// @returns - bản đồ truy vấn và lỗi nếu có
func (customBson *CustomBson) Unset(data interface{}) (map[string]interface{}, error) {
	s := BsonWrapper{Unset: data}
	return ToMap(s)
}

// AddToSet tạo truy vấn để thêm một giá trị vào một mảng trừ khi giá trị đã có.
// @params - dữ liệu cần thêm vào set
// @returns - bản đồ truy vấn và lỗi nếu có
func (customBson *CustomBson) AddToSet(data interface{}) (map[string]interface{}, error) {
	s := BsonWrapper{AddToSet: data}
	return ToMap(s)
}

// ****************************************************  Bson End  *******************************************
