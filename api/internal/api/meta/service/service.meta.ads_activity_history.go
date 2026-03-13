// Package metasvc - Service quản lý lịch sử hoạt động Meta Ads (ads_activity_history).
package metasvc

import (
	"context"
	"fmt"
	"time"

	basesvc "meta_commerce/internal/api/base/service"
	metamodels "meta_commerce/internal/api/meta/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GetChsFromYesterday lấy CHS từ ads_activity_history — bản ghi mới nhất của campaign trong ngày hôm qua (FolkForm v4.1 CHS Kill exception).
// Camp HEALTHY hôm qua (CHS >= 60) mà hôm nay CHS critical đột ngột → có thể data anomaly → chờ 1 checkpoint.
// Trả về (chs, ok). ok=false khi không có dữ liệu. Dùng bởi metasvc computeFinalActions (tránh import cycle với adssvc).
func GetChsFromYesterday(ctx context.Context, campaignId string, ownerOrgID primitive.ObjectID) (float64, bool) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsActivityHistory)
	if !ok {
		return 0, false
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	yesterdayStart := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, loc).UnixMilli()
	yesterdayEnd := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).UnixMilli()
	opts := mongoopts.FindOne().SetSort(bson.D{{Key: "activityAt", Value: -1}}).SetProjection(bson.M{"snapshot.metrics": 1})
	var doc struct {
		Snapshot struct {
			Metrics map[string]interface{} `bson:"metrics"`
		} `bson:"snapshot"`
	}
	err := coll.FindOne(ctx, bson.M{
		"objectType":          "campaign",
		"objectId":            campaignId,
		"ownerOrganizationId": ownerOrgID,
		"activityAt":          bson.M{"$gte": yesterdayStart, "$lt": yesterdayEnd},
	}, opts).Decode(&doc)
	if err != nil {
		return 0, false
	}
	layer3, _ := doc.Snapshot.Metrics["layer3"].(map[string]interface{})
	if layer3 == nil {
		return 0, false
	}
	v := layer3["chs"]
	if v == nil {
		return 0, false
	}
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	}
	return 0, false
}

// MetaAdsActivityHistoryService service quản lý lịch sử thay đổi metrics (Campaign/AdSet/Ad).
// Dữ liệu được ghi tự động bởi hệ thống khi currentMetrics thay đổi; API chỉ hỗ trợ đọc.
type MetaAdsActivityHistoryService struct {
	*basesvc.BaseServiceMongoImpl[metamodels.AdsActivityHistory]
}

// NewMetaAdsActivityHistoryService tạo MetaAdsActivityHistoryService.
func NewMetaAdsActivityHistoryService() (*MetaAdsActivityHistoryService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsActivityHistory)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s", global.MongoDB_ColNames.AdsActivityHistory)
	}
	return &MetaAdsActivityHistoryService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[metamodels.AdsActivityHistory](coll),
	}, nil
}
