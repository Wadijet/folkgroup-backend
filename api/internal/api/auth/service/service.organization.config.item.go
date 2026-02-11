// Package authsvc - service organization config item.
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

// OrganizationConfigItemService xử lý config theo từng key (1 document per key).
type OrganizationConfigItemService struct {
	*basesvc.BaseServiceMongoImpl[models.OrganizationConfigItem]
	organizationService *OrganizationService
}

// NewOrganizationConfigItemService tạo mới OrganizationConfigItemService.
func NewOrganizationConfigItemService() (*OrganizationConfigItemService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.OrganizationConfigItems)
	if !exist {
		return nil, fmt.Errorf("failed to get organization_config_items collection: %v", common.ErrNotFound)
	}
	organizationService, err := NewOrganizationService()
	if err != nil {
		return nil, fmt.Errorf("failed to create organization service: %w", err)
	}
	return &OrganizationConfigItemService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[models.OrganizationConfigItem](collection),
		organizationService:  organizationService,
	}, nil
}

// GetByOwnerOrganizationIDAndKey lấy một config item theo org và key.
func (s *OrganizationConfigItemService) GetByOwnerOrganizationIDAndKey(ctx context.Context, orgID primitive.ObjectID, key string) (*models.OrganizationConfigItem, error) {
	filter := bson.M{"ownerOrganizationId": orgID, "key": key}
	doc, err := s.BaseServiceMongoImpl.FindOne(ctx, filter, nil)
	if err != nil {
		return nil, err
	}
	var result models.OrganizationConfigItem
	bsonBytes, _ := bson.Marshal(doc)
	if err := bson.Unmarshal(bsonBytes, &result); err != nil {
		return nil, common.ErrInvalidFormat
	}
	return &result, nil
}

