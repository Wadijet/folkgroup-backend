package decisionlive

import (
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// orgIntakeBuf đếm event **mới tạo** trong queue (sau InsertOne thành công) — không phải lúc consumer bắt đầu xử lý.
type orgIntakeBuf struct {
	mu           sync.Mutex
	byEventType  map[string]int64
	byEventSource map[string]int64
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
	orgHex := ownerOrgID.Hex()
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

func intakeSnapshotForOrg(orgHex string) (byType, bySource map[string]int64) {
	byType = make(map[string]int64)
	bySource = make(map[string]int64)
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
