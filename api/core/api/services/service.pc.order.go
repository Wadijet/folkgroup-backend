package services

import (
	"context"
	"fmt"

	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// PcOrderService là cấu trúc chứa các phương thức liên quan đến đơn hàng
type PcOrderService struct {
	*BaseServiceMongoImpl[models.PcOrder]
}

// NewPcOrderService tạo mới PcOrderService
func NewPcOrderService() (*PcOrderService, error) {
	orderCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.PcOrders)
	if !exist {
		return nil, fmt.Errorf("failed to get pc_orders collection: %v", common.ErrNotFound)
	}

	return &PcOrderService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.PcOrder](orderCollection),
	}, nil
}

// IsPancakeOrderIdExist kiểm tra ID đơn hàng Pancake có tồn tại hay không
func (s *PcOrderService) IsPancakeOrderIdExist(ctx context.Context, pancakeOrderId string) (bool, error) {
	filter := bson.M{"pancakeOrderId": pancakeOrderId}
	var pcOrder models.PcOrder
	err := s.BaseServiceMongoImpl.collection.FindOne(ctx, filter).Decode(&pcOrder)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Delete xóa một document theo ObjectId
//
// LÝ DO PHẢI TẠO METHOD NÀY (không dùng BaseServiceMongoImpl.DeleteById trực tiếp):
// 1. Backward compatibility:
//    - Method này có thể được gọi từ các nơi khác trong codebase
//    - Giữ nguyên interface để không phá vỡ existing code
//
// LƯU Ý:
// - Method này chỉ là wrapper đơn giản, không có business logic đặc biệt
// - Đã refactor để dùng BaseServiceMongoImpl.DeleteById() để nhất quán với base service
//
// ĐẢM BẢO LOGIC CƠ BẢN:
// ✅ Xóa document theo ID bằng BaseServiceMongoImpl.DeleteById()
// ✅ Trả về error nếu có
func (s *PcOrderService) Delete(ctx context.Context, id primitive.ObjectID) error {
	return s.BaseServiceMongoImpl.DeleteById(ctx, id)
}

// Update cập nhật một document theo ObjectId
//
// LÝ DO PHẢI TẠO METHOD NÀY (không dùng BaseServiceMongoImpl.UpdateById trực tiếp):
// 1. Backward compatibility:
//    - Method này có thể được gọi từ các nơi khác trong codebase
//    - Giữ nguyên interface để không phá vỡ existing code
// 2. Return updated document:
//    - Method này trả về document đã được update
//    - BaseServiceMongoImpl.UpdateById() trả về UpdateResult, không trả về document
//
// LƯU Ý:
// - Method này chỉ là wrapper đơn giản, không có business logic đặc biệt
// - Đã refactor để dùng BaseServiceMongoImpl.UpdateById() với UpdateData struct để nhất quán
//
// ĐẢM BẢO LOGIC CƠ BẢN:
// ✅ Update document theo ID bằng BaseServiceMongoImpl.UpdateById()
// ✅ Trả về document đã được update bằng FindOneById()
// ✅ Trả về error nếu có
func (s *PcOrderService) Update(ctx context.Context, id primitive.ObjectID, pcOrder models.PcOrder) (models.PcOrder, error) {
	// Convert PcOrder sang UpdateData để dùng base method
	updateData, err := ToUpdateData(pcOrder)
	if err != nil {
		return models.PcOrder{}, fmt.Errorf("failed to convert to UpdateData: %w", err)
	}

	_, err = s.BaseServiceMongoImpl.UpdateById(ctx, id, *updateData)
	if err != nil {
		return models.PcOrder{}, err
	}

	// Trả về document đã được update
	return s.BaseServiceMongoImpl.FindOneById(ctx, id)
}
