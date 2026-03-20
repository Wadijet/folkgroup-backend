// Package crmvc - Resolve unifiedId từ customerId (order, conversation).
package crmvc

import (
	"context"
	"strings"

	crmmodels "meta_commerce/internal/api/crm/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GetCustomerIdForReply trả về customerId dùng để trả lời khách qua pageId.
// Dùng khi cần biết trả lời qua kênh nào (nhiều page FB, Zalo).
// Trả về "" nếu không có mapping cho pageId.
func GetCustomerIdForReply(c *crmmodels.CrmCustomer, pageId string) string {
	if c == nil || pageId == "" {
		return ""
	}
	if strings.HasPrefix(pageId, "pzl_") {
		if c.SourceIds.ZaloByPage != nil {
			if id, ok := c.SourceIds.ZaloByPage[pageId]; ok {
				return id
			}
		}
		return c.SourceIds.Zalo // fallback primary
	}
	if c.SourceIds.FbByPage != nil {
		if id, ok := c.SourceIds.FbByPage[pageId]; ok {
			return id
		}
	}
	return c.SourceIds.Fb // fallback primary
}

// ResolveUnifiedId tìm unifiedId từ customerId (có thể là pos, fb hoặc zalo).
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
			{"sourceIds.zalo": customerId},
			{"sourceIds.allInboxIds": customerId},
			{"unifiedId": customerId},
			{"uid": customerId},
		},
	}, nil)
	if err != nil {
		return "", false
	}
	return c.UnifiedId, true
}
