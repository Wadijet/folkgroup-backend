package decisionlive

import (
	"strings"
	"sync"
	"time"

	"meta_commerce/internal/api/aidecision/queuedepth"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	// intakeRecentCap — giới hạn mốc thời gian intake gần đây (tránh slice phình).
	intakeRecentCap = 16384
	// intakeRecentMaxAgeMs — prune mốc cũ hơn 6 phút (đủ cho cửa sổ 5 phút + dư).
	intakeRecentMaxAgeMs = 6 * 60 * 1000
)

// orgIntakeBuf đếm event **mới tạo** trong queue (sau InsertOne thành công) — không phải lúc consumer bắt đầu xử lý.
type orgIntakeBuf struct {
	mu            sync.Mutex
	byEventType   map[string]int64
	byEventSource map[string]int64
	// recentIntakeMs — thời điểm (Unix ms) mỗi lần ghi queue thành công; dùng đếm intake theo cửa sổ trên WS.
	recentIntakeMs []int64
}

var memIntake sync.Map // orgHex -> *orgIntakeBuf

// RecordCommandCenterIntake gọi sau mỗi lần ghi queue thành công (EmitEvent). Phân tách hoàn toàn với bySourceKind của timeline Publish.
func RecordCommandCenterIntake(ownerOrgID primitive.ObjectID, eventType, eventSource string) {
	if ownerOrgID.IsZero() {
		return
	}
	et := strings.TrimSpace(eventType)
	if et == "" {
		et = SourceUnknown
	}
	es := strings.TrimSpace(eventSource)
	if es == "" {
		es = SourceUnknown
	}
	orgHex := queuedepth.NormalizeOrgHex(ownerOrgID.Hex())
	v, _ := memIntake.LoadOrStore(orgHex, &orgIntakeBuf{
		byEventType:   make(map[string]int64),
		byEventSource: make(map[string]int64),
	})
	buf := v.(*orgIntakeBuf)
	buf.mu.Lock()
	if buf.byEventType == nil {
		buf.byEventType = make(map[string]int64)
		buf.byEventSource = make(map[string]int64)
	}
	buf.byEventType[et]++
	buf.byEventSource[es]++
	nowMs := time.Now().UnixMilli()
	buf.recentIntakeMs = append(buf.recentIntakeMs, nowMs)
	pruneIntakeRecentLocked(buf, nowMs)
	if len(buf.recentIntakeMs) > intakeRecentCap {
		buf.recentIntakeMs = buf.recentIntakeMs[len(buf.recentIntakeMs)-intakeRecentCap:]
	}
	nType := buf.byEventType[et]
	nSrc := buf.byEventSource[es]
	buf.mu.Unlock()

	if metricsChangeLogEnabled() {
		logrus.WithFields(logrus.Fields{
			"orgHex":       orgHex,
			"eventType":    et,
			"eventSource":  es,
			"demTheoLoai":  nType,
			"demTheoNguon": nSrc,
			"demTu":        "emit_queue",
		}).Info("AI Decision metrics: đổ vào queue (RAM) +1 — event vừa tạo")
	}
}

func pruneIntakeRecentLocked(buf *orgIntakeBuf, nowMs int64) {
	if buf == nil {
		return
	}
	cut := nowMs - intakeRecentMaxAgeMs
	i := 0
	for i < len(buf.recentIntakeMs) && buf.recentIntakeMs[i] < cut {
		i++
	}
	if i > 0 {
		buf.recentIntakeMs = append([]int64(nil), buf.recentIntakeMs[i:]...)
	}
}

// intakeWindowCountsForOrg đếm số lần intake (ghi queue OK) trong cửa sổ 60s và 5 phút — phục vụ strip WS.
func intakeWindowCountsForOrg(orgHex string, asOfMs int64) (last60s, last5m int64) {
	orgHex = queuedepth.NormalizeOrgHex(orgHex)
	if orgHex == "" || asOfMs <= 0 {
		return 0, 0
	}
	v, ok := memIntake.Load(orgHex)
	if !ok {
		return 0, 0
	}
	buf := v.(*orgIntakeBuf)
	buf.mu.Lock()
	defer buf.mu.Unlock()
	pruneIntakeRecentLocked(buf, asOfMs)
	cut1m := asOfMs - 60*1000
	cut5m := asOfMs - 5*60*1000
	for _, ts := range buf.recentIntakeMs {
		if ts >= cut5m {
			last5m++
		}
		if ts >= cut1m {
			last60s++
		}
	}
	return last60s, last5m
}

func intakeSnapshotForOrg(orgHex string) (byType, bySource map[string]int64) {
	byType = make(map[string]int64)
	bySource = make(map[string]int64)
	orgHex = queuedepth.NormalizeOrgHex(orgHex)
	if orgHex == "" {
		return byType, bySource
	}
	v, ok := memIntake.Load(orgHex)
	if !ok {
		return byType, bySource
	}
	buf := v.(*orgIntakeBuf)
	buf.mu.Lock()
	defer buf.mu.Unlock()
	for k, n := range buf.byEventType {
		byType[k] = n
	}
	for k, n := range buf.byEventSource {
		bySource[k] = n
	}
	return byType, bySource
}
