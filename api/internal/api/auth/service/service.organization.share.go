// Package authsvc - service organization share.
package authsvc

import (
	"context"
	"fmt"
	models "meta_commerce/internal/api/auth/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// OrganizationShareService quản lý sharing giữa các organizations
type OrganizationShareService struct {
	*basesvc.BaseServiceMongoImpl[models.OrganizationShare]
}

// NewOrganizationShareService tạo mới OrganizationShareService
func NewOrganizationShareService() (*OrganizationShareService, error) {
	collectionName := "auth_organization_shares"
	collection, exist := global.RegistryCollections.Get(collectionName)
	if !exist {
		if global.MongoDB_Session == nil {
			return nil, fmt.Errorf("MongoDB session chưa được khởi tạo")
		}
		if global.MongoDB_ServerConfig == nil {
			return nil, fmt.Errorf("MongoDB config chưa được khởi tạo")
		}
		db := global.MongoDB_Session.Database(global.MongoDB_ServerConfig.MongoDB_DBName_Auth)
		newCollection := db.Collection(collectionName)
		_, err := global.RegistryCollections.Register(collectionName, newCollection)
		if err != nil {
			return nil, fmt.Errorf("failed to register collection: %v", err)
		}
		collection = newCollection
	}

	return &OrganizationShareService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[models.OrganizationShare](collection),
	}, nil
}

// InsertOne override để validate và duplicate check
func (s *OrganizationShareService) InsertOne(ctx context.Context, data models.OrganizationShare) (models.OrganizationShare, error) {
	for _, toOrgID := range data.ToOrgIDs {
		if toOrgID == data.OwnerOrganizationID {
			return data, common.NewError(common.ErrCodeValidationInput, "ownerOrganizationId không được có trong toOrgIds", common.StatusBadRequest, nil)
		}
	}

	existingShares, err := s.BaseServiceMongoImpl.Find(ctx, bson.M{"ownerOrganizationId": data.OwnerOrganizationID}, nil)
	if err != nil && err != common.ErrNotFound {
		return data, err
	}
	for _, existingShare := range existingShares {
		if compareShareSets(data, existingShare) {
			return data, common.NewError(common.ErrCodeBusinessOperation, "Share với các organizations này đã tồn tại với cùng permissions", common.StatusConflict, nil)
		}
	}
	return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}

func compareShareSets(share1, share2 models.OrganizationShare) bool {
	if !compareObjectIDSets(share1.ToOrgIDs, share2.ToOrgIDs) {
		return false
	}
	return compareStringSets(share1.PermissionNames, share2.PermissionNames)
}

func compareObjectIDSets(ids1, ids2 []primitive.ObjectID) bool {
	if len(ids1) != len(ids2) {
		return false
	}
	if len(ids1) == 0 {
		return true
	}
	ids1Map := make(map[primitive.ObjectID]bool)
	for _, id := range ids1 {
		ids1Map[id] = true
	}
	for _, id := range ids2 {
		if !ids1Map[id] {
			return false
		}
	}
	return true
}

func compareStringSets(strs1, strs2 []string) bool {
	if len(strs1) != len(strs2) {
		return false
	}
	if len(strs1) == 0 {
		return true
	}
	strs1Map := make(map[string]bool)
	for _, s := range strs1 {
		strs1Map[s] = true
	}
	for _, s := range strs2 {
		if !strs1Map[s] {
			return false
		}
	}
	return true
}

// GetSharedOrganizationIDs lấy organizations được share với user's organizations
func GetSharedOrganizationIDs(ctx context.Context, userOrgIDs []primitive.ObjectID, permissionName string) ([]primitive.ObjectID, error) {
	shareService, err := NewOrganizationShareService()
	if err != nil {
		return nil, err
	}
	if len(userOrgIDs) == 0 {
		return []primitive.ObjectID{}, nil
	}

	filter := bson.M{
		"$or": []bson.M{
			{"toOrgIds": bson.M{"$in": userOrgIDs}},
			{"$or": []bson.M{
				{"toOrgIds": bson.M{"$exists": false}},
				{"toOrgIds": bson.M{"$size": 0}},
				{"toOrgIds": nil},
			}},
		},
	}
	if permissionName != "" {
		permissionFilter := bson.M{
			"$or": []bson.M{
				{"permissionNames": bson.M{"$exists": false}},
				{"permissionNames": bson.M{"$size": 0}},
				{"permissionNames": bson.M{"$in": []string{permissionName}}},
			},
		}
		filter = bson.M{"$and": []bson.M{filter, permissionFilter}}
	}

	shares, err := shareService.BaseServiceMongoImpl.Find(ctx, filter, nil)
	if err != nil {
		if err == common.ErrNotFound {
			return []primitive.ObjectID{}, nil
		}
		return nil, err
	}

	sharedOrgIDsMap := make(map[primitive.ObjectID]bool)
	for _, share := range shares {
		if permissionName != "" && len(share.PermissionNames) > 0 {
			hasPermission := false
			for _, pn := range share.PermissionNames {
				if pn == permissionName {
					hasPermission = true
					break
				}
			}
			if !hasPermission {
				continue
			}
		}
		if len(share.ToOrgIDs) == 0 {
			sharedOrgIDsMap[share.OwnerOrganizationID] = true
		} else {
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
	result := make([]primitive.ObjectID, 0, len(sharedOrgIDsMap))
	for orgID := range sharedOrgIDsMap {
		result = append(result, orgID)
	}
	return result, nil
}
