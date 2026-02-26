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
			return s.MergeFromPosCustomer(ctx, &doc) == nil
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
		Name:                      c.Name,
		PhoneNumbers:              c.PhoneNumbers,
		Emails:                    c.Emails,
		Birthday:                  c.Birthday,
		Gender:                    c.Gender,
		LivesIn:                   c.LivesIn,
		Addresses:                 c.Addresses,
		ReferralCode:              c.ReferralCode,
		HasConversation:           c.HasConversation,
		TotalSpent:                c.TotalSpent,
		OrderCount:                c.OrderCount,
		OrderCountOnline:          c.OrderCountOnline,
		OrderCountOffline:         c.OrderCountOffline,
		FirstOrderChannel:         c.FirstOrderChannel,
		LastOrderChannel:          c.LastOrderChannel,
		IsOmnichannel:             c.IsOmnichannel,
		LastOrderAt:               c.LastOrderAt,
		AvgOrderValue:             c.AvgOrderValue,
		CancelledOrderCount:       c.CancelledOrderCount,
		OrdersLast30d:             c.OrdersLast30d,
		OrdersLast90d:             c.OrdersLast90d,
		OrdersFromAds:             c.OrdersFromAds,
		OrdersFromOrganic:         c.OrdersFromOrganic,
		OrdersFromDirect:          c.OrdersFromDirect,
		OwnedSkuQuantities:        c.OwnedSkuQuantities,
		ConversationCount:         c.ConversationCount,
		ConversationCountByInbox:  c.ConversationCountByInbox,
		ConversationCountByComment: c.ConversationCountByComment,
		LastConversationAt:        c.LastConversationAt,
		FirstConversationAt:       c.FirstConversationAt,
		TotalMessages:             c.TotalMessages,
		LastMessageFromCustomer:   c.LastMessageFromCustomer,
		ConversationFromAds:       c.ConversationFromAds,
		ConversationTags:          c.ConversationTags,
		SourceIds: map[string]string{
			"pos": c.SourceIds.Pos,
			"fb":  c.SourceIds.Fb,
		},
		OwnerOrganizationId: c.OwnerOrganizationID,
	}
	// Phân loại 6 thành phần theo CUSTOMER_CLASSIFICATION_SYSTEM_DESIGN
	resp.ValueTier = ComputeValueTier(c.TotalSpent)
	resp.LifecycleStage = ComputeLifecycleStage(c.LastOrderAt)
	resp.JourneyStage = ComputeJourneyStage(c)
	resp.Channel = ComputeChannel(c)
	resp.LoyaltyStage = ComputeLoyaltyStage(c.OrderCount)
	resp.MomentumStage = ComputeMomentumStage(c)
	return resp
}
