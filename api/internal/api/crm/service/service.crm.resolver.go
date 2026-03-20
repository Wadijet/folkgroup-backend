// Package crmvc — Resolver cho identity (external id → uid).
package crmvc

import (
	"context"

	"meta_commerce/internal/utility"
	"meta_commerce/internal/utility/identity"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CrmResolver implement identity.Resolver — resolve external customer id → uid (cust_xxx).
type CrmResolver struct {
	*CrmCustomerService
}

// ResolveToUid tìm customer theo external id và trả về uid (cust_ + _id.Hex()).
func (r *CrmResolver) ResolveToUid(ctx context.Context, externalId string, source string, ownerOrgID primitive.ObjectID) (string, bool) {
	if externalId == "" {
		return "", false
	}
	c, err := r.FindOne(ctx, bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$or": []bson.M{
			{"sourceIds.pos": externalId},
			{"sourceIds.fb": externalId},
			{"sourceIds.zalo": externalId},
			{"sourceIds.allInboxIds": externalId},
			{"unifiedId": externalId},
			{"uid": externalId},
		},
	}, nil)
	if err != nil {
		return "", false
	}
	// uid = prefix + _id.Hex() (chuẩn 4 lớp)
	return utility.UIDFromObjectID(utility.UIDPrefixCustomer, c.ID), true
}

// Đảm bảo CrmResolver implement identity.Resolver.
var _ identity.Resolver = (*CrmResolver)(nil)
