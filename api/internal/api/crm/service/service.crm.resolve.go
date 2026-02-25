// Package crmvc - Resolve unifiedId từ customerId (order, conversation).
package crmvc

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ResolveUnifiedId tìm unifiedId từ customerId (có thể là pos hoặc fb).
// Trả về ("", false) nếu không tìm thấy.
func (s *CrmCustomerService) ResolveUnifiedId(ctx context.Context, customerId string, ownerOrgID primitive.ObjectID) (string, bool) {
	if customerId == "" {
		return "", false
	}
	c, err := s.FindOne(ctx, bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$or": []bson.M{
			{"sourceIds.pos": customerId},
			{"sourceIds.fb": customerId},
			{"unifiedId": customerId},
		},
	}, nil)
	if err != nil {
		return "", false
	}
	return c.UnifiedId, true
}
