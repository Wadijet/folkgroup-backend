package cta

import (
	"context"
	"fmt"

	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GetSystemOrganizationID lấy System Organization ID
// System Organization có: level=-1, code="SYSTEM", type=OrganizationTypeSystem
func GetSystemOrganizationID(ctx context.Context) (primitive.ObjectID, error) {
	orgService, err := services.NewOrganizationService()
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to create organization service: %v", err)
	}

	systemFilter := bson.M{
		"level": -1,
		"code":  "SYSTEM",
		"type":  models.OrganizationTypeSystem,
	}

	systemOrg, err := orgService.FindOne(ctx, systemFilter, nil)
	if err != nil {
		if err == common.ErrNotFound {
			return primitive.NilObjectID, fmt.Errorf("System Organization not found. Please run init first")
		}
		return primitive.NilObjectID, fmt.Errorf("failed to find System Organization: %v", err)
	}

	return systemOrg.ID, nil
}
