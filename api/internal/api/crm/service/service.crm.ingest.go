// Package crmvc - Hàm trung tâm IngestCustomerTouchpoint.
// Hook và job backfill đều đẩy dữ liệu qua các hàm này.
package crmvc

import (
	"context"

	fbmodels "meta_commerce/internal/api/fb/models"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// IngestOrderTouchpoint xử lý order: resolve unifiedId, refresh metrics, log activity.
// skipIfExists=true dùng cho backfill để tránh ghi trùng.
func (s *CrmCustomerService) IngestOrderTouchpoint(ctx context.Context, customerId string, ownerOrgID primitive.ObjectID, orderId int64, isUpdate bool, channel string, skipIfExists bool) error {
	if customerId == "" {
		return nil
	}
	unifiedId, found := s.ResolveUnifiedId(ctx, customerId, ownerOrgID)
	if !found {
		posColl, _ := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosCustomers)
		if posColl != nil {
			var posCustomer pcmodels.PcPosCustomer
			if posColl.FindOne(ctx, bson.M{"customerId": customerId, "ownerOrganizationId": ownerOrgID}).Decode(&posCustomer) == nil {
				_ = s.MergeFromPosCustomer(ctx, &posCustomer)
				unifiedId, found = s.ResolveUnifiedId(ctx, customerId, ownerOrgID)
			}
		}
	}
	if !found || unifiedId == "" {
		return nil
	}
	_ = s.RefreshMetrics(ctx, unifiedId, ownerOrgID)
	activityType := "order_created"
	if isUpdate {
		activityType = "order_completed"
	}
	sourceRef := map[string]interface{}{"orderId": orderId}
	metadata := map[string]interface{}{"channel": channel}
	actSvc, err := NewCrmActivityService()
	if err != nil {
		return err
	}
	if skipIfExists {
		_, _ = actSvc.LogActivityIfNotExists(ctx, unifiedId, ownerOrgID, activityType, "pos", sourceRef, metadata)
	} else {
		_ = actSvc.LogActivity(ctx, unifiedId, ownerOrgID, activityType, "pos", sourceRef, metadata)
	}
	return nil
}

// IngestConversationTouchpoint xử lý conversation: resolve unifiedId, refresh metrics, log activity.
func (s *CrmCustomerService) IngestConversationTouchpoint(ctx context.Context, customerId string, ownerOrgID primitive.ObjectID, conversationId string, skipIfExists bool) error {
	if customerId == "" {
		return nil
	}
	unifiedId, found := s.ResolveUnifiedId(ctx, customerId, ownerOrgID)
	if !found {
		fbColl, _ := global.RegistryCollections.Get(global.MongoDB_ColNames.FbCustomers)
		if fbColl != nil {
			var fbCustomer fbmodels.FbCustomer
			if fbColl.FindOne(ctx, bson.M{"customerId": customerId, "ownerOrganizationId": ownerOrgID}).Decode(&fbCustomer) == nil {
				_ = s.MergeFromFbCustomer(ctx, &fbCustomer)
				unifiedId, found = s.ResolveUnifiedId(ctx, customerId, ownerOrgID)
			}
		}
	}
	if !found || unifiedId == "" {
		return nil
	}
	_ = s.RefreshMetrics(ctx, unifiedId, ownerOrgID)
	sourceRef := map[string]interface{}{"conversationId": conversationId}
	actSvc, err := NewCrmActivityService()
	if err != nil {
		return err
	}
	if skipIfExists {
		_, _ = actSvc.LogActivityIfNotExists(ctx, unifiedId, ownerOrgID, "conversation_started", "fb", sourceRef, nil)
	} else {
		_ = actSvc.LogActivity(ctx, unifiedId, ownerOrgID, "conversation_started", "fb", sourceRef, nil)
	}
	return nil
}

// IngestNoteTouchpoint xử lý note: log activity (customerId đã là unifiedId).
func (s *CrmCustomerService) IngestNoteTouchpoint(ctx context.Context, customerId string, ownerOrgID primitive.ObjectID, noteId string, skipIfExists bool) error {
	if customerId == "" || noteId == "" {
		return nil
	}
	actSvc, err := NewCrmActivityService()
	if err != nil {
		return err
	}
	sourceRef := map[string]interface{}{"noteId": noteId}
	if skipIfExists {
		_, _ = actSvc.LogActivityIfNotExists(ctx, customerId, ownerOrgID, "note_added", "system", sourceRef, nil)
	} else {
		_ = actSvc.LogActivity(ctx, customerId, ownerOrgID, "note_added", "system", sourceRef, nil)
	}
	return nil
}
