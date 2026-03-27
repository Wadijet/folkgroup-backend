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
	// StreamAIDecision tên luồng cố định cho client.
	StreamAIDecision = "aidecision.live"
	// MaxEventsPerTrace giới hạn ring replay trên bộ nhớ (mỗi org+trace).
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

// orgChannelKey khóa fan-out cho toàn bộ trace trong một tổ chức (màn hình live stream).
func orgChannelKey(ownerOrgID primitive.ObjectID) string {
	return ownerOrgID.Hex() + ":__org_feed__"
}

// liveEnabled mặc định bật; AI_DECISION_LIVE_ENABLED=0 tắt ring/WS (metrics trung tâm chỉ huy vẫn chạy).
func liveEnabled() bool {
	v := strings.TrimSpace(os.Getenv("AI_DECISION_LIVE_ENABLED"))
	return v == "" || v == "1"
}

var livePublishDisabledLogged sync.Once

// Publish: live bật — ring + WebSocket + persist; live tắt — chỉ cập nhật metrics (byPhase, bySourceKind, gauge).
func Publish(ownerOrgID primitive.ObjectID, traceID string, ev DecisionLiveEvent) {
	if traceID == "" || ownerOrgID.IsZero() {
		if metricsChangeLogEnabled() {
			logrus.WithFields(logrus.Fields{
				"orgHex":  ownerOrgID.Hex(),
				"traceId": traceID,
			}).Warn("AI Decision live: bỏ qua Publish — trace/org rỗng; không tăng metrics phễu")
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
				"AI Decision live: AI_DECISION_LIVE_ENABLED=0 — WS/ring tắt; " +
					"trung tâm chỉ huy vẫn nhận phase/gauge. Đặt =1 nếu cần timeline/WS.",
			)
		})
		if metricsChangeLogEnabled() {
			logrus.WithFields(logrus.Fields{
				"orgHex":  ownerOrgID.Hex(),
				"traceId": traceID,
				"phase":   ev.Phase,
			}).Info("AI Decision live: chỉ cập nhật metrics (live tắt)")
		}
		recordCommandCenterPublish(ownerOrgID, ev, "publish_chi_metrics")
		return
	}

	if metricsChangeLogEnabled() {
		logrus.WithFields(logrus.Fields{
			"orgHex":  ownerOrgID.Hex(),
			"traceId": traceID,
			"phase":   ev.Phase,
		}).Info("AI Decision live: Publish — cộng metrics phễu + fan-out WS")
	}
	final := globalStore.append(ownerOrgID, traceID, ev)
	RecordCommandCenterPublish(ownerOrgID, final)
	globalHub.broadcast(channelKey(ownerOrgID, traceID), final)
	orgEv := globalOrgFeed.appendOrg(ownerOrgID, final)
	globalHub.broadcast(orgChannelKey(ownerOrgID), orgEv)
	persistOrgLiveEventAsync(ownerOrgID, orgEv)
}

// Timeline trả snapshot replay (GET hoặc gửi ngay sau khi mở WS).
func Timeline(ownerOrgID primitive.ObjectID, traceID string) []DecisionLiveEvent {
	if traceID == "" || ownerOrgID.IsZero() {
		return nil
	}
	out := globalStore.snapshot(ownerOrgID, traceID)
	backfillLiveEventsDerivedFields(out)
	return out
}

// Subscribe đăng ký nhận event sau thời điểm hiện tại (replay dùng Timeline trước).
func Subscribe(ownerOrgID primitive.ObjectID, traceID string) (<-chan DecisionLiveEvent, func()) {
	if traceID == "" || ownerOrgID.IsZero() {
		ch := make(chan DecisionLiveEvent)
		close(ch)
		return ch, func() {}
	}
	return globalHub.subscribe(channelKey(ownerOrgID, traceID))
}

// OrgTimeline replay buffer live theo tổ chức (GET hoặc mở WS org-live).
func OrgTimeline(ownerOrgID primitive.ObjectID) []DecisionLiveEvent {
	out := globalOrgFeed.snapshotOrg(ownerOrgID)
	backfillLiveEventsDerivedFields(out)
	return out
}

// SubscribeOrg đăng ký mọi sự kiện AI Decision của org (mọi trace).
func SubscribeOrg(ownerOrgID primitive.ObjectID) (<-chan DecisionLiveEvent, func()) {
	if ownerOrgID.IsZero() {
		ch := make(chan DecisionLiveEvent)
		close(ch)
		return ch, func() {}
	}
	return globalHub.subscribe(orgChannelKey(ownerOrgID))
}
