// Package crmvc - List customers cần tính lại phân loại cho ClassificationRefreshWorker.
package crmvc

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ClassificationRefreshMode chế độ lấy danh sách khách cần refresh.
// full: Tất cả khách có ít nhất 1 đơn (orderCount >= 1).
// smart: Chỉ khách có lastOrderAt nằm gần ngưỡng lifecycle (28–33, 88–96, 178–186 ngày).
const (
	ClassificationRefreshModeFull  = "full"
	ClassificationRefreshModeSmart = "smart"
)

// msPerDay milliseconds trong 1 ngày (đồng bộ với classification).
const msPerDayRefresh = 24 * 60 * 60 * 1000

// CustomerIdForRefresh thông tin tối thiểu để gọi RefreshMetrics.
type CustomerIdForRefresh struct {
	UnifiedId      string
	OwnerOrgID     primitive.ObjectID
}

// ListCustomerIdsForClassificationRefresh trả về danh sách (unifiedId, ownerOrgID) cần gọi RefreshMetrics.
//
// Tham số:
//   - ctx: Context
//   - mode: "full" (tất cả khách có đơn) hoặc "smart" (chỉ khách gần ngưỡng lifecycle)
//   - batchSize: Số lượng tối đa mỗi lần (0 = dùng mặc định 500)
//   - skip: Số bản ghi bỏ qua (cho pagination)
//
// Trả về:
//   - []CustomerIdForRefresh: Danh sách cần refresh
//   - error: Lỗi nếu có
//
// Smart mode lọc lastOrderAt trong vùng:
//   - 28–33 ngày: active ↔ cooling
//   - 88–96 ngày: cooling ↔ inactive, journey → inactive
//   - 178–186 ngày: inactive ↔ dead
func (s *CrmCustomerService) ListCustomerIdsForClassificationRefresh(ctx context.Context, mode string, batchSize, skip int) ([]CustomerIdForRefresh, error) {
	if batchSize <= 0 {
		batchSize = 500
	}
	if skip < 0 {
		skip = 0
	}

	filter := s.buildClassificationRefreshFilter(mode)
	opts := options.Find().
		SetProjection(bson.M{"unifiedId": 1, "ownerOrganizationId": 1}).
		SetSkip(int64(skip)).
		SetLimit(int64(batchSize)).
		SetSort(bson.D{{Key: "_id", Value: 1}})

	cursor, err := s.Collection().Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var result []CustomerIdForRefresh
	for cursor.Next(ctx) {
		var doc struct {
			UnifiedId          string              `bson:"unifiedId"`
			OwnerOrganizationId primitive.ObjectID `bson:"ownerOrganizationId"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		result = append(result, CustomerIdForRefresh{
			UnifiedId:  doc.UnifiedId,
			OwnerOrgID: doc.OwnerOrganizationId,
		})
	}
	return result, nil
}

// buildClassificationRefreshFilter tạo filter MongoDB theo mode.
func (s *CrmCustomerService) buildClassificationRefreshFilter(mode string) bson.M {
	// Chỉ xét khách có ít nhất 1 đơn — lifecycle/journey có nghĩa.
	filter := bson.M{"orderCount": bson.M{"$gte": 1}}

	if mode == ClassificationRefreshModeSmart {
		now := time.Now().UnixMilli()
		// Vùng ngưỡng (ngày): 28–33, 88–96, 178–186
		zone1From := now - 33*msPerDayRefresh // 33 ngày trước
		zone1To := now - 28*msPerDayRefresh   // 28 ngày trước
		zone2From := now - 96*msPerDayRefresh
		zone2To := now - 88*msPerDayRefresh
		zone3From := now - 186*msPerDayRefresh
		zone3To := now - 178*msPerDayRefresh

		filter["lastOrderAt"] = bson.M{"$gte": int64(1)}
		filter["$or"] = []bson.M{
			{"lastOrderAt": bson.M{"$gte": zone1From, "$lte": zone1To}},
			{"lastOrderAt": bson.M{"$gte": zone2From, "$lte": zone2To}},
			{"lastOrderAt": bson.M{"$gte": zone3From, "$lte": zone3To}},
		}
	}

	return filter
}
