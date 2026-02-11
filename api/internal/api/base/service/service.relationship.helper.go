package basesvc

import (
	"context"
	"fmt"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RelationshipCheck dinh nghia mot quan he can kiem tra
type RelationshipCheck struct {
	CollectionName string
	FieldName      string
	ErrorMessage   string
	Optional       bool
}

// CheckRelationshipExists kiem tra co record nao trong collection khac dang tro toi record nay khong
func CheckRelationshipExists(ctx context.Context, recordID primitive.ObjectID, checks []RelationshipCheck) error {
	for _, check := range checks {
		collection, exists := global.RegistryCollections.Get(check.CollectionName)
		if !exists {
			if check.Optional {
				continue
			}
			return common.NewError(
				common.ErrCodeInternalServer,
				fmt.Sprintf("Khong tim thay collection '%s' de kiem tra quan he", check.CollectionName),
				common.StatusInternalServerError,
				nil,
			)
		}
		filter := bson.M{check.FieldName: recordID}
		count, err := collection.CountDocuments(ctx, filter)
		if err != nil {
			return common.ConvertMongoError(err)
		}
		if count > 0 {
			errorMsg := check.ErrorMessage
			if errorMsg == "" {
				errorMsg = fmt.Sprintf("Khong the xoa record vi co %d record trong collection '%s' dang tham chieu toi record nay", count, check.CollectionName)
			} else {
				errorMsg = fmt.Sprintf(check.ErrorMessage, count)
			}
			return common.NewError(common.ErrCodeBusinessOperation, errorMsg, common.StatusConflict, nil)
		}
	}
	return nil
}

// CheckRelationshipExistsWithFilter kiem tra quan he voi filter tuy chinh
func CheckRelationshipExistsWithFilter(ctx context.Context, filter bson.M, checks []RelationshipCheck) error {
	for _, check := range checks {
		collection, exists := global.RegistryCollections.Get(check.CollectionName)
		if !exists {
			if check.Optional {
				continue
			}
			return common.NewError(
				common.ErrCodeInternalServer,
				fmt.Sprintf("Khong tim thay collection '%s' de kiem tra quan he", check.CollectionName),
				common.StatusInternalServerError,
				nil,
			)
		}
		count, err := collection.CountDocuments(ctx, filter)
		if err != nil {
			return common.ConvertMongoError(err)
		}
		if count > 0 {
			errorMsg := check.ErrorMessage
			if errorMsg == "" {
				errorMsg = fmt.Sprintf("Khong the xoa record vi co %d record trong collection '%s' dang tham chieu toi record nay", count, check.CollectionName)
			} else {
				errorMsg = fmt.Sprintf(check.ErrorMessage, count)
			}
			return common.NewError(common.ErrCodeBusinessOperation, errorMsg, common.StatusConflict, nil)
		}
	}
	return nil
}

// GetRelationshipCount tra ve so luong record dang tham chieu toi record nay
func GetRelationshipCount(ctx context.Context, recordID primitive.ObjectID, collectionName, fieldName string) (int64, error) {
	collection, exists := global.RegistryCollections.Get(collectionName)
	if !exists {
		return 0, common.NewError(common.ErrCodeInternalServer, fmt.Sprintf("Khong tim thay collection '%s'", collectionName), common.StatusInternalServerError, nil)
	}
	filter := bson.M{fieldName: recordID}
	return collection.CountDocuments(ctx, filter)
}

// ValidateBeforeDeleteRole kiem tra cac quan he cua Role truoc khi xoa
func ValidateBeforeDeleteRole(ctx context.Context, roleID primitive.ObjectID) error {
	checks := []RelationshipCheck{
		{CollectionName: global.MongoDB_ColNames.UserRoles, FieldName: "roleId", ErrorMessage: "Khong the xoa role vi co %d user dang su dung role nay. Vui long go role khoi cac user truoc."},
		{CollectionName: global.MongoDB_ColNames.RolePermissions, FieldName: "roleId", ErrorMessage: "Khong the xoa role vi co %d permission dang duoc gan cho role nay. Vui long go cac permission truoc."},
	}
	return CheckRelationshipExists(ctx, roleID, checks)
}

// ValidateBeforeDeleteOrganization kiem tra cac quan he cua Organization truoc khi xoa
func ValidateBeforeDeleteOrganization(ctx context.Context, orgID primitive.ObjectID) error {
	checks := []RelationshipCheck{
		{CollectionName: global.MongoDB_ColNames.Roles, FieldName: "organizationId", ErrorMessage: "Khong the xoa to chuc vi co %d role truc thuoc. Vui long xoa hoac di chuyen cac role truoc."},
	}
	return CheckRelationshipExists(ctx, orgID, checks)
}

// ValidateBeforeDeletePermission kiem tra cac quan he cua Permission truoc khi xoa
func ValidateBeforeDeletePermission(ctx context.Context, permissionID primitive.ObjectID) error {
	checks := []RelationshipCheck{
		{CollectionName: global.MongoDB_ColNames.RolePermissions, FieldName: "permissionId", ErrorMessage: "Khong the xoa permission vi co %d role dang su dung permission nay. Vui long go permission khoi cac role truoc."},
	}
	return CheckRelationshipExists(ctx, permissionID, checks)
}

// ValidateBeforeDeleteUser kiem tra cac quan he cua User truoc khi xoa
func ValidateBeforeDeleteUser(ctx context.Context, userID primitive.ObjectID) error {
	checks := []RelationshipCheck{
		{CollectionName: global.MongoDB_ColNames.UserRoles, FieldName: "userId", ErrorMessage: "Khong the xoa user vi co %d role dang duoc gan cho user nay. Vui long go cac role truoc."},
	}
	return CheckRelationshipExists(ctx, userID, checks)
}
