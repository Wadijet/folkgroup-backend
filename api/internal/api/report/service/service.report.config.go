// Package reportsvc - Config cho báo cáo report (tạm tắt chu kỳ nặng để giảm tải).
// Thống nhất: chỉ tính daily snapshot; weekly/monthly/yearly tính on-demand khi xem.
package reportsvc

// --- Customer report ---

// Các reportKey customer đầy đủ (khi không tắt chu kỳ nào).
var allCustomerReportKeys = []string{"customer_daily", "customer_weekly", "customer_monthly", "customer_yearly"}

// GetActiveCustomerReportKeys trả về danh sách customer report keys đang bật.
// Bỏ qua các chu kỳ trong customerReportKeysDisabled (fix cứng tạm thời).
func GetActiveCustomerReportKeys() []string {
	return filterActiveReportKeys(allCustomerReportKeys, getDisabledCustomerReportKeys())
}

// IsCustomerReportKeyDisabled trả về true nếu reportKey đang bị tắt (fix cứng).
func IsCustomerReportKeyDisabled(reportKey string) bool {
	return isReportKeyDisabled(reportKey, getDisabledCustomerReportKeys())
}

// GetDisabledCustomerReportKeys trả về danh sách reportKey bị tắt (để log).
func GetDisabledCustomerReportKeys() []string {
	return getDisabledCustomerReportKeys()
}

// customerReportKeysDisabled — fix cứng: chỉ tính daily; weekly/monthly/yearly tính on-demand khi xem.
var customerReportKeysDisabled = []string{
	"customer_weekly",
	"customer_monthly",
	"customer_yearly",
}

// getDisabledCustomerReportKeys trả về danh sách reportKey bị tắt.
func getDisabledCustomerReportKeys() []string {
	return customerReportKeysDisabled
}

// --- Order report ---

// Các reportKey order đầy đủ (khi không tắt chu kỳ nào).
var allOrderReportKeys = []string{"order_daily", "order_weekly", "order_monthly", "order_yearly"}

// GetActiveOrderReportKeys trả về danh sách order report keys đang bật.
// Chỉ order_daily; weekly/monthly/yearly tính on-demand từ daily khi xem.
func GetActiveOrderReportKeys() []string {
	return filterActiveReportKeys(allOrderReportKeys, getDisabledOrderReportKeys())
}

// IsOrderReportKeyDisabled trả về true nếu reportKey đang bị tắt (fix cứng).
func IsOrderReportKeyDisabled(reportKey string) bool {
	return isReportKeyDisabled(reportKey, getDisabledOrderReportKeys())
}

// GetDisabledOrderReportKeys trả về danh sách reportKey bị tắt (để log).
func GetDisabledOrderReportKeys() []string {
	return getDisabledOrderReportKeys()
}

// orderReportKeysDisabled — fix cứng: chỉ tính daily; weekly/monthly/yearly tính on-demand khi xem.
var orderReportKeysDisabled = []string{
	"order_weekly",
	"order_monthly",
	"order_yearly",
}

// getDisabledOrderReportKeys trả về danh sách reportKey bị tắt.
func getDisabledOrderReportKeys() []string {
	return orderReportKeysDisabled
}

// --- Ads report ---

// allAdsReportKeys danh sách report keys ads (ads_daily dùng custom engine).
var allAdsReportKeys = []string{"ads_daily"}

// GetActiveAdsReportKeys trả về danh sách ads report keys đang bật.
func GetActiveAdsReportKeys() []string {
	return filterActiveReportKeys(allAdsReportKeys, getDisabledAdsReportKeys())
}

// IsAdsReportKeyDisabled trả về true nếu reportKey đang bị tắt (fix cứng).
func IsAdsReportKeyDisabled(reportKey string) bool {
	return isReportKeyDisabled(reportKey, getDisabledAdsReportKeys())
}

// adsReportKeysDisabled — fix cứng: tạm thời không tắt ads_daily.
var adsReportKeysDisabled = []string{}

func getDisabledAdsReportKeys() []string {
	return adsReportKeysDisabled
}

// --- Helpers ---

func filterActiveReportKeys(all []string, disabled []string) []string {
	if len(disabled) == 0 {
		return all
	}
	disabledSet := make(map[string]bool)
	for _, k := range disabled {
		disabledSet[k] = true
	}
	out := make([]string, 0, len(all))
	for _, k := range all {
		if !disabledSet[k] {
			out = append(out, k)
		}
	}
	return out
}

func isReportKeyDisabled(reportKey string, disabled []string) bool {
	for _, k := range disabled {
		if k == reportKey {
			return true
		}
	}
	return false
}
