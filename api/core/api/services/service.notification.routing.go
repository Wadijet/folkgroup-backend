package services

import (
	"context"
	"fmt"

	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// NotificationRoutingService là cấu trúc chứa các phương thức liên quan đến Notification Routing Rule
type NotificationRoutingService struct {
	*BaseServiceMongoImpl[models.NotificationRoutingRule]
}

// NewNotificationRoutingService tạo mới NotificationRoutingService
func NewNotificationRoutingService() (*NotificationRoutingService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.NotificationRoutingRules)
	if !exist {
		return nil, fmt.Errorf("failed to get notification_routing_rules collection: %v", common.ErrNotFound)
	}

	return &NotificationRoutingService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.NotificationRoutingRule](collection),
	}, nil
}

// FindByEventType tìm rules theo eventType và organizationID (hoặc system organization)
// Lưu ý: EventType giờ là pointer field, cần query với giá trị cụ thể
// Logic: Tìm rule của organization trước, nếu không có → tìm system rule
func (s *NotificationRoutingService) FindByEventType(ctx context.Context, eventType string, organizationID *primitive.ObjectID) ([]models.NotificationRoutingRule, error) {
	// Query đơn giản: MongoDB sẽ tự động match string với pointer field
	filter := bson.M{
		"eventType": eventType, // Query với giá trị cụ thể (MongoDB sẽ match cả string và pointer)
		"isActive":  true,
	}

	// Nếu có organizationID, filter theo organization hoặc system organization
	if organizationID != nil && !organizationID.IsZero() {
		// Lấy System Organization ID để tìm system rules
		systemOrgID, err := s.getSystemOrganizationID(ctx)
		if err == nil {
			filter["ownerOrganizationId"] = bson.M{
				"$in": []primitive.ObjectID{*organizationID, systemOrgID}, // Organization-specific hoặc system rules
			}
		} else {
			// Nếu không lấy được system org ID, chỉ filter theo organization
			filter["ownerOrganizationId"] = *organizationID
		}
	}

	opts := options.Find().SetSort(bson.M{"createdAt": -1})
	cursor, err := s.BaseServiceMongoImpl.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rules []models.NotificationRoutingRule
	if err := cursor.All(ctx, &rules); err != nil {
		return nil, err
	}

	return rules, nil
}

// FindByDomain tìm rules theo domain và organizationID (hoặc system organization)
// Logic: Tìm rule của organization trước, nếu không có → tìm system rule
func (s *NotificationRoutingService) FindByDomain(ctx context.Context, domain string, organizationID *primitive.ObjectID) ([]models.NotificationRoutingRule, error) {
	filter := bson.M{
		"domain":   domain,
		"isActive": true,
	}

	// Nếu có organizationID, filter theo organization hoặc system organization
	if organizationID != nil && !organizationID.IsZero() {
		// Lấy System Organization ID để tìm system rules
		systemOrgID, err := s.getSystemOrganizationID(ctx)
		if err == nil {
			filter["ownerOrganizationId"] = bson.M{
				"$in": []primitive.ObjectID{*organizationID, systemOrgID}, // Organization-specific hoặc system rules
			}
		} else {
			// Nếu không lấy được system org ID, chỉ filter theo organization
			filter["ownerOrganizationId"] = *organizationID
		}
	}

	opts := options.Find().SetSort(bson.M{"createdAt": -1})
	cursor, err := s.BaseServiceMongoImpl.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rules []models.NotificationRoutingRule
	if err := cursor.All(ctx, &rules); err != nil {
		return nil, err
	}

	return rules, nil
}

// getSystemOrganizationID lấy System Organization ID (helper function)
func (s *NotificationRoutingService) getSystemOrganizationID(ctx context.Context) (primitive.ObjectID, error) {
	// Import cta package để dùng GetSystemOrganizationID
	// Tạm thời dùng cách đơn giản: query trực tiếp
	orgService, err := NewOrganizationService()
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to create organization service: %v", err)
	}

	systemFilter := bson.M{
		"level": -1,
		"code":  "SYSTEM",
		"type":  models.OrganizationTypeSystem,
	}

	systemOrg, err := orgService.FindOne(ctx, systemFilter, nil)
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to find System Organization: %v", err)
	}

	return systemOrg.ID, nil
}

