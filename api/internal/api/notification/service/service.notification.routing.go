package notifsvc

import (
	"context"
	"fmt"

	authmodels "meta_commerce/internal/api/auth/models"
	authsvc "meta_commerce/internal/api/auth/service"
	notifmodels "meta_commerce/internal/api/notification/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// NotificationRoutingService là cấu trúc chứa các phương thức liên quan đến Notification Routing Rule
type NotificationRoutingService struct {
	*basesvc.BaseServiceMongoImpl[notifmodels.NotificationRoutingRule]
}

// NewNotificationRoutingService tạo mới NotificationRoutingService
func NewNotificationRoutingService() (*NotificationRoutingService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.NotificationRoutingRules)
	if !exist {
		return nil, fmt.Errorf("failed to get notification_routing_rules collection: %v", common.ErrNotFound)
	}

	return &NotificationRoutingService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[notifmodels.NotificationRoutingRule](collection),
	}, nil
}

// FindByEventType tìm rules theo eventType và organizationID (hoặc system organization)
func (s *NotificationRoutingService) FindByEventType(ctx context.Context, eventType string, organizationID *primitive.ObjectID) ([]notifmodels.NotificationRoutingRule, error) {
	filter := bson.M{
		"eventType": eventType,
		"isActive":  true,
	}

	if organizationID != nil && !organizationID.IsZero() {
		systemOrgID, err := s.getSystemOrganizationID(ctx)
		if err == nil {
			filter["ownerOrganizationId"] = bson.M{
				"$in": []primitive.ObjectID{*organizationID, systemOrgID},
			}
		} else {
			filter["ownerOrganizationId"] = *organizationID
		}
	}

	opts := options.Find().SetSort(bson.M{"createdAt": -1})
	cursor, err := s.Collection().Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rules []notifmodels.NotificationRoutingRule
	if err := cursor.All(ctx, &rules); err != nil {
		return nil, err
	}

	return rules, nil
}

// FindByDomain tìm rules theo domain và organizationID (hoặc system organization)
func (s *NotificationRoutingService) FindByDomain(ctx context.Context, domain string, organizationID *primitive.ObjectID) ([]notifmodels.NotificationRoutingRule, error) {
	filter := bson.M{
		"domain":   domain,
		"isActive": true,
	}

	if organizationID != nil && !organizationID.IsZero() {
		systemOrgID, err := s.getSystemOrganizationID(ctx)
		if err == nil {
			filter["ownerOrganizationId"] = bson.M{
				"$in": []primitive.ObjectID{*organizationID, systemOrgID},
			}
		} else {
			filter["ownerOrganizationId"] = *organizationID
		}
	}

	opts := options.Find().SetSort(bson.M{"createdAt": -1})
	cursor, err := s.Collection().Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rules []notifmodels.NotificationRoutingRule
	if err := cursor.All(ctx, &rules); err != nil {
		return nil, err
	}

	return rules, nil
}

// getSystemOrganizationID lấy System Organization ID (helper function)
func (s *NotificationRoutingService) getSystemOrganizationID(ctx context.Context) (primitive.ObjectID, error) {
	orgService, err := authsvc.NewOrganizationService()
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to create organization service: %v", err)
	}

	systemFilter := bson.M{
		"level": -1,
		"code":  "SYSTEM",
		"type":  authmodels.OrganizationTypeSystem,
	}

	systemOrg, err := orgService.FindOne(ctx, systemFilter, nil)
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to find System Organization: %v", err)
	}

	return systemOrg.ID, nil
}

// ValidateUniqueness validate uniqueness của routing rule (business logic validation)
func (s *NotificationRoutingService) ValidateUniqueness(ctx context.Context, rule notifmodels.NotificationRoutingRule) error {
	if rule.EventType == "" {
		return common.NewError(
			common.ErrCodeValidationInput,
			"EventType là bắt buộc và không được để trống",
			common.StatusBadRequest,
			nil,
		)
	}

	filter := bson.M{
		"eventType":            rule.EventType,
		"ownerOrganizationId":  rule.OwnerOrganizationID,
		"isActive":             true,
	}
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

	if rule.Domain != nil && *rule.Domain != "" {
		domainFilter := bson.M{
			"domain":              *rule.Domain,
			"ownerOrganizationId": rule.OwnerOrganizationID,
			"isActive":            true,
		}
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
func (s *NotificationRoutingService) InsertOne(ctx context.Context, data notifmodels.NotificationRoutingRule) (notifmodels.NotificationRoutingRule, error) {
	if err := s.ValidateUniqueness(ctx, data); err != nil {
		return data, err
	}
	return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}
