package services

import (
	"context"
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// OrganizationShareService là service quản lý sharing giữa các organizations
type OrganizationShareService struct {
	*BaseServiceMongoImpl[models.OrganizationShare]
}

// NewOrganizationShareService tạo mới OrganizationShareService
func NewOrganizationShareService() (*OrganizationShareService, error) {
	collectionName := "auth_organization_shares"
	collection, exist := global.RegistryCollections.Get(collectionName)
	if !exist {
		// Nếu chưa có, tạo mới collection từ MongoDB database
		if global.MongoDB_Session == nil {
			return nil, fmt.Errorf("MongoDB session chưa được khởi tạo")
		}
		if global.MongoDB_ServerConfig == nil {
			return nil, fmt.Errorf("MongoDB config chưa được khởi tạo")
		}

		// Lấy database
		db := global.MongoDB_Session.Database(global.MongoDB_ServerConfig.MongoDB_DBName_Auth)
		// Tạo collection
		newCollection := db.Collection(collectionName)
		// Đăng ký vào registry
		_, err := global.RegistryCollections.Register(collectionName, newCollection)
		if err != nil {
			return nil, fmt.Errorf("failed to register collection: %v", err)
		}
		collection = newCollection
	}

	return &OrganizationShareService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.OrganizationShare](collection),
	}, nil
}

// GetSharedOrganizationIDs lấy organizations được share với user's organizations
// userOrgIDs: Danh sách organization IDs của user (từ scope)
// permissionName: Permission name cụ thể (nếu rỗng = tất cả permissions)
func GetSharedOrganizationIDs(ctx context.Context, userOrgIDs []primitive.ObjectID, permissionName string) ([]primitive.ObjectID, error) {
	shareService, err := NewOrganizationShareService()
	if err != nil {
		return nil, err
	}

	if len(userOrgIDs) == 0 {
		return []primitive.ObjectID{}, nil
	}

	// Query: Tìm shares có:
	// 1. ToOrgIDs chứa ít nhất 1 org trong userOrgIDs (share với orgs cụ thể)
	// 2. ToOrgIDs rỗng/null (share với tất cả)
	filter := bson.M{
		"$or": []bson.M{
			// Share với orgs cụ thể: ToOrgIDs chứa ít nhất 1 org trong userOrgIDs
			{"toOrgIds": bson.M{"$in": userOrgIDs}},
			// Share với tất cả: ToOrgIDs rỗng hoặc null
			{"$or": []bson.M{
				{"toOrgIds": bson.M{"$exists": false}},
				{"toOrgIds": bson.M{"$size": 0}},
				{"toOrgIds": nil},
			}},
		},
	}

	// Nếu có permissionName, filter thêm
	if permissionName != "" {
		// Share nếu:
		// 1. PermissionNames rỗng/nil (share tất cả permissions)
		// 2. PermissionNames chứa permissionName cụ thể
		permissionFilter := bson.M{
			"$or": []bson.M{
				{"permissionNames": bson.M{"$exists": false}},                // Không có field
				{"permissionNames": bson.M{"$size": 0}},                      // Array rỗng
				{"permissionNames": bson.M{"$in": []string{permissionName}}}, // Chứa permissionName
			},
		}
		filter = bson.M{
			"$and": []bson.M{
				filter,
				permissionFilter,
			},
		}
	}

	shares, err := shareService.Find(ctx, filter, nil)
	if err != nil {
		if err == common.ErrNotFound {
			return []primitive.ObjectID{}, nil
		}
		return nil, err
	}

	// Lấy OwnerOrganizationID từ shares (organizations share data với user)
	sharedOrgIDsMap := make(map[primitive.ObjectID]bool)
	for _, share := range shares {
		// Nếu có permissionName, kiểm tra kỹ hơn
		if permissionName != "" {
			// Nếu PermissionNames không rỗng và không chứa permissionName → skip
			if len(share.PermissionNames) > 0 {
				hasPermission := false
				for _, pn := range share.PermissionNames {
					if pn == permissionName {
						hasPermission = true
						break
					}
				}
				if !hasPermission {
					continue // Skip share này
				}
			}
		}

		// Kiểm tra share có áp dụng cho user không
		// Nếu ToOrgIDs rỗng → share với tất cả → luôn áp dụng
		// Nếu ToOrgIDs có giá trị → kiểm tra có chứa org của user không
		if len(share.ToOrgIDs) == 0 {
			// Share với tất cả → luôn áp dụng
			sharedOrgIDsMap[share.OwnerOrganizationID] = true
		} else {
			// Share với orgs cụ thể → kiểm tra có org của user trong ToOrgIDs không
			for _, userOrgID := range userOrgIDs {
				for _, shareToOrgID := range share.ToOrgIDs {
					if userOrgID == shareToOrgID {
						sharedOrgIDsMap[share.OwnerOrganizationID] = true
						break
					}
				}
			}
		}
	}

	// Convert to slice
	result := make([]primitive.ObjectID, 0, len(sharedOrgIDsMap))
	for orgID := range sharedOrgIDsMap {
		result = append(result, orgID)
	}

	return result, nil
}
