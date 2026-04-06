// Package hooks — Bộ lọc: collection đăng ký source_sync_registry; chỉ một số collection ghi decision_events_queue.
//
// Thứ tự: ghi đè YAML emit_to_decision_queue (datachangedrouting) nếu có; không thì mặc định datachangedemit (map + Meta).
//
// campaign_intel_recomputed → AI Decision từ worker sau recompute (meta_ads_intel).
package hooks

import (
	"strings"

	"meta_commerce/internal/api/aidecision/datachangedemit"
	"meta_commerce/internal/api/aidecision/datachangedrouting"
)

// DatachangedEmitPerCollection trỏ cùng map với datachangedemit.EmitPerCollection (chỉnh map ở một nơi).
var DatachangedEmitPerCollection = datachangedemit.EmitPerCollection

// ShouldEmitDatachangedToDecisionQueue quyết định sau OnDataChanged có gọi EmitEvent → decision_events_queue hay không.
func ShouldEmitDatachangedToDecisionQueue(collectionName string) bool {
	c := strings.TrimSpace(collectionName)
	if c == "" {
		return false
	}
	if v, ok := datachangedrouting.EmitToQueueFromYAML(c); ok {
		return v
	}
	return datachangedemit.DefaultShouldEmitToDecisionQueue(c)
}

// IsMetaAdsSyncedCollection — true nếu collection thuộc nhóm Meta Marketing đồng bộ (định tuyến, doc, test).
func IsMetaAdsSyncedCollection(name string) bool {
	return datachangedemit.IsMetaAdsSyncedCollection(name)
}
