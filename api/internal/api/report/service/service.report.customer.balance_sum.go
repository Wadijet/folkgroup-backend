// Package reportsvc - Cộng phát sinh từ snapshots thành số dư (in - out).
package reportsvc

import (
	reportmodels "meta_commerce/internal/api/report/models"
)

// sumPhatSinhToBalance cộng phát sinh (in - out) từ nhiều snapshot thành số dư cuối kỳ.
// Số dư đầu kỳ = 0. Cấu trúc trả về: raw, layer1, layer2, layer3 (giống buildPeriodEndBalance).
func sumPhatSinhToBalance(snapshots []reportmodels.ReportSnapshot) map[string]interface{} {
	if len(snapshots) == 0 {
		return emptyBalance()
	}

	rawIn := map[string]float64{"totalCustomers": 0, "newCustomersInPeriod": 0, "activeInPeriod": 0, "reactivationValue": 0, "totalLTV": 0}
	rawOut := map[string]float64{"totalCustomers": 0, "reactivationValue": 0, "totalLTV": 0}

	layer1JourneyIn := make(map[string]int64)
	layer1JourneyOut := make(map[string]int64)

	layer2Dims := []string{"valueTier", "lifecycleStage", "channel", "loyaltyStage", "momentumStage", "ceoGroup"}
	layer2LTVDims := []string{"valueTierLTV", "lifecycleStageLTV", "channelLTV", "loyaltyStageLTV", "momentumStageLTV", "ceoGroupLTV"}
	layer2In := make(map[string]map[string]int64)
	layer2Out := make(map[string]map[string]int64)
	layer2LTVIn := make(map[string]map[string]float64)
	layer2LTVOut := make(map[string]map[string]float64)
	for _, d := range layer2Dims {
		layer2In[d] = make(map[string]int64)
		layer2Out[d] = make(map[string]int64)
	}
	for _, d := range layer2LTVDims {
		layer2LTVIn[d] = make(map[string]float64)
		layer2LTVOut[d] = make(map[string]float64)
	}

	layer3Groups := []string{"first", "repeat", "vip", "inactive", "engaged"}
	layer3In := make(map[string]map[string]map[string]int64)
	layer3Out := make(map[string]map[string]map[string]int64)
	for _, g := range layer3Groups {
		layer3In[g] = make(map[string]map[string]int64)
		layer3Out[g] = make(map[string]map[string]int64)
	}

	for _, snap := range snapshots {
		m := snap.Metrics
		if m == nil {
			continue
		}
		accumulatePhatSinh(m, &rawIn, &rawOut, layer1JourneyIn, layer1JourneyOut,
			layer2In, layer2Out, layer2LTVIn, layer2LTVOut, layer3In, layer3Out)
	}

	return buildBalanceFromAccum(rawIn, rawOut, layer1JourneyIn, layer1JourneyOut,
		layer2In, layer2Out, layer2LTVIn, layer2LTVOut, layer3In, layer3Out)
}

func emptyBalance() map[string]interface{} {
	return map[string]interface{}{
		"raw":    map[string]interface{}{"totalCustomers": int64(0), "activeInPeriod": int64(0), "reactivationValue": 0.0, "totalLTV": 0.0},
		"layer1": map[string]interface{}{"journeyStage": map[string]interface{}{}},
		"layer2": map[string]interface{}{
			"valueTier": map[string]interface{}{}, "lifecycleStage": map[string]interface{}{}, "channel": map[string]interface{}{},
			"loyaltyStage": map[string]interface{}{}, "momentumStage": map[string]interface{}{}, "ceoGroup": map[string]interface{}{},
			"valueTierLTV": map[string]interface{}{}, "lifecycleStageLTV": map[string]interface{}{}, "channelLTV": map[string]interface{}{},
			"loyaltyStageLTV": map[string]interface{}{}, "momentumStageLTV": map[string]interface{}{}, "ceoGroupLTV": map[string]interface{}{},
		},
		"layer3": map[string]interface{}{
			"first": map[string]interface{}{}, "repeat": map[string]interface{}{}, "vip": map[string]interface{}{},
			"inactive": map[string]interface{}{}, "engaged": map[string]interface{}{},
		},
	}
}