// ValidateUniqueness validate uniqueness của routing rule (business logic validation)
//
// LÝ DO PHẢI TẠO METHOD NÀY (không dùng CRUD base):
// 1. Business rules - Uniqueness constraints:
//    - Mỗi organization chỉ có thể có 1 rule cho mỗi eventType (khi isActive = true)
//    - Mỗi organization chỉ có thể có 1 rule cho mỗi domain (khi isActive = true)
//    - Đảm bảo không có duplicate rules trong cùng organization
//
// Tham số:
//   - ctx: Context
//   - rule: Notification routing rule cần validate
//
// Trả về:
//   - error: Lỗi nếu validation thất bại (duplicate rule), nil nếu hợp lệ
func (s *NotificationRoutingService) ValidateUniqueness(ctx context.Context, rule models.NotificationRoutingRule) error {
	// Validate EventType: EventType là bắt buộc
	if rule.EventType == "" {
		return common.NewError(
			common.ErrCodeValidationInput,
			"EventType là bắt buộc và không được để trống",
			common.StatusBadRequest,
			nil,
		)
	}

	// Kiểm tra rule với eventType (EventType là bắt buộc)
	filter := bson.M{
		"eventType":           rule.EventType,
		"ownerOrganizationId": rule.OwnerOrganizationID,
		"isActive":            true, // Chỉ check rules đang active
	}
	
	// Nếu đang update, exclude chính document đó
	if !rule.ID.IsZero() {
		filter["_id"] = bson.M{"$ne": rule.ID}
	}

	_, err := s.FindOne(ctx, filter, nil)
	if err == nil {
		return common.NewError(
			common.ErrCodeBusinessOperation,
			fmt.Sprintf("Đã tồn tại routing rule cho eventType '%s' và organization này. Mỗi organization chỉ có thể có 1 rule cho mỗi eventType", rule.EventType),
			common.StatusConflict,
			nil,
		)
	}
	if err != common.ErrNotFound {
		return fmt.Errorf("lỗi khi kiểm tra uniqueness: %v", err)
	}

	// Kiểm tra rule với domain (nếu có)
	if rule.Domain != nil && *rule.Domain != "" {
		domainFilter := bson.M{
			"domain":              *rule.Domain,
			"ownerOrganizationId": rule.OwnerOrganizationID,
			"isActive":            true, // Chỉ check rules đang active
		}
		
		// Nếu đang update, exclude chính document đó
		if !rule.ID.IsZero() {
			domainFilter["_id"] = bson.M{"$ne": rule.ID}
		}

		_, err := s.FindOne(ctx, domainFilter, nil)
		if err == nil {
			return common.NewError(
				common.ErrCodeBusinessOperation,
				fmt.Sprintf("Đã tồn tại routing rule cho domain '%s' và organization này. Mỗi organization chỉ có thể có 1 rule cho mỗi domain", *rule.Domain),
				common.StatusConflict,
				nil,
			)
		}
		if err != common.ErrNotFound {
			return fmt.Errorf("lỗi khi kiểm tra uniqueness domain: %v", err)
		}
	}

	return nil
}

// InsertOne override để thêm business logic validation trước khi insert
//
// LÝ DO PHẢI OVERRIDE (không dùng BaseServiceMongoImpl.InsertOne trực tiếp):
// 1. Business logic validation:
//    - Validate uniqueness (eventType + ownerOrganizationId, domain + ownerOrganizationId)
//    - Đảm bảo không có duplicate rules trong cùng organization
//
// ĐẢM BẢO LOGIC CƠ BẢN:
// ✅ Validate uniqueness bằng ValidateUniqueness()
// ✅ Gọi BaseServiceMongoImpl.InsertOne để đảm bảo:
//   - Set timestamps (CreatedAt, UpdatedAt)
//   - Generate ID nếu chưa có
//   - Insert vào MongoDB
func (s *NotificationRoutingService) InsertOne(ctx context.Context, data models.NotificationRoutingRule) (models.NotificationRoutingRule, error) {
	// Validate uniqueness (business logic validation)
	if err := s.ValidateUniqueness(ctx, data); err != nil {
		return data, err
	}

	// Gọi InsertOne của base service
	return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}

// ✅ Các method DeleteById, UpdateById đã được xử lý bởi BaseServiceMongoImpl
// với cơ chế bảo vệ dữ liệu hệ thống chung (IsSystem)
