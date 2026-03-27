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

// orgPersistEnabled: mặc định bật qua config (AIDecisionLiveOrgPersist / AI_DECISION_LIVE_ORG_PERSIST).
// Tắt: AI_DECISION_LIVE_ORG_PERSIST=false — chỉ ring RAM process (restart mất replay org-live từ server).
func orgPersistEnabled() bool {
	if global.MongoDB_ServerConfig != nil {
		return global.MongoDB_ServerConfig.AIDecisionLiveOrgPersist
	}
	v := strings.TrimSpace(os.Getenv("AI_DECISION_LIVE_ORG_PERSIST"))
	return v == "1" || strings.EqualFold(v, "true") || v == "yes"
}

func persistOrgLiveEventAsync(ownerOrgID primitive.ObjectID, ev DecisionLiveEvent) {
	if !orgPersistEnabled() || ownerOrgID.IsZero() {
		return
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AIDecisionOrgLiveEvents)
	if !ok || coll == nil {
		return
	}
	docID := primitive.NewObjectID()
	createdAt := time.Now().UnixMilli()
	doc := BuildOrgLivePersistDocument(ownerOrgID, docID, createdAt, ev)
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

// OrgTimelineForAPI đọc replay org-live: ưu tiên Mongo (đa replica), lỗi/rỗng thì fallback RAM.
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

// PersistedOrgLiveListFilter phân trang + lọc đọc trực tiếp collection decision_org_live_events (không fallback RAM).
type PersistedOrgLiveListFilter struct {
	OwnerOrgID     primitive.ObjectID
	Page           int
	Limit          int
	TraceID        string
	DecisionCaseID string
	FromCreatedMs  *int64
	ToCreatedMs    *int64
}

// ListPersistedOrgLiveEventsFromMongo đọc chỉ từ Mongo — sort createdAt giảm dần (mới nhất trước).
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
	return out, total, cur.Err()
}
