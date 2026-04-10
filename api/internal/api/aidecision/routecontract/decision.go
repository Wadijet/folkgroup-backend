// Package routecontract — Hợp đồng định tuyến datachanged (struct chung, không import hooks/service) để tránh vòng import với datachangedsidefx.
package routecontract

// Decision — ảnh chụp định tuyến theo collection sau Resolve (code + YAML).
type Decision struct {
	Version    string
	Collection string
	RuleID     string

	EmitToDecisionQueue bool

	CustomerPendingMergeCollection    bool
	ReportTouchPipeline               bool
	AdsProfilePipeline                bool
	CixIntelPipeline                  bool
	OrderIntelPipeline                bool
	CustomerIntelRefreshDeferPipeline bool
}
