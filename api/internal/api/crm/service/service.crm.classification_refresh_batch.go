// Package crmvc — Batch refresh phân loại (gọi từ AI Decision consumer hoặc tách từ ClassificationRefreshWorker).
package crmvc

import (
	"context"

	"github.com/sirupsen/logrus"
)

// RunClassificationRefreshBatch duyệt khách theo mode và gọi RefreshMetrics từng batch.
// Trả về tổng số khách đã RefreshMetrics thành công.
func (s *CrmCustomerService) RunClassificationRefreshBatch(ctx context.Context, log *logrus.Logger, mode string, batchSize int) (totalOK int) {
	if mode != ClassificationRefreshModeFull && mode != ClassificationRefreshModeSmart {
		mode = ClassificationRefreshModeSmart
	}
	if batchSize <= 0 {
		batchSize = 200
	}
	skip := 0
	for {
		list, err := s.ListCustomerIdsForClassificationRefresh(ctx, mode, batchSize, skip)
		if err != nil {
			if log != nil {
				log.WithError(err).Error("[CRM] RunClassificationRefreshBatch: lỗi lấy danh sách khách")
			}
			return totalOK
		}
		if len(list) == 0 {
			break
		}
		for _, c := range list {
			if err := s.RefreshMetrics(ctx, c.UnifiedId, c.OwnerOrgID); err != nil {
				if log != nil {
					log.WithError(err).WithFields(map[string]interface{}{
						"unifiedId": c.UnifiedId,
					}).Warn("[CRM] RunClassificationRefreshBatch: RefreshMetrics thất bại")
				}
				continue
			}
			totalOK++
		}
		if mode == ClassificationRefreshModeSmart || len(list) < batchSize {
			break
		}
		skip += batchSize
	}
	return totalOK
}
