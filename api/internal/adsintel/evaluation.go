// Package adsintel — RecalculateForEntity, UpdateRawFromSource.
// Ủy quyền cho metasvc (Phase 1 chỉ Ad).
package adsintel

import (
	"context"

	metasvc "meta_commerce/internal/api/meta/service"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RecalculateForEntity tính lại currentMetrics cho 1 entity. Bottom-up: Ad tính từ nguồn; parent aggregate từ con.
// Phase 1: Ủy quyền metasvc (chỉ hỗ trợ ad).
func RecalculateForEntity(ctx context.Context, objectType, objectId, adAccountId string, ownerOrgID primitive.ObjectID) error {
	return metasvc.RecalculateForEntity(ctx, objectType, objectId, adAccountId, ownerOrgID)
}

// UpdateRawFromSource chỉ cập nhật raw từ 1 nguồn, giữ nguyên raw khác, rồi tính lại layers.
func UpdateRawFromSource(ctx context.Context, objectType, objectId, adAccountId string, ownerOrgID primitive.ObjectID, source string) error {
	return metasvc.UpdateRawFromSource(ctx, objectType, objectId, adAccountId, ownerOrgID, source)
}
