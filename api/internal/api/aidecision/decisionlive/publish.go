package decisionlive

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	// StreamAIDecision — Tên kênh WebSocket / schema cho luồng live AI Decision (client đăng ký theo trace hoặc theo org).
	StreamAIDecision = "aidecision.live"
	// MaxEventsPerTrace — Số sự kiện tối đa lưu trong bộ nhớ cho mỗi cặp (org, trace) khi replay timeline.
	MaxEventsPerTrace = 256
)

var (
	globalHub     = newHub()
	globalStore   = newTraceStore()
	globalOrgFeed = newOrgFeedStore()
)

func channelKey(ownerOrgID primitive.ObjectID, traceID string) string {
	return ownerOrgID.Hex() + ":" + traceID
}

// orgChannelKey — Khóa broadcast: mọi trace của một tổ chức (màn hình «live toàn org»).
func orgChannelKey(ownerOrgID primitive.ObjectID) string {
	return ownerOrgID.Hex() + ":__org_feed__"
}

// liveEnabled — Mặc định bật. AI_DECISION_LIVE_ENABLED=0: không ghi ring / không đẩy WebSocket; vẫn cộng metrics trung tâm chỉ huy.
func liveEnabled() bool {
	v := strings.TrimSpace(os.Getenv("AI_DECISION_LIVE_ENABLED"))
	return v == "" || v == "1"
}

var livePublishDisabledLogged sync.Once

// Publish — Đưa một DecisionLiveEvent vào hệ thống live:
// bật live → lưu ring replay, fan-out WebSocket (theo trace và theo org), ghi persist bất đồng bộ;
// tắt live → chỉ cập nhật bộ đếm / gauge trung tâm chỉ huy (không làm đầy ring).
func Publish(ownerOrgID primitive.ObjectID, traceID string, ev DecisionLiveEvent) {
	if traceID == "" || ownerOrgID.IsZero() {
		if metricsChangeLogEnabled() {
			logrus.WithFields(logrus.Fields{
				"orgHex":  ownerOrgID.Hex(),
				"traceId": traceID,
			}).Warn("AI Decision live: bỏ qua Publish — thiếu trace hoặc org; không cộng bộ đếm phễu")
		}
		return
	}
	if ev.SchemaVersion == 0 {
		ev.SchemaVersion = 1
	}
	if ev.Stream == "" {
		ev.Stream = StreamAIDecision
	}
	ev.TraceID = traceID
	ev.OrgIDHex = ownerOrgID.Hex()
	if ev.TsMs == 0 {
		ev.TsMs = time.Now().UnixMilli()
	}
	if ev.Severity == "" {
		ev.Severity = SeverityInfo
	}
	enrichW3CTraceContext(&ev, traceID)
	enrichLiveEventOpsTier(&ev)
	enrichLiveEventFeedSource(&ev)

	if !liveEnabled() {
		livePublishDisabledLogged.Do(func() {
			logrus.Warn(
				"AI Decision live: AI_DECISION_LIVE_ENABLED=0 — WebSocket và vòng replay tắt; " +
					"trung tâm chỉ huy vẫn nhận phase và gauge. Đặt =1 nếu cần timeline và WS.",
			)
		})
		if metricsChangeLogEnabled() {
			logrus.WithFields(logrus.Fields{
				"orgHex":  ownerOrgID.Hex(),
				"traceId": traceID,
				"phase":   ev.Phase,
			}).Info("AI Decision live: chỉ cập nhật bộ đếm / gauge (chế độ live tắt)")
		}
		recordCommandCenterPublish(ownerOrgID, ev, "publish_chi_metrics")
		return
	}

	if metricsChangeLogEnabled() {
		logrus.WithFields(logrus.Fields{
			"orgHex":  ownerOrgID.Hex(),
			"traceId": traceID,
			"phase":   ev.Phase,
		}).Info("AI Decision live: Publish — cộng bộ đếm phễu và đẩy WebSocket")
	}
	final := globalStore.append(ownerOrgID, traceID, ev)
	RecordCommandCenterPublish(ownerOrgID, final)
	globalHub.broadcast(channelKey(ownerOrgID, traceID), final)
	orgEv := globalOrgFeed.appendOrg(ownerOrgID, final)
	globalHub.broadcast(orgChannelKey(ownerOrgID), orgEv)
	persistOrgLiveEventAsync(ownerOrgID, orgEv)
}

// Timeline — Trả về bản sao các sự kiện đã publish cho một trace (GET replay hoặc nối sau khi mở WS).
func Timeline(ownerOrgID primitive.ObjectID, traceID string) []DecisionLiveEvent {
	if traceID == "" || ownerOrgID.IsZero() {
		return nil
	}
	out := globalStore.snapshot(ownerOrgID, traceID)
	backfillLiveEventsDerivedFields(out)
	return out
}

// Subscribe — Đăng ký kênh nhận sự kiện realtime cho một trace (client nên gọi Timeline trước để không lỡ mốc cũ).
func Subscribe(ownerOrgID primitive.ObjectID, traceID string) (<-chan DecisionLiveEvent, func()) {
	if traceID == "" || ownerOrgID.IsZero() {
		ch := make(chan DecisionLiveEvent)
		close(ch)
		return ch, func() {}
	}
	return globalHub.subscribe(channelKey(ownerOrgID, traceID))
}

// OrgTimeline — Buffer replay gộp mọi trace trong một tổ chức (GET hoặc WS org-live).
func OrgTimeline(ownerOrgID primitive.ObjectID) []DecisionLiveEvent {
	out := globalOrgFeed.snapshotOrg(ownerOrgID)
	backfillLiveEventsDerivedFields(out)
	return out
}

// SubscribeOrg — Đăng ký nhận mọi sự kiện live của org (không lọc theo trace).
func SubscribeOrg(ownerOrgID primitive.ObjectID) (<-chan DecisionLiveEvent, func()) {
	if ownerOrgID.IsZero() {
		ch := make(chan DecisionLiveEvent)
		close(ch)
		return ch, func() {}
	}
	return globalHub.subscribe(orgChannelKey(ownerOrgID))
}
