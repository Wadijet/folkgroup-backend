// Package metahdl - Helper chung cho Meta handlers.
package metahdl

import (
	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// resolveOwnerOrgIDFromCtx lấy ownerOrganizationId: ưu tiên từ body, sau đó từ context (active_organization_id).
func resolveOwnerOrgIDFromCtx(c fiber.Ctx, fromBody string) primitive.ObjectID {
	if fromBody != "" {
		if oid, err := primitive.ObjectIDFromHex(fromBody); err == nil {
			return oid
		}
	}
	if orgIDStr, ok := c.Locals("active_organization_id").(string); ok && orgIDStr != "" {
		if oid, err := primitive.ObjectIDFromHex(orgIDStr); err == nil {
			return oid
		}
	}
	return primitive.NilObjectID
}
