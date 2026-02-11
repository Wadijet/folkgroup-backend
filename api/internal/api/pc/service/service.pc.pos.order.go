package pcsvc

import (
	"context"
	"fmt"
	"time"

	pcmodels "meta_commerce/internal/api/pc/models"
	basesvc "meta_commerce/internal/api/base/service"
	reportsvc "meta_commerce/internal/api/report/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// PcPosOrderService là cấu trúc chứa các phương thức liên quan đến Pancake POS Order
type PcPosOrderService struct {
	*basesvc.BaseServiceMongoImpl[pcmodels.PcPosOrder]
	reportService *reportsvc.ReportService
}

// NewPcPosOrderService tạo mới PcPosOrderService.
func NewPcPosOrderService() (*PcPosOrderService, error) {
	orderCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !exist {
		return nil, fmt.Errorf("failed to get pc_pos_orders collection: %v", common.ErrNotFound)
	}
	reportSvc, _ := reportsvc.NewReportService()

	return &PcPosOrderService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[pcmodels.PcPosOrder](orderCollection),
		reportService:       reportSvc,
	}, nil
}

// InsertOne thêm mới một đơn hàng, sau đó đánh dấu dirty các báo cáo theo chu kỳ.
func (s *PcPosOrderService) InsertOne(ctx context.Context, data pcmodels.PcPosOrder) (pcmodels.PcPosOrder, error) {
	out, err := s.BaseServiceMongoImpl.InsertOne(ctx, data)
	if err != nil {
		return out, err
	}
	s.reportHookAfterSave(ctx, &out)
	return out, nil
}

// UpdateOne cập nhật một đơn hàng, sau đó đánh dấu dirty các báo cáo theo chu kỳ.
func (s *PcPosOrderService) UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts *options.UpdateOptions) (pcmodels.PcPosOrder, error) {
	out, err := s.BaseServiceMongoImpl.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return out, err
	}
	s.reportHookAfterSave(ctx, &out)
	return out, nil
}

// SyncFlattenedFromPosData đọc document theo id, chạy extract từ posData vào các field flatten (billFullName, status, posCreatedAt, ...) rồi ghi lại document.
// Dùng để sửa document cũ thiếu field flatten (ví dụ do webhook cũ hoặc extract lỗi trước khi có unwrap Extended JSON).
func (s *PcPosOrderService) SyncFlattenedFromPosData(ctx context.Context, id primitive.ObjectID) (pcmodels.PcPosOrder, error) {
	var zero pcmodels.PcPosOrder
	order, err := s.BaseServiceMongoImpl.FindOneById(ctx, id)
	if err != nil {
		return zero, err
	}
	if len(order.PosData) == 0 {
		return zero, fmt.Errorf("document không có posData để sync")
	}
	if err := utility.ExtractDataIfExists(&order); err != nil {
		return zero, fmt.Errorf("extract từ posData thất bại: %w", err)
	}
	order.UpdatedAt = time.Now().UnixMilli()
	dataMap, err := utility.ToMap(&order)
	if err != nil {
		return zero, fmt.Errorf("ToMap thất bại: %w", err)
	}
	_, err = s.Collection().ReplaceOne(ctx, bson.M{"_id": id}, dataMap)
	if err != nil {
		return zero, common.ConvertMongoError(err)
	}
	s.reportHookAfterSave(ctx, &order)
	return order, nil
}

func (s *PcPosOrderService) reportHookAfterSave(ctx context.Context, order *pcmodels.PcPosOrder) {
	if s.reportService == nil {
		return
	}
	keys, err := s.reportService.GetReportKeysByCollection(ctx, global.MongoDB_ColNames.PcPosOrders)
	if err != nil || len(keys) == 0 {
		return
	}
	ts := order.InsertedAt
	if ts == 0 {
		ts = order.CreatedAt
	}
	if ts > 1e12 {
		ts = ts / 1000
	}
	periodKey := periodKeyFromUnixSeconds(ts)
	for _, reportKey := range keys {
		_ = s.reportService.MarkDirty(ctx, reportKey, periodKey, order.OwnerOrganizationID)
	}
}

func periodKeyFromUnixSeconds(unixSec int64) string {
	loc, _ := time.LoadLocation(reportsvc.ReportTimezone)
	t := time.Unix(unixSec, 0).In(loc)
	return t.Format("2006-01-02")
}
