// Package migration — Helper cho Rule Intelligence seed.
package migration

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"meta_commerce/internal/cta"
)

// GetSystemOrgIDForSeed lấy System Organization ID. Best-effort: nếu chưa có (ví dụ seed gọi trước InitRootOrganization) thì trả về NilObjectID.
func GetSystemOrgIDForSeed(ctx context.Context) primitive.ObjectID {
	id, err := cta.GetSystemOrganizationID(ctx)
	if err != nil {
		return primitive.NilObjectID
	}
	return id
}
