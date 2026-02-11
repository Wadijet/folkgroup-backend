// Package authsvc - service tổ chức (Organization).
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
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

// OrganizationService là cấu trúc chứa các phương thức liên quan đến tổ chức
type OrganizationService struct {
	*basesvc.BaseServiceMongoImpl[models.Organization]
	roleService *RoleService
}

// NewOrganizationService tạo mới OrganizationService
func NewOrganizationService() (*OrganizationService, error) {
	organizationCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.Organizations)
	if !exist {
		return nil, fmt.Errorf("failed to get organizations collection: %v", common.ErrNotFound)
	}

	roleService, err := NewRoleService()
	if err != nil {
		return nil, fmt.Errorf("failed to create role service: %v", err)
	}

	return &OrganizationService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[models.Organization](organizationCollection),
		roleService:          roleService,
	}, nil
}

// GetChildrenIDs lấy tất cả ID của organization con (dùng cho Scope = 1)
func (s *OrganizationService) GetChildrenIDs(ctx context.Context, parentID primitive.ObjectID) ([]primitive.ObjectID, error) {
	parent, err := s.BaseServiceMongoImpl.FindOneById(ctx, parentID)
	if err != nil {
		return nil, err
	}

	filter := bson.M{
		"path":     bson.M{"$regex": "^" + parent.Path},
		"isActive": true,
	}

	orgs, err := s.BaseServiceMongoImpl.Find(ctx, filter, nil)
	if err != nil {
		return nil, err
	}

	ids := make([]primitive.ObjectID, 0, len(orgs))
	for _, org := range orgs {
		ids = append(ids, org.ID)
	}
	return ids, nil
}

// GetParentIDs lấy tất cả ID của organization cha
func (s *OrganizationService) GetParentIDs(ctx context.Context, childID primitive.ObjectID) ([]primitive.ObjectID, error) {
	child, err := s.BaseServiceMongoImpl.FindOneById(ctx, childID)
	if err != nil {
		return nil, err
	}

	if child.ParentID == nil {
		return []primitive.ObjectID{}, nil
	}

	parentIDs := make([]primitive.ObjectID, 0)
	currentID := *child.ParentID

	for {
		parent, err := s.BaseServiceMongoImpl.FindOneById(ctx, currentID)
		if err != nil {
			break
		}
		parentIDs = append(parentIDs, parent.ID)
		if parent.ParentID == nil {
			break
		}
		currentID = *parent.ParentID
	}
	return parentIDs, nil
}

// validateBeforeDelete kiểm tra các điều kiện trước khi xóa organization
func (s *OrganizationService) validateBeforeDelete(ctx context.Context, orgID primitive.ObjectID) error {
	org, err := s.BaseServiceMongoImpl.FindOneById(ctx, orgID)
	if err != nil {
		return err
	}

	var modelOrg models.Organization
	bsonBytes, _ := bson.Marshal(org)
	if err := bson.Unmarshal(bsonBytes, &modelOrg); err != nil {
		return common.ErrInvalidFormat
	}

	if modelOrg.Type == models.OrganizationTypeSystem && modelOrg.Code == "SYSTEM" && modelOrg.Level == -1 {
		return common.NewError(
			common.ErrCodeBusinessOperation,
			"Không thể xóa System organization. Đây là tổ chức cấp cao nhất chứa Administrator và không thể xóa.",
			common.StatusForbidden,
			nil,
		)
	}

	childrenFilter := bson.M{
		"$or": []bson.M{
			{"parentId": modelOrg.ID},
			{"path": bson.M{"$regex": "^" + modelOrg.Path + "/"}},
		},
	}
	children, err := s.BaseServiceMongoImpl.Find(ctx, childrenFilter, nil)
	if err != nil && err != common.ErrNotFound {
		return err
	}
	if err == nil && len(children) > 0 {
		return common.NewError(
			common.ErrCodeBusinessOperation,
			fmt.Sprintf("Không thể xóa tổ chức '%s' vì có %d tổ chức con. Vui lòng xóa hoặc di chuyển các tổ chức con trước.", modelOrg.Name, len(children)),
			common.StatusConflict,
			nil,
		)
	}

	rolesFilter := bson.M{"organizationId": modelOrg.ID}
	roles, err := s.roleService.BaseServiceMongoImpl.Find(ctx, rolesFilter, nil)
	if err != nil && err != common.ErrNotFound {
		return err
	}
	if err == nil && len(roles) > 0 {
		return common.NewError(
			common.ErrCodeBusinessOperation,
			fmt.Sprintf("Không thể xóa tổ chức '%s' vì có %d role trực thuộc. Vui lòng xóa hoặc di chuyển các role trước.", modelOrg.Name, len(roles)),
			common.StatusConflict,
			nil,
		)
	}
	return nil
}

