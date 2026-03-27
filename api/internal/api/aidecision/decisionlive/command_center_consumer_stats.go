package decisionlive

import (
	"strings"
	"sync"
	"time"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Giới hạn số mốc hoàn tất giữ trong RAM (tránh slice phình vô hạn).
const consumerRecentCap = 4096

// CommandCenterConsumerMetrics — consumer decision_events_queue: đếm chạy, theo loại event, thời gian trung bình, throughput gần đây.
// Bổ sung cho phễu Publish (byPhase): mọi lần lease→xử lý xong đều ghi vào đây.
type CommandCenterConsumerMetrics struct {
	TotalCompleted int64 `json:"totalCompleted"`
	// TotalCompletedNoHandler — đóng job nhưng chưa có handler đăng ký (cần bổ sung logic).
	TotalCompletedNoHandler int64 `json:"totalCompletedNoHandler"`
	// TotalCompletedRoutingSkipped — rule routing noop (không dispatch handler).
	TotalCompletedRoutingSkipped int64 `json:"totalCompletedRoutingSkipped"`
	TotalFailed    int64 `json:"totalFailed"`
	// LastActivityMs — lần hoàn tất (ok hoặc fail) gần nhất (Unix ms).
	LastActivityMs int64 `json:"lastActivityMs"`
	// RunsLastMinute / RunsLast5Minutes — số lần hoàn tất trong cửa sổ (nhìn từ asOfMs).
	RunsLastMinute   int64 `json:"runsLastMinute"`
	RunsLast5Minutes int64 `json:"runsLast5Minutes"`
	// ByEventType — key = event_type từ queue (vd. datachanged, execute_requested).
	ByEventType map[string]ConsumerEventTypeMetrics `json:"byEventType"`
}

// ConsumerEventTypeMetrics thống kê theo một event_type.
type ConsumerEventTypeMetrics struct {
	Completed       int64 `json:"completed"`
	CompletedProcessed      int64 `json:"completedProcessed"`
	CompletedNoHandler        int64 `json:"completedNoHandler"`
	CompletedRoutingSkipped   int64 `json:"completedRoutingSkipped"`
	Failed          int64 `json:"failed"`
	TotalSuccessMs  int64 `json:"totalSuccessMs"`
	TotalFailMs     int64 `json:"totalFailMs"`
	AvgSuccessMs    int64 `json:"avgSuccessMs"`
	AvgFailMs       int64 `json:"avgFailMs"`
	LastCompletedMs int64 `json:"lastCompletedMs"`
}

type consumerTypeAgg struct {
	completed       int64
	completedProcessed int64
	completedNoHandler int64
	completedRoutingSkipped int64
	failed          int64
	sumSuccessMs    int64
	sumFailMs       int64
	lastCompletedMs int64
}

type orgConsumerBuf struct {
	mu sync.Mutex
	// Hoàn tất gần đây (Unix ms), đã prune < 6 phút khi ghi; dùng đếm throughput.
	recentEnds []int64
	totalOK    int64
	totalNoHandler int64
	totalRoutingSkipped int64
	totalFail  int64
	lastActMs  int64
	byType     map[string]*consumerTypeAgg
}

var memConsumer sync.Map // orgHex -> *orgConsumerBuf

// countCompletionsSince đếm số mốc ts >= asOfMs - windowMs (dùng cho snapshot & test).
func countCompletionsSince(ends []int64, asOfMs, windowMs int64) int64 {
	if windowMs <= 0 {
		return 0
	}
	cut := asOfMs - windowMs
	var n int64
	for _, ts := range ends {
		if ts >= cut {
			n++
		}
	}
	return n
}

// RecordConsumerCompletion ghi nhận mỗi lần consumer xử lý xong một event đã lease (thành công hoặc fail).
// completionKind: khi ok=true — processed | no_handler | routing_skipped (theo aidecisionmodels.ConsumerCompletionKind); khi ok=false bỏ qua.
// durationMs: thời gian từ sau lease đến CompleteEvent/FailEvent.
// Event không trace: gauge consuming đã + lúc RecordConsumerWorkBegin; lúc này gauge − (recordConsumerWorkEndGauge) + phễu lũy kế done/error.
func RecordConsumerCompletion(ownerOrgID primitive.ObjectID, eventType, traceID string, ok bool, durationMs int64, completionKind aidecisionmodels.ConsumerCompletionKind) {
	if ownerOrgID.IsZero() {
		return
	}
	if durationMs < 0 {
		durationMs = 0
	}
	et := strings.TrimSpace(eventType)
	if et == "" {
		et = SourceUnknown
	}
	orgHex := ownerOrgID.Hex()
	nowMs := time.Now().UnixMilli()
	trace := strings.TrimSpace(traceID)

	v, _ := memConsumer.LoadOrStore(orgHex, &orgConsumerBuf{byType: make(map[string]*consumerTypeAgg)})
	buf := v.(*orgConsumerBuf)
	buf.mu.Lock()
	if buf.byType == nil {
		buf.byType = make(map[string]*consumerTypeAgg)
	}
	agg, okAgg := buf.byType[et]
	if !okAgg {
		agg = &consumerTypeAgg{}
		buf.byType[et] = agg
	}
	buf.lastActMs = nowMs
	if ok {
		buf.totalOK++
		agg.completed++
		agg.sumSuccessMs += durationMs
		agg.lastCompletedMs = nowMs
		switch completionKind {
		case aidecisionmodels.ConsumerCompletionKindNoHandler:
			buf.totalNoHandler++
			agg.completedNoHandler++
		case aidecisionmodels.ConsumerCompletionKindRoutingSkipped:
			buf.totalRoutingSkipped++
			agg.completedRoutingSkipped++
		default:
			agg.completedProcessed++
		}
	} else {
		buf.totalFail++
		agg.failed++
		agg.sumFailMs += durationMs
	}
	cutoff := nowMs - 6*60*1000
	r := buf.recentEnds
	off := 0
	for off < len(r) && r[off] < cutoff {
		off++
	}
	if off > 0 {
		r = append([]int64(nil), r[off:]...)
	}
	r = append(r, nowMs)
	if len(r) > consumerRecentCap {
		r = r[len(r)-consumerRecentCap:]
	}
	buf.recentEnds = r
	buf.mu.Unlock()

	recordConsumerWorkEndGauge(orgHex, trace)
	if trace == "" {
		// Chỉ lũy kế phase; không bump bySourceKind — nguồn đổ vào dùng intakeByEvent* (EmitEvent).
		if ok {
			incrementMemLivePublish(orgHex, PhaseDone, et, "consumer_hoan_tat_khong_trace", false, "", "")
		} else {
			incrementMemLivePublish(orgHex, PhaseError, et, "consumer_loi_khong_trace", false, "", "")
		}
	}
}

// consumerSnapshotForOrg đọc RAM consumer cho một org (gọi từ BuildCommandCenterSnapshot).
func consumerSnapshotForOrg(orgHex string, asOfMs int64) CommandCenterConsumerMetrics {
	out := CommandCenterConsumerMetrics{
		ByEventType: make(map[string]ConsumerEventTypeMetrics),
	}
	if orgHex == "" {
		return out
	}
	v, ok := memConsumer.Load(orgHex)
	if !ok {
		return out
	}
	buf := v.(*orgConsumerBuf)
	buf.mu.Lock()
	defer buf.mu.Unlock()
	out.TotalCompleted = buf.totalOK
	out.TotalCompletedNoHandler = buf.totalNoHandler
	out.TotalCompletedRoutingSkipped = buf.totalRoutingSkipped
	out.TotalFailed = buf.totalFail
	out.LastActivityMs = buf.lastActMs
	out.RunsLastMinute = countCompletionsSince(buf.recentEnds, asOfMs, 60*1000)
	out.RunsLast5Minutes = countCompletionsSince(buf.recentEnds, asOfMs, 5*60*1000)
	for et, a := range buf.byType {
		m := ConsumerEventTypeMetrics{
			Completed:       a.completed,
			CompletedProcessed:      a.completedProcessed,
			CompletedNoHandler:        a.completedNoHandler,
			CompletedRoutingSkipped:   a.completedRoutingSkipped,
			Failed:          a.failed,
			TotalSuccessMs:  a.sumSuccessMs,
			TotalFailMs:     a.sumFailMs,
			LastCompletedMs: a.lastCompletedMs,
		}
		if a.completed > 0 {
			m.AvgSuccessMs = a.sumSuccessMs / a.completed
		}
		if a.failed > 0 {
			m.AvgFailMs = a.sumFailMs / a.failed
		}
		out.ByEventType[et] = m
	}
	return out
}
