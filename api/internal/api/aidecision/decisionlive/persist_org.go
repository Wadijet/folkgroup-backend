package decisionlive

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"meta_commerce/internal/global"
)

// orgPersistEnabled — Bật ghi/đọc Mongo decision_org_live_events (config AIDecisionLiveOrgPersist hoặc env AI_DECISION_LIVE_ORG_PERSIST).
// Tắt: chỉ ring RAM process; OrgTimelineForAPI không đọc Mongo; ListPersistedOrgLiveEventsFromMongo trả lỗi ErrOrgLivePersistDisabled.
func orgPersistEnabled() bool {
	if global.MongoDB_ServerConfig != nil {
		return global.MongoDB_ServerConfig.AIDecisionLiveOrgPersist
	}
	v := strings.TrimSpace(os.Getenv("AI_DECISION_LIVE_ORG_PERSIST"))
	return v == "1" || strings.EqualFold(v, "true") || v == "yes"
}

// persistOrgLiveEventAsync — Publish bước 7: ghi Mongo decision_org_live_events (chỉ khi live bật — hàm chỉ gọi từ nhánh đó).
//
//	Bước 1 — orgPersistEnabled() false hoặc org zero → return (không ghi).
//	Bước 2 — Không có collection trong registry → return.
//	Bước 3 — Sinh _id Mongo, createdAt ms, BuildOrgLivePersistDocument(owner, id, createdAt, orgEv).
//	Bước 4 — Goroutine InsertOne timeout 5s; lỗi chỉ log (GET timeline vẫn có thể dùng RAM).
func persistOrgLiveEventAsync(ownerOrgID primitive.ObjectID, ev DecisionLiveEvent) {
	// Bước 1
	if !orgPersistEnabled() || ownerOrgID.IsZero() {
		return
	}
	// Bước 2
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AIDecisionOrgLiveEvents)
	if !ok || coll == nil {
		return
	}
	// Bước 3
	docID := primitive.NewObjectID()
	createdAt := time.Now().UnixMilli()
	doc := BuildOrgLivePersistDocument(ownerOrgID, docID, createdAt, ev)
	// Bước 4
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.WithField("panic", r).Warn("AI Decision org-live: ghi Mongo panic (bỏ qua)")
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := coll.InsertOne(ctx, doc); err != nil {
			logrus.WithError(err).Debug("AI Decision org-live: InsertOne Mongo thất bại (GET vẫn dùng RAM nếu có)")
		}
	}()
}

// OrgTimelineForAPI — Replay org-live cho HTTP/WS (thứ tự giống ring RAM sau khi đảo chiều Mongo).
//
//	Bước 1 — Persist org tắt hoặc không có collection: snapshotOrg (RAM) → backfill → return.
//	Bước 2 — Persist bật: Find Mongo (mới nhất trước, giới hạn MaxEventsPerOrgFeed), unmarshal payload.
//	Bước 3 — Find lỗi / kết quả rỗng: fallback như bước 1.
//	Bước 4 — Có dữ liệu Mongo: đảo slice về thời gian tăng (khớp thứ tự ring).
//	Bước 5 — backfillLiveEventsDerivedFields (opsTier, feed…) rồi return.
func OrgTimelineForAPI(ctx context.Context, ownerOrgID primitive.ObjectID) []DecisionLiveEvent {
	var out []DecisionLiveEvent
	if !orgPersistEnabled() {
		out = globalOrgFeed.snapshotOrg(ownerOrgID)
		backfillLiveEventsDerivedFields(out)
		return out
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AIDecisionOrgLiveEvents)
	if !ok || coll == nil {
		out = globalOrgFeed.snapshotOrg(ownerOrgID)
		backfillLiveEventsDerivedFields(out)
		return out
	}
	qctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	cur, err := coll.Find(qctx, bson.M{"ownerOrganizationId": ownerOrgID}, options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetLimit(int64(MaxEventsPerOrgFeed)))
	if err != nil {
		logrus.WithError(err).Debug("AI Decision org-live: Find Mongo thất bại, fallback RAM")
		out = globalOrgFeed.snapshotOrg(ownerOrgID)
		backfillLiveEventsDerivedFields(out)
		return out
	}
	defer cur.Close(qctx)
	var desc []DecisionLiveEvent
	for cur.Next(qctx) {
		var row struct {
			Payload []byte `bson:"payload"`
		}
		if err := cur.Decode(&row); err != nil {
			continue
		}
		var ev DecisionLiveEvent
		if err := json.Unmarshal(row.Payload, &ev); err != nil {
			continue
		}
		desc = append(desc, ev)
	}
	if len(desc) == 0 {
		out = globalOrgFeed.snapshotOrg(ownerOrgID)
		backfillLiveEventsDerivedFields(out)
		return out
	}
	// Đảo lại thành thời gian tăng (giống thứ tự ring RAM).
	for i, j := 0, len(desc)-1; i < j; i, j = i+1, j-1 {
		desc[i], desc[j] = desc[j], desc[i]
	}
	backfillLiveEventsDerivedFields(desc)
	return desc
}

