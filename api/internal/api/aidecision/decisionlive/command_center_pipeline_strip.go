// Package decisionlive — Strip “Intake (cửa sổ) | Event queue | Debounce | Domains” trong snapshot WS/HTTP metrics.
package decisionlive

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"meta_commerce/internal/api/aidecision/queuedepth"
	"meta_commerce/internal/global"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CommandCenterPipelineStrip — khối tóm tắt toàn ống cho UI thanh trạng thái (org-live WS aggregate + GET metrics).
type CommandCenterPipelineStrip struct {
	SchemaVersion int `json:"schemaVersion"`
	// IntakeWindow — số lần ghi decision_events_queue thành công trong cửa sổ (RAM process này).
	IntakeWindow PipelineStripIntakeWindow `json:"intakeWindow"`
	// EventQueue — trích từ queue.depth (decision_events_queue).
	EventQueue PipelineStripEventQueueSummary `json:"eventQueue"`
	// Debounce — giảm chấn theo domain (byDomain); field phẳng giữ tương thích client cũ.
	Debounce PipelineStripDebounce `json:"debounce"`
	// Domains — số job *_intel_compute đang chờ trong hàng đợi (chưa claim; không gồm bản ghi processedAt âm đang chạy).
	Domains PipelineStripDomainQueues `json:"domains"`
	// ReconciledAtMs — thời điểm đếm Mongo debounce + domains lần gần nhất (0 nếu chưa đếm).
	ReconciledAtMs int64 `json:"reconciledAtMs"`
	// RefreshIntervalMs — khoảng tối thiểu giữa hai lần đếm Mongo cho strip (theo process).
	RefreshIntervalMs int64 `json:"refreshIntervalMs"`
}

// PipelineStripIntakeWindow — intake theo cửa sổ thời gian.
type PipelineStripIntakeWindow struct {
	CountLast60Seconds int64 `json:"countLast60Seconds"`
	CountLast5Minutes  int64 `json:"countLast5Minutes"`
}

// PipelineStripEventQueueSummary — độ sâu tóm tắt event queue trung tâm.
type PipelineStripEventQueueSummary struct {
	Pending         int64 `json:"pending"`
	InFlight        int64 `json:"inFlight"`
	Deferred        int64 `json:"deferred"`
	FailedRetryable int64 `json:"failedRetryable"`
	ReconciledAtMs  int64 `json:"reconciledAtMs"`
}

// Khóa debounce theo domain — khớp tinh thần "giảm chấn từng domain" + cùng tên với domains (ads/cix/crm/order).
const (
	PipelineDebounceDomainAIDecisionMessage = "ai_decision_message" // decision_debounce_state → message.batch_ready
	PipelineDebounceDomainAds               = "ads"                 // decision_recompute_debounce_queue (Meta Ads intel trước ads_intel_compute)
	PipelineDebounceDomainCix               = "cix"
	PipelineDebounceDomainCrm               = "crm"
	PipelineDebounceDomainOrder             = "order"
)

// PipelineStripDebounceDomainEntry — một domain: số slot đang chờ cửa sổ debounce.
type PipelineStripDebounceDomainEntry struct {
	// OpenKeys — số khóa / bản ghi đang trong trạng thái debounce (scheduled hoặc đang gom).
	OpenKeys int64 `json:"openKeys"`
	// DebounceEnabled — chỉ có nghĩa với ai_decision_message (env AI_DECISION_DEBOUNCE_ENABLED).
	DebounceEnabled bool `json:"debounceEnabled,omitempty"`
	// ByEntityType — chỉ domain ads: tách theo recalcObjectType (campaign, adset, ad, ad_account, …) — chỉ bản ghi map vào domain ads.
	ByEntityType map[string]int64 `json:"byEntityType,omitempty"`
	// BySourceKind — chỉ domain ads: đếm theo phần tử sourceKinds trên từng bản ghi scheduled (một bản ghi có thể cộng nhiều khóa).
	BySourceKind map[string]int64 `json:"bySourceKind,omitempty"`
	// SourceCollection — collection nguồn đếm (chuỗi rỗng = không có bảng debounce riêng cho domain này).
	SourceCollection string `json:"sourceCollection,omitempty"`
}

