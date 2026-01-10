package services

import (
	"context"
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ContentNodeService là service quản lý content nodes (L1-L6)
type ContentNodeService struct {
	*BaseServiceMongoImpl[models.ContentNode]
}

// NewContentNodeService tạo mới ContentNodeService
func NewContentNodeService() (*ContentNodeService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.ContentNodes)
	if !exist {
		return nil, fmt.Errorf("failed to get content_nodes collection: %v", common.ErrNotFound)
	}

	return &ContentNodeService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.ContentNode](collection),
	}, nil
}

// GetChildren lấy tất cả children nodes của một parent node
// Tham số:
//   - ctx: Context
//   - parentID: ID của parent node
// Trả về:
//   - []models.ContentNode: Danh sách children nodes
//   - error: Lỗi nếu có
func (s *ContentNodeService) GetChildren(ctx context.Context, parentID primitive.ObjectID) ([]models.ContentNode, error) {
	filter := map[string]interface{}{
		"parentId": parentID,
	}
	return s.Find(ctx, filter, nil)
}

// GetAncestors lấy tất cả ancestors (tổ tiên) của một node bằng cách traverse lên parent chain
// Tham số:
//   - ctx: Context
//   - nodeID: ID của node cần lấy ancestors
// Trả về:
//   - []models.ContentNode: Danh sách ancestors từ root đến parent (theo thứ tự)
//   - error: Lỗi nếu có
func (s *ContentNodeService) GetAncestors(ctx context.Context, nodeID primitive.ObjectID) ([]models.ContentNode, error) {
	var ancestors []models.ContentNode
	currentID := nodeID

	for {
		node, err := s.FindOneById(ctx, currentID)
		if err != nil {
			if err == common.ErrNotFound {
				break // Không tìm thấy node hoặc đã đến root
			}
			return nil, err
		}

		if node.ParentID == nil {
			break // Đã đến root node
		}

		// Lấy parent node
		parent, err := s.FindOneById(ctx, *node.ParentID)
		if err != nil {
			if err == common.ErrNotFound {
				break // Parent không tồn tại
			}
			return nil, err
		}

		ancestors = append([]models.ContentNode{parent}, ancestors...) // Thêm vào đầu để giữ thứ tự từ root đến parent
		currentID = *node.ParentID
	}

	return ancestors, nil
}
