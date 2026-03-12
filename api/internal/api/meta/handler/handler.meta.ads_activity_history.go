// Package metahdl - Handler cho Meta Ads Activity History (lịch sử hoạt động).
package metahdl

import (
	"fmt"

	basehdl "meta_commerce/internal/api/base/handler"
	metamodels "meta_commerce/internal/api/meta/models"
	metasvc "meta_commerce/internal/api/meta/service"
)

// MetaAdsActivityHistoryHandler xử lý request lịch sử hoạt động Meta Ads.
// Chỉ hỗ trợ đọc (find, find-one, find-by-id, paginate, count) — dữ liệu được ghi tự động bởi hệ thống.
type MetaAdsActivityHistoryHandler struct {
	*basehdl.BaseHandler[metamodels.AdsActivityHistory, metamodels.AdsActivityHistory, metamodels.AdsActivityHistory]
}

// NewMetaAdsActivityHistoryHandler tạo MetaAdsActivityHistoryHandler.
func NewMetaAdsActivityHistoryHandler() (*MetaAdsActivityHistoryHandler, error) {
	svc, err := metasvc.NewMetaAdsActivityHistoryService()
	if err != nil {
		return nil, fmt.Errorf("tạo MetaAdsActivityHistoryService: %w", err)
	}
	hdl := &MetaAdsActivityHistoryHandler{
		BaseHandler: basehdl.NewBaseHandler[metamodels.AdsActivityHistory, metamodels.AdsActivityHistory, metamodels.AdsActivityHistory](svc),
	}
	hdl.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields:     []string{},
		AllowedOperators: []string{"$eq", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$exists"},
		MaxFields:        10,
	})
	return hdl, nil
}
