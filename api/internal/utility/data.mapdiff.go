package utility

import (
	"reflect"
)

// MyMapDiff so sánh hai bản đồ (map) và trả về sự khác biệt giữa chúng.
// newMap: bản đồ mới
// oldMap: bản đồ cũ
// resultDiff: danh sách các sự khác biệt
func MyMapDiff(newMap, oldMap map[string]interface{}) (resultDiff []interface{}) {

	var result []interface{}

	// Duyệt qua từng phần tử trong newMap
	for newKey, newVal := range newMap {
		// Duyệt qua từng phần tử trong oldMap
		for oldKey, oldVal := range oldMap {
			// Nếu khóa (key) giống nhau
			if newKey == oldKey {
				// Nếu giá trị (value) khác nhau
				if !reflect.DeepEqual(newVal, oldVal) {

					test := reflect.TypeOf(newVal)
					// fmt.Println(test.String())

					// Nếu giá trị là một bản đồ khác
					if test.String() == "map[string]interface {}" {
						old, err := ToMap(oldVal)
						if err != nil {
							continue
						}
						new, err := ToMap(newVal)
						if err != nil {
							continue
						}
						// Đệ quy để tìm sự khác biệt trong bản đồ con
						diff := map[string]interface{}{newKey: MyMapDiff(new, old)}
						result = append(result, diff)

					} else {
						// Nếu giá trị không phải là bản đồ
						var changes []interface{}
						changes = append(changes, oldVal)
						changes = append(changes, newVal)
						diff := map[string]interface{}{newKey: changes}
						result = append(result, diff)
					}
				}
			}
		}
	}

	// Trả về kết quả nếu có sự khác biệt, ngược lại trả về nil
	if len(result) > 0 {
		return result
	} else {
		return nil
	}
}