// FindByOwnerOrganizationID lấy tất cả config item của một tổ chức.
func (s *OrganizationConfigItemService) FindByOwnerOrganizationID(ctx context.Context, orgID primitive.ObjectID) ([]models.OrganizationConfigItem, error) {
	filter := bson.M{"ownerOrganizationId": orgID}
	docs, err := s.BaseServiceMongoImpl.Find(ctx, filter, nil)
	if err != nil {
		return nil, err
	}
	result := make([]models.OrganizationConfigItem, 0, len(docs))
	for _, doc := range docs {
		var item models.OrganizationConfigItem
		bsonBytes, _ := bson.Marshal(doc)
		if err := bson.Unmarshal(bsonBytes, &item); err != nil {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *OrganizationConfigItemService) validateLockedKey(ctx context.Context, orgID primitive.ObjectID, key string) error {
	parentIDs, err := s.organizationService.GetParentIDs(ctx, orgID)
	if err != nil || len(parentIDs) == 0 {
		return nil
	}
	for _, pid := range parentIDs {
		item, err := s.GetByOwnerOrganizationIDAndKey(ctx, pid, key)
		if err != nil || item == nil {
			continue
		}
		if !item.AllowOverride {
			return common.NewError(common.ErrCodeBusinessOperation, fmt.Sprintf("Key '%s' đã bị khóa bởi tổ chức cấp trên, không thể thay đổi.", key), common.StatusForbidden, nil)
		}
	}
	return nil
}

// ConfigValueForValidation struct dùng cho global.Validate.
type ConfigValueForValidation struct {
	Value       interface{} `validate:"config_value"`
	DataType    string
	Constraints string
}

// Upsert override: validate locked key + config_value rồi gọi base Upsert.
func (s *OrganizationConfigItemService) Upsert(ctx context.Context, filter interface{}, data interface{}) (models.OrganizationConfigItem, error) {
	var zero models.OrganizationConfigItem
	if updateData, ok := data.(*basesvc.UpdateData); ok {
		if updateData.Set != nil {
			if keyStr, _ := updateData.Set["key"].(string); keyStr != "" {
				var orgID primitive.ObjectID
				switch v := updateData.Set["ownerOrganizationId"].(type) {
				case primitive.ObjectID:
					orgID = v
				case string:
					orgID, _ = primitive.ObjectIDFromHex(v)
				}
				if !orgID.IsZero() {
					if err := s.validateLockedKey(ctx, orgID, keyStr); err != nil {
						return zero, err
					}
				}
			}
			if constraints, _ := updateData.Set["constraints"].(string); constraints != "" {
				dataType := ""
				if dt, ok := updateData.Set["dataType"].(string); ok {
					dataType = dt
				}
				v := ConfigValueForValidation{Value: updateData.Set["value"], DataType: dataType, Constraints: constraints}
				if err := global.Validate.Struct(v); err != nil {
					return zero, common.NewError(common.ErrCodeValidationFormat, "Giá trị config không thỏa ràng buộc: "+err.Error(), common.StatusBadRequest, err)
				}
			}
		}
		return s.BaseServiceMongoImpl.Upsert(ctx, filter, updateData)
	}
	item, ok := data.(*models.OrganizationConfigItem)
	if !ok {
		if v, ok2 := data.(models.OrganizationConfigItem); ok2 {
			item = &v
		} else {
			return zero, common.ErrInvalidFormat
		}
	}
	if err := s.validateLockedKey(ctx, item.OwnerOrganizationID, item.Key); err != nil {
		return zero, err
	}
	if item.Constraints != "" {
		v := ConfigValueForValidation{Value: item.Value, DataType: item.DataType, Constraints: item.Constraints}
		if err := global.Validate.Struct(v); err != nil {
			return zero, common.NewError(common.ErrCodeValidationFormat, "Giá trị config không thỏa ràng buộc: "+err.Error(), common.StatusBadRequest, err)
		}
	}
	item.IsSystem = false
	result, err := s.BaseServiceMongoImpl.Upsert(ctx, filter, item)
	if err != nil {
		return zero, err
	}
	return result, nil
}

// UpsertItem tạo hoặc cập nhật một config item.
func (s *OrganizationConfigItemService) UpsertItem(ctx context.Context, item *models.OrganizationConfigItem) (*models.OrganizationConfigItem, error) {
	filter := bson.M{"ownerOrganizationId": item.OwnerOrganizationID, "key": item.Key}
	doc, err := s.Upsert(ctx, filter, item)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// GetResolvedConfig merge config theo cây từ root xuống org.
func (s *OrganizationConfigItemService) GetResolvedConfig(ctx context.Context, orgID primitive.ObjectID) (map[string]interface{}, error) {
	parentIDs, err := s.organizationService.GetParentIDs(ctx, orgID)
	if err != nil {
		return nil, err
	}
	chain := make([]primitive.ObjectID, 0, len(parentIDs)+1)
	for i := len(parentIDs) - 1; i >= 0; i-- {
		chain = append(chain, parentIDs[i])
	}
	chain = append(chain, orgID)
	lockedKeys := make(map[string]bool)
	resolved := make(map[string]interface{})
	for _, oid := range chain {
		items, _ := s.FindByOwnerOrganizationID(ctx, oid)
		for _, it := range items {
			if !lockedKeys[it.Key] {
				resolved[it.Key] = it.Value
			}
		}
		for _, it := range items {
			if !it.AllowOverride {
				lockedKeys[it.Key] = true
			}
		}
	}
	return resolved, nil
}

// ValidateBeforeDeleteItem kiểm tra không xóa item hệ thống.
func (s *OrganizationConfigItemService) ValidateBeforeDeleteItem(ctx context.Context, item *models.OrganizationConfigItem) error {
	if item.IsSystem {
		return common.NewError(common.ErrCodeBusinessOperation, "Không thể xóa config item của hệ thống.", common.StatusForbidden, nil)
	}
	systemOrg, err := s.organizationService.BaseServiceMongoImpl.FindOne(ctx, bson.M{"type": models.OrganizationTypeSystem, "code": "SYSTEM"}, nil)
	if err != nil {
		return nil
	}
	var systemModel models.Organization
	bsonBytes, _ := bson.Marshal(systemOrg)
	if bson.Unmarshal(bsonBytes, &systemModel) != nil {
		return nil
	}
	if item.OwnerOrganizationID == systemModel.ID {
		return common.NewError(common.ErrCodeBusinessOperation, "Không thể xóa config item của hệ thống.", common.StatusForbidden, nil)
	}
	return nil
}

// DeleteOne override
func (s *OrganizationConfigItemService) DeleteOne(ctx context.Context, filter interface{}) error {
	doc, err := s.BaseServiceMongoImpl.FindOne(ctx, filter, nil)
	if err != nil {
		return err
	}
	var item models.OrganizationConfigItem
	bsonBytes, _ := bson.Marshal(doc)
	if err := bson.Unmarshal(bsonBytes, &item); err != nil {
		return common.ErrInvalidFormat
	}
	if err := s.ValidateBeforeDeleteItem(ctx, &item); err != nil {
		return err
	}
	return s.BaseServiceMongoImpl.DeleteOne(ctx, filter)
}
