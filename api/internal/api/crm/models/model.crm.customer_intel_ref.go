package models

// CrmCustomerIntelRef — pointer tới lần chạy intel gần nhất (đọc chi tiết tại crm_customer_intel_runs).
type CrmCustomerIntelRef struct {
	LastRunHex     string `json:"lastRunHex,omitempty" bson:"lastRunHex,omitempty"`
	LastComputedAt int64  `json:"lastComputedAt,omitempty" bson:"lastComputedAt,omitempty"`
	LastOperation  string `json:"lastOperation,omitempty" bson:"lastOperation,omitempty"`
}