func accumulatePhatSinh(m map[string]interface{},
	rawIn, rawOut *map[string]float64,
	layer1In, layer1Out map[string]int64,
	layer2In, layer2Out map[string]map[string]int64,
	layer2LTVIn, layer2LTVOut map[string]map[string]float64,
	layer3In, layer3Out map[string]map[string]map[string]int64) {

	// raw — cấu trúc mới: mỗi metric có { in, out } trong cùng nhóm
	if raw, ok := m["raw"].(map[string]interface{}); ok {
		readInOut := func(key string) {
			if v, ok := raw[key].(map[string]interface{}); ok {
				if x := v["in"]; x != nil {
					(*rawIn)[key] += toFloat64Balance(x)
				}
				if x := v["out"]; x != nil {
					(*rawOut)[key] += toFloat64Balance(x)
				}
			}
		}
		readInOut("totalCustomers")
		readInOut("reactivationValue")
		readInOut("totalLTV")
		if v, ok := raw["newCustomersInPeriod"].(map[string]interface{}); ok && v["in"] != nil {
			(*rawIn)["newCustomersInPeriod"] += toFloat64Balance(v["in"])
		}
		if v, ok := raw["activeInPeriod"].(map[string]interface{}); ok && v["in"] != nil {
			(*rawIn)["activeInPeriod"] += toFloat64Balance(v["in"])
		}
	}

	// layer1 — journeyStage.stage = { in, out }
	if l1, ok := m["layer1"].(map[string]interface{}); ok {
		if js, ok := l1["journeyStage"].(map[string]interface{}); ok {
			for k, v := range js {
				if io, ok := v.(map[string]interface{}); ok {
					if x := io["in"]; x != nil {
						layer1In[k] += toInt64Balance(x)
					}
					if x := io["out"]; x != nil {
						layer1Out[k] += toInt64Balance(x)
					}
				}
			}
		}
	}

	// layer2 — dim.group = { in, out }
	if l2, ok := m["layer2"].(map[string]interface{}); ok {
		for _, dim := range []string{"valueTier", "lifecycleStage", "channel", "loyaltyStage", "momentumStage", "ceoGroup"} {
			if sub, ok := l2[dim].(map[string]interface{}); ok {
				for k, v := range sub {
					if io, ok := v.(map[string]interface{}); ok {
						if x := io["in"]; x != nil {
							layer2In[dim][k] += toInt64Balance(x)
						}
						if x := io["out"]; x != nil {
							layer2Out[dim][k] += toInt64Balance(x)
						}
					}
				}
			}
		}
		for _, dim := range []string{"valueTierLTV", "lifecycleStageLTV", "channelLTV", "loyaltyStageLTV", "momentumStageLTV", "ceoGroupLTV"} {
			if sub, ok := l2[dim].(map[string]interface{}); ok {
				for k, v := range sub {
					if io, ok := v.(map[string]interface{}); ok {
						if x := io["in"]; x != nil {
							layer2LTVIn[dim][k] += toFloat64Balance(x)
						}
						if x := io["out"]; x != nil {
							layer2LTVOut[dim][k] += toFloat64Balance(x)
						}
					}
				}
			}
		}
	}

	// layer3 — group.dim.key = { in, out }
	if l3, ok := m["layer3"].(map[string]interface{}); ok {
		for _, group := range []string{"first", "repeat", "vip", "inactive", "engaged"} {
			if gMap, ok := l3[group].(map[string]interface{}); ok {
				for dim, val := range gMap {
					if sub, ok := val.(map[string]interface{}); ok {
						for k, v := range sub {
							if io, ok := v.(map[string]interface{}); ok {
								if layer3In[group][dim] == nil {
									layer3In[group][dim] = make(map[string]int64)
								}
								if x := io["in"]; x != nil {
									layer3In[group][dim][k] += toInt64Balance(x)
								}
								if layer3Out[group][dim] == nil {
									layer3Out[group][dim] = make(map[string]int64)
								}
								if x := io["out"]; x != nil {
									layer3Out[group][dim][k] += toInt64Balance(x)
								}
							}
						}
					}
				}
			}
		}
	}
}

func toInt64Balance(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	}
	return 0
}

func toFloat64Balance(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}

