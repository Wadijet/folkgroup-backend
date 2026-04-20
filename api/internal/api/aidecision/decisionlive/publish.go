package decisionlive

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"meta_commerce/internal/api/aidecision/eventtypes"
)

const (
	// StreamAIDecision — Tên kênh WebSocket / schema cho luồng live AI Decision (client đăng ký theo trace hoặc theo org).
	StreamAIDecision = "aidecision.live"
	// MaxEventsPerTrace — Số sự kiện tối đa lưu trong bộ nhớ cho mỗi cặp (org, trace) khi replay timeline (chuỗi span/parent nối trong cửa sổ này).
	MaxEventsPerTrace = 2048
)

var (
	globalHub     = newHub()
	globalStore   = newTraceStore()
	globalOrgFeed = newOrgFeedStore()
)

// channelKey — Khóa ring + WS một trace (orgHex:traceId); Subscribe/Timeline/broadcast trace dùng chung.
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

// Publish — đẩy một mốc timeline (DecisionLiveEvent) vào live: cùng traceId với luồng E2E (G1 pha ghi thô → G2–G6 theo miền).
//
// Tóm tắt: (0) bỏ qua nếu thiếu org/trace — (1) envelope + enrich (tier, feed, E2E Gx-Syy qua enrichPublishE2ERef) —
// (2) nếu live tắt: W3C span (không nối parent) + chỉ metrics CHI — (3) nếu bật: append ring (nối parentSpanId + W3C) → metrics → WS (trace + org feed) → persist org-live async.
//
// Khác EmitEvent/intake: Publish chỉ hiển thị/ghi timeline, không tạo job queue.
// Chi tiết: THIET_KE… §4.5–4.7; bảng bước E2E: docs/flows/bang-pha-buoc-event-e2e.md.
func Publish(ownerOrgID primitive.ObjectID, traceID string, ev DecisionLiveEvent) {
	// Bước 0 — Điều kiện bắt buộc: mọi fan-out và phễu theo trace đều cần cặp (org, trace).
	if traceID == "" || ownerOrgID.IsZero() {
		if metricsChangeLogEnabled() {
			logrus.WithFields(logrus.Fields{
				"orgHex":  ownerOrgID.Hex(),
				"traceId": traceID,
			}).Warn("AI Decision live: bỏ qua Publish — thiếu trace hoặc org; không cộng bộ đếm phễu")
		}
		return
	}

	// Bước 1 — Chuẩn hóa envelope trước enrich và metrics.
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

	// Bước 2 — Làm giàu: tier vận hành, nguồn feed, E2E (w3cTraceId/spanId/parentSpanId gắn ở append khi live bật để nối chuỗi span đúng thứ tự).
	enrichLiveEventOpsTier(&ev)
	enrichLiveEventFeedSource(&ev)
	enrichLiveBusinessDomain(&ev)
	enrichPublishE2ERef(&ev)
	enrichPublishHandoffNarrative(&ev)
	CapDecisionLiveProcessTrace(&ev)
	EnrichLiveOutcomeMetadata(&ev)
	enrichLiveEventUIPresentation(&ev)

	// Bước 3a — Live tắt: chỉ cập nhật trung tâm chỉ huy (publishCounters + gauge), không đụng ring/WS/persist; không có ring nên không nối parentSpanId.
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
		enrichW3CTraceContext(&ev, traceID)
		recordCommandCenterPublish(ownerOrgID, ev, "publish_chi_metrics")
		return
	}

	// Bước 3b trở đi — Live bật: ring + metrics đầy đủ + WS + persist (nếu bật).
	if metricsChangeLogEnabled() {
		logrus.WithFields(logrus.Fields{
			"orgHex":  ownerOrgID.Hex(),
			"traceId": traceID,
			"phase":   ev.Phase,
		}).Info("AI Decision live: Publish — cộng bộ đếm phễu và đẩy WebSocket")
	}

	// Bước 4 — Ring RAM theo trace (replay REST/WS); final mang Seq do append gán.
	final := globalStore.append(ownerOrgID, traceID, ev)

	// Bước 5 — Cùng hàm lõi metrics với nhánh tắt nhưng nhãn demTu = publish_ws (phân biệt nguồn bump).
	RecordCommandCenterPublish(ownerOrgID, final)

	// Bước 6a — Subscriber WS theo trace (khóa orgHex:traceId).
	globalHub.broadcast(channelKey(ownerOrgID, traceID), final)

	// Bước 6b — Feed gộp org + broadcast màn org-live (orgEv có thể khác final ở field dẫn xuất).
	orgEv := globalOrgFeed.appendOrg(ownerOrgID, final)
	globalHub.broadcast(orgChannelKey(ownerOrgID), orgEv)

	// Bước 7 — decision_org_live_events: InsertOne async (org persist bật); document = BuildOrgLivePersistDocument(..., orgEv).
	persistOrgLiveEventAsync(ownerOrgID, orgEv)
}

// Timeline trả về bản sao ring replay một trace (đọc ngược với Publish bước 4 — cùng globalStore).
//
//	Luồng: (1) snapshot (org, trace) từ RAM → (2) backfill opsTier/feed cho bản ghi cũ.
//	Chỉ có dữ liệu sau các lần Publish (live bật); thiếu org/trace → nil.
func Timeline(ownerOrgID primitive.ObjectID, traceID string) []DecisionLiveEvent {
	if traceID == "" || ownerOrgID.IsZero() {
		return nil
	}
	out := globalStore.snapshot(ownerOrgID, traceID)
	backfillLiveEventsDerivedFields(out)
	return out
}