// ErrOrgLivePersistDisabled persist org-live Mongo đang tắt — không đọc được decision_org_live_events.
var ErrOrgLivePersistDisabled = errors.New("org-live persist Mongo đang tắt — bật AI_DECISION_LIVE_ORG_PERSIST")

const (
	persistedOrgLiveDefaultLimit = 50
	persistedOrgLiveMaxLimit     = 100
)

// PersistedOrgLiveListFilter — Tham số GET persisted-events: owner org bắt buộc; traceId/decisionCaseId/createdAt khớp trường phẳng trên document (mục 4.7 THIET_KE).
type PersistedOrgLiveListFilter struct {
	OwnerOrgID     primitive.ObjectID
	Page           int
	Limit          int
	TraceID        string
	DecisionCaseID string
	FromCreatedMs  *int64
	ToCreatedMs    *int64
}

// ListPersistedOrgLiveEventsFromMongo — GET /org-live/persisted-events: chỉ Mongo, không fallback RAM.
//
//	Bước 1 — Kiểm tra persist bật + org + collection.
//	Bước 2 — filter ownerOrganizationId + tùy chọn traceId, decisionCaseId, createdAt range.
//	Bước 3 — CountDocuments; Find sort createdAt giảm, skip/limit.
//	Bước 4 — Mỗi row: decode payload → json.Unmarshal → DecisionLiveEvent; sau đó backfillLiveEventsDerivedFields (đồng bộ với OrgTimelineForAPI).
func ListPersistedOrgLiveEventsFromMongo(ctx context.Context, f PersistedOrgLiveListFilter) ([]DecisionLiveEvent, int64, error) {
	if !orgPersistEnabled() {
		return nil, 0, ErrOrgLivePersistDisabled
	}
	if f.OwnerOrgID.IsZero() {
		return nil, 0, errors.New("ownerOrganizationId bắt buộc")
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AIDecisionOrgLiveEvents)
	if !ok || coll == nil {
		return nil, 0, errors.New("không tìm thấy collection decision_org_live_events")
	}
	page := f.Page
	if page < 1 {
		page = 1
	}
	limit := f.Limit
	if limit < 1 {
		limit = persistedOrgLiveDefaultLimit
	}
	if limit > persistedOrgLiveMaxLimit {
		limit = persistedOrgLiveMaxLimit
	}
	filter := bson.M{"ownerOrganizationId": f.OwnerOrgID}
	if t := strings.TrimSpace(f.TraceID); t != "" {
		filter["traceId"] = t
	}
	if t := strings.TrimSpace(f.DecisionCaseID); t != "" {
		filter["decisionCaseId"] = t
	}
	if f.FromCreatedMs != nil || f.ToCreatedMs != nil {
		rng := bson.M{}
		if f.FromCreatedMs != nil {
			rng["$gte"] = *f.FromCreatedMs
		}
		if f.ToCreatedMs != nil {
			rng["$lte"] = *f.ToCreatedMs
		}
		filter["createdAt"] = rng
	}
	total, err := coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []DecisionLiveEvent{}, 0, nil
	}
	skip := int64(page-1) * int64(limit)
	if skip < 0 {
		skip = 0
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(limit))
	cur, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var out []DecisionLiveEvent
	for cur.Next(ctx) {
		var row struct {
			Payload []byte `bson:"payload"`
		}
		if err := cur.Decode(&row); err != nil {
			continue
		}
		var ev DecisionLiveEvent
		if err := json.Unmarshal(row.Payload, &ev); err != nil {
			continue
		}
		out = append(out, ev)
	}
	// Cùng enrich như OrgTimelineForAPI: bản ghi cũ thiếu businessDomain / uiTitle / phaseLabelVi trên payload vẫn đủ field cho UI.
	backfillLiveEventsDerivedFields(out)
	return out, total, cur.Err()
}