// PipelineStripDebounce — giảm chấn theo domain (byDomain) + field phẳng legacy.
type PipelineStripDebounce struct {
	// ByDomain — luôn có đủ 5 khóa: ai_decision_message, ads, cix, crm, order (cix/crm/order = 0 nếu chưa có store debounce).
	ByDomain map[string]PipelineStripDebounceDomainEntry `json:"byDomain"`
	// AiDecisionMessageDebounceEnabled — AI_DECISION_DEBOUNCE_ENABLED=true (trùng byDomain[ai_decision_message].debounceEnabled).
	AiDecisionMessageDebounceEnabled bool `json:"aiDecisionMessageDebounceEnabled"`
	// MessageBurstOpenKeys — = byDomain[ai_decision_message].openKeys (legacy).
	MessageBurstOpenKeys int64 `json:"messageBurstOpenKeys"`
	// RecomputeScheduled — = byDomain[ads].openKeys (legacy).
	RecomputeScheduled int64 `json:"recomputeScheduled"`
}

// PipelineStripDomainQueues — backlog “chờ xử lý” theo domain: processedAt không có hoặc null (chưa bị worker claim).
type PipelineStripDomainQueues struct {
	Ads   int64 `json:"ads"`
	Cix   int64 `json:"cix"`
	Crm   int64 `json:"crm"`
	Order int64 `json:"order"`
	Total int64 `json:"total"`
}

type pipelineStripMongoCache struct {
	mu                     sync.Mutex
	lastRefreshMs          int64
	reconciledAtMs         int64
	messageBurstOpenKeys   int64
	recomputeScheduled     int64
	recomputeByPipelineDom map[string]int64
	recomputeByObjTypeAds  map[string]int64
	recomputeBySourceKind  map[string]int64
	ads, cix, crm, order   int64
}

var memPipelineStripMongo sync.Map // orgHex -> *pipelineStripMongoCache

func pipelineStripRefreshInterval() time.Duration {
	v := strings.TrimSpace(os.Getenv("AI_DECISION_PIPELINE_STRIP_REFRESH_SEC"))
	if v == "" {
		return 5 * time.Second
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 5 * time.Second
	}
	return time.Duration(n) * time.Second
}

func aiDecisionDebounceEnabled() bool {
	return strings.TrimSpace(strings.ToLower(os.Getenv("AI_DECISION_DEBOUNCE_ENABLED"))) == "true"
}

// filterIntelComputePendingOnly — chỉ job còn nằm trong hàng đợi chờ (chưa claim). Bỏ qua processedAt < 0 (đang xử lý).
func filterIntelComputePendingOnly(ownerOrgID primitive.ObjectID) bson.M {
	return bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$or": bson.A{
			bson.M{"processedAt": bson.M{"$exists": false}},
			bson.M{"processedAt": nil},
		},
	}
}

// mapSourceKindsToPipelineDomain — map sourceKinds trên decision_recompute_debounce_queue → khóa domain (cùng bộ với byDomain).
func mapSourceKindsToPipelineDomain(sourceKinds []string) string {
	if len(sourceKinds) == 0 {
		return PipelineDebounceDomainAds
	}
	for _, raw := range sourceKinds {
		sk := strings.TrimSpace(strings.ToLower(raw))
		if sk == "" {
			continue
		}
		switch {
		case strings.HasPrefix(sk, "customer_"), sk == "customer_core_records", sk == "customer_activity",
			strings.HasPrefix(sk, "crm_"), sk == "crm_customers", sk == "crm_activity": // legacy debounce keys
			return PipelineDebounceDomainCrm
		case strings.HasPrefix(sk, "cix_"), sk == "cix_analysis", sk == "fb_src_message_items":
			return PipelineDebounceDomainCix
		case strings.HasPrefix(sk, "order_"), sk == "order_intel":
			return PipelineDebounceDomainOrder
		case sk == global.MongoDB_ColNames.PcPosOrders, sk == global.MongoDB_ColNames.ManualPosOrders, sk == global.MongoDB_ColNames.OrderCanonical, sk == "fb_src_conversations", strings.HasPrefix(sk, "meta_"):
			return PipelineDebounceDomainAds
		}
	}
	return PipelineDebounceDomainAds
}

