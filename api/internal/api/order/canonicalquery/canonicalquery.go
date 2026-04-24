// Package canonicalquery — truy vấn aggregate trên order_canonical (L2), thay cho đọc trực tiếp pc_pos_orders (L1).
package canonicalquery

import (
	"context"
	"fmt"

	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// CollOrderCanonical trả về collection order_canonical (order_core_records) hoặc lỗi.
func CollOrderCanonical() (*mongo.Collection, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.OrderCanonical)
	if !ok || coll == nil {
		return nil, fmt.Errorf("không tìm thấy collection %s", global.MongoDB_ColNames.OrderCanonical)
	}
	return coll, nil
}

// MatchInsertedAtTimeWindowOr tương đương filter thời gian trên L1 (posCreatedAt|insertedAt × sec|ms),
// áp dụng cho L2 chỉ có insertedAt (và posData đầy đủ).
func MatchInsertedAtTimeWindowOr(startMs, endMs int64) bson.M {
	startSec := startMs / 1000
	endSec := endMs / 1000
	return bson.M{
		"$or": []bson.M{
			{"insertedAt": bson.M{"$gte": startSec, "$lte": endSec}},
			{"insertedAt": bson.M{"$gte": startMs, "$lte": endMs}},
		},
	}
}

// MatchInsertedAtExclusiveSlotOr filter slot [start, end) — insertedAt lưu sec hoặc ms (tương đương posCreatedAt|insertedAt trên L1).
func MatchInsertedAtExclusiveSlotOr(startSec, endSec int64) bson.M {
	startMs := startSec * 1000
	endMs := endSec * 1000
	return bson.M{
		"$or": []bson.M{
			{"insertedAt": bson.M{"$gte": startSec, "$lt": endSec}},
			{"insertedAt": bson.M{"$gte": startMs, "$lt": endMs}},
		},
	}
}

// AggregateOrdersRevenueByAdID đếm đơn và cộng doanh thu theo posData.ad_id trong cửa sổ thời gian.
// Tương đương pipeline cũ trên pc_pos_orders (fetchRawPosFromOrders).
func AggregateOrdersRevenueByAdID(ctx context.Context, adID string, ownerOrgID primitive.ObjectID, startMs, endMs int64) (orders int64, revenue float64, err error) {
	coll, err := CollOrderCanonical()
	if err != nil {
		return 0, 0, err
	}
	tm := MatchInsertedAtTimeWindowOr(startMs, endMs)
	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"posData.ad_id":       adID,
		"$or":                 tm["$or"],
	}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$group", Value: bson.M{
			"_id":    nil,
			"orders": bson.M{"$sum": 1},
			"revenue": bson.M{"$sum": bson.M{"$convert": bson.M{
				"input": bson.M{"$ifNull": bson.A{"$posData.total_price_after_sub_discount", 0}},
				"to": "double", "onError": 0, "onNull": 0,
			}}},
		}}},
	}
	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, 0, err
	}
	defer cursor.Close(ctx)
	if !cursor.Next(ctx) {
		return 0, 0, nil
	}
	var doc struct {
		Orders  int64   `bson:"orders"`
		Revenue float64 `bson:"revenue"`
	}
	if err := cursor.Decode(&doc); err != nil {
		return 0, 0, err
	}
	return doc.Orders, doc.Revenue, nil
}
