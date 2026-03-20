// Package crmvc - Service khách hàng CRM (crm_customers).
// Merge logic, metrics, profile.
package crmvc

import (
	"context"
	"errors"
	"fmt"
	"time"

	crmdto "meta_commerce/internal/api/crm/dto"
	crmmodels "meta_commerce/internal/api/crm/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	fbmodels "meta_commerce/internal/api/fb/models"
	pcmodels "meta_commerce/internal/api/pc/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CrmCustomerService xử lý logic khách hàng unified.
type CrmCustomerService struct {
	*basesvc.BaseServiceMongoImpl[crmmodels.CrmCustomer]
}

// NewCrmCustomerService tạo CrmCustomerService mới.
func NewCrmCustomerService() (*CrmCustomerService, error) {
	coll, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.CrmCustomers)
	if !exist {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.CrmCustomers, common.ErrNotFound)
	}
	return &CrmCustomerService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[crmmodels.CrmCustomer](coll),
	}, nil
}

// GetProfile trả về profile đầy đủ của khách theo unifiedId hoặc uid.
// Nếu chưa có trong crm_customers, thử merge từ POS/FB rồi trả về.
func (s *CrmCustomerService) GetProfile(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID) (*crmdto.CrmCustomerProfileResponse, error) {
	filter := buildCustomerFilterByIdOrUid(unifiedId, ownerOrgID)
	customer, err := s.FindOne(ctx, filter, nil)
	if err != nil {
		// Thử merge từ nguồn (POS hoặc FB) nếu chưa có
		if errors.Is(err, common.ErrNotFound) {
			if s.tryMergeFromSource(ctx, unifiedId, ownerOrgID) {
				customer, err = s.FindOne(ctx, filter, nil)
			}
		}
		if err != nil {
			return nil, err
		}
	}
	return s.toProfileResponse(ctx, &customer), nil
}

// tryMergeFromSource thử merge từ POS hoặc FB customer. Trả về true nếu đã merge thành công.
func (s *CrmCustomerService) tryMergeFromSource(ctx context.Context, customerId string, ownerOrgID primitive.ObjectID) bool {
	posColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosCustomers)
	if ok {
		var doc pcmodels.PcPosCustomer
		if posColl.FindOne(ctx, bson.M{"customerId": customerId, "ownerOrganizationId": ownerOrgID}).Decode(&doc) == nil {
			return s.MergeFromPosCustomer(ctx, &doc, 0) == nil
		}
	}
	fbColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbCustomers)
	if ok {
		var doc fbmodels.FbCustomer
		if fbColl.FindOne(ctx, bson.M{"customerId": customerId, "ownerOrganizationId": ownerOrgID}).Decode(&doc) == nil {
			return s.MergeFromFbCustomer(ctx, &doc, 0) == nil
		}
	}
	return false
}

// OnCixSignalUpdate cập nhật Layer 3 signals (buyingIntent, sentiment) từ CIX vào crm_customers.
// Làm giàu profile — psychographic tags, intent signals. Lưu trong currentMetrics.cix.
func (s *CrmCustomerService) OnCixSignalUpdate(ctx context.Context, customerUid string, ownerOrgID primitive.ObjectID, buyingIntent, sentiment, objectionLevel string, traceID string) error {
	if customerUid == "" {
		return nil
	}
	now := time.Now().UnixMilli()
	update := bson.M{
		"$set": bson.M{
			"currentMetrics.cix": bson.M{
				"buyingIntent":   buyingIntent,
				"sentiment":      sentiment,
				"objectionLevel": objectionLevel,
				"updatedAt":      now,
				"traceId":        traceID,
			},
			"updatedAt": now,
		},
	}
	filter := buildCustomerFilterByIdOrUid(customerUid, ownerOrgID)
	_, err := s.Collection().UpdateOne(ctx, filter, update)
	return err
}

// buildCustomerFilterByIdOrUid tạo filter lookup customer theo uid hoặc unifiedId (ưu tiên mới, fallback cũ).
func buildCustomerFilterByIdOrUid(idOrUid string, ownerOrgID primitive.ObjectID) bson.M {
	return bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$or": []bson.M{
			{"uid": idOrUid},
			{"unifiedId": idOrUid},
		},
	}
}

