// Package crmvc - Service khách hàng CRM (crm_customers).
// Merge logic, metrics, profile.
package crmvc

import (
	"context"
	"errors"
	"fmt"

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

// GetProfile trả về profile đầy đủ của khách theo unifiedId.
// Nếu chưa có trong crm_customers, thử merge từ POS/FB rồi trả về.
func (s *CrmCustomerService) GetProfile(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID) (*crmdto.CrmCustomerProfileResponse, error) {
	filter := bson.M{
		"unifiedId":            unifiedId,
		"ownerOrganizationId": ownerOrgID,
	}
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
	return s.toProfileResponse(&customer), nil
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

// toProfileResponse chuyển CrmCustomer sang CrmCustomerProfileResponse.
func (s *CrmCustomerService) toProfileResponse(c *crmmodels.CrmCustomer) *crmdto.CrmCustomerProfileResponse {
	resp := &crmdto.CrmCustomerProfileResponse{
		UnifiedId:                 c.UnifiedId,
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
		SourceIds: map[string]string{
			"pos": c.SourceIds.Pos,
			"fb":  c.SourceIds.Fb,
		},
		OwnerOrganizationId: c.OwnerOrganizationID,
	}
	// Phân loại — ưu tiên từ top-level (denormalized), fallback compute
	if c.ValueTier != "" {
		resp.ValueTier = c.ValueTier
	} else {
		resp.ValueTier = ComputeValueTier(GetTotalSpentFromCustomer(c))
	}
	if c.LifecycleStage != "" {
		resp.LifecycleStage = c.LifecycleStage
	} else {
		resp.LifecycleStage = ComputeLifecycleStage(GetLastOrderAtFromCustomer(c))
	}
	if c.JourneyStage != "" {
		resp.JourneyStage = c.JourneyStage
	} else {
		resp.JourneyStage = ComputeJourneyStage(c)
	}
	if c.Channel != "" {
		resp.Channel = c.Channel
	} else {
		resp.Channel = ComputeChannel(c)
	}
	if c.LoyaltyStage != "" {
		resp.LoyaltyStage = c.LoyaltyStage
	} else {
		resp.LoyaltyStage = ComputeLoyaltyStage(GetOrderCountFromCustomer(c))
	}
	if c.MomentumStage != "" {
		resp.MomentumStage = c.MomentumStage
	} else {
		resp.MomentumStage = ComputeMomentumStage(c)
	}
	return resp
}
