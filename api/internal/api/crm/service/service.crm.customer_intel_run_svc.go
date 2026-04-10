// Package crmvc — Đọc lịch sử intel khách (crm_customer_intel_runs) cho API / báo cáo.
package crmvc

import (
	"context"
	"fmt"
	"strings"

	basemodels "meta_commerce/internal/api/base/models"
	basesvc "meta_commerce/internal/api/base/service"
	crmmodels "meta_commerce/internal/api/crm/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CrmCustomerIntelRunService CRUD/đọc crm_customer_intel_runs.
type CrmCustomerIntelRunService struct {
	*basesvc.BaseServiceMongoImpl[crmmodels.CrmCustomerIntelRun]
}

// NewCrmCustomerIntelRunService tạo service cho collection crm_customer_intel_runs.
func NewCrmCustomerIntelRunService() (*CrmCustomerIntelRunService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CustomerIntelRuns)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.CustomerIntelRuns, common.ErrNotFound)
	}
	return &CrmCustomerIntelRunService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[crmmodels.CrmCustomerIntelRun](coll),
	}, nil
}

// ListIntelRunsByUnifiedID — phân trang lịch sử intel của một khách (chỉ bản ghi có unifiedId khớp; job đa khách org-wide không nằm trong danh sách này).
// newestFirst: true = mới nhất theo (causalOrderingAt, intelSequence, _id) lên trước.
func (s *CrmCustomerIntelRunService) ListIntelRunsByUnifiedID(ctx context.Context, ownerOrgID primitive.ObjectID, unifiedID string, page, limit int64, newestFirst bool) (*basemodels.PaginateResult[crmmodels.CrmCustomerIntelRun], error) {
	unifiedID = strings.TrimSpace(unifiedID)
	if unifiedID == "" || ownerOrgID.IsZero() {
		return nil, fmt.Errorf("thiếu unifiedId hoặc ownerOrganizationId")
	}
	order := 1
	if newestFirst {
		order = -1
	}
	opts := options.Find().SetSort(bson.D{
		{Key: "causalOrderingAt", Value: order},
		{Key: "intelSequence", Value: order},
		{Key: "_id", Value: order},
	})
	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"unifiedId":           unifiedID,
	}
	return s.FindWithPagination(ctx, filter, page, limit, opts)
}
