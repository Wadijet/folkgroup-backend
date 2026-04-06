// Package adssvc — Service load ads_metric_definitions từ DB.
// Fallback sang hardcode khi collection rỗng. Dùng cho evaluation engine và API metadata.
package adssvc

import (
	"context"

	adsmodels "meta_commerce/internal/api/ads_meta/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

// GetMetricDefinitions lấy tất cả metric definitions đang active từ DB.
// Trả về rỗng nếu collection chưa có hoặc chưa seed.
func GetMetricDefinitions(ctx context.Context) ([]adsmodels.AdsMetricDefinition, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsMetricDefinitions)
	if !ok {
		return nil, nil
	}
	filter := bson.M{"isActive": true}
	opts := mongoopts.Find().SetSort(bson.M{"order": 1})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var out []adsmodels.AdsMetricDefinition
	for cursor.Next(ctx) {
		var doc adsmodels.AdsMetricDefinition
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		out = append(out, doc)
	}
	return out, nil
}

// GetMetricDefinitionByKey lấy definition theo key.
func GetMetricDefinitionByKey(ctx context.Context, key string) (*adsmodels.AdsMetricDefinition, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsMetricDefinitions)
	if !ok {
		return nil, nil
	}
	filter := bson.M{"key": key, "isActive": true}
	var doc adsmodels.AdsMetricDefinition
	err := coll.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
}

// GetMetricDefinitionsByWindow lấy definitions theo window (7d, 2h, 1h, 30p).
func GetMetricDefinitionsByWindow(ctx context.Context, window string) ([]adsmodels.AdsMetricDefinition, error) {
	all, err := GetMetricDefinitions(ctx)
	if err != nil {
		return nil, err
	}
	var out []adsmodels.AdsMetricDefinition
	for _, d := range all {
		if d.Window == window {
			out = append(out, d)
		}
	}
	return out, nil
}

// GetWindowMsFromDefinition trả về windowMs cho window 7d (currentMetrics).
// Dùng khi DB không có definitions — fallback.
func GetWindowMsFromDefinition(ctx context.Context, window string) int64 {
	defs, err := GetMetricDefinitionsByWindow(ctx, window)
	if err != nil || len(defs) == 0 {
		return windowMsFallback(window)
	}
	return defs[0].WindowMs
}

// windowMsFallback fallback khi không tìm thấy definition.
func windowMsFallback(window string) int64 {
	switch window {
	case adsmodels.Window7d:
		return 7 * 24 * 60 * 60 * 1000
	case adsmodels.Window2h:
		return 2 * 60 * 60 * 1000
	case adsmodels.Window1h:
		return 60 * 60 * 1000
	case adsmodels.Window30p:
		return 30 * 60 * 1000
	default:
		return 7 * 24 * 60 * 60 * 1000
	}
}