type recomputeDebounceScheduledRow struct {
	SourceKinds      []string `bson:"sourceKinds"`
	RecalcObjectType string   `bson:"recalcObjectType"`
}

// scanRecomputeDebounceScheduled — đếm decision_recompute_debounce_queue (scheduled) theo domain pipeline (sourceKinds) + chi tiết ads.
func scanRecomputeDebounceScheduled(ctx context.Context, ownerOrgID primitive.ObjectID) (
	total int64,
	byPipelineDomain map[string]int64,
	byRecalcObjectTypeAds map[string]int64,
	bySourceKindAds map[string]int64,
) {
	byPipelineDomain = map[string]int64{
		PipelineDebounceDomainAds:   0,
		PipelineDebounceDomainCix:   0,
		PipelineDebounceDomainCrm:   0,
		PipelineDebounceDomainOrder: 0,
	}
	byRecalcObjectTypeAds = make(map[string]int64)
	bySourceKindAds = make(map[string]int64)
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.RecomputeDebounceQueue)
	if !ok || coll == nil {
		return 0, byPipelineDomain, byRecalcObjectTypeAds, bySourceKindAds
	}
	cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cur, err := coll.Find(cctx, bson.M{
		"ownerOrgId": ownerOrgID,
		"status":     "scheduled",
	}, options.Find().SetProjection(bson.M{
		"sourceKinds":      1,
		"recalcObjectType": 1,
	}))
	if err != nil {
		logrus.WithError(err).Debug("AI Decision pipeline strip: Find decision_recompute_debounce_queue thất bại")
		return 0, byPipelineDomain, byRecalcObjectTypeAds, bySourceKindAds
	}
	defer cur.Close(cctx)
	for cur.Next(cctx) {
		var doc recomputeDebounceScheduledRow
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		total++
		domain := mapSourceKindsToPipelineDomain(doc.SourceKinds)
		byPipelineDomain[domain]++
		if domain == PipelineDebounceDomainAds {
			ot := strings.TrimSpace(strings.ToLower(doc.RecalcObjectType))
			if ot == "" {
				ot = "unknown"
			}
			byRecalcObjectTypeAds[ot]++
			if len(doc.SourceKinds) == 0 {
				bySourceKindAds["(none)"]++
			} else {
				for _, raw := range doc.SourceKinds {
					sk := strings.TrimSpace(strings.ToLower(raw))
					if sk == "" {
						sk = "(empty)"
					}
					bySourceKindAds[sk]++
				}
			}
		}
	}
	if err := cur.Err(); err != nil {
		logrus.WithError(err).Debug("AI Decision pipeline strip: cursor decision_recompute_debounce_queue")
	}
	return total, byPipelineDomain, byRecalcObjectTypeAds, bySourceKindAds
}

func countColl(ctx context.Context, collName string, filter bson.M) int64 {
	coll, ok := global.RegistryCollections.Get(collName)
	if !ok || coll == nil {
		return 0
	}
	cctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	n, err := coll.CountDocuments(cctx, filter)
	if err != nil {
		logrus.WithError(err).WithField("collection", collName).Debug("AI Decision pipeline strip: CountDocuments thất bại")
		return 0
	}
	return n
}

