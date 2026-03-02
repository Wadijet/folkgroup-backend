package basesvc

import (
	"go.mongodb.org/mongo-driver/bson"
)

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
