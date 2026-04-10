// Package decisionlive — Metrics cho màn hình trung tâm chỉ huy; số liệu chủ yếu giữ trong RAM process.
//
// Snapshot (schemaVersion 2) — từng nhóm có ý nghĩa riêng, dễ đọc trên UI:
//   - queue: độ sâu hàng đợi sự kiện (đồng bộ từ Mongo vào RAM theo chu kỳ).
//   - intake: số lần ghi job vào hàng đợi thành công (mỗi EmitEvent insert OK).
//   - publishCounters: bộ đếm lũy kế từ đầu process — mỗi lần Publish live (và vài mốc consumer không có trace).
//   - realtime.gaugeByPhase: chỉ số tức thời (ví dụ job đang giữ, phase đang chạy).
//   - consumer: thống kê sau mỗi lần consumer xử lý xong trên process này.
package decisionlive

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"meta_commerce/internal/api/aidecision/queuedepth"
)

const defaultWSAggregateInterval = 3 * time.Second

// metricsChangeLogEnabled — Bật log từng lần cộng metrics (Publish, intake, đồng bộ độ sâu queue, consumer lease). Mặc định tắt; bật: AI_DECISION_METRICS_CHANGE_LOG=1.
func metricsChangeLogEnabled() bool {
	return strings.TrimSpace(os.Getenv("AI_DECISION_METRICS_CHANGE_LOG")) == "1"
}

// MetricsChangeLogEnabled — Export cờ dùng chung cho worker và Publish (cùng biến môi trường).
func MetricsChangeLogEnabled() bool {
	return metricsChangeLogEnabled()
}

// CommandCenterAlert — Một dòng cảnh báo hiển thị trên trung tâm chỉ huy (không chặn API).
type CommandCenterAlert struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// CommandCenterMeta — Trạng thái môi trường (live bật/tắt, Redis, …) — client nên đọc trước khi diễn giải các khối số.
type CommandCenterMeta struct {
	LivePublishEnabled    bool   `json:"livePublishEnabled"`
	RedisEnabled          bool   `json:"redisEnabled"`
	MetricsStore          string `json:"metricsStore"`
	LiveFunnelFromPublish bool   `json:"liveFunnelFromPublish"`
}

// CommandCenterQueueMetrics — Số job theo trạng thái trên hàng đợi (nguồn: đồng bộ Mongo → RAM).
type CommandCenterQueueMetrics struct {
	Depth                         map[string]int64 `json:"depth"`
	ReconciledAtMs                int64            `json:"reconciledAtMs"`
	RefreshedFromMongoThisRequest bool             `json:"refreshedFromMongoThisRequest,omitempty"`
}

// CommandCenterIntakeMetrics — Đếm job được đưa vào hàng đợi (EmitEvent thành công), không phải số job consumer đã xử lý.
type CommandCenterIntakeMetrics struct {
	ByEventType   map[string]int64 `json:"byEventType"`
	ByEventSource map[string]int64 `json:"byEventSource"`
}

// CommandCenterPublishCounters — Bộ đếm lũy kế từ khi process khởi động: mỗi lần gọi Publish (theo phase / nguồn / …).
type CommandCenterPublishCounters struct {
	ByPhase               map[string]int64 `json:"byPhase"`
	BySourceKind          map[string]int64 `json:"bySourceKind"`
	ByFeedSourceCategory  map[string]int64 `json:"byFeedSourceCategory"`
	ByOpsTier             map[string]int64 `json:"byOpsTier"`
}

// CommandCenterRealtimeMetrics — Ảnh chụp tức thời (gauge), không cộng dồn như bộ đếm lifetime.
type CommandCenterRealtimeMetrics struct {
	GaugeByPhase map[string]int64 `json:"gaugeByPhase"`
}

// CommandCenterSnapshot — Gói số liệu gửi qua GET metrics hoặc WebSocket (kiểu aggregate).
// schemaVersion 2: cấu trúc nhóm theo nghĩa nghiệp vụ; client parse bản flat cũ cần nâng cấp.
type CommandCenterSnapshot struct {
	SchemaVersion             int                          `json:"schemaVersion"`
	AsOfMs                    int64                        `json:"asOfMs"`
	Meta                      CommandCenterMeta            `json:"meta"`
	Queue                     CommandCenterQueueMetrics    `json:"queue"`
	Intake                    CommandCenterIntakeMetrics   `json:"intake"`
	PublishCounters           CommandCenterPublishCounters `json:"publishCounters"`
	Realtime                  CommandCenterRealtimeMetrics `json:"realtime"`
	Consumer                  CommandCenterConsumerMetrics `json:"consumer"`
	Workers                   CommandCenterWorkersSnapshot `json:"workers"`
	HasRecentConsumerActivity bool                         `json:"hasRecentConsumerActivity"`
	Alerts                    []CommandCenterAlert         `json:"alerts,omitempty"`
	// PipelineStrip — Dữ liệu thanh pipeline (intake → hàng đợi → debounce → miền) cho WS org-live và GET metrics.
	PipelineStrip CommandCenterPipelineStrip `json:"pipelineStrip"`
}

