package pcsvc

import (
	"context"
	"fmt"

	pcmodels "meta_commerce/internal/api/pc/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// PcOrderService là cấu trúc chứa các phương thức liên quan đến đơn hàng
type PcOrderService struct {
	*basesvc.BaseServiceMongoImpl[pcmodels.PcOrder]
}

// NewPcOrderService tạo mới PcOrderService
func NewPcOrderService() (*PcOrderService, error) {
	orderCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.PcOrders)
	if !exist {
		return nil, fmt.Errorf("failed to get pc_orders collection: %v", common.ErrNotFound)
	}

	return &PcOrderService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[pcmodels.PcOrder](orderCollection),
	}, nil
}

// IsPancakeOrderIdExist kiểm tra ID đơn hàng Pancake có tồn tại hay không
func (s *PcOrderService) IsPancakeOrderIdExist(ctx context.Context, pancakeOrderId string) (bool, error) {
	filter := bson.M{"pancakeOrderId": pancakeOrderId}
	var pcOrder pcmodels.PcOrder
	err := s.Collection().FindOne(ctx, filter).Decode(&pcOrder)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Delete xóa một document theo ObjectId
func (s *PcOrderService) Delete(ctx context.Context, id primitive.ObjectID) error {
	return s.BaseServiceMongoImpl.DeleteById(ctx, id)
}

// Update cập nhật một document theo ObjectId với chỉ các field có trong updateData.Set.
func (s *PcOrderService) Update(ctx context.Context, id primitive.ObjectID, updateData *basesvc.UpdateData) (pcmodels.PcOrder, error) {
	if updateData == nil || (updateData.Set != nil && len(updateData.Set) == 0) {
		return s.BaseServiceMongoImpl.FindOneById(ctx, id)
	}
	return s.BaseServiceMongoImpl.UpdateById(ctx, id, updateData)
}
