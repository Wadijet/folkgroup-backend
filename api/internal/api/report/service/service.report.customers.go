// Package reportsvc - Customer Intelligence (Tab 4): helpers cho báo cáo khách hàng.
// Chỉ dùng hệ CRM (crm_activity_history, crm_customers). Legacy (pc_pos_*) đã bỏ.
package reportsvc

import (
	reportdto "meta_commerce/internal/api/report/dto"
)

// applyCustomersDefaults áp dụng giá trị mặc định cho CustomersQueryParams.
func applyCustomersDefaults(p *reportdto.CustomersQueryParams) {
	if p.Limit <= 0 {
		p.Limit = 20
	}
	if p.Limit > 2000 {
		p.Limit = 2000
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	if p.Period == "" {
		p.Period = "month"
	}
	if p.Filter == "" {
		p.Filter = "all"
	}
	if p.SortField == "" {
		p.SortField = "daysSinceLast"
	}
	if p.SortOrder != 1 && p.SortOrder != -1 {
		p.SortOrder = -1
	}
	if p.VipInactiveLimit <= 0 {
		p.VipInactiveLimit = 15
	}
	if p.VipInactiveLimit > 20 {
		p.VipInactiveLimit = 20
	}
	if p.ActiveDays <= 0 {
		p.ActiveDays = 30
	}
	if p.CoolingDays <= 0 {
		p.CoolingDays = 60
	}
	if p.InactiveDays <= 0 {
		p.InactiveDays = 90
	}
}