func refreshPipelineStripMongo(ctx context.Context, ownerOrgID primitive.ObjectID, orgHex string) PipelineStripMongoFresh {
	orgHex = queuedepth.NormalizeOrgHex(orgHex)
	iv := pipelineStripRefreshInterval().Milliseconds()
	if ownerOrgID.IsZero() || orgHex == "" {
		return PipelineStripMongoFresh{
			RefreshIntervalMs: iv,
		}
	}
	nowMs := time.Now().UnixMilli()
	v, _ := memPipelineStripMongo.LoadOrStore(orgHex, &pipelineStripMongoCache{})
	c := v.(*pipelineStripMongoCache)
	c.mu.Lock()
	defer c.mu.Unlock()
	minGap := pipelineStripRefreshInterval()
	if nowMs-c.lastRefreshMs < minGap.Milliseconds() && c.lastRefreshMs > 0 {
		return PipelineStripMongoFresh{
			MessageBurstOpenKeys:   c.messageBurstOpenKeys,
			RecomputeScheduled:     c.recomputeScheduled,
			RecomputeByPipelineDom: cloneInt64Map(c.recomputeByPipelineDom),
			RecomputeByObjTypeAds:  cloneInt64Map(c.recomputeByObjTypeAds),
			RecomputeBySourceKindAds: cloneInt64Map(c.recomputeBySourceKind),
			Domains: PipelineStripDomainQueues{
				Ads: c.ads, Cix: c.cix, Crm: c.crm, Order: c.order,
				Total: c.ads + c.cix + c.crm + c.order,
			},
			ReconciledAtMs:    c.reconciledAtMs,
			RefreshIntervalMs: iv,
		}
	}

	reconAt := nowMs
	msgBurst := countColl(ctx, global.MongoDB_ColNames.DecisionDebounceState, bson.M{"ownerOrgId": ownerOrgID})
	recomputeTotal, byPipelineDomain, byRecalcAds, bySourceKindAds := scanRecomputeDebounceScheduled(ctx, ownerOrgID)
	if recomputeTotal == 0 {
		fb := countColl(ctx, global.MongoDB_ColNames.RecomputeDebounceQueue, bson.M{
			"ownerOrgId": ownerOrgID,
			"status":     "scheduled",
		})
		if fb > 0 {
			recomputeTotal = fb
			byPipelineDomain[PipelineDebounceDomainAds] = fb
		}
	}
	flt := filterIntelComputePendingOnly(ownerOrgID)
	ads := countColl(ctx, global.MongoDB_ColNames.AdsIntelCompute, flt)
	cix := countColl(ctx, global.MongoDB_ColNames.CixIntelCompute, flt)
	crm := countColl(ctx, global.MongoDB_ColNames.CustomerIntelCompute, flt)
	ord := countColl(ctx, global.MongoDB_ColNames.OrderIntelCompute, flt)

	c.lastRefreshMs = nowMs
	c.reconciledAtMs = reconAt
	c.messageBurstOpenKeys = msgBurst
	c.recomputeScheduled = recomputeTotal
	c.recomputeByPipelineDom = cloneInt64Map(byPipelineDomain)
	c.recomputeByObjTypeAds = cloneInt64Map(byRecalcAds)
	c.recomputeBySourceKind = cloneInt64Map(bySourceKindAds)
	c.ads, c.cix, c.crm, c.order = ads, cix, crm, ord

	return PipelineStripMongoFresh{
		MessageBurstOpenKeys:   msgBurst,
		RecomputeScheduled:     recomputeTotal,
		RecomputeByPipelineDom: cloneInt64Map(byPipelineDomain),
		RecomputeByObjTypeAds:  cloneInt64Map(byRecalcAds),
		RecomputeBySourceKindAds: cloneInt64Map(bySourceKindAds),
		Domains: PipelineStripDomainQueues{
			Ads: ads, Cix: cix, Crm: crm, Order: ord,
			Total: ads + cix + crm + ord,
		},
		ReconciledAtMs:    reconAt,
		RefreshIntervalMs: iv,
	}
}

