// Package crmvc - Test buildMetricsSnapshot trả về nested (raw, layer1, layer2, layer3).
package crmvc

import (
	"testing"

	crmmodels "meta_commerce/internal/api/crm/models"
)

func TestBuildCurrentMetricsSnapshot_ReturnsNested(t *testing.T) {
	c := &crmmodels.CrmCustomer{OrderCount: 0}
	m := BuildCurrentMetricsSnapshot(c)
	if m == nil {
		t.Fatal("BuildCurrentMetricsSnapshot trả về nil")
	}
	if _, ok := m["raw"]; !ok {
		t.Errorf("metricsSnapshot thiếu key 'raw', có keys: %v", keys(m))
	}
	if _, ok := m["layer1"]; !ok {
		t.Errorf("metricsSnapshot thiếu key 'layer1', có keys: %v", keys(m))
	}
	if _, ok := m["layer2"]; !ok {
		t.Errorf("metricsSnapshot thiếu key 'layer2', có keys: %v", keys(m))
	}
	if _, ok := m["layer3"]; !ok {
		t.Errorf("metricsSnapshot thiếu key 'layer3', có keys: %v", keys(m))
	}
	if _, ok := m["valueTier"]; ok {
		t.Error("metricsSnapshot phải nested; valueTier không được ở top-level (flat)")
	}
}

func TestBuildSnapshotForNewCustomer_MetricsSnapshotNested(t *testing.T) {
	c := &crmmodels.CrmCustomer{Profile: crmmodels.CrmCustomerProfile{Name: "Test"}, OrderCount: 0}
	snap := BuildSnapshotForNewCustomer(c, 0, true, nil)
	if snap == nil {
		t.Fatal("BuildSnapshotForNewCustomer trả về nil")
	}
	metrics, ok := snap["metricsSnapshot"].(map[string]interface{})
	if !ok || metrics == nil {
		t.Fatal("metricsSnapshot không phải map")
	}
	if _, ok := metrics["raw"]; !ok {
		t.Errorf("metricsSnapshot thiếu raw, có keys: %v", keys(metrics))
	}
}

func keys(m map[string]interface{}) []string {
	var k []string
	for x := range m {
		k = append(k, x)
	}
	return k
}