func buildBalanceFromAccum(rawIn, rawOut map[string]float64,
	layer1In, layer1Out map[string]int64,
	layer2In, layer2Out map[string]map[string]int64,
	layer2LTVIn, layer2LTVOut map[string]map[string]float64,
	layer3In, layer3Out map[string]map[string]map[string]int64) map[string]interface{} {

	netInt64Map := func(in, out map[string]int64) map[string]interface{} {
		m := make(map[string]interface{})
		allKeys := make(map[string]bool)
		for k := range in {
			allKeys[k] = true
		}
		for k := range out {
			allKeys[k] = true
		}
		for k := range allKeys {
			net := in[k] - out[k]
			if net != 0 {
				m[k] = net
			}
		}
		return m
	}
	netFloat64Map := func(in, out map[string]float64) map[string]interface{} {
		m := make(map[string]interface{})
		allKeys := make(map[string]bool)
		for k := range in {
			allKeys[k] = true
		}
		for k := range out {
			allKeys[k] = true
		}
		for k := range allKeys {
			net := in[k] - out[k]
			if net != 0 {
				m[k] = net
			}
		}
		return m
	}

	raw := map[string]interface{}{
		"totalCustomers":       int64(rawIn["totalCustomers"] - rawOut["totalCustomers"]),
		"newCustomersInPeriod": int64(rawIn["newCustomersInPeriod"]),
		"activeInPeriod":      int64(rawIn["activeInPeriod"]),
		"reactivationValue":   rawIn["reactivationValue"] - rawOut["reactivationValue"],
		"totalLTV":            rawIn["totalLTV"] - rawOut["totalLTV"],
	}

	layer1 := map[string]interface{}{
		"journeyStage": netInt64Map(layer1In, layer1Out),
	}

	layer2 := map[string]interface{}{
		"valueTier":         netInt64Map(layer2In["valueTier"], layer2Out["valueTier"]),
		"lifecycleStage":    netInt64Map(layer2In["lifecycleStage"], layer2Out["lifecycleStage"]),
		"channel":           netInt64Map(layer2In["channel"], layer2Out["channel"]),
		"loyaltyStage":      netInt64Map(layer2In["loyaltyStage"], layer2Out["loyaltyStage"]),
		"momentumStage":     netInt64Map(layer2In["momentumStage"], layer2Out["momentumStage"]),
		"ceoGroup":          netInt64Map(layer2In["ceoGroup"], layer2Out["ceoGroup"]),
		"valueTierLTV":      netFloat64Map(layer2LTVIn["valueTierLTV"], layer2LTVOut["valueTierLTV"]),
		"lifecycleStageLTV": netFloat64Map(layer2LTVIn["lifecycleStageLTV"], layer2LTVOut["lifecycleStageLTV"]),
		"channelLTV":        netFloat64Map(layer2LTVIn["channelLTV"], layer2LTVOut["channelLTV"]),
		"loyaltyStageLTV":   netFloat64Map(layer2LTVIn["loyaltyStageLTV"], layer2LTVOut["loyaltyStageLTV"]),
		"momentumStageLTV":  netFloat64Map(layer2LTVIn["momentumStageLTV"], layer2LTVOut["momentumStageLTV"]),
		"ceoGroupLTV":       netFloat64Map(layer2LTVIn["ceoGroupLTV"], layer2LTVOut["ceoGroupLTV"]),
	}

	layer3 := make(map[string]interface{})
	for _, group := range []string{"first", "repeat", "vip", "inactive", "engaged"} {
		groupOut := make(map[string]interface{})
		for dim := range layer3In[group] {
			if layer3Out[group][dim] == nil {
				layer3Out[group][dim] = make(map[string]int64)
			}
			groupOut[dim] = netInt64Map(layer3In[group][dim], layer3Out[group][dim])
		}
		for dim := range layer3Out[group] {
			if _, has := layer3In[group][dim]; !has {
				if layer3In[group][dim] == nil {
					layer3In[group][dim] = make(map[string]int64)
				}
				groupOut[dim] = netInt64Map(layer3In[group][dim], layer3Out[group][dim])
			}
		}
		layer3[group] = groupOut
	}

	return map[string]interface{}{
		"raw":    raw,
		"layer1": layer1,
		"layer2": layer2,
		"layer3": layer3,
	}
}
