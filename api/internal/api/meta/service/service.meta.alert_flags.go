// Package metasvc - computeAlertFlags: tính tất cả tiêu chí cảnh báo từ raw, layer1, layer2, layer3.
// Theo FolkForm AI Agent Master Rules v4.1. Lưu vào currentMetrics.alertFlags (field riêng).
// Chưa gửi notification; chỉ lưu trạng thái để UI hiển thị.
package metasvc

// Ngưỡng theo Master Rules (đơn vị VND, %)
const (
	CPA_MESS_KILL_VND      = 180_000
	CPA_PURCHASE_HARD_STOP = 1_050_000
	CONV_RATE_MESS_TRAP    = 0.05
	CONV_RATE_MESS_TRAP_6  = 0.06
	CTR_KILL               = 0.0035
	MSG_RATE_LOW           = 0.02
	CPM_MESS_TRAP_LOW      = 60_000
	CPM_HIGH               = 180_000
	CPM_KO_C_MULTIPLIER    = 2.5
	FREQUENCY_HIGH         = 3.0
	FREQUENCY_TRIM         = 2.2
	MESS_TRAP_SUSPECT_MIN  = 20
	MESS_TRAP_SL_D_MIN     = 15
	CTR_TRAFFIC_RAC        = 0.018
	CHS_WARNING_THRESHOLD  = 40
	SAFETY_NET_ORDERS_MIN  = 3
	SAFETY_NET_CR_MIN      = 0.10
	SL_E_ORDERS_MIN        = 3
	SL_E_CR_MAX            = 0.10
)

// computeAlertFlags tính danh sách flags cảnh báo từ metrics.
// Trả về []string — các mã flag đang trigger (vd: chs_critical, mess_trap_suspect).
func computeAlertFlags(raw, layer1, layer2, layer3 map[string]interface{}) []string {
	var flags []string

	meta, _ := raw["meta"].(map[string]interface{})
	pancake, _ := raw["pancake"].(map[string]interface{})
	pos, _ := mapOrNil(pancake, "pos").(map[string]interface{})

	spend := toFloat(meta, "spend")
	mess := toInt64(meta, "mess")
	impressions := toInt64(meta, "impressions")
	cpm := toFloat(meta, "cpm")
	ctr := toFloat(meta, "ctr")
	frequency := toFloat(meta, "frequency")
	orders := toInt64(pos, "orders")

	msgRate := toFloat(layer1, "msgRate")
	cpaMess := toFloat(layer1, "cpaMess")
	cpaPurchase := toFloat(layer1, "cpaPurchase")
	convRate := toFloat(layer1, "convRate")

	chs := toFloat(layer3, "chs")
	healthState, _ := layer3["healthState"].(string)
	portfolioCell, _ := layer3["portfolioCell"].(string)

	// --- CHS & Health ---
	if healthState == "critical" || chs < CHS_WARNING_THRESHOLD {
		flags = append(flags, "chs_critical")
	}
	if healthState == "warning" || (chs >= CHS_WARNING_THRESHOLD && chs < 60) {
		flags = append(flags, "chs_warning")
	}

	// --- CPA / Conv ---
	if cpaMess > CPA_MESS_KILL_VND && mess > 0 {
		flags = append(flags, "cpa_mess_high")
	}
	if cpaPurchase > CPA_PURCHASE_HARD_STOP && orders > 0 {
		flags = append(flags, "cpa_purchase_high")
	}
	if convRate > 0 && convRate < CONV_RATE_MESS_TRAP && mess >= MESS_TRAP_SL_D_MIN {
		flags = append(flags, "conv_rate_low")
	}

	// --- CTR / MsgRate ---
	if ctr > 0 && ctr < CTR_KILL {
		flags = append(flags, "ctr_critical")
	}
	if msgRate > 0 && msgRate < MSG_RATE_LOW {
		flags = append(flags, "msg_rate_low")
	}

	// --- CPM ---
	if cpm > 0 && cpm < CPM_MESS_TRAP_LOW {
		flags = append(flags, "cpm_low")
	}
	if cpm > CPM_HIGH {
		flags = append(flags, "cpm_high")
	}

	// --- Frequency ---
	if frequency > FREQUENCY_HIGH {
		flags = append(flags, "frequency_high")
	}

	// --- Mess Trap Suspect ---
	if cpm < CPM_MESS_TRAP_LOW && convRate < CONV_RATE_MESS_TRAP_6 &&
		mess >= MESS_TRAP_SUSPECT_MIN && orders == 0 {
		flags = append(flags, "mess_trap_suspect")
	}

	// --- Stop Loss SL-A ---
	if cpaMess > CPA_MESS_KILL_VND && mess < 3 {
		flags = append(flags, "sl_a")
	}

	// --- Stop Loss SL-B ---
	if spend > 0 && mess == 0 {
		flags = append(flags, "sl_b")
	}

	// --- Stop Loss SL-C ---
	if ctr > 0 && ctr < CTR_KILL && cpm > CPM_HIGH {
		flags = append(flags, "sl_c")
	}

	// --- Stop Loss SL-D ---
	if mess >= MESS_TRAP_SL_D_MIN && convRate < CONV_RATE_MESS_TRAP && spend > 0 {
		flags = append(flags, "sl_d")
	}

	// --- Stop Loss SL-E ---
	if cpaPurchase > CPA_PURCHASE_HARD_STOP && orders >= SL_E_ORDERS_MIN && convRate < SL_E_CR_MAX {
		flags = append(flags, "sl_e")
	}

	// --- Kill Off KO-B ---
	if ctr > CTR_TRAFFIC_RAC && msgRate < MSG_RATE_LOW && orders == 0 && spend > 0 {
		flags = append(flags, "ko_b")
	}

	// --- Kill Off KO-C ---
	if cpm > CPM_HIGH*CPM_KO_C_MULTIPLIER && impressions < 800 {
		flags = append(flags, "ko_c")
	}

	// --- Trim eligible ---
	if frequency > FREQUENCY_TRIM && chs < 60 {
		flags = append(flags, "trim_eligible")
	}

	// --- Safety Net ---
	if orders >= SAFETY_NET_ORDERS_MIN && convRate >= SAFETY_NET_CR_MIN && chs >= 60 {
		flags = append(flags, "safety_net")
	}

	// --- Portfolio attention ---
	if portfolioCell == "fix" || portfolioCell == "recover" {
		flags = append(flags, "portfolio_attention")
	}

	// --- Conv rate strong (exception) ---
	if convRate >= 0.20 {
		flags = append(flags, "conv_rate_strong")
	}

	// --- Diagnoses ---
	if diag, ok := layer3["diagnoses"].([]interface{}); ok && len(diag) > 0 {
		for _, d := range diag {
			if s, ok := d.(string); ok && s != "" {
				flags = append(flags, "diagnosis_"+s)
			}
		}
	}

	return dedupeStrings(flags)
}

// dedupeStrings loại bỏ trùng lặp, giữ thứ tự.
func dedupeStrings(ss []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
