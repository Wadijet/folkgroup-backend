// Package service — Đọc lịch sử phân tích CIX theo session (lớp A cix_analysis_results), chuẩn tương tự GET …/intel-runs CRM.
package service

import (
	"context"
	"fmt"
	"strings"

	basemodels "meta_commerce/internal/api/base/models"
	cixmodels "meta_commerce/internal/api/cix/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ListAnalysisRunsBySession — phân trang lịch sử cix_analysis_results theo sessionUid (conversationId).
// Sort: causalOrderingAt, cixIntelSequence, _id — newestFirst đảo chiều (mặc true).
func (s *CixAnalysisService) ListAnalysisRunsBySession(ctx context.Context, ownerOrgID primitive.ObjectID, sessionUID string, page, limit int64, newestFirst bool) (*basemodels.PaginateResult[cixmodels.CixAnalysisResult], error) {
	sessionUID = strings.TrimSpace(sessionUID)
	if sessionUID == "" || ownerOrgID.IsZero() {
		return nil, fmt.Errorf("thiếu sessionUid hoặc ownerOrganizationId")
	}
	order := 1
	if newestFirst {
		order = -1
	}
	opts := options.Find().SetSort(bson.D{
		{Key: "causalOrderingAt", Value: order},
		{Key: "cixIntelSequence", Value: order},
		{Key: "_id", Value: order},
	})
	filter := bson.M{
		"sessionUid":          sessionUID,
		"ownerOrganizationId": ownerOrgID,
	}
	return s.FindWithPagination(ctx, filter, page, limit, opts)
}