func cloneInt64Map(m map[string]int64) map[string]int64 {
	if len(m) == 0 {
		return map[string]int64{}
	}
	out := make(map[string]int64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// PipelineStripMongoFresh — kết quả đếm Mongo (hoặc cache) cho phần debounce + domains.
type PipelineStripMongoFresh struct {
	MessageBurstOpenKeys     int64
	RecomputeScheduled       int64
	RecomputeByPipelineDom   map[string]int64
	RecomputeByObjTypeAds    map[string]int64
	RecomputeBySourceKindAds map[string]int64
	Domains                  PipelineStripDomainQueues
	ReconciledAtMs           int64
	RefreshIntervalMs        int64
}

func openKeysFromDomainMap(m map[string]int64, key string) int64 {
	if m == nil {
		return 0
	}
	return m[key]
}

func buildPipelineStripDebounce(
	enabled bool,
	msgBurst int64,
	recomputeTotal int64,
	byPipelineDomain map[string]int64,
	byRecalcAds map[string]int64,
	bySourceKindAds map[string]int64,
) PipelineStripDebounce {
	recColl := global.MongoDB_ColNames.RecomputeDebounceQueue
	adsKeys := openKeysFromDomainMap(byPipelineDomain, PipelineDebounceDomainAds)
	cixKeys := openKeysFromDomainMap(byPipelineDomain, PipelineDebounceDomainCix)
	crmKeys := openKeysFromDomainMap(byPipelineDomain, PipelineDebounceDomainCrm)
	orderKeys := openKeysFromDomainMap(byPipelineDomain, PipelineDebounceDomainOrder)
	return PipelineStripDebounce{
		ByDomain: map[string]PipelineStripDebounceDomainEntry{
			PipelineDebounceDomainAIDecisionMessage: {
				OpenKeys:         msgBurst,
				DebounceEnabled:  enabled,
				SourceCollection: global.MongoDB_ColNames.DecisionDebounceState,
			},
			PipelineDebounceDomainAds: {
				OpenKeys:         adsKeys,
				ByEntityType:     cloneInt64Map(byRecalcAds),
				BySourceKind:     cloneInt64Map(bySourceKindAds),
				SourceCollection: recColl,
			},
			PipelineDebounceDomainCix: {
				OpenKeys:         cixKeys,
				SourceCollection: recColl,
			},
			PipelineDebounceDomainCrm: {
				OpenKeys:         crmKeys,
				SourceCollection: recColl,
			},
			PipelineDebounceDomainOrder: {
				OpenKeys:         orderKeys,
				SourceCollection: recColl,
			},
		},
		AiDecisionMessageDebounceEnabled: enabled,
		MessageBurstOpenKeys:             msgBurst,
		RecomputeScheduled:                 recomputeTotal,
	}
}

func buildPipelineStrip(ctx context.Context, ownerOrgID primitive.ObjectID, orgHex string, asOfMs int64, q *CommandCenterQueueMetrics) CommandCenterPipelineStrip {
	orgHex = normalizeQueueOrgHex(orgHex)
	c60, c5 := intakeWindowCountsForOrg(orgHex, asOfMs)
	mongo := refreshPipelineStripMongo(ctx, ownerOrgID, orgHex)
	depth := map[string]int64{}
	var queueRecon int64
	if q != nil {
		queueRecon = q.ReconciledAtMs
		if q.Depth != nil {
			for k, v := range q.Depth {
				depth[k] = v
			}
		}
	}
	inFlight := depth["in_flight"]
	if inFlight == 0 {
		inFlight = depth["leased"] + depth["processing"]
	}
	return CommandCenterPipelineStrip{
		SchemaVersion: 3,
		IntakeWindow: PipelineStripIntakeWindow{
			CountLast60Seconds: c60,
			CountLast5Minutes:  c5,
		},
		EventQueue: PipelineStripEventQueueSummary{
			Pending:         depth["pending"],
			InFlight:        inFlight,
			Deferred:        depth["deferred"],
			FailedRetryable: depth["failed_retryable"],
			ReconciledAtMs:  queueRecon,
		},
		Debounce: buildPipelineStripDebounce(
			aiDecisionDebounceEnabled(),
			mongo.MessageBurstOpenKeys,
			mongo.RecomputeScheduled,
			mongo.RecomputeByPipelineDom,
			mongo.RecomputeByObjTypeAds,
			mongo.RecomputeBySourceKindAds,
		),
		Domains:           mongo.Domains,
		ReconciledAtMs:    mongo.ReconciledAtMs,
		RefreshIntervalMs: mongo.RefreshIntervalMs,
	}
}