// toProfileResponse chuyển CrmCustomer sang CrmCustomerProfileResponse.
func (s *CrmCustomerService) toProfileResponse(ctx context.Context, c *crmmodels.CrmCustomer) *crmdto.CrmCustomerProfileResponse {
	resp := &crmdto.CrmCustomerProfileResponse{
		UnifiedId:                 c.UnifiedId,
		Uid:                       c.Uid,
		Name:                      GetNameFromCustomer(c),
		PhoneNumbers:              GetPhoneNumbersFromCustomer(c),
		Emails:                    GetEmailsFromCustomer(c),
		Birthday:                  GetBirthdayFromCustomer(c),
		Gender:                    GetGenderFromCustomer(c),
		LivesIn:                   GetLivesInFromCustomer(c),
		Addresses:                 GetAddressesFromCustomer(c),
		ReferralCode:              GetReferralCodeFromCustomer(c),
		HasConversation:           GetBoolFromCustomer(c, "hasConversation"),
		TotalSpent:                GetTotalSpentFromCustomer(c),
		OrderCount:                GetOrderCountFromCustomer(c),
		OrderCountOnline:          GetIntFromCustomer(c, "orderCountOnline"),
		OrderCountOffline:         GetIntFromCustomer(c, "orderCountOffline"),
		FirstOrderChannel:         getStrFromCustomer(c, "firstOrderChannel"),
		LastOrderChannel:          getStrFromCustomer(c, "lastOrderChannel"),
		IsOmnichannel:             GetIntFromCustomer(c, "orderCountOnline") > 0 && GetIntFromCustomer(c, "orderCountOffline") > 0,
		LastOrderAt:               GetLastOrderAtFromCustomer(c),
		AvgOrderValue:             GetFloatFromCustomer(c, "avgOrderValue"),
		CancelledOrderCount:       GetIntFromCustomer(c, "cancelledOrderCount"),
		OrdersLast30d:             GetIntFromCustomer(c, "ordersLast30d"),
		OrdersLast90d:             GetIntFromCustomer(c, "ordersLast90d"),
		OrdersFromAds:             GetIntFromCustomer(c, "ordersFromAds"),
		OrdersFromOrganic:         GetIntFromCustomer(c, "ordersFromOrganic"),
		OrdersFromDirect:          GetIntFromCustomer(c, "ordersFromDirect"),
		OwnedSkuQuantities:        c.OwnedSkuQuantities,
		ConversationCount:         GetIntFromCustomer(c, "conversationCount"),
		ConversationCountByInbox:  GetIntFromCustomer(c, "conversationCountByInbox"),
		ConversationCountByComment: GetIntFromCustomer(c, "conversationCountByComment"),
		LastConversationAt:        GetInt64FromCustomer(c, "lastConversationAt"),
		FirstConversationAt:       GetInt64FromCustomer(c, "firstConversationAt"),
		TotalMessages:             GetIntFromCustomer(c, "totalMessages"),
		LastMessageFromCustomer:   GetBoolFromCustomer(c, "lastMessageFromCustomer"),
		ConversationFromAds:       GetBoolFromCustomer(c, "conversationFromAds"),
		ConversationTags:          c.ConversationTags,
		SourceIds: map[string]interface{}{
			"pos":        c.SourceIds.Pos,
			"fb":         c.SourceIds.Fb,
			"zalo":       c.SourceIds.Zalo,
			"fbByPage":   c.SourceIds.FbByPage,
			"zaloByPage": c.SourceIds.ZaloByPage,
		},
		OwnerOrganizationId: c.OwnerOrganizationID,
	}
	// Phân loại — ưu tiên từ top-level (denormalized), else Rule Engine qua GetClassificationFromCustomer
	class := GetClassificationFromCustomer(ctx, c)
	setFromClass := func(top string, key string) string {
		if top != "" {
			return top
		}
		return getStrFromMap(class, key)
	}
	resp.ValueTier = setFromClass(c.ValueTier, "valueTier")
	resp.LifecycleStage = setFromClass(c.LifecycleStage, "lifecycleStage")
	resp.JourneyStage = setFromClass(c.JourneyStage, "journeyStage")
	resp.Channel = setFromClass(c.Channel, "channel")
	resp.LoyaltyStage = setFromClass(c.LoyaltyStage, "loyaltyStage")
	resp.MomentumStage = setFromClass(c.MomentumStage, "momentumStage")
	return resp
}
