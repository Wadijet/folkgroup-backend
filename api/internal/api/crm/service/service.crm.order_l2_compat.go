// Package crmvc — map order_canonical (L2) sang view tương thích ingest dùng *PcPosOrder (chỉ field CRM cần).
package crmvc

import (
	ordermodels "meta_commerce/internal/api/order/models"
	pcmodels "meta_commerce/internal/api/pc/models"
)

// commerceOrderAsPosViewForIngest map CommerceOrder → PcPosOrder tối thiểu cho extractCustomerDataFromOrder / IngestOrderTouchpoint.
func commerceOrderAsPosViewForIngest(co *ordermodels.CommerceOrder) *pcmodels.PcPosOrder {
	if co == nil {
		return nil
	}
	return &pcmodels.PcPosOrder{
		OrderId:    co.OrderId,
		Status:     co.Status,
		PageId:     co.PageId,
		CustomerId: co.CustomerId,
		InsertedAt: co.InsertedAt,
		PosData:    co.PosData,
	}
}
