package datachangedrouting

import (
	_ "embed"
	"os"
	"strings"
	"sync"

	"meta_commerce/internal/api/aidecision/routecontract"
	"meta_commerce/internal/logger"

	"gopkg.in/yaml.v3"
)

//go:embed routing.default.yaml
var embeddedRoutingYAML []byte

// envRoutingConfigPath — đường dẫn file YAML thay thế bản embed (toàn bộ file, không merge file).
const envRoutingConfigPath = "DATACHANGED_ROUTING_CONFIG"

type yamlRoot struct {
	ConfigVersion       string                            `yaml:"config_version"`
	CollectionOverrides map[string]collectionOverrideYAML `yaml:"collection_overrides"`
}

type collectionOverrideYAML struct {
	EmitToDecisionQueue         *bool `yaml:"emit_to_decision_queue"`
	CustomerPendingMerge      *bool `yaml:"customer_pending_merge"`
	ReportTouch               *bool `yaml:"report_touch"`
	AdsProfile                *bool `yaml:"ads_profile"`
	CixIntel                  *bool `yaml:"cix_intel"`
	OrderIntel                *bool `yaml:"order_intel"`
	CustomerIntelRefreshDefer *bool `yaml:"customer_intel_refresh_defer"`
}

var (
	routingLoadOnce sync.Once
	routingRoot     *yamlRoot
)

func loadRoutingFileOnce() {
	routingLoadOnce.Do(func() {
		data := embeddedRoutingYAML
		if p := strings.TrimSpace(os.Getenv(envRoutingConfigPath)); p != "" {
			b, err := os.ReadFile(p)
			if err != nil {
				logger.GetAppLogger().WithError(err).WithField("path", p).Warn("[DATACHANGED_ROUTING] Không đọc được file cấu hình, dùng bản embed")
			} else {
				data = b
			}
		}
		var root yamlRoot
		if err := yaml.Unmarshal(data, &root); err != nil {
			logger.GetAppLogger().WithError(err).Warn("[DATACHANGED_ROUTING] Parse YAML thất bại, bỏ qua ghi đè collection")
			return
		}
		routingRoot = &root
		if cv := strings.TrimSpace(root.ConfigVersion); cv != "" && cv != Version {
			logger.GetAppLogger().WithFields(map[string]interface{}{
				"fileConfigVersion": cv, "codeVersion": Version,
			}).Warn("[DATACHANGED_ROUTING] config_version trong file khác Version code — vẫn áp dụng ghi đè; nên đồng bộ")
		}
	})
}

// applyCollectionOverrides áp ghi đè từ YAML lên Decision (nếu có).
func applyCollectionOverrides(d routecontract.Decision) routecontract.Decision {
	loadRoutingFileOnce()
	root := routingRoot
	if root == nil || len(root.CollectionOverrides) == 0 {
		return d
	}
	o, ok := root.CollectionOverrides[d.Collection]
	if !ok {
		return d
	}
	if o.EmitToDecisionQueue != nil {
		d.EmitToDecisionQueue = *o.EmitToDecisionQueue
	}
	if o.CustomerPendingMerge != nil {
		d.CustomerPendingMergeCollection = *o.CustomerPendingMerge
	}
	if o.ReportTouch != nil {
		d.ReportTouchPipeline = *o.ReportTouch
	}
	if o.AdsProfile != nil {
		d.AdsProfilePipeline = *o.AdsProfile
	}
	if o.CixIntel != nil {
		d.CixIntelPipeline = *o.CixIntel
	}
	if o.OrderIntel != nil {
		d.OrderIntelPipeline = *o.OrderIntel
	}
	if o.CustomerIntelRefreshDefer != nil {
		d.CustomerIntelRefreshDeferPipeline = *o.CustomerIntelRefreshDefer
	}
	return d
}

// EmitToQueueFromYAML trả (giá trị, true) nếu collection_overrides có emit_to_decision_queue cho collection.
func EmitToQueueFromYAML(collection string) (bool, bool) {
	loadRoutingFileOnce()
	root := routingRoot
	if root == nil || len(root.CollectionOverrides) == 0 {
		return false, false
	}
	o, ok := root.CollectionOverrides[strings.TrimSpace(collection)]
	if !ok || o.EmitToDecisionQueue == nil {
		return false, false
	}
	return *o.EmitToDecisionQueue, true
}