type orgLiveBuf struct {
	mu             sync.Mutex
	byPhase        map[string]int64
	bySource       map[string]int64
	byFeedCategory map[string]int64
	byOpsTier      map[string]int64
}

var memLive sync.Map // orgHex -> *orgLiveBuf

// demTu — Nhãn gợi ý nguồn đếm khi bật log chi tiết (ví dụ publish_ws, consumer_bat_dau, publish_chi_metrics…).
// bumpSource — false: chỉ tăng bộ đếm theo phase; tránh đếm trùng nguồn khi cùng một sự kiện đã tăng ở bước khác.
func incrementMemLivePublish(orgHex, phase, sk, demTu string, bumpSource bool, feedCat, opsTier string) {
	v, _ := memLive.LoadOrStore(orgHex, &orgLiveBuf{
		byPhase:        map[string]int64{},
		bySource:       map[string]int64{},
		byFeedCategory: map[string]int64{},
		byOpsTier:      map[string]int64{},
	})
	buf := v.(*orgLiveBuf)
	buf.mu.Lock()
	if buf.byPhase == nil {
		buf.byPhase = map[string]int64{}
		buf.bySource = map[string]int64{}
	}
	if buf.byFeedCategory == nil {
		buf.byFeedCategory = map[string]int64{}
	}
	if buf.byOpsTier == nil {
		buf.byOpsTier = map[string]int64{}
	}
	buf.byPhase[phase]++
	if bumpSource {
		buf.bySource[sk]++
		if fc := strings.TrimSpace(feedCat); fc != "" {
			buf.byFeedCategory[fc]++
		}
		if ot := strings.TrimSpace(opsTier); ot != "" {
			buf.byOpsTier[ot]++
		}
	}
	phaseCount := buf.byPhase[phase]
	sourceCount := buf.bySource[sk]
	buf.mu.Unlock()

	if metricsChangeLogEnabled() {
		if demTu == "" {
			demTu = "unknown"
		}
		fields := logrus.Fields{
			"orgHex":       orgHex,
			"phase":        phase,
			"sourceKind":   sk,
			"demTheoPhase": phaseCount,
			"demTu":        demTu,
			"demNguon":     bumpSource,
		}
		if bumpSource {
			fields["demTheoNguon"] = sourceCount
		}
		logrus.WithFields(fields).Info("AI Decision metrics: bộ đếm phễu (RAM) +1; đếm theo nguồn khi demNguon=true")
	}
}

func queueDepthDocFromMemMap(depth map[string]int64, reconciledAtMs int64) queueDepthDoc {
	if depth == nil {
		depth = map[string]int64{}
	}
	return queueDepthDoc{
		Pending:         depth["pending"],
		Leased:          depth["leased"],
		Processing:      depth["processing"],
		FailedRetryable: depth["failed_retryable"],
		FailedTerminal:  depth["failed_terminal"],
		Deferred:        depth["deferred"],
		OtherActive:     depth["other_active"],
		ReconciledAtMs:  reconciledAtMs,
	}
}

// RecordConsumerWorkBegin — Gọi sau khi worker lease job: tăng gauge «đang giữ job».
// Job không có traceId: thêm gauge «đang consume» và bộ đếm phễu tương ứng.
// Job có trace: chỉ cập nhật gauge giữ job tại đây; phase chi tiết theo từng bước Publish sau đó.
func RecordConsumerWorkBegin(ownerOrgID primitive.ObjectID, eventType, traceID string) {
	if ownerOrgID.IsZero() {
		return
	}
	orgHex := ownerOrgID.Hex()
	recordConsumerWorkBeginGauge(orgHex, traceID)
	if strings.TrimSpace(traceID) != "" {
		return
	}
	sk := strings.TrimSpace(eventType)
	if sk == "" {
		sk = SourceUnknown
	}
	incrementMemLivePublish(orgHex, PhaseConsuming, sk, "consumer_bat_dau", false, "", "")
}

