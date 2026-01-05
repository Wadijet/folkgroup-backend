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

// ✅ Các method InsertOne, DeleteById, UpdateById đã được xử lý bởi BaseServiceMongoImpl
// với cơ chế bảo vệ dữ liệu hệ thống chung (IsSystem)