// DeleteOne override
func (s *OrganizationService) DeleteOne(ctx context.Context, filter interface{}) error {
	org, err := s.BaseServiceMongoImpl.FindOne(ctx, filter, nil)
	if err != nil {
		return err
	}
	var modelOrg models.Organization
	bsonBytes, _ := bson.Marshal(org)
	if err := bson.Unmarshal(bsonBytes, &modelOrg); err != nil {
		return common.ErrInvalidFormat
	}
	if err := s.validateBeforeDelete(ctx, modelOrg.ID); err != nil {
		return err
	}
	return s.BaseServiceMongoImpl.DeleteOne(ctx, filter)
}

// DeleteById override
func (s *OrganizationService) DeleteById(ctx context.Context, id primitive.ObjectID) error {
	if err := s.validateBeforeDelete(ctx, id); err != nil {
		return err
	}
	return s.BaseServiceMongoImpl.DeleteById(ctx, id)
}

// DeleteMany override
func (s *OrganizationService) DeleteMany(ctx context.Context, filter interface{}) (int64, error) {
	orgs, err := s.BaseServiceMongoImpl.Find(ctx, filter, nil)
	if err != nil && err != common.ErrNotFound {
		return 0, err
	}
	for _, org := range orgs {
		var modelOrg models.Organization
		bsonBytes, _ := bson.Marshal(org)
		if err := bson.Unmarshal(bsonBytes, &modelOrg); err != nil {
			continue
		}
		if err := s.validateBeforeDelete(ctx, modelOrg.ID); err != nil {
			return 0, err
		}
	}
	return s.BaseServiceMongoImpl.DeleteMany(ctx, filter)
}

// FindOneAndDelete override
func (s *OrganizationService) FindOneAndDelete(ctx context.Context, filter interface{}, opts *mongoopts.FindOneAndDeleteOptions) (models.Organization, error) {
	var zero models.Organization
	org, err := s.BaseServiceMongoImpl.FindOne(ctx, filter, nil)
	if err != nil {
		return zero, err
	}
	var modelOrg models.Organization
	bsonBytes, _ := bson.Marshal(org)
	if err := bson.Unmarshal(bsonBytes, &modelOrg); err != nil {
		return zero, common.ErrInvalidFormat
	}
	if err := s.validateBeforeDelete(ctx, modelOrg.ID); err != nil {
		return zero, err
	}
	return s.BaseServiceMongoImpl.FindOneAndDelete(ctx, filter, opts)
}

// CalculatePathAndLevel tính toán Path và Level cho organization
func (s *OrganizationService) CalculatePathAndLevel(ctx context.Context, org models.Organization) (string, int, error) {
	if org.ParentID != nil && !org.ParentID.IsZero() {
		parent, err := s.BaseServiceMongoImpl.FindOneById(ctx, *org.ParentID)
		if err != nil {
			return "", 0, common.NewError(
				common.ErrCodeBusinessOperation,
				fmt.Sprintf("Không tìm thấy tổ chức cha với ID: %s", org.ParentID.Hex()),
				common.StatusBadRequest,
				err,
			)
		}
		var modelParent models.Organization
		bsonBytes, _ := bson.Marshal(parent)
		if err := bson.Unmarshal(bsonBytes, &modelParent); err != nil {
			return "", 0, common.ErrInvalidFormat
		}
		path := modelParent.Path + "/" + org.Code
		level := s.calculateLevel(org.Type, modelParent.Level)
		return path, level, nil
	}

	if org.Type == models.OrganizationTypeSystem {
		return "/" + org.Code, -1, nil
	}
	if org.Type == models.OrganizationTypeGroup {
		return "/" + org.Code, 0, nil
	}
	return "", 0, common.NewError(
		common.ErrCodeBusinessOperation,
		fmt.Sprintf("Loại tổ chức '%s' phải có parent. Chỉ 'system' và 'group' mới có thể không có parent.", org.Type),
		common.StatusBadRequest,
		nil,
	)
}

func (s *OrganizationService) calculateLevel(orgType string, parentLevel int) int {
	switch orgType {
	case models.OrganizationTypeSystem:
		return -1
	case models.OrganizationTypeGroup:
		return 0
	case models.OrganizationTypeCompany:
		return 1
	case models.OrganizationTypeDepartment:
		return 2
	case models.OrganizationTypeDivision:
		return 3
	case models.OrganizationTypeTeam:
		return parentLevel + 1
	default:
		return parentLevel + 1
	}
}

// InsertOne override để tính Path/Level trước khi insert
func (s *OrganizationService) InsertOne(ctx context.Context, data models.Organization) (models.Organization, error) {
	path, level, err := s.CalculatePathAndLevel(ctx, data)
	if err != nil {
		return data, err
	}
	data.Path = path
	data.Level = level
	return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}