// RecordCommandCenterPublish — Bước 5 trong Publish (live bật): sau khi đã append ring, cộng publishCounters + gauge.
// demTu nội bộ = "publish_ws" (đối chiếu bước 3a dùng "publish_chi_metrics" khi live tắt).
func RecordCommandCenterPublish(ownerOrgID primitive.ObjectID, ev DecisionLiveEvent) {
	recordCommandCenterPublish(ownerOrgID, ev, "publish_ws")
}

// recordCommandCenterPublish — Lõi metrics cho mọi nhánh Publish có org hợp lệ:
// incrementMemLivePublish (byPhase, bySourceKind, feed, opsTier) rồi adjustGaugeOnLivePublish theo trace+phase.
// demTu: nhãn nguồn bump (publish_chi_metrics | publish_ws | …) để phân tích nội bộ.
func recordCommandCenterPublish(ownerOrgID primitive.ObjectID, ev DecisionLiveEvent, demTu string) {
	if ownerOrgID.IsZero() {
		return
	}
	orgHex := ownerOrgID.Hex()
	phase := strings.TrimSpace(ev.Phase)
	if phase == "" {
		phase = "unknown"
	}
	sk := strings.TrimSpace(ev.SourceKind)
	if sk == "" {
		sk = SourceUnknown
	}
	if demTu == "" {
		demTu = "publish_unknown"
	}
	fc := strings.TrimSpace(ev.FeedSourceCategory)
	if fc == "" {
		fc = FeedSourceOther
	}
	ot := strings.TrimSpace(ev.OpsTier)
	if ot == "" {
		ot = "unknown"
	}
	incrementMemLivePublish(orgHex, phase, sk, demTu, true, fc, ot)
	adjustGaugeOnLivePublish(orgHex, ev.TraceID, phase)
}

// BuildCommandCenterSnapshot — Dựng snapshot theo nhóm (schema 2).
// refreshQueueDepthFromMongo=true: ép đọc độ sâu queue từ Mongo ngay (ít dùng); false: dùng RAM đã reconcile (mặc định GET/WS).
func BuildCommandCenterSnapshot(ctx context.Context, ownerOrgID primitive.ObjectID, refreshQueueDepthFromMongo bool) CommandCenterSnapshot {
	now := time.Now().UnixMilli()
	out := CommandCenterSnapshot{
		SchemaVersion: 2,
		AsOfMs:        now,
		Meta: CommandCenterMeta{
			LivePublishEnabled:    liveEnabled(),
			RedisEnabled:          false,
			MetricsStore:          "memory",
			LiveFunnelFromPublish: true,
		},
		Queue: CommandCenterQueueMetrics{
			Depth:          map[string]int64{},
			ReconciledAtMs: 0,
		},
		Intake: CommandCenterIntakeMetrics{
			ByEventType:   map[string]int64{},
			ByEventSource: map[string]int64{},
		},
		PublishCounters: CommandCenterPublishCounters{
			ByPhase:              defaultPhaseCounts(),
			BySourceKind:         map[string]int64{},
			ByFeedSourceCategory: map[string]int64{},
			ByOpsTier:            map[string]int64{},
		},
		Realtime: CommandCenterRealtimeMetrics{
			GaugeByPhase: defaultGaugeByPhase(),
		},
		Consumer: CommandCenterConsumerMetrics{ByEventType: map[string]ConsumerEventTypeMetrics{}},
		Workers:  BuildCommandCenterWorkersSnapshot(),
	}
	if ownerOrgID.IsZero() {
		return out
	}
	orgHex := normalizeQueueOrgHex(ownerOrgID.Hex())
	out.Consumer = consumerSnapshotForOrg(orgHex, now)
	out.Realtime.GaugeByPhase = gaugeSnapshotForOrg(orgHex)
	out.Intake.ByEventType, out.Intake.ByEventSource = intakeSnapshotForOrg(orgHex)

	if v, ok := memLive.Load(orgHex); ok {
		buf := v.(*orgLiveBuf)
		buf.mu.Lock()
		for k, n := range buf.byPhase {
			out.PublishCounters.ByPhase[k] = n
		}
		for k, n := range buf.bySource {
			out.PublishCounters.BySourceKind[k] = n
		}
		for k, n := range buf.byFeedCategory {
			out.PublishCounters.ByFeedSourceCategory[k] = n
		}
		for k, n := range buf.byOpsTier {
			out.PublishCounters.ByOpsTier[k] = n
		}
		buf.mu.Unlock()
	}

	if refreshQueueDepthFromMongo {
		if err := queuedepth.RefreshOrg(ctx, ownerOrgID); err != nil {
			logrus.WithError(err).WithField("orgHex", orgHex).Debug("AI Decision metrics: đọc queueDepth Mongo cho GET thất bại — dùng RAM reconcile")
			if d, rt, ok := queuedepth.MemSnapshotForOrg(orgHex); ok {
				applyQueueDepthDocToSnapshot(&out, queueDepthDocFromMemMap(d, rt))
			}
		} else {
			if d, rt, ok := queuedepth.MemSnapshotForOrg(orgHex); ok {
				applyQueueDepthDocToSnapshot(&out, queueDepthDocFromMemMap(d, rt))
			}
			out.Queue.RefreshedFromMongoThisRequest = true
		}
	} else if d, rt, ok := queuedepth.MemSnapshotForOrg(orgHex); ok {
		applyQueueDepthDocToSnapshot(&out, queueDepthDocFromMemMap(d, rt))
	} else {
		// RAM chưa có org (process mới, reconcile chưa chạy hoặc lỗi lần đầu) — đọc Mongo một lần.
		if err := queuedepth.RefreshOrg(ctx, ownerOrgID); err != nil {
			logrus.WithError(err).WithField("orgHex", orgHex).Debug("AI Decision metrics: lazy queue depth thất bại — depth rỗng đến lần sau")
		} else if d, rt, ok2 := queuedepth.MemSnapshotForOrg(orgHex); ok2 {
			applyQueueDepthDocToSnapshot(&out, queueDepthDocFromMemMap(d, rt))
		}
	}

	if !out.Meta.LivePublishEnabled {
		out.Alerts = append(out.Alerts, CommandCenterAlert{
			Code:     "live_disabled",
			Severity: SeverityWarn,
			Message: "Chế độ live tắt (AI_DECISION_LIVE_ENABLED=0): không có timeline, WebSocket và vòng replay trong RAM; " +
				"bộ đếm phễu và gauge vẫn cập nhật theo worker. Độ sâu hàng đợi vẫn đồng bộ từ Mongo.",
		})
	}
	inFlight := out.Queue.Depth["in_flight"]
	if inFlight == 0 {
		inFlight = out.Queue.Depth["leased"] + out.Queue.Depth["processing"]
	}
	held := out.Realtime.GaugeByPhase[GaugeKeyWorkerHeld]
	out.HasRecentConsumerActivity = out.Consumer.RunsLastMinute > 0 || inFlight > 0 || held > 0
	out.PipelineStrip = buildPipelineStrip(ctx, ownerOrgID, orgHex, now, &out.Queue)
	return out
}