// Subscribe mở kênh WS nội bộ cho trace (khóa orgHex:traceId — cùng kênh Publish bước 6a broadcast).
//
//	Client WS khuyến nghị: Subscribe trước → Timeline replay → gửi replay → drain trùng Seq → đọc liveCh (handler HandleTraceLiveWS).
func Subscribe(ownerOrgID primitive.ObjectID, traceID string) (<-chan DecisionLiveEvent, func()) {
	if traceID == "" || ownerOrgID.IsZero() {
		ch := make(chan DecisionLiveEvent)
		close(ch)
		return ch, func() {}
	}
	return globalHub.subscribe(channelKey(ownerOrgID, traceID))
}

// OrgTimeline đọc ring RAM gộp mọi trace trong org (chưa qua Mongo). Dùng nhanh nội bộ; API HTTP thường dùng OrgTimelineForAPI.
//
//	Luồng: snapshotOrg → backfill (giống Timeline nhưng theo FeedSeq / buffer org).
func OrgTimeline(ownerOrgID primitive.ObjectID) []DecisionLiveEvent {
	out := globalOrgFeed.snapshotOrg(ownerOrgID)
	backfillLiveEventsDerivedFields(out)
	return out
}

// SubscribeOrg đăng ký kênh org (__org_feed__) — mọi Publish bước 6b broadcast vào đây (không lọc trace).
func SubscribeOrg(ownerOrgID primitive.ObjectID) (<-chan DecisionLiveEvent, func()) {
	if ownerOrgID.IsZero() {
		ch := make(chan DecisionLiveEvent)
		close(ch)
		return ch, func() {}
	}
	return globalHub.subscribe(orgChannelKey(ownerOrgID))
}

// enrichPublishE2ERef gắn tham chiếu luồng chuẩn (docs/flows/bang-pha-buoc-event-e2e.md) và chèn một dòng mô tả E2E vào đầu DetailBullets (nếu có).
func enrichPublishE2ERef(ev *DecisionLiveEvent) {
	if ev == nil {
		return
	}
	ref := resolveE2ERefForPublish(ev)
	if strings.TrimSpace(ev.E2EStepID) == "" {
		applyE2ERefToLiveEvent(ev, ref)
	} else {
		ref = eventtypes.E2ERef{
			Stage:   strings.TrimSpace(ev.E2EStage),
			StepID:  strings.TrimSpace(ev.E2EStepID),
			LabelVi: strings.TrimSpace(ev.E2EStepLabelVi),
		}
	}
	if ref.StepID != "" && !detailBulletsHaveE2EPrefix(ev.DetailBullets) {
		ev.DetailBullets = prependE2EPublishNarrative(ev.DetailBullets, ref)
	}
}

// resolveE2ERefForPublish — bám §5.3 / E2ELivePhaseCatalog: nếu field phase map được một bước catalog đầy đủ (stageId + stepId), ưu tiên ResolveE2EForLivePhase; ngược lại mới dùng envelope queue trong Refs (eventType…), cuối cùng lại phase (nhãn thiếu map / phase rỗng).
func resolveE2ERefForPublish(ev *DecisionLiveEvent) eventtypes.E2ERef {
	if ev == nil {
		return eventtypes.E2ERef{}
	}
	phase := strings.TrimSpace(ev.Phase)
	if phase != "" {
		liveRef := eventtypes.ResolveE2EForLivePhase(phase)
		if liveRef.Stage != "" && liveRef.StepID != "" {
			return liveRef
		}
	}
	if ev.Refs != nil {
		et := strings.TrimSpace(ev.Refs["eventType"])
		if et != "" {
			ref := eventtypes.ResolveE2EForQueueEnvelope(et, ev.Refs["eventSource"], ev.Refs["pipelineStage"])
			if ref.Stage != "" {
				return ref
			}
		}
	}
	return eventtypes.ResolveE2EForLivePhase(phase)
}

func detailBulletsHaveE2EPrefix(bullets []string) bool {
	if len(bullets) == 0 {
		return false
	}
	return eventtypes.IsLiveDetailBulletE2ENarrative(bullets[0])
}

// prependE2EPublishNarrative chèn một dòng ngắn (Gx-Syy + nhãn) thân thiện người dùng; mã E2E vẫn nằm trong refs/e2e*.
func prependE2EPublishNarrative(bullets []string, ref eventtypes.E2ERef) []string {
	line := eventtypes.ResolveLiveE2EPublishNarrative(ref)
	return append([]string{line}, bullets...)
}

func applyE2ERefToLiveEvent(ev *DecisionLiveEvent, ref eventtypes.E2ERef) {
	ev.E2EStage = ref.Stage
	ev.E2EStepID = ref.StepID
	ev.E2EStepLabelVi = ref.LabelVi
	if ev.Refs == nil {
		ev.Refs = make(map[string]string)
	}
	if ref.Stage != "" {
		ev.Refs[eventtypes.E2EPayloadKeyStage] = ref.Stage
	}
	if ref.StepID != "" {
		ev.Refs[eventtypes.E2EPayloadKeyStepID] = ref.StepID
	}
	if ref.LabelVi != "" {
		ev.Refs[eventtypes.E2EPayloadKeyLabelVi] = ref.LabelVi
	}
}
