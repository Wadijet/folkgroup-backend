package datachanged

import (
	"time"

	"meta_commerce/internal/global"
)

// CustomerIntelTrailingDebounce — cửa sổ trailing tính lại CRM intelligence (customer); Realtime / rule → 0.
const CustomerIntelTrailingDebounce = 10 * time.Minute

// IsCustomerIntelligenceSourceCollection — nguồn datachanged làm thay đổi chỉ số / intelligence gắn khách.
func IsCustomerIntelligenceSourceCollection(src string) bool {
	switch src {
	case global.MongoDB_ColNames.FbCustomers,
		global.MongoDB_ColNames.PcPosCustomers,
		global.MongoDB_ColNames.FbConvesations,
		global.MongoDB_ColNames.PcPosOrders,
		global.MongoDB_ColNames.CustomerCustomers:
		return true
	default:
		return false
	}
}
