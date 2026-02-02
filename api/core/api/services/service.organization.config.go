package services

import (
	"context"
	"fmt"
	"time"

	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// OrganizationConfigService xử lý config theo từng tổ chức: CRUD, GetResolvedConfig (merge theo cây), validate config hệ thống và key bị khóa.
type OrganizationConfigService struct {
	*BaseServiceMongoImpl[models.OrganizationConfig]
	organizationService *OrganizationService
}

// NewOrganizationConfigService tạo mới OrganizationConfigService.
// Trả về: *OrganizationConfigService, error.
func NewOrganizationConfigService() (*OrganizationConfigService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.OrganizationConfigs)
	if !exist {
		return nil, fmt.Errorf("failed to get organization_configs collection: %v", common.ErrNotFound)
	}

	organizationService, err := NewOrganizationService()
	if err != nil {
		return nil, fmt.Errorf("failed to create organization service: %w", err)
	}

	return &OrganizationConfigService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.OrganizationConfig](collection),
		organizationService:  organizationService,
	}, nil
}

// GetByOwnerOrganizationID lấy config raw của một tổ chức theo ownerOrganizationId.
// Trả về: *models.OrganizationConfig, error. ErrNotFound nếu chưa có config.
func (s *OrganizationConfigService) GetByOwnerOrganizationID(ctx context.Context, orgID primitive.ObjectID) (*models.OrganizationConfig, error) {
	filter := bson.M{"ownerOrganizationId": orgID}
	doc, err := s.FindOne(ctx, filter, nil)
	if err != nil {
		return nil, err
	}

	var result models.OrganizationConfig
	bsonBytes, _ := bson.Marshal(doc)
	if err := bson.Unmarshal(bsonBytes, &result); err != nil {
		return nil, common.ErrInvalidFormat
	}
	return &result, nil
}

