package contentsvc

import (
	"context"
	"fmt"

	contentmodels "meta_commerce/internal/api/content/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	basesvc "meta_commerce/internal/api/base/service"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ContentNodeService là service quản lý content nodes (L1-L6)
type ContentNodeService struct {
	*basesvc.BaseServiceMongoImpl[contentmodels.ContentNode]
}

// NewContentNodeService tạo mới ContentNodeService
func NewContentNodeService() (*ContentNodeService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.ContentNodes)
	if !exist {
		return nil, fmt.Errorf("failed to get content_nodes collection: %v", common.ErrNotFound)
	}
	return &ContentNodeService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[contentmodels.ContentNode](collection),
	}, nil
}

// GetChildren lấy tất cả children nodes của một parent node
func (s *ContentNodeService) GetChildren(ctx context.Context, parentID primitive.ObjectID) ([]contentmodels.ContentNode, error) {
	filter := map[string]interface{}{"parentId": parentID}
	return s.Find(ctx, filter, nil)
}

// GetAncestors lấy tất cả ancestors (tổ tiên) của một node bằng cách traverse lên parent chain
func (s *ContentNodeService) GetAncestors(ctx context.Context, nodeID primitive.ObjectID) ([]contentmodels.ContentNode, error) {
	var ancestors []contentmodels.ContentNode
	currentID := nodeID
	for {
		node, err := s.FindOneById(ctx, currentID)
		if err != nil {
			if err == common.ErrNotFound {
				break
			}
			return nil, err
		}
		if node.ParentID == nil {
			break
		}
		parent, err := s.FindOneById(ctx, *node.ParentID)
		if err != nil {
			if err == common.ErrNotFound {
				break
			}
			return nil, err
		}
		ancestors = append([]contentmodels.ContentNode{parent}, ancestors...)
		currentID = *node.ParentID
	}
	return ancestors, nil
}