func defaultPhaseCounts() map[string]int64 {
	return map[string]int64{
		PhaseQueued:    0,
		PhaseConsuming: 0,
		PhaseSkipped:   0,
		PhaseParse:     0,
		PhaseLLM:       0,
		PhaseDecision:  0,
		PhasePolicy:    0,
		PhasePropose:   0,
		PhaseEmpty:     0,
		PhaseDone:      0,
		PhaseError:     0,
		PhaseQueueProcessing:    0,
		PhaseQueueDone:          0,
		PhaseQueueError:         0,
		PhaseDatachangedEffects: 0,
		"unknown":      0,
	}
}

type queueDepthDoc struct {
	Pending         int64 `json:"pending"`
	Leased          int64 `json:"leased"`
	Processing      int64 `json:"processing"`
	FailedRetryable int64 `json:"failed_retryable"`
	FailedTerminal  int64 `json:"failed_terminal"`
	Deferred        int64 `json:"deferred"`
	// OtherActive status không thuộc bucket chuẩn và không phải completed (legacy / typo).
	OtherActive    int64 `json:"otherActive"`
	ReconciledAtMs int64 `json:"reconciledAtMs"`
}

func applyQueueDepthDocToSnapshot(out *CommandCenterSnapshot, doc queueDepthDoc) {
	out.Queue.ReconciledAtMs = doc.ReconciledAtMs
	if out.Queue.Depth == nil {
		out.Queue.Depth = map[string]int64{}
	}
	d := out.Queue.Depth
	d["pending"] = doc.Pending
	d["leased"] = doc.Leased
	d["processing"] = doc.Processing
	d["failed_retryable"] = doc.FailedRetryable
	d["failed_terminal"] = doc.FailedTerminal
	d["deferred"] = doc.Deferred
	d["other_active"] = doc.OtherActive
	d["in_flight"] = doc.Leased + doc.Processing
}

// WSCommandCenterAggregateInterval — Khoảng thời gian gửi một bản snapshot tổng hợp qua WebSocket org-live.
func WSCommandCenterAggregateInterval() time.Duration {
	v := strings.TrimSpace(os.Getenv("AI_DECISION_WS_AGGREGATE_SEC"))
	if v == "" {
		return defaultWSAggregateInterval
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return defaultWSAggregateInterval
	}
	return time.Duration(n) * time.Second
}