// UpsertByOwnerOrganizationID tạo hoặc cập nhật config cho một tổ chức (upsert theo ownerOrganizationId).
// Validate: không cho cấp dưới set key đã bị khóa (ConfigMeta[k].AllowOverride == false) bởi tổ chức cha.
// Tham số: ctx, orgID, config (giá trị), configMeta (metadata từng key), isSystem (chỉ init được set true).
// Trả về: *models.OrganizationConfig, error.
func (s *OrganizationConfigService) UpsertByOwnerOrganizationID(ctx context.Context, orgID primitive.ObjectID, config map[string]interface{}, configMeta map[string]models.ConfigKeyMeta, isSystem bool) (*models.OrganizationConfig, error) {
	now := time.Now().UnixMilli()

	// Validate key bị khóa: nếu org không phải root, kiểm tra tổ chức cha đã khóa key nào
	if config != nil && configMeta != nil {
		if err := s.validateLockedKeysOnUpdate(ctx, orgID, config); err != nil {
			return nil, err
		}
	}

	filter := bson.M{"ownerOrganizationId": orgID}
	update := &UpdateData{
		Set: map[string]interface{}{
			"ownerOrganizationId": orgID,
			"config":              config,
			"configMeta":          configMeta,
			"isSystem":            isSystem,
			"updatedAt":           now,
		},
		SetOnInsert: map[string]interface{}{
			"createdAt": now,
		},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	result, err := s.BaseServiceMongoImpl.FindOneAndUpdate(ctx, filter, update, opts)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	return &result, nil
}

// validateLockedKeysOnUpdate kiểm tra khi cập nhật config: nếu key nằm trong ConfigMeta của tổ chức cha với AllowOverride == false thì trả lỗi.
func (s *OrganizationConfigService) validateLockedKeysOnUpdate(ctx context.Context, orgID primitive.ObjectID, config map[string]interface{}) error {
	parentIDs, err := s.organizationService.GetParentIDs(ctx, orgID)
	if err != nil || len(parentIDs) == 0 {
		return nil
	}

	lockedKeys := make(map[string]bool)
	for i := len(parentIDs) - 1; i >= 0; i-- {
		doc, err := s.GetByOwnerOrganizationID(ctx, parentIDs[i])
		if err != nil || doc == nil || doc.ConfigMeta == nil {
			continue
		}
		for k, meta := range doc.ConfigMeta {
			if !meta.AllowOverride {
				lockedKeys[k] = true
			}
		}
	}

	for k := range config {
		if lockedKeys[k] {
			return common.NewError(common.ErrCodeBusinessOperation,
				fmt.Sprintf("Key '%s' đã bị khóa bởi tổ chức cấp trên, không thể thay đổi.", k),
				common.StatusForbidden, nil)
		}
	}
	return nil
}

// GetResolvedConfig merge config theo cây từ root xuống org hiện tại; key có ConfigMeta[k].AllowOverride == false thì cấp dưới không ghi đè.
// Trả về: map[string]interface{} (chỉ giá trị config đã resolve), error.
func (s *OrganizationConfigService) GetResolvedConfig(ctx context.Context, orgID primitive.ObjectID) (map[string]interface{}, error) {
	parentIDs, err := s.organizationService.GetParentIDs(ctx, orgID)
	if err != nil {
		return nil, err
	}

	resolved := make(map[string]interface{})
	lockedKeys := make(map[string]bool)

	// Duyệt từ root xuống org hiện tại: parentIDs[0]=parent trực tiếp, parentIDs[len-1]=root → thứ tự merge: parentIDs[len-1], ..., parentIDs[0], orgID
	chain := make([]primitive.ObjectID, 0, len(parentIDs)+1)
	for i := len(parentIDs) - 1; i >= 0; i-- {
		chain = append(chain, parentIDs[i])
	}
	chain = append(chain, orgID)

	for _, oid := range chain {
		doc, err := s.GetByOwnerOrganizationID(ctx, oid)
		if err != nil || doc == nil {
			continue
		}
		if doc.Config != nil {
			for k, v := range doc.Config {
				if !lockedKeys[k] {
					resolved[k] = v
				}
			}
		}
		if doc.ConfigMeta != nil {
			for k, meta := range doc.ConfigMeta {
				if !meta.AllowOverride {
					lockedKeys[k] = true
				}
			}
		}
	}

	return resolved, nil
}

// ValidateBeforeDelete kiểm tra trước khi xóa: config của hệ thống (IsSystem = true hoặc OwnerOrganizationID = System Org) không cho xóa.
// Trả về: error nếu không được phép xóa.
func (s *OrganizationConfigService) ValidateBeforeDelete(ctx context.Context, doc *models.OrganizationConfig) error {
	if doc.IsSystem {
		return common.NewError(common.ErrCodeBusinessOperation, "Không thể xóa config của hệ thống.", common.StatusForbidden, nil)
	}

	systemOrg, err := s.organizationService.FindOne(ctx, bson.M{"type": models.OrganizationTypeSystem, "code": "SYSTEM"}, nil)
	if err != nil {
		return nil
	}
	var systemModel models.Organization
	bsonBytes, _ := bson.Marshal(systemOrg)
	if bson.Unmarshal(bsonBytes, &systemModel) != nil {
		return nil
	}
	if doc.OwnerOrganizationID == systemModel.ID {
		return common.NewError(common.ErrCodeBusinessOperation, "Không thể xóa config của hệ thống.", common.StatusForbidden, nil)
	}
	return nil
}

// DeleteByOwnerOrganizationID xóa config của một tổ chức (sau khi validate không phải config hệ thống).
// Trả về: error nếu không tìm thấy hoặc không được phép xóa.
func (s *OrganizationConfigService) DeleteByOwnerOrganizationID(ctx context.Context, orgID primitive.ObjectID) error {
	doc, err := s.GetByOwnerOrganizationID(ctx, orgID)
	if err != nil {
		return err
	}
	if err := s.ValidateBeforeDelete(ctx, doc); err != nil {
		return err
	}
	return s.DeleteOne(ctx, bson.M{"ownerOrganizationId": orgID})
}
