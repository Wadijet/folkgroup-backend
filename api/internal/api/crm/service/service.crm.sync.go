// Package crmvc - Sync initial crm_customers từ pc_pos_customers và fb_customers.
package crmvc

import (
	"context"
	"fmt"

	fbmodels "meta_commerce/internal/api/fb/models"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SyncAllCustomers đồng bộ toàn bộ khách từ POS và FB vào crm_customers.
// Gọi khi cần sync lần đầu hoặc rebuild.
func (s *CrmCustomerService) SyncAllCustomers(ctx context.Context, ownerOrgID primitive.ObjectID) (posCount, fbCount int, err error) {
	posColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosCustomers)
	if !ok {
		return 0, 0, fmt.Errorf("không tìm thấy pc_pos_customers")
	}
	fbColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbCustomers)
	if !ok {
		return 0, 0, fmt.Errorf("không tìm thấy fb_customers")
	}

	// Sync POS trước (ưu tiên vì có metrics)
	posCursor, err := posColl.Find(ctx, bson.M{"ownerOrganizationId": ownerOrgID}, nil)
	if err != nil {
		return 0, 0, err
	}
	defer posCursor.Close(ctx)
	for posCursor.Next(ctx) {
		var doc pcmodels.PcPosCustomer
		if posCursor.Decode(&doc) == nil {
			if e := s.MergeFromPosCustomer(ctx, &doc); e == nil {
				posCount++
			}
		}
	}

	// Sync FB
	fbCursor, err := fbColl.Find(ctx, bson.M{"ownerOrganizationId": ownerOrgID}, nil)
	if err != nil {
		return posCount, 0, err
	}
	defer fbCursor.Close(ctx)
	for fbCursor.Next(ctx) {
		var doc fbmodels.FbCustomer
		if fbCursor.Decode(&doc) == nil {
			if e := s.MergeFromFbCustomer(ctx, &doc); e == nil {
				fbCount++
			}
		}
	}

	return posCount, fbCount, nil
}
